package api

import (
	"fmt"
	"io"
	"net/http"
	"time"
)

// palaceManagerClient is a shared HTTP client for forwarding requests to individual
// palace instances' /api/v1/palacemanager/* endpoints. Kept separate from any
// default transport so timeouts are tight and connections are not reused across
// unrelated goroutines in the manager UI polling path.
var palaceManagerClient = &http.Client{
	Timeout: 4 * time.Second,
}

// handlePalaceStats proxies GET /api/palaces/:name/stats to the palace's own
// HTTP server at http://127.0.0.1:<httpPort>/api/v1/palacemanager/info.json.
// Returns 503 when the palace has no HTTP port configured or is unreachable.
func (s *Server) handlePalaceStats(w http.ResponseWriter, r *http.Request, name string) {
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
	proxyPalaceManagerEndpoint(w, inst.HTTPPort, "info.json")
}

// handlePalaceUsers proxies GET /api/palaces/:name/palace-users to the palace's own
// HTTP server at http://127.0.0.1:<httpPort>/api/v1/palacemanager/users.json.
func (s *Server) handlePalaceUsers(w http.ResponseWriter, r *http.Request, name string) {
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
	proxyPalaceManagerEndpoint(w, inst.HTTPPort, "users.json")
}

// proxyPalaceManagerEndpoint makes a localhost GET request to the palace's
// /api/v1/palacemanager/<endpoint> and streams the response body back to the
// manager UI client. Only loopback is used so the palace endpoint's localhost
// restriction is satisfied.
func proxyPalaceManagerEndpoint(w http.ResponseWriter, httpPort int, endpoint string) {
	url := fmt.Sprintf("http://127.0.0.1:%d/api/v1/palacemanager/%s", httpPort, endpoint)
	resp, err := palaceManagerClient.Get(url) //nolint:noctx
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "palace not reachable: "+err.Error())
		return
	}
	defer resp.Body.Close()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body) //nolint:errcheck
}
