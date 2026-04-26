package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// handlePalacePages proxies GET /api/palaces/:name/pages to the palace's own
// HTTP server at http://127.0.0.1:<httpPort>/api/v1/palacemanager/pages.json.
func (s *Server) handlePalacePages(w http.ResponseWriter, r *http.Request, name string) {
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
	proxyPalaceManagerEndpoint(w, inst.HTTPPort, "pages.json")
}

// handlePalacePagesSend proxies POST /api/palaces/:name/pages/send to the palace,
// injecting the requesting manager username as operator so in-game system pages
// identify who sent them from Palace Manager.
func (s *Server) handlePalacePagesSend(w http.ResponseWriter, r *http.Request, name string) {
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

	var req struct {
		Message string `json:"message"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	id, _ := IdentityFrom(r.Context())
	operator := id.Username
	if operator == "" {
		operator = "admin"
	}

	payload, _ := json.Marshal(map[string]string{
		"message":  req.Message,
		"operator": operator,
	})
	url := fmt.Sprintf("http://127.0.0.1:%d/api/v1/palacemanager/pages/send", inst.HTTPPort)
	resp, err := palaceManagerClient.Post(url, "application/json", bytes.NewReader(payload)) //nolint:noctx
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "palace not reachable: "+err.Error())
		return
	}
	defer resp.Body.Close()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body) //nolint:errcheck
}
