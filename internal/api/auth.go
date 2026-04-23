package api

import (
	"crypto/subtle"
	"net/http"
)

func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		wantUser := s.cfg.Manager.Username
		wantPass := s.cfg.Manager.Password

		// If no password is configured, allow all requests.
		if wantPass == "" {
			next.ServeHTTP(w, r)
			return
		}

		user, pass, ok := r.BasicAuth()
		userOK := subtle.ConstantTimeCompare([]byte(user), []byte(wantUser)) == 1
		passOK := subtle.ConstantTimeCompare([]byte(pass), []byte(wantPass)) == 1
		if !ok || !userOK || !passOK {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}
