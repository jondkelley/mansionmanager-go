package api

import (
	"net/http"
	"strconv"
	"strings"

	"palace-manager/internal/auditlog"
	"palace-manager/internal/authstore"
)

func (s *Server) handleAuditLog(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	id, ok := IdentityFrom(r.Context())
	if !ok {
		unauthorized(w)
		return
	}
	if id.Role != authstore.RoleAdmin && id.Role != authstore.RoleTenant {
		writeError(w, http.StatusForbidden, "admin or tenant only")
		return
	}
	if s.audit == nil {
		writeJSON(w, http.StatusOK, []auditlog.Entry{})
		return
	}

	qPalace := strings.TrimSpace(r.URL.Query().Get("palace"))
	qTenant := strings.TrimSpace(r.URL.Query().Get("tenant"))
	qActor := strings.TrimSpace(r.URL.Query().Get("actor"))
	limit := 200
	if ls := strings.TrimSpace(r.URL.Query().Get("limit")); ls != "" {
		if n, err := strconv.Atoi(ls); err == nil && n > 0 {
			limit = n
		}
	}
	if limit > 2000 {
		limit = 2000
	}

	if id.Role == authstore.RoleTenant {
		if qTenant != "" && qTenant != id.Username {
			writeError(w, http.StatusForbidden, "tenant filter must be your own account")
			return
		}
		if qActor != "" {
			u, ok := s.authStore.Get(qActor)
			if !ok || u.Role != authstore.RoleSubaccount || u.ParentTenant != id.Username {
				writeError(w, http.StatusForbidden, "actor must be a subaccount you own")
				return
			}
		}
	}

	all, err := s.audit.ReadRecent()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	var tenantPalaces map[string]struct{}
	var filterTenantPalaces map[string]struct{}
	if id.Role == authstore.RoleTenant {
		tenantPalaces = palaceSetForUser(s.authStore, id.Username)
	}
	if id.Role == authstore.RoleAdmin && qTenant != "" {
		filterTenantPalaces = palaceSetForUser(s.authStore, qTenant)
	}

	var out []auditlog.Entry
	for i := len(all) - 1; i >= 0; i-- {
		e := all[i]
		if id.Role == authstore.RoleTenant {
			if !tenantSeesEntry(e, id.Username, tenantPalaces) {
				continue
			}
		}
		if qPalace != "" && e.Palace != qPalace {
			continue
		}
		if qActor != "" && e.Actor != qActor {
			continue
		}
		if qTenant != "" {
			if id.Role == authstore.RoleAdmin {
				if !adminTenantFilterMatch(e, qTenant, filterTenantPalaces) {
					continue
				}
			}
		}
		out = append(out, e)
		if len(out) >= limit {
			break
		}
	}
	writeJSON(w, http.StatusOK, out)
}

func palaceSetForUser(store *authstore.Store, username string) map[string]struct{} {
	out := make(map[string]struct{})
	u, ok := store.Get(username)
	if !ok {
		return out
	}
	for _, p := range u.Palaces {
		if p != "" {
			out[p] = struct{}{}
		}
	}
	return out
}

func tenantSeesEntry(e auditlog.Entry, tenantUser string, tenantPalaces map[string]struct{}) bool {
	if e.ScopeTenant == tenantUser {
		return true
	}
	if e.Palace != "" {
		if _, ok := tenantPalaces[e.Palace]; ok {
			return true
		}
	}
	return false
}

func adminTenantFilterMatch(e auditlog.Entry, tenantName string, tenantPalaces map[string]struct{}) bool {
	if e.ScopeTenant == tenantName || e.Actor == tenantName {
		return true
	}
	if e.Palace != "" {
		if _, ok := tenantPalaces[e.Palace]; ok {
			return true
		}
	}
	return false
}
