package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"palace-manager/internal/authstore"
)

// handlePalaceBanlist proxies GET /api/palaces/:name/banlist to the palace's own
// HTTP server at http://127.0.0.1:<httpPort>/api/v1/palacemanager/banlist.json.
func (s *Server) handlePalaceBanlist(w http.ResponseWriter, r *http.Request, name string) {
	if !requirePalacePerm(w, r, name, authstore.PermBans) {
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
	proxyPalaceManagerEndpoint(w, inst.HTTPPort, "banlist.json")
}

// handlePalaceBanlistUnban proxies POST /api/palaces/:name/banlist/unban to the palace,
// injecting the requesting user's username as the operator so the in-game page reads
// "Page from System: Unbanned by PalaceManager (<username>)".
func (s *Server) handlePalaceBanlistUnban(w http.ResponseWriter, r *http.Request, name string) {
	if !requirePalacePerm(w, r, name, authstore.PermBans) {
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
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if req.ID == "" {
		writeError(w, http.StatusBadRequest, "id required")
		return
	}

	id, _ := IdentityFrom(r.Context())
	operator := id.Username
	if operator == "" {
		operator = "admin"
	}

	payload, _ := json.Marshal(map[string]string{
		"id":       req.ID,
		"operator": operator,
	})

	url := fmt.Sprintf("http://127.0.0.1:%d/api/v1/palacemanager/banlist/unban", inst.HTTPPort)
	resp, err := palaceManagerClient.Post(url, "application/json", bytes.NewReader(payload)) //nolint:noctx
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "palace not reachable: "+err.Error())
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		s.writeAudit(r.Context(), "palace.banlist.unban", name, map[string]string{"id": req.ID})
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body) //nolint:errcheck
}
