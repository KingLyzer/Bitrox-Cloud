package handlers

import (
	"net"
	"net/http"
	"strings"

	"cloud/backend/internal/interfaces/http/clientipctx"
)

func requestAuditIP(r *http.Request) string {
	if resolved, ok := clientipctx.FromContext(r.Context()); ok {
		return normalizeAuditIP(resolved)
	}
	return normalizeAuditIP(r.RemoteAddr)
}

func normalizeAuditIP(raw string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "unknown"
	}
	if host, _, err := net.SplitHostPort(value); err == nil {
		value = host
	}
	value = strings.TrimPrefix(value, "[")
	value = strings.TrimSuffix(value, "]")
	if value == "" {
		return "unknown"
	}
	return value
}
