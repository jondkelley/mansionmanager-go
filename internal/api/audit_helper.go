package api

import (
	"context"
	"time"

	"palace-manager/internal/auditlog"
	"palace-manager/internal/authstore"
)

func (s *Server) writeAudit(ctx context.Context, action, palace string, detail map[string]string) {
	s.writeAuditScope(ctx, action, palace, "", detail)
}

// writeAuditScope records a mutation. scopeTenantOverride, when non-empty, sets scopeTenant instead of inferring from context.
func (s *Server) writeAuditScope(ctx context.Context, action, palace, scopeTenantOverride string, detail map[string]string) {
	if s.audit == nil {
		return
	}
	id, ok := IdentityFrom(ctx)
	if !ok {
		return
	}
	scope := scopeTenantOverride
	if scope == "" {
		switch id.Role {
		case authstore.RoleTenant:
			scope = id.Username
		case authstore.RoleSubaccount:
			scope = id.ParentTenant
		case authstore.RoleAdmin:
			if palace != "" && s.authStore != nil {
				scope = s.authStore.SingleTenantForPalace(palace)
			}
		}
	}
	d := detail
	if d == nil {
		d = map[string]string{}
	}
	e := auditlog.Entry{
		TS:          time.Now().UTC().Format(time.RFC3339Nano),
		Actor:       id.Username,
		ActorRole:   string(id.Role),
		ScopeTenant: scope,
		Palace:      palace,
		Action:      action,
		Detail:      d,
	}
	_ = s.audit.Append(e)
}
