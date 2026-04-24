package api

import (
	"net/http"
)

// handleUIConfig exposes read-only UI settings that must load before auth (login page).
func (s *Server) handleUIConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"theme":      s.cfg.Manager.Theme,
		"version":    normaliseVersion(s.version),
		"gitHash":    s.gitHash,
		"githubRepo": s.cfg.Manager.GithubRepo,
	})
}
