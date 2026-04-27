package authctx

import (
	"context"

	"cloud/backend/internal/domain/identity"

	"github.com/google/uuid"
)

type Context struct {
	UserID    uuid.UUID
	SessionID uuid.UUID
	Role      identity.Role
}

type key string

const authContextKey key = "auth_context"

func WithContext(ctx context.Context, value Context) context.Context {
	return context.WithValue(ctx, authContextKey, value)
}

func FromContext(ctx context.Context) (Context, bool) {
	v := ctx.Value(authContextKey)
	if v == nil {
		return Context{}, false
	}
	auth, ok := v.(Context)
	return auth, ok
}
