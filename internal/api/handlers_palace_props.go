package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// handlePalaceProps proxies GET /api/palaces/:name/props to the palace endpoint
// /api/v1/palacemanager/props.json.
func (s *Server) handlePalaceProps(w http.ResponseWriter, r *http.Request, name string) {
	if !canAccessPalace(r.Context(), name) {
		writeError(w, http.StatusNotFound, fmt.Sprintf("palace %q not found", name))
		return
	}
	inst, err := s.instances.Get(name)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	if inst.HTTPPort == 0 {
		writeError(w, http.StatusServiceUnavailable, "palace has no HTTP port configured (-H flag not set)")
		return
	}
	proxyPalaceManagerEndpoint(w, inst.HTTPPort, "props.json")
}

// handlePalacePropsCommand proxies POST /api/palaces/:name/props/command to the palace
// endpoint /api/v1/palacemanager/props/command, injecting the current manager user.
func (s *Server) handlePalacePropsCommand(w http.ResponseWriter, r *http.Request, name string) {
	if !canAccessPalace(r.Context(), name) {
		writeError(w, http.StatusNotFound, fmt.Sprintf("palace %q not found", name))
		return
	}
	inst, err := s.instances.Get(name)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	if inst.HTTPPort == 0 {
		writeError(w, http.StatusServiceUnavailable, "palace has no HTTP port configured (-H flag not set)")
		return
	}

	var req map[string]any
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	id, _ := IdentityFrom(r.Context())
	operator := strings.TrimSpace(id.Username)
	if operator == "" {
		operator = "PalaceManager"
	}
	req["operator"] = operator

	payload, _ := json.Marshal(req)
	url := fmt.Sprintf("http://127.0.0.1:%d/api/v1/palacemanager/props/command", inst.HTTPPort)
	resp, err := palaceManagerClient.Post(url, "application/json", bytes.NewReader(payload)) //nolint:noctx
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "palace not reachable: "+err.Error())
		return
	}
	defer resp.Body.Close()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(w, resp.Body)
}
