package handlers

import (
	"net/http"
	"strings"
)

func writeAPIError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{
		"error": map[string]any{
			"code":    code,
			"message": message,
		},
	})
}

func writeInternalOrSchemaError(w http.ResponseWriter, err error, fallbackMessage string) {
	if isSchemaError(err) {
		writeAPIError(
			w,
			http.StatusServiceUnavailable,
			"schema_not_ready",
			"database schema is not ready; run migrations and retry",
		)
		return
	}

	writeAPIError(w, http.StatusInternalServerError, "internal_error", fallbackMessage)
}

func isSchemaError(err error) bool {
	if err == nil {
		return false
	}

	msg := strings.ToLower(strings.TrimSpace(err.Error()))
	return strings.Contains(msg, "undefined table") ||
		strings.Contains(msg, "undefined column") ||
		strings.Contains(msg, "relation") && strings.Contains(msg, "does not exist") ||
		strings.Contains(msg, "column") && strings.Contains(msg, "does not exist")
}
