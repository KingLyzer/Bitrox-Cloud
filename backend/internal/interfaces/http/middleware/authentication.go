package middleware

import (
	"context"
	"net/http"
	"strings"

	"cloud/backend/internal/application/auth"
	"cloud/backend/internal/interfaces/http/authctx"
)

const (
	legacyAccessCookieName = "cloud_access_token"
	hostAccessCookieName   = "__Host-cloud_access_token"
)

type AccessAuthenticator interface {
	AuthenticateAccessToken(ctx context.Context, accessToken string) (auth.AccessTokenClaims, error)
}

func RequireAuth(authenticator AccessAuthenticator) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := extractAccessToken(r)
			if token == "" {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}

			claims, err := authenticator.AuthenticateAccessToken(r.Context(), token)
			if err != nil {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}

			ctx := authctx.WithContext(r.Context(), authctx.Context{
				UserID:    claims.UserID,
				SessionID: claims.SessionID,
				Role:      claims.Role,
			})

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func extractAccessToken(r *http.Request) string {
	if c, err := r.Cookie(hostAccessCookieName); err == nil && strings.TrimSpace(c.Value) != "" {
		return strings.TrimSpace(c.Value)
	}
	if c, err := r.Cookie(legacyAccessCookieName); err == nil && strings.TrimSpace(c.Value) != "" {
		return strings.TrimSpace(c.Value)
	}

	const bearerPrefix = "Bearer "
	authHeader := strings.TrimSpace(r.Header.Get("Authorization"))
	if strings.HasPrefix(authHeader, bearerPrefix) {
		return strings.TrimSpace(strings.TrimPrefix(authHeader, bearerPrefix))
	}

	return ""
}
