package api

import (
	"context"

	"palace-manager/internal/authstore"
)

type Identity struct {
	Username           string
	Role               authstore.Role
	Palaces            []string
	ParentTenant       string
	PalacePerms        map[string][]string // subaccount only
	MustChangePassword bool
}

type identityCtxKey struct{}

var identityKey = identityCtxKey{}

func WithIdentity(ctx context.Context, id Identity) context.Context {
	return context.WithValue(ctx, identityKey, id)
}

func IdentityFrom(ctx context.Context) (Identity, bool) {
	id, ok := ctx.Value(identityKey).(Identity)
	return id, ok
}
