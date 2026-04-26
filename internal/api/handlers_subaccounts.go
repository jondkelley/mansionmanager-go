package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"palace-manager/internal/authstore"
)

type subaccountResponse struct {
	Username           string              `json:"username"`
	Role               authstore.Role      `json:"role"`
	ParentTenant       string              `json:"parentTenant,omitempty"`
	PalacePerms        map[string][]string `json:"palacePerms,omitempty"`
	MustChangePassword bool                `json:"mustChangePassword"`
}

func subaccountToResponse(u authstore.User) subaccountResponse {
	perms := authstore.NormalizePalacePerms(u.PalacePerms)
	if perms == nil {
		perms = map[string][]string{}
	}
	return subaccountResponse{
		Username:           u.Username,
		Role:               u.Role,
		ParentTenant:       u.ParentTenant,
		PalacePerms:        perms,
		MustChangePassword: u.MustChangePassword,
	}
}

func (s *Server) routeSubaccounts(w http.ResponseWriter, r *http.Request) {
	id, ok := IdentityFrom(r.Context())
	if !ok {
		unauthorized(w)
		return
	}
	if id.Role != authstore.RoleTenant {
		writeError(w, http.StatusForbidden, "tenant only")
		return
	}
	switch r.Method {
	case http.MethodGet:
		list := s.authStore.ListSubaccounts(id.Username)
		out := make([]subaccountResponse, 0, len(list))
		for _, u := range list {
			out = append(out, subaccountToResponse(u))
		}
		writeJSON(w, http.StatusOK, out)
	case http.MethodPost:
		s.handleSubaccountsCreate(w, r, id.Username)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleSubaccountsCreate(w http.ResponseWriter, r *http.Request, parent string) {
	var req struct {
		Username    string              `json:"username"`
		Password    string              `json:"password"`
		PalacePerms map[string][]string `json:"palacePerms"`
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
	u := authstore.User{
		Username:           strings.TrimSpace(req.Username),
		Role:               authstore.RoleSubaccount,
		PalacePerms:        req.PalacePerms,
		MustChangePassword: false,
	}
	if err := s.authStore.CreateSubaccount(u, parent, req.Password); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"ok": "true", "username": u.Username})
}

func (s *Server) routeSubaccountByName(w http.ResponseWriter, r *http.Request) {
	id, ok := IdentityFrom(r.Context())
	if !ok {
		unauthorized(w)
		return
	}
	if id.Role != authstore.RoleTenant {
		writeError(w, http.StatusForbidden, "tenant only")
		return
	}
	raw := strings.TrimPrefix(r.URL.Path, "/api/subaccounts/")
	name, err := url.PathUnescape(raw)
	if err != nil || name == "" {
		writeError(w, http.StatusBadRequest, "username required")
		return
	}
	switch r.Method {
	case http.MethodPatch:
		s.handleSubaccountPatch(w, r, id.Username, name)
	case http.MethodDelete:
		if err := s.authStore.DeleteSubaccount(name, id.Username); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleSubaccountPatch(w http.ResponseWriter, r *http.Request, parent, subName string) {
	var req struct {
		PalacePerms map[string][]string `json:"palacePerms"`
		Password    *string             `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if req.PalacePerms == nil && (req.Password == nil || *req.Password == "") {
		writeError(w, http.StatusBadRequest, "palacePerms and/or password required")
		return
	}
	if req.Password != nil && *req.Password != "" && len(*req.Password) < minPasswordLength {
		writeError(w, http.StatusBadRequest,
			fmt.Sprintf("password must be at least %d characters", minPasswordLength))
		return
	}
	perms := req.PalacePerms
	if perms == nil {
		u, ok := s.authStore.Get(subName)
		if !ok || u.Role != authstore.RoleSubaccount || u.ParentTenant != parent {
			writeError(w, http.StatusNotFound, "subaccount not found")
			return
		}
		perms = u.PalacePerms
	}
	var passPtr *string
	if req.Password != nil && *req.Password != "" {
		passPtr = req.Password
	}
	if err := s.authStore.UpdateSubaccount(subName, parent, perms, passPtr); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
