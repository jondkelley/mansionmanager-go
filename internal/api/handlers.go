package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"palace-manager/internal/bootstrap"
	"palace-manager/internal/provisioner"
	"palace-manager/internal/registry"
)

// --- helpers -----------------------------------------------------------------

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}

func pathParam(r *http.Request, prefix string) string {
	return strings.TrimPrefix(r.URL.Path, prefix)
}

// sseWriter wraps a ResponseWriter for Server-Sent Events streaming.
func sseWriter(w http.ResponseWriter) (io.Writer, bool) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		return nil, false
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	flusher.Flush()

	return &sseWriter_{w: w, f: flusher}, true
}

type sseWriter_ struct {
	w http.ResponseWriter
	f http.Flusher
}

func (s *sseWriter_) Write(p []byte) (int, error) {
	n, err := s.w.Write(p)
	s.f.Flush()
	return n, err
}

// streamLine writes a plain-text line as an SSE data event.
func streamLine(w io.Writer, line string) {
	fmt.Fprintf(w, "data: %s\n\n", strings.TrimRight(line, "\n"))
}

// --- palace list / get -------------------------------------------------------

func (s *Server) handleListPalaces(w http.ResponseWriter, r *http.Request) {
	instances, err := s.instances.List()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, instances)
}

func (s *Server) handleGetPalace(w http.ResponseWriter, r *http.Request, name string) {
	inst, err := s.instances.Get(name)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, inst)
}

// --- provision ---------------------------------------------------------------

func (s *Server) handleProvision(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name     string `json:"name"`
		TCPPort  int    `json:"tcpPort"`
		HTTPPort int    `json:"httpPort"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if req.Name == "" || req.TCPPort == 0 || req.HTTPPort == 0 {
		writeError(w, http.StatusBadRequest, "name, tcpPort, and httpPort are required")
		return
	}
	if s.reg.PortInUse(req.TCPPort, req.HTTPPort) {
		writeError(w, http.StatusConflict, "port already in use by another palace")
		return
	}

	// Pre-flight: pserver template must exist before we touch any system state.
	if fi, err := os.Stat(s.cfg.Pserver.TemplateDir); err != nil || !fi.IsDir() {
		writeError(w, http.StatusPreconditionFailed,
			fmt.Sprintf("pserver template not ready (%s) — go to Update Binary and click \"Update Binary\" first to download the pserver template", s.cfg.Pserver.TemplateDir))
		return
	}

	sw, ok := sseWriter(w)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming not supported")
		return
	}

	result, err := s.prov.Provision(req.Name, req.TCPPort, req.HTTPPort, sw)
	if err != nil {
		streamLine(sw, fmt.Sprintf("ERROR: %v", err))
		return
	}

	if result != nil {
		entry := provisioner.RegistryEntry(req.Name, result)
		entry.ProvisionedAt = time.Now()
		if err := s.reg.Add(entry); err != nil {
			streamLine(sw, fmt.Sprintf("WARNING: could not update registry: %v", err))
		}
		// Trigger nginx regen so the new palace gets a location block immediately
		s.nginx.Trigger()
		streamLine(sw, fmt.Sprintf(`{"ok":true,"name":"%s","tcpPort":%d,"httpPort":%d}`,
			req.Name, req.TCPPort, req.HTTPPort))
	}
}

// --- palace lifecycle --------------------------------------------------------

func (s *Server) handlePalaceAction(w http.ResponseWriter, r *http.Request, name, action string) {
	var err error
	switch action {
	case "start":
		err = s.instances.Start(name)
	case "stop":
		err = s.instances.Stop(name)
	case "restart":
		err = s.instances.Restart(name)
	default:
		writeError(w, http.StatusBadRequest, "unknown action: "+action)
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"ok": "true", "action": action, "name": name})
}

func (s *Server) handleDeletePalace(w http.ResponseWriter, r *http.Request, name string) {
	purge := r.URL.Query().Get("purge") == "true"

	if err := s.instances.Disable(name); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if purge {
		if err := s.prov.PurgeUser(name); err != nil {
			writeError(w, http.StatusInternalServerError, "disabled unit but failed to remove user: "+err.Error())
			return
		}
	}

	_ = s.reg.Remove(name)
	s.nginx.Trigger()
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "name": name, "purged": purge})
}

// --- logs --------------------------------------------------------------------

func (s *Server) handleLogs(w http.ResponseWriter, r *http.Request, name string) {
	linesStr := r.URL.Query().Get("lines")
	lines := 100
	if linesStr != "" {
		if n, err := strconv.Atoi(linesStr); err == nil && n > 0 {
			lines = n
		}
	}

	tail, err := s.instances.TailLog(name, lines)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"name": name, "lines": tail})
}

// --- binary update -----------------------------------------------------------

func (s *Server) handleUpdate(w http.ResponseWriter, r *http.Request) {
	restartAll := r.URL.Query().Get("restartAll") == "true"

	sw, ok := sseWriter(w)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming not supported")
		return
	}

	if _, err := s.prov.Update(restartAll, sw); err != nil {
		streamLine(sw, fmt.Sprintf("ERROR: %v", err))
		return
	}
	streamLine(sw, `{"ok":true}`)
}

// --- nginx -------------------------------------------------------------------

func (s *Server) handleNginxStatus(w http.ResponseWriter, r *http.Request) {
	status := s.nginx.Status()
	writeJSON(w, http.StatusOK, status)
}

func (s *Server) handleNginxRegen(w http.ResponseWriter, r *http.Request) {
	sw, ok := sseWriter(w)
	if !ok {
		// Fallback to synchronous for non-streaming clients
		if err := s.nginx.RegenWithWriter(w); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	if err := s.nginx.RegenWithWriter(sw); err != nil {
		streamLine(sw, fmt.Sprintf("ERROR: %v", err))
		return
	}
	streamLine(sw, `{"ok":true}`)
}

// --- bootstrap ---------------------------------------------------------------

func (s *Server) handleBootstrapStatus(w http.ResponseWriter, r *http.Request) {
	statuses := s.boot.CheckStatus()
	writeJSON(w, http.StatusOK, statuses)
}

func (s *Server) handleBootstrapRun(w http.ResponseWriter, r *http.Request) {
	var opts bootstrap.Options
	if err := json.NewDecoder(r.Body).Decode(&opts); err != nil && r.ContentLength > 0 {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	sw, ok := sseWriter(w)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming not supported")
		return
	}

	if err := s.boot.Run(r.Context(), opts, sw); err != nil {
		streamLine(sw, fmt.Sprintf(`{"error":"%s"}`, err.Error()))
		return
	}
	streamLine(sw, `{"ok":true,"done":true}`)
}

// --- registry entry helper ---------------------------------------------------

func entryFromPalace(p registry.Palace) registry.Palace {
	return p
}
