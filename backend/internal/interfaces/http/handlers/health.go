package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

type ReadinessCheck struct {
	Name  string
	Check func(context.Context) error
}

type HealthHandler struct {
	readinessChecks []ReadinessCheck
}

func NewHealthHandler(readinessChecks []ReadinessCheck) *HealthHandler {
	return &HealthHandler{
		readinessChecks: readinessChecks,
	}
}

func (h *HealthHandler) Live(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
		"time":   time.Now().UTC(),
	})
}

func (h *HealthHandler) Ready(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	failures := map[string]string{}
	for _, check := range h.readinessChecks {
		if check.Check == nil {
			continue
		}
		if err := check.Check(ctx); err != nil {
			failures[check.Name] = err.Error()
		}
	}

	if len(failures) > 0 {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{
			"status":   "not_ready",
			"time":     time.Now().UTC(),
			"failures": failures,
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ready",
		"time":   time.Now().UTC(),
	})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
