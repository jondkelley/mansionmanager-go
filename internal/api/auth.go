package api

import (
	"context"
	"fmt"
	"net/http"

	"palace-manager/internal/authstore"
)

func unauthorized(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	_, _ = w.Write([]byte(`{"error":"unauthorized"}`))
}

func passwordChangeAllowed(path, method string) bool {
	if path == "/api/session" && method == http.MethodGet {
		return true
	}
	if path == "/api/session/password" && method == http.MethodPost {
		return true
	}
	return false
}

func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok {
			unauthorized(w)
			return
		}

		u, verified := s.authStore.Verify(user, pass)
		if !verified {
			unauthorized(w)
			return
		}

		if u.Role == authstore.RoleSubaccount {
			parent, ok := s.authStore.Get(u.ParentTenant)
			if !ok || parent.Role != authstore.RoleTenant {
				unauthorized(w)
				return
			}
			if err := authstore.ValidateSubaccountPalacePerms(parent.Palaces, u.PalacePerms); err != nil {
				unauthorized(w)
				return
			}
		}

		id := Identity{
			Username:           u.Username,
			Role:               u.Role,
			Palaces:            append([]string(nil), u.Palaces...),
			ParentTenant:       u.ParentTenant,
			PalacePerms:        clonePalacePerms(u.PalacePerms),
			MustChangePassword: u.MustChangePassword,
		}
		if u.Role == authstore.RoleSubaccount {
			id.Palaces = authstore.SubaccountPalaceKeys(u.PalacePerms)
		}
		ctx := WithIdentity(r.Context(), id)
		r = r.WithContext(ctx)

		if id.MustChangePassword && !passwordChangeAllowed(r.URL.Path, r.Method) {
			writeJSON(w, http.StatusForbidden, map[string]string{
				"error": "password change required",
				"code":  "password_change_required",
			})
			return
		}

		next.ServeHTTP(w, r)
	})
}

func requireAdmin(w http.ResponseWriter, r *http.Request) bool {
	id, ok := IdentityFrom(r.Context())
	if !ok || id.Role != authstore.RoleAdmin {
		writeError(w, http.StatusForbidden, "admin only")
		return false
	}
	return true
}

func (s *Server) requirePrimaryAdmin(w http.ResponseWriter, r *http.Request) bool {
	if !requireAdmin(w, r) {
		return false
	}
	id, ok := IdentityFrom(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return false
	}
	if !s.authStore.IsPrimaryAdmin(id.Username) {
		writeError(w, http.StatusForbidden, "primary admin only")
		return false
	}
	return true
}

func clonePalacePerms(m map[string][]string) map[string][]string {
	if len(m) == 0 {
		return nil
	}
	out := make(map[string][]string, len(m))
	for k, v := range m {
		out[k] = append([]string(nil), v...)
	}
	return out
}

func canAccessPalace(ctx context.Context, palaceName string) bool {
	id, ok := IdentityFrom(ctx)
	if !ok {
		return false
	}
	return authstore.CanAccessPalace(id.Role, id.Palaces, id.PalacePerms, palaceName)
}

// requirePalacePerm enforces palace visibility plus subaccount RBAC (admin/tenant: full access).
func requirePalacePerm(w http.ResponseWriter, r *http.Request, palaceName, perm string) bool {
	if !canAccessPalace(r.Context(), palaceName) {
		writeError(w, http.StatusNotFound, fmt.Sprintf("palace %q not found", palaceName))
		return false
	}
	id, ok := IdentityFrom(r.Context())
	if !ok {
		unauthorized(w)
		return false
	}
	if id.Role == authstore.RoleAdmin || id.Role == authstore.RoleTenant {
		return true
	}
	if id.Role == authstore.RoleSubaccount {
		if authstore.HasPalacePerm(id.PalacePerms, palaceName, perm) {
			return true
		}
		writeError(w, http.StatusForbidden, "permission denied")
		return false
	}
	writeError(w, http.StatusForbidden, "permission denied")
	return false
}
