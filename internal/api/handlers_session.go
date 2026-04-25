package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"palace-manager/internal/authstore"
)

const minPasswordLength = 10

func (s *Server) routeSession(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	s.handleSessionGet(w, r)
}

func (s *Server) handleSessionGet(w http.ResponseWriter, r *http.Request) {
	id, ok := IdentityFrom(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"username":           id.Username,
		"role":               string(id.Role),
		"mustChangePassword": id.MustChangePassword,
		"palaces":            id.Palaces,
		"isPrimaryAdmin":     s.authStore.IsPrimaryAdmin(id.Username),
	})
}

func (s *Server) handleSessionPassword(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	id, ok := IdentityFrom(r.Context())
	if !ok {
		unauthorized(w)
		return
	}
	var req struct {
		Current string `json:"current"`
		New     string `json:"new"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if len(req.New) < minPasswordLength {
		writeError(w, http.StatusBadRequest,
			fmt.Sprintf("new password must be at least %d characters", minPasswordLength))
		return
	}
	if _, verified := s.authStore.Verify(id.Username, req.Current); !verified {
		writeError(w, http.StatusUnauthorized, "current password incorrect")
		return
	}
	if err := s.authStore.SetPassword(id.Username, req.New); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// --- users (admin) -----------------------------------------------------------

type userResponse struct {
	Username           string         `json:"username"`
	Role               authstore.Role `json:"role"`
	Palaces            []string       `json:"palaces"`
	MustChangePassword bool           `json:"mustChangePassword"`
}

func (s *Server) routeUsers(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	switch r.Method {
	case http.MethodGet:
		s.handleUsersList(w, r)
	case http.MethodPost:
		s.handleUsersCreate(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleUsersList(w http.ResponseWriter, r *http.Request) {
	list := s.authStore.List()
	out := make([]userResponse, 0, len(list))
	for _, u := range list {
		out = append(out, userResponse{
			Username:           u.Username,
			Role:               u.Role,
			Palaces:            append([]string(nil), u.Palaces...),
			MustChangePassword: u.MustChangePassword,
		})
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleUsersCreate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string         `json:"username"`
		Password string         `json:"password"`
		Role     authstore.Role `json:"role"`
		Palaces  []string       `json:"palaces"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if req.Username == "" || req.Password == "" {
		writeError(w, http.StatusBadRequest, "username and password required")
		return
	}
	if len(req.Password) < minPasswordLength {
		writeError(w, http.StatusBadRequest,
			fmt.Sprintf("password must be at least %d characters", minPasswordLength))
		return
	}
	if req.Role != authstore.RoleAdmin && req.Role != authstore.RoleTenant {
		writeError(w, http.StatusBadRequest, `role must be "admin" or "tenant"`)
		return
	}
	u := authstore.User{
		Username:           req.Username,
		Role:               req.Role,
		Palaces:            append([]string(nil), req.Palaces...),
		MustChangePassword: false,
	}
	if err := s.authStore.Create(u, req.Password); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"ok": "true", "username": req.Username})
}

func (s *Server) routeUserByName(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	raw := strings.TrimPrefix(r.URL.Path, "/api/users/")
	name, err := url.PathUnescape(raw)
	if err != nil || name == "" {
		writeError(w, http.StatusBadRequest, "username required")
		return
	}
	switch r.Method {
	case http.MethodPatch:
		s.handleUserPatch(w, r, name)
	case http.MethodDelete:
		s.handleUserDelete(w, r, name)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleUserPatch(w http.ResponseWriter, r *http.Request, name string) {
	var req struct {
		Role     *authstore.Role `json:"role"`
		Palaces  []string        `json:"palaces"`
		Password *string         `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if req.Role != nil && *req.Role != authstore.RoleAdmin && *req.Role != authstore.RoleTenant {
		writeError(w, http.StatusBadRequest, `role must be "admin" or "tenant"`)
		return
	}
	u, ok := s.authStore.Get(name)
	if !ok {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}
	role := u.Role
	if req.Role != nil {
		role = *req.Role
	}
	palaces := u.Palaces
	if req.Palaces != nil {
		palaces = append([]string(nil), req.Palaces...)
	}
	if req.Password != nil && *req.Password != "" {
		if len(*req.Password) < minPasswordLength {
			writeError(w, http.StatusBadRequest,
				fmt.Sprintf("password must be at least %d characters", minPasswordLength))
			return
		}
	}
	var passPtr *string
	if req.Password != nil && *req.Password != "" {
		passPtr = req.Password
	}
	if err := s.authStore.Update(name, role, palaces, passPtr); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleUserDelete(w http.ResponseWriter, r *http.Request, name string) {
	if err := s.authStore.Delete(name); err != nil {
		if err == authstore.ErrLastAdmin {
			writeError(w, http.StatusConflict, err.Error())
			return
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
