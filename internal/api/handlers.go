package api

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"palace-manager/internal/authstore"
	"palace-manager/internal/bootstrap"
	"palace-manager/internal/config"
	"palace-manager/internal/instance"
	"palace-manager/internal/provisioner"
	"palace-manager/internal/pserverprefs"
	"palace-manager/internal/registry"
)

func filterInstances(ctx context.Context, instances []instance.Instance) []instance.Instance {
	id, ok := IdentityFrom(ctx)
	if !ok {
		return instances
	}
	if id.Role == authstore.RoleAdmin {
		return instances
	}
	out := make([]instance.Instance, 0, len(instances))
	for _, inst := range instances {
		if authstore.CanAccessPalace(id.Role, id.Palaces, inst.Name) {
			out = append(out, inst)
		}
	}
	return out
}

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

// resolveYPPort chooses the directory-listing port: when ypHost is set and ypPort is zero,
// the palace TCP listen port is used (common when the public port matches the server port).
func resolveYPPort(ypHost string, ypPort, tcpFallback int) int {
	if strings.TrimSpace(ypHost) == "" {
		return 0
	}
	if ypPort > 0 {
		return ypPort
	}
	return tcpFallback
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

func (s *Server) applyPserverVersionDisplay(inst *instance.Instance) {
	if p, ok := s.reg.Get(inst.Name); ok {
		if p.PserverVersion == "" {
			inst.PserverVersion = "latest"
		} else {
			inst.PserverVersion = p.PserverVersion
		}
		return
	}
	if s.unreg != nil {
		if rec, ok := s.unreg.Get(inst.Name); ok && rec.PserverVersion != "" {
			inst.PserverVersion = rec.PserverVersion
			return
		}
	}
	if inst.PserverVersion != "" {
		return
	}
	inst.PserverVersion = "latest"
}

func (s *Server) enrichInstance(inst *instance.Instance) {
	s.applyPserverVersionDisplay(inst)
	if md, err := instance.DiscoverMediaDir(inst.Name); err == nil {
		inst.MediaDir = md
	}
}

func (s *Server) handleListPalaces(w http.ResponseWriter, r *http.Request) {
	instances, err := s.instances.List()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	instances = filterInstances(r.Context(), instances)
	for i := range instances {
		s.enrichInstance(&instances[i])
	}
	writeJSON(w, http.StatusOK, instances)
}

func (s *Server) handleGetPalace(w http.ResponseWriter, r *http.Request, name string) {
	if !canAccessPalace(r.Context(), name) {
		writeError(w, http.StatusNotFound, fmt.Sprintf("palace %q not found", name))
		return
	}
	inst, err := s.instances.Get(name)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	s.enrichInstance(&inst)
	writeJSON(w, http.StatusOK, inst)
}

// --- provision ---------------------------------------------------------------

func (s *Server) handleProvision(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	var req struct {
		Name     string `json:"name"`
		TCPPort  int    `json:"tcpPort"`
		HTTPPort int    `json:"httpPort"`
		YPHost   string `json:"ypHost"`
		YPPort   int    `json:"ypPort"`
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
	if err := instance.CheckTCPListenPortsFree(req.TCPPort, req.HTTPPort); err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}

	// Pre-flight: pserver template must exist before we touch any system state.
	if fi, err := os.Stat(s.cfg.Pserver.TemplateDir); err != nil || !fi.IsDir() {
		writeError(w, http.StatusPreconditionFailed,
			fmt.Sprintf("pserver template not ready (%s) — go to Updates and click \"Updates\" first to download the pserver template", s.cfg.Pserver.TemplateDir))
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
		ypPort := resolveYPPort(req.YPHost, req.YPPort, req.TCPPort)
		entry := provisioner.RegistryEntry(req.Name, result, req.YPHost, ypPort)
		entry.ProvisionedAt = time.Now()

		// Auto-pin to the current template semver so this palace stays on a known-good
		// build. The operator can later opt it into "latest" via the rollout panel.
		if ti, err := s.vers.ReadTemplateInfo(); err == nil && ti != nil {
			sem := ti.Semver
			if sem == "" {
				sem = ti.Tag
			}
			if sem != "" {
				// Ensure the version is in the archive index (idempotent).
				_ = s.vers.ArchiveFromTemplate()
				entry.PserverVersion = sem
			}
		}

		if err := s.reg.Add(entry); err != nil {
			streamLine(sw, fmt.Sprintf("WARNING: could not update registry: %v", err))
		} else {
			// If we pinned to a semver, patch the systemd unit to use the archived binary.
			if entry.PserverVersion != "" {
				if err := s.vers.ApplyPalaceVersion(s.reg, req.Name, entry.PserverVersion, false); err != nil {
					streamLine(sw, fmt.Sprintf("NOTE: pinned pserver version (%s) but could not patch unit: %v", entry.PserverVersion, err))
				}
			}
			if err := s.ensurePalaceYPInPrefs(req.Name); err != nil {
				streamLine(sw, fmt.Sprintf("WARNING: could not sync pserver.prefs YP lines: %v", err))
			}
		}
		// Async: queue gen-media scan + nginx reload without blocking this handler path.
		go func() { s.nginx.Trigger() }()
		streamLine(sw, fmt.Sprintf(`{"ok":true,"name":"%s","tcpPort":%d,"httpPort":%d}`,
			req.Name, req.TCPPort, req.HTTPPort))
	}
}

// --- palace lifecycle --------------------------------------------------------

func (s *Server) handlePalaceAction(w http.ResponseWriter, r *http.Request, name, action string) {
	if !canAccessPalace(r.Context(), name) {
		writeError(w, http.StatusNotFound, fmt.Sprintf("palace %q not found", name))
		return
	}
	if err := s.ensurePalaceYPInPrefs(name); err != nil {
		writeError(w, http.StatusInternalServerError, "prefs: "+err.Error())
		return
	}
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
	if action == "start" || action == "restart" {
		go func() {
			s.nginx.Trigger()
			s.nginx.TriggerDelayed(2 * time.Second)
		}()
	}
	writeJSON(w, http.StatusOK, map[string]string{"ok": "true", "action": action, "name": name})
}

func (s *Server) handleDeletePalace(w http.ResponseWriter, r *http.Request, name string) {
	if !requireAdmin(w, r) {
		return
	}
	purge := r.URL.Query().Get("purge") == "true"

	palaceSnap, hadReg := s.reg.Get(name)

	if err := s.instances.Disable(name, purge); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if purge {
		if err := s.prov.PurgeUser(name); err != nil {
			writeError(w, http.StatusInternalServerError, "disabled unit but failed to remove user: "+err.Error())
			return
		}
	}

	if hadReg && !purge && s.unreg != nil {
		_ = s.unreg.UpsertFromPalace(palaceSnap, time.Now())
	}

	_ = s.reg.Remove(name)

	if purge && s.unreg != nil {
		_ = s.unreg.Remove(name)
	}

	s.nginx.Trigger()
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "name": name, "purged": purge})
}

func (s *Server) handleDiscoverPalace(w http.ResponseWriter, r *http.Request, name string) {
	if !requireAdmin(w, r) {
		return
	}
	if _, ok := s.reg.Get(name); ok {
		writeError(w, http.StatusConflict, "palace already in registry")
		return
	}
	user, tcp, httpPort, dd, err := instance.DiscoverFromUnit(name)
	if err != nil && s.unreg != nil {
		if rec, ok := s.unreg.Get(name); ok {
			user, tcp, httpPort, dd = rec.User, rec.TCPPort, rec.HTTPPort, rec.DataDir
			err = nil
		}
	}
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"name":      name,
		"linuxUser": user,
		"tcpPort":   tcp,
		"httpPort":  httpPort,
		"dataDir":   dd,
	})
}

func (s *Server) handleRegisterPalace(w http.ResponseWriter, r *http.Request, name string) {
	if !requireAdmin(w, r) {
		return
	}
	if _, ok := s.reg.Get(name); ok {
		writeError(w, http.StatusConflict, "palace already registered")
		return
	}

	var req struct {
		TcpPort   int    `json:"tcpPort"`
		HttpPort  int    `json:"httpPort"`
		DataDir   string `json:"dataDir"`
		LinuxUser string `json:"linuxUser"`
		EnableNow bool   `json:"enableNow"`
		YPHost    string `json:"ypHost"`
		YPPort    int    `json:"ypPort"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && err != io.EOF {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	dUser, dTcp, dHttp, dDd, discoverErr := instance.DiscoverFromUnit(name)
	if discoverErr != nil && s.unreg != nil {
		if rec, ok := s.unreg.Get(name); ok {
			dUser, dTcp, dHttp, dDd = rec.User, rec.TCPPort, rec.HTTPPort, rec.DataDir
			discoverErr = nil
		}
	}
	if discoverErr != nil {
		writeError(w, http.StatusBadRequest,
			"missing systemd unit and no recovery snapshot — restore /etc/systemd/system/palman-"+name+".service or reprovision: "+discoverErr.Error())
		return
	}

	tcp := req.TcpPort
	httpPort := req.HttpPort
	if tcp == 0 {
		tcp = dTcp
	}
	if httpPort == 0 {
		httpPort = dHttp
	}
	user := strings.TrimSpace(req.LinuxUser)
	if user == "" {
		user = dUser
	}
	if user == "" {
		user = name
	}
	dd := strings.TrimSpace(req.DataDir)
	if dd == "" {
		dd = dDd
	}
	if dd == "" {
		dd = filepath.Join("/home", user, "palace")
	}

	if tcp == 0 || httpPort == 0 {
		writeError(w, http.StatusBadRequest, "tcpPort and httpPort are required (could not derive from unit file or recovery snapshot)")
		return
	}

	if s.reg.PortInUse(tcp, httpPort) {
		writeError(w, http.StatusConflict, "port already in use by another palace")
		return
	}

	ypHost := strings.TrimSpace(req.YPHost)
	ypPort := resolveYPPort(req.YPHost, req.YPPort, tcp)
	if ypHost == "" {
		ypPort = 0
	}
	entry := registry.Palace{
		Name:          name,
		User:          user,
		TCPPort:       tcp,
		HTTPPort:      httpPort,
		DataDir:       dd,
		YPHost:        ypHost,
		YPPort:        ypPort,
		ProvisionedAt: time.Now(),
	}
	if s.unreg != nil {
		if rec, ok := s.unreg.Get(name); ok && rec.PserverVersion != "" {
			entry.PserverVersion = rec.PserverVersion
		}
	}
	if err := s.reg.Add(entry); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if s.unreg != nil {
		_ = s.unreg.Remove(name)
	}
	if err := s.ensurePalaceYPInPrefs(name); err != nil {
		writeError(w, http.StatusInternalServerError, "registered but prefs: "+err.Error())
		return
	}
	s.nginx.Trigger()

	resp := map[string]any{"ok": true, "name": name}
	if req.EnableNow {
		if err := s.instances.EnableNow(name); err != nil {
			resp["enableWarning"] = err.Error()
		}
	}
	writeJSON(w, http.StatusOK, resp)
}

// --- logs --------------------------------------------------------------------

func (s *Server) handleLogs(w http.ResponseWriter, r *http.Request, name string) {
	if !canAccessPalace(r.Context(), name) {
		writeError(w, http.StatusNotFound, fmt.Sprintf("palace %q not found", name))
		return
	}
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
	if !requireAdmin(w, r) {
		return
	}
	restartAll := r.URL.Query().Get("restartAll") == "true"

	// Reject if already running (background or another manual trigger).
	st := s.pserverUpdate
	st.mu.Lock()
	if st.running {
		st.mu.Unlock()
		writeError(w, http.StatusConflict, "pserver update already in progress")
		return
	}
	st.running = true
	st.startedAt = time.Now()
	st.mu.Unlock()

	defer func() {
		st.mu.Lock()
		st.running = false
		st.lastRun = time.Now()
		st.mu.Unlock()
	}()

	sw, ok := sseWriter(w)
	if !ok {
		st.mu.Lock()
		st.running = false
		st.mu.Unlock()
		writeError(w, http.StatusInternalServerError, "streaming not supported")
		return
	}

	if _, err := s.prov.Update(restartAll, sw); err != nil {
		st.mu.Lock()
		st.lastErr = err.Error()
		st.mu.Unlock()
		streamLine(sw, fmt.Sprintf("ERROR: %v", err))
		return
	}
	if err := s.vers.ArchiveFromTemplate(); err != nil {
		streamLine(sw, fmt.Sprintf("NOTE (version archive): %v", err))
	} else {
		streamLine(sw, fmt.Sprintf("NOTE: indexed this build under %s (versions.json)", s.cfg.Pserver.VersionsDir))
	}

	// Record success state.
	if ti, err := s.vers.ReadTemplateInfo(); err == nil && ti != nil {
		ver := ti.Semver
		if ver == "" {
			ver = ti.Tag
		}
		st.mu.Lock()
		st.lastVersion = ver
		st.lastErr = ""
		st.mu.Unlock()
	}

	streamLine(sw, `{"ok":true}`)
}

func (s *Server) handleBinaryVersions(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	snap, err := s.vers.Snapshot()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, snap)
}

func (s *Server) handleRolloutAll(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	var req struct {
		Semver  string `json:"semver"`
		Restart bool   `json:"restart"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if err := s.vers.ApplyAllPalaces(s.reg, req.Semver, req.Restart); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "semver": req.Semver, "restart": req.Restart})
}

func (s *Server) handlePalacePserverVersion(w http.ResponseWriter, r *http.Request, name string) {
	if !requireAdmin(w, r) {
		return
	}
	var req struct {
		Semver  string `json:"semver"`
		Restart bool   `json:"restart"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if err := s.vers.ApplyPalaceVersion(s.reg, name, req.Semver, req.Restart); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "name": name, "semver": req.Semver, "restart": req.Restart})
}

// --- nginx -------------------------------------------------------------------

func (s *Server) handleNginxStatus(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	status := s.nginx.Status()
	writeJSON(w, http.StatusOK, status)
}

func (s *Server) handleNginxRegen(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
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

func (s *Server) handleNginxSettings(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	if r.Method == http.MethodGet {
		writeJSON(w, http.StatusOK, map[string]any{
			"genScript":         s.cfg.Nginx.GenScript,
			"regenInterval":     s.cfg.Nginx.RegenInterval.String(),
			"mediaHost":         s.cfg.Nginx.MediaHost,
			"certDir":           s.cfg.Nginx.CertDir,
			"edgeScheme":        s.cfg.Nginx.EdgeScheme,
			"matchScheme":       s.cfg.Nginx.MatchScheme,
			"reverseProxyMedia": config.ReverseProxyMediaBase(s.cfg.Nginx.EdgeScheme, s.cfg.Nginx.MediaHost),
		})
		return
	}

	var req struct {
		MediaHost    string `json:"mediaHost"`
		CertDir      string `json:"certDir"`
		EdgeScheme   string `json:"edgeScheme"`
		MatchScheme  string `json:"matchScheme"`
		RestartAll   bool   `json:"restartAll"`
		RewriteUnits *bool  `json:"rewriteUnits"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if strings.TrimSpace(req.MediaHost) == "" {
		writeError(w, http.StatusBadRequest, "mediaHost is required")
		return
	}

	prevBase := config.ReverseProxyMediaBase(s.cfg.Nginx.EdgeScheme, s.cfg.Nginx.MediaHost)

	s.cfg.Nginx.MediaHost = strings.TrimSpace(req.MediaHost)
	s.cfg.Nginx.CertDir = strings.TrimSpace(req.CertDir)

	es := strings.ToLower(strings.TrimSpace(req.EdgeScheme))
	if es == "" {
		es = "https"
	}
	if es != "http" && es != "https" && es != "dual" {
		writeError(w, http.StatusBadRequest, "edgeScheme must be http, https, or dual")
		return
	}
	s.cfg.Nginx.EdgeScheme = es

	ms := strings.ToLower(strings.TrimSpace(req.MatchScheme))
	if ms == "" {
		ms = "both"
	}
	if ms != "http" && ms != "https" && ms != "both" {
		writeError(w, http.StatusBadRequest, "matchScheme must be http, https, or both")
		return
	}
	s.cfg.Nginx.MatchScheme = ms

	s.cfg.ApplyDefaults()

	newBase := config.ReverseProxyMediaBase(s.cfg.Nginx.EdgeScheme, s.cfg.Nginx.MediaHost)

	rewrite := true
	if req.RewriteUnits != nil {
		rewrite = *req.RewriteUnits
	}

	if err := s.cfg.Save(s.configPath); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	resp := map[string]any{
		"ok":                true,
		"reverseProxyMedia": newBase,
		"unitsRewritten":    false,
		"restartAll":        req.RestartAll,
	}

	if rewrite && prevBase != newBase {
		if err := s.instances.RewriteReverseProxyMedia(newBase); err != nil {
			writeError(w, http.StatusInternalServerError, "saved config but systemd units: "+err.Error())
			return
		}
		resp["unitsRewritten"] = true
	}

	s.nginx.Trigger()

	if req.RestartAll {
		if err := s.instances.RestartAll(); err != nil {
			resp["restartWarning"] = err.Error()
		}
	}

	writeJSON(w, http.StatusOK, resp)
}

// handleNginxDNSCheck resolves ?host= and reports whether it points at this machine.
func (s *Server) handleNginxDNSCheck(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	host := strings.TrimSpace(r.URL.Query().Get("host"))
	if host == "" {
		writeError(w, http.StatusBadRequest, "host query parameter required")
		return
	}
	result := bootstrap.CheckDNS(host)
	writeJSON(w, http.StatusOK, result)
}

// --- bootstrap ---------------------------------------------------------------

func (s *Server) handleBootstrapStatus(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	statuses := s.boot.CheckStatus()
	writeJSON(w, http.StatusOK, statuses)
}

func (s *Server) handleBootstrapRun(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
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

func (s *Server) handleHostLogrotateEnableAll(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}

	instances, err := s.instances.List()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	sw, ok := sseWriter(w)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming not supported")
		return
	}

	okCount := 0
	for _, inst := range instances {
		unit := strings.TrimSpace(inst.UnitName)
		if strings.HasPrefix(unit, "palace-") {
			streamLine(sw, fmt.Sprintf("[%s] skip: legacy palace-*.service layouts are not handled here (manual logrotate or migrate to palman-<name>)", inst.Name))
			continue
		}
		if unit == "" {
			unit = fmt.Sprintf("palman-%s.service", inst.Name)
		}
		lu := strings.TrimSpace(inst.User)
		if lu == "" {
			lu = inst.Name
		}
		dd := strings.TrimSpace(inst.DataDir)
		if dd == "" {
			dd = filepath.Join("/home", lu, "palace")
		}
		streamLine(sw, fmt.Sprintf("[%s] installing logrotate (user=%s dataDir=%s unit=%s)", inst.Name, lu, dd, unit))
		if _, err := s.prov.EnsureLogrotate(lu, dd, unit, sw); err != nil {
			streamLine(sw, fmt.Sprintf("ERROR [%s]: %v", inst.Name, err))
			continue
		}
		okCount++
	}
	streamLine(sw, fmt.Sprintf(`{"ok":true,"configured":%d,"palaces":%d}`, okCount, len(instances)))
}

// --- palace server root (pserver.pat, pserver.prefs, *.json, pserver.log + logrotate) ---

const maxPalaceServerFile = 8 << 20

type serverRootEntry struct {
	Name   string `json:"name"`
	Size   int64  `json:"size"`
	IsText bool   `json:"isText"`
}

func (s *Server) palaceDataDir(palaceName string) (string, error) {
	inst, err := s.instances.Get(palaceName)
	if err != nil {
		return "", err
	}
	dd := inst.DataDir
	if dd == "" {
		_, _, _, dDir, dErr := instance.DiscoverFromUnit(palaceName)
		if dErr == nil {
			dd = dDir
		}
	}
	if dd == "" {
		return "", fmt.Errorf("no data directory for palace %q", palaceName)
	}
	return filepath.Clean(dd), nil
}

func (s *Server) mergedPrefsWithRegistry(palaceName, userContent string) (string, error) {
	p, ok := s.reg.Get(palaceName)
	if !ok {
		return userContent, nil
	}
	return pserverprefs.MergeYPAnnounce(userContent, p.YPHost, p.YPPort), nil
}

// ensurePalaceYPInPrefs rewrites YPMYEXTADDR / YPMYEXTPORT in pserver.prefs from registry values.
func (s *Server) ensurePalaceYPInPrefs(name string) error {
	p, ok := s.reg.Get(name)
	if !ok {
		return nil
	}
	dd := strings.TrimSpace(p.DataDir)
	if dd == "" {
		if inst, err := s.instances.Get(name); err == nil {
			dd = strings.TrimSpace(inst.DataDir)
		}
	}
	if dd == "" {
		if _, _, _, dDir, err := instance.DiscoverFromUnit(name); err == nil {
			dd = dDir
		}
	}
	if dd == "" {
		return nil
	}
	path := filepath.Join(dd, "pserver.prefs")
	var content string
	b, err := os.ReadFile(path)
	switch {
	case err == nil:
		content = string(b)
	case os.IsNotExist(err):
		content = ""
	default:
		return err
	}
	merged := pserverprefs.MergeYPAnnounce(content, p.YPHost, p.YPPort)
	if merged == content {
		return nil
	}
	return writeFileAtomicAs(path, p.User, strings.NewReader(merged))
}

func allowedServerRootName(base string) bool {
	if base == "" || strings.ContainsRune(base, filepath.Separator) {
		return false
	}
	if strings.Contains(base, "..") {
		return false
	}
	if allowedPserverLogVariant(base) {
		return true
	}
	if base == "pserver.pat" || base == "pserver.prefs" {
		return true
	}
	return strings.HasSuffix(strings.ToLower(base), ".json")
}

// isServerFileTextish is for JSON API + directory listing: show as text, not base64.
func isServerFileTextish(name string) bool {
	low := strings.ToLower(name)
	if strings.HasSuffix(low, ".json") || strings.HasSuffix(low, ".pat") {
		return true
	}
	if low == "pserver.prefs" {
		return true
	}
	if allowedPserverLogVariant(name) && !strings.HasSuffix(low, ".gz") {
		return true
	}
	return false
}

// allowedPserverLogVariant permits the active log and names produced by standard logrotate:
// pserver.log, pserver.log.1 … pserver.log.N, and compressed pserver.log.1.gz … pserver.log.N.gz.
func allowedPserverLogVariant(base string) bool {
	if base == "pserver.log" {
		return true
	}
	const prefix = "pserver.log."
	if !strings.HasPrefix(base, prefix) || len(base) <= len(prefix) {
		return false
	}
	suffix := base[len(prefix):]
	lowSuf := strings.ToLower(suffix)
	if strings.HasSuffix(lowSuf, ".gz") {
		if len(suffix) <= 4 {
			return false
		}
		core := suffix[:len(suffix)-4]
		return core != "" && onlyDigitsASCII(core)
	}
	return onlyDigitsASCII(suffix)
}

func onlyDigitsASCII(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

func (s *Server) handlePalaceServerRoot(w http.ResponseWriter, r *http.Request, palaceName string) {
	if !canAccessPalace(r.Context(), palaceName) {
		writeError(w, http.StatusNotFound, fmt.Sprintf("palace %q not found", palaceName))
		return
	}
	dir, err := s.palaceDataDir(palaceName)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := make([]serverRootEntry, 0)
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		n := e.Name()
		if !allowedServerRootName(n) {
			continue
		}
		fi, err := e.Info()
		if err != nil {
			continue
		}
		out = append(out, serverRootEntry{Name: n, Size: fi.Size(), IsText: isServerFileTextish(n)})
	}
	writeJSON(w, http.StatusOK, map[string]any{"palace": palaceName, "dir": dir, "files": out})
}

func (s *Server) handlePalaceServerFile(w http.ResponseWriter, r *http.Request, palaceName, fileName string) {
	if !canAccessPalace(r.Context(), palaceName) {
		writeError(w, http.StatusNotFound, fmt.Sprintf("palace %q not found", palaceName))
		return
	}
	base := filepath.Base(fileName)
	if base != fileName || !allowedServerRootName(base) {
		writeError(w, http.StatusBadRequest, "file name not allowed")
		return
	}
	dir, err := s.palaceDataDir(palaceName)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	fullPath := filepath.Join(dir, base)
	rel, err := filepath.Rel(dir, fullPath)
	if err != nil || strings.HasPrefix(rel, "..") {
		writeError(w, http.StatusBadRequest, "invalid path")
		return
	}

	fi, err := os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			writeError(w, http.StatusNotFound, "file not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	q := r.URL.Query()
	if q.Get("inline") == "1" || q.Get("download") == "1" {
		f, err := os.Open(fullPath)
		if err != nil {
			if os.IsNotExist(err) {
				writeError(w, http.StatusNotFound, "file not found")
				return
			}
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		defer f.Close()

		if q.Get("inline") == "1" {
			// Do not use ServeContent — it replaces Content-Type. Browser "View" should show
			// everything in this list as readable text (not application/json, not forced download).
			low := strings.ToLower(base)
			ct := "text/plain; charset=utf-8"
			if strings.HasSuffix(low, ".gz") {
				ct = "application/gzip"
			}
			w.Header().Set("Content-Type", ct)
			w.Header().Set("Content-Disposition", fmt.Sprintf(`inline; filename="%s"`, base))
			w.Header().Set("Content-Length", strconv.FormatInt(fi.Size(), 10))
			_, err = io.Copy(w, f)
			if err != nil {
				return
			}
			return
		}

		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, base))
		w.Header().Set("Content-Type", "application/octet-stream")
		http.ServeContent(w, r, base, fi.ModTime(), f)
		return
	}

	if fi.Size() > maxPalaceServerFile {
		writeError(w, http.StatusRequestEntityTooLarge, "file too large for editor; use Download or View in browser")
		return
	}

	b, err := os.ReadFile(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			writeError(w, http.StatusNotFound, "file not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if len(b) > maxPalaceServerFile {
		writeError(w, http.StatusRequestEntityTooLarge, "file too large for editor; use Download or View in browser")
		return
	}

	textish := isServerFileTextish(base)
	if textish && utf8.Valid(b) {
		writeJSON(w, http.StatusOK, map[string]any{"name": base, "size": len(b), "content": string(b)})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"name":     base,
		"size":     len(b),
		"encoding": "base64",
		"content":  base64.StdEncoding.EncodeToString(b),
	})
}

// handlePalaceServerFileSave writes UTF-8 text for editable server-root files, then restarts the palace unit.
func (s *Server) handlePalaceServerFileSave(w http.ResponseWriter, r *http.Request, palaceName, fileName string) {
	if r.Method != http.MethodPut {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !canAccessPalace(r.Context(), palaceName) {
		writeError(w, http.StatusNotFound, fmt.Sprintf("palace %q not found", palaceName))
		return
	}
	base := filepath.Base(fileName)
	if base != fileName || !allowedServerRootName(base) || !isServerFileTextish(base) || allowedPserverLogVariant(base) {
		writeError(w, http.StatusBadRequest, "file cannot be edited here")
		return
	}
	dir, err := s.palaceDataDir(palaceName)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	fullPath := filepath.Join(dir, base)
	rel, err := filepath.Rel(dir, fullPath)
	if err != nil || strings.HasPrefix(rel, "..") {
		writeError(w, http.StatusBadRequest, "invalid path")
		return
	}

	var req struct {
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if !utf8.ValidString(req.Content) {
		writeError(w, http.StatusBadRequest, "content must be valid UTF-8")
		return
	}
	if len(req.Content) > maxPalaceServerFile {
		writeError(w, http.StatusRequestEntityTooLarge, "content too large")
		return
	}

	toWrite := req.Content
	if base == "pserver.prefs" {
		merged, err := s.mergedPrefsWithRegistry(palaceName, req.Content)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		toWrite = merged
	}

	if err := writeFileAtomicAs(fullPath, s.palaceLinuxUser(palaceName), strings.NewReader(toWrite)); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := s.instances.Restart(palaceName); err != nil {
		writeError(w, http.StatusInternalServerError, "saved file but restart failed: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "name": palaceName, "file": base, "restarted": true})
}

// handlePalacePrefsForm returns structured fields + unknown tail for the intelligent prefs editor.
func (s *Server) handlePalacePrefsForm(w http.ResponseWriter, r *http.Request, palaceName string) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !canAccessPalace(r.Context(), palaceName) {
		writeError(w, http.StatusNotFound, fmt.Sprintf("palace %q not found", palaceName))
		return
	}
	dir, err := s.palaceDataDir(palaceName)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	path := filepath.Join(dir, "pserver.prefs")
	var content string
	if b, err := os.ReadFile(path); err == nil {
		content = string(b)
	} else if os.IsNotExist(err) {
		content = ""
	} else {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	st, unk, warns := pserverprefs.ParsePrefState(content)
	dto := pserverprefs.StateToDTO(st)
	writeJSON(w, http.StatusOK, map[string]any{
		"form":        dto,
		"unknownTail": unk,
		"warnings":    warns,
		"schema":      "prefs-form v1 — directives aligned with mansionsource-go internal/script/prefs.go",
	})
}

// handlePalaceServerPrefsSave updates registry YP fields (admin only), merges YP lines into prefs, writes, restarts.
// Body may use mode "raw" with "content", or mode "form" with "form" and "unknownTail".
func (s *Server) handlePalaceServerPrefsSave(w http.ResponseWriter, r *http.Request, palaceName string) {
	if r.Method != http.MethodPut {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !canAccessPalace(r.Context(), palaceName) {
		writeError(w, http.StatusNotFound, fmt.Sprintf("palace %q not found", palaceName))
		return
	}
	var req struct {
		Mode        string                    `json:"mode"`
		Content     string                    `json:"content"`
		Form        pserverprefs.PrefsFormDTO `json:"form"`
		UnknownTail string                    `json:"unknownTail"`
		YPHost      string                    `json:"ypHost"`
		YPPort      int                       `json:"ypPort"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	mode := strings.ToLower(strings.TrimSpace(req.Mode))
	if mode == "" {
		if req.Content != "" {
			mode = "raw"
		} else {
			mode = "form"
		}
	}

	var outContent string
	switch mode {
	case "raw":
		if !utf8.ValidString(req.Content) {
			writeError(w, http.StatusBadRequest, "content must be valid UTF-8")
			return
		}
		if len(req.Content) > maxPalaceServerFile {
			writeError(w, http.StatusRequestEntityTooLarge, "content too large")
			return
		}
		outContent = req.Content
	case "form":
		dir, err := s.palaceDataDir(palaceName)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		path := filepath.Join(dir, "pserver.prefs")
		var existing string
		if b, err := os.ReadFile(path); err == nil {
			existing = string(b)
		} else if os.IsNotExist(err) {
			existing = ""
		} else {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		oldState, _, _ := pserverprefs.ParsePrefState(existing)
		mergedState := pserverprefs.MergeDTO(req.Form, oldState)
		outContent = pserverprefs.RenderWithUnknown(mergedState, req.UnknownTail)
		if !utf8.ValidString(outContent) {
			writeError(w, http.StatusBadRequest, "generated content must be valid UTF-8")
			return
		}
		if len(outContent) > maxPalaceServerFile {
			writeError(w, http.StatusRequestEntityTooLarge, "content too large")
			return
		}
	default:
		writeError(w, http.StatusBadRequest, "mode must be raw or form")
		return
	}

	id, ok := IdentityFrom(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if id.Role == authstore.RoleAdmin {
		pal, ok := s.reg.Get(palaceName)
		if ok {
			pal.YPHost = strings.TrimSpace(req.YPHost)
			pal.YPPort = resolveYPPort(req.YPHost, req.YPPort, pal.TCPPort)
			if pal.YPHost == "" {
				pal.YPPort = 0
			}
			if err := s.reg.Add(pal); err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
		}
	}
	merged, err := s.mergedPrefsWithRegistry(palaceName, outContent)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	dir, err := s.palaceDataDir(palaceName)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	fullPath := filepath.Join(dir, "pserver.prefs")
	if err := writeFileAtomicAs(fullPath, s.palaceLinuxUser(palaceName), strings.NewReader(merged)); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := s.instances.Restart(palaceName); err != nil {
		writeError(w, http.StatusInternalServerError, "saved prefs but restart failed: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "name": palaceName, "restarted": true})
}

func (s *Server) resolvePalaceUnixHome(palaceName string) (string, error) {
	inst, err := s.instances.Get(palaceName)
	if err != nil {
		return "", err
	}
	u := strings.TrimSpace(inst.User)
	if u == "" {
		u = palaceName
	}
	candidate := filepath.Join("/home", u)
	if fi, err := os.Stat(candidate); err == nil && fi.IsDir() {
		return filepath.Clean(candidate), nil
	}
	if inst.DataDir != "" {
		dd := filepath.Clean(inst.DataDir)
		parent := filepath.Dir(dd)
		if fi, err := os.Stat(parent); err == nil && fi.IsDir() && strings.HasPrefix(parent, "/home/") {
			return filepath.Clean(parent), nil
		}
	}
	lu, _, _, dd, dErr := instance.DiscoverFromUnit(palaceName)
	if dErr == nil {
		if lu != "" {
			c2 := filepath.Join("/home", lu)
			if fi, err := os.Stat(c2); err == nil && fi.IsDir() {
				return filepath.Clean(c2), nil
			}
		}
		if dd != "" {
			p := filepath.Dir(filepath.Clean(dd))
			if fi, err := os.Stat(p); err == nil && fi.IsDir() {
				return filepath.Clean(p), nil
			}
		}
	}
	return "", fmt.Errorf("could not resolve Unix home directory for palace %q", palaceName)
}

func (s *Server) handlePalaceHomeBackup(w http.ResponseWriter, r *http.Request, palaceName string) {
	if !canAccessPalace(r.Context(), palaceName) {
		writeError(w, http.StatusNotFound, fmt.Sprintf("palace %q not found", palaceName))
		return
	}
	homeAbs, err := s.resolvePalaceUnixHome(palaceName)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	fi, err := os.Stat(homeAbs)
	if err != nil || !fi.IsDir() {
		writeError(w, http.StatusNotFound, "home directory not accessible")
		return
	}

	safe := strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' {
			return r
		}
		return '-'
	}, palaceName)
	if safe == "" {
		safe = "palace"
	}

	var backupRoot string
	if dd, derr := s.palaceDataDir(palaceName); derr == nil {
		backupRoot = filepath.Join(dd, "backups")
	}

	stamp := time.Now().UTC().Format("2006-01-02T150405Z")
	dlName := fmt.Sprintf("%s-home-backup-%s.tar.gz", safe, stamp)
	w.Header().Set("Content-Type", "application/gzip")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, dlName))

	gzw, err := gzip.NewWriterLevel(w, gzip.BestCompression)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer gzw.Close()

	tw := tar.NewWriter(gzw)
	defer tw.Close()

	_ = filepath.WalkDir(homeAbs, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		rel, err := filepath.Rel(homeAbs, path)
		if err != nil || strings.HasPrefix(rel, "..") {
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return nil
		}

		cp := filepath.Clean(path)
		if backupRoot != "" {
			br := filepath.Clean(backupRoot)
			if cp == br || strings.HasPrefix(cp, br+string(filepath.Separator)) {
				if d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
		}

		link := ""
		if d.Type()&fs.ModeSymlink != 0 {
			link, err = os.Readlink(path)
			if err != nil {
				return nil
			}
		}

		hdr, err := tar.FileInfoHeader(info, link)
		if err != nil {
			return nil
		}
		hdr.Name = filepath.ToSlash(rel)
		if info.IsDir() && hdr.Name != "" && !strings.HasSuffix(hdr.Name, "/") {
			hdr.Name += "/"
		}

		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}

		if d.Type()&fs.ModeSymlink != 0 || info.IsDir() {
			return nil
		}
		if !info.Mode().IsRegular() {
			return nil
		}

		f, err := os.Open(path)
		if err != nil {
			return nil
		}
		_, copyErr := io.Copy(tw, f)
		closeErr := f.Close()
		if copyErr != nil {
			return copyErr
		}
		return closeErr
	})
}

const maxPatUploadBytes = 32 << 20

// handlePalacePatUpload accepts multipart field "file", writes data-dir/pserver.pat atomically, then restarts the palace unit.
func (s *Server) handlePalacePatUpload(w http.ResponseWriter, r *http.Request, palaceName string) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !canAccessPalace(r.Context(), palaceName) {
		writeError(w, http.StatusNotFound, fmt.Sprintf("palace %q not found", palaceName))
		return
	}
	dir, err := s.palaceDataDir(palaceName)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	if err := r.ParseMultipartForm(maxPatUploadBytes); err != nil {
		writeError(w, http.StatusBadRequest, "invalid multipart or file too large")
		return
	}
	fhs := r.MultipartForm.File["file"]
	if len(fhs) != 1 {
		writeError(w, http.StatusBadRequest, "exactly one file field \"file\" required")
		return
	}
	fh := fhs[0]
	src, err := fh.Open()
	if err != nil {
		writeError(w, http.StatusBadRequest, "could not read upload")
		return
	}
	defer src.Close()

	dest := filepath.Join(dir, "pserver.pat")
	if err := writeFileAtomicAs(dest, s.palaceLinuxUser(palaceName), io.LimitReader(src, maxPatUploadBytes)); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if err := s.ensurePalaceYPInPrefs(palaceName); err != nil {
		writeError(w, http.StatusInternalServerError, "prefs: "+err.Error())
		return
	}

	if err := s.instances.Restart(palaceName); err != nil {
		writeError(w, http.StatusInternalServerError, "pat saved but restart failed: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "name": palaceName, "restarted": true})
}

func writeFileAtomic(dest string, src io.Reader) error {
	tmp := dest + ".partial." + strconv.FormatInt(time.Now().UnixNano(), 10)
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	if _, err := io.Copy(f, src); err != nil {
		_ = f.Close()
		_ = os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, dest); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

// chownPath changes the owner of path to linuxUser (best-effort; no-op on lookup failure).
// The manager runs as root; this ensures files written to palace data dirs are owned by
// the unprivileged palace user so pserver can write them at runtime.
func chownPath(path, linuxUser string) error {
	if linuxUser == "" {
		return nil
	}
	u, err := user.Lookup(linuxUser)
	if err != nil {
		return err
	}
	uid, _ := strconv.Atoi(u.Uid)
	gid, _ := strconv.Atoi(u.Gid)
	return os.Chown(path, uid, gid)
}

// writeFileAtomicAs writes dest atomically then chowns it to linuxUser.
func writeFileAtomicAs(dest, linuxUser string, src io.Reader) error {
	if err := writeFileAtomic(dest, src); err != nil {
		return err
	}
	_ = chownPath(dest, linuxUser) // best-effort; don't fail the write on chown error
	return nil
}

// palaceLinuxUser returns the Linux username that owns the palace data dir.
func (s *Server) palaceLinuxUser(palaceName string) string {
	if p, ok := s.reg.Get(palaceName); ok && p.User != "" {
		return p.User
	}
	if inst, err := s.instances.Get(palaceName); err == nil && inst.User != "" {
		return inst.User
	}
	u, _, _, _, _ := instance.DiscoverFromUnit(palaceName)
	return u
}

// --- registry entry helper ---------------------------------------------------

func entryFromPalace(p registry.Palace) registry.Palace {
	return p
}
