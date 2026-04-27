package middleware

import (
	"net/http"

	"cloud/backend/internal/interfaces/http/clientipctx"
)

type RequestIPResolver interface {
	Resolve(req *http.Request) string
}

func WithClientIP(resolver RequestIPResolver) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if resolver == nil {
				next.ServeHTTP(w, r)
				return
			}
			clientIP := resolver.Resolve(r)
			next.ServeHTTP(w, r.WithContext(clientipctx.WithClientIP(r.Context(), clientIP)))
		})
	}
}
