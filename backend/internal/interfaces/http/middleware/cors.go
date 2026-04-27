package middleware

import (
	"net/http"
	"net/url"
	"strings"
)

type originMatcher struct {
	allowAny bool
	exact    map[string]struct{}
}

func newOriginMatcher(allowedOrigins []string) originMatcher {
	matcher := originMatcher{
		exact: make(map[string]struct{}, len(allowedOrigins)),
	}
	for _, raw := range allowedOrigins {
		candidate := strings.TrimSpace(raw)
		if candidate == "" {
			continue
		}
		if candidate == "*" {
			matcher.allowAny = true
			continue
		}
		normalized := normalizeOrigin(candidate)
		if normalized != "" {
			matcher.exact[normalized] = struct{}{}
		}
	}
	return matcher
}

func (m originMatcher) allows(origin string) bool {
	if origin == "" {
		return false
	}
	if m.allowAny {
		return true
	}
	normalized := normalizeOrigin(origin)
	if normalized == "" {
		return false
	}
	_, ok := m.exact[normalized]
	return ok
}

func normalizeOrigin(raw string) string {
	trimmed := strings.TrimRight(strings.TrimSpace(raw), "/")
	if trimmed == "" {
		return ""
	}

	parsed, err := url.Parse(trimmed)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return strings.ToLower(trimmed)
	}

	return strings.ToLower(parsed.Scheme) + "://" + strings.ToLower(parsed.Host)
}

func CORS(allowedOrigins []string) func(http.Handler) http.Handler {
	matcher := newOriginMatcher(allowedOrigins)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := strings.TrimSpace(r.Header.Get("Origin"))
			if matcher.allows(origin) {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Vary", "Origin")
			}

			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Requested-With, X-CSRF-Token, Idempotency-Key, X-Chunk-SHA256")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
