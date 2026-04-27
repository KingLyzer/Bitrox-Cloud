package handlers

import (
	"net/http"

	appquota "cloud/backend/internal/application/quota"
	"cloud/backend/internal/interfaces/http/authctx"
)

type QuotaHandler struct {
	service *appquota.Service
}

func NewQuotaHandler(service *appquota.Service) *QuotaHandler {
	return &QuotaHandler{service: service}
}

func (h *QuotaHandler) GetMyQuota(w http.ResponseWriter, r *http.Request) {
	auth, ok := authctx.FromContext(r.Context())
	if !ok {
		writeAPIError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}

	usage, err := h.service.GetUserUsage(r.Context(), auth.UserID)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "internal_error", "failed to load quota")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"used_bytes":       usage.UsedBytes,
		"limit_bytes":      usage.LimitBytes,
		"remaining_bytes":  usage.RemainingBytes,
		"is_limit_exceeded": usage.IsLimitExceeded,
	})
}
