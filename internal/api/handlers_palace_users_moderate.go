package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"palace-manager/internal/authstore"
)

// handlePalaceUsersModerate proxies POST /api/palaces/:name/palace-users/moderate
// to the palace at /api/v1/palacemanager/users/moderate and injects the current
// manager username as the operator label for in-world audit pages.
func (s *Server) handlePalaceUsersModerate(w http.ResponseWriter, r *http.Request, name string) {
	if !requirePalacePerm(w, r, name, authstore.PermUsers) {
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
	operator := id.Username
	if operator == "" {
		operator = "admin"
	}
	req["operator"] = operator

	payload, _ := json.Marshal(req)
	url := fmt.Sprintf("http://127.0.0.1:%d/api/v1/palacemanager/users/moderate", inst.HTTPPort)
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
