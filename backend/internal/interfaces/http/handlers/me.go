package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"cloud/backend/internal/domain/identity"
	"cloud/backend/internal/interfaces/http/authctx"

	"github.com/google/uuid"
)

type UserReader interface {
	GetByID(ctx context.Context, id uuid.UUID) (identity.User, error)
	UpdateProfile(ctx context.Context, userID uuid.UUID, displayName string, preferredLanguage *string) (identity.User, error)
}

type MeHandler struct {
	users UserReader
}

func NewMeHandler(users UserReader) *MeHandler {
	return &MeHandler{users: users}
}

func (h *MeHandler) GetMe(w http.ResponseWriter, r *http.Request) {
	auth, ok := authctx.FromContext(r.Context())
	if !ok {
		writeAPIError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}

	user, err := h.users.GetByID(r.Context(), auth.UserID)
	if err != nil {
		if errors.Is(err, identity.ErrUserNotFound) {
			writeAPIError(w, http.StatusUnauthorized, "unauthorized", "user does not exist")
			return
		}
		writeAPIError(w, http.StatusInternalServerError, "internal_error", "failed to load user")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"id":                 user.ID.String(),
		"email":              user.Email,
		"display_name":       user.DisplayName,
		"preferred_language": user.PreferredLanguage,
		"role":               string(user.Role),
		"is_active":          user.IsActive,
	})
}

type updateMeRequest struct {
	DisplayName       *string `json:"display_name"`
	PreferredLanguage *string `json:"preferred_language"`
}

func (h *MeHandler) UpdateMe(w http.ResponseWriter, r *http.Request) {
	auth, ok := authctx.FromContext(r.Context())
	if !ok {
		writeAPIError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}

	current, err := h.users.GetByID(r.Context(), auth.UserID)
	if err != nil {
		if errors.Is(err, identity.ErrUserNotFound) {
			writeAPIError(w, http.StatusUnauthorized, "unauthorized", "user does not exist")
			return
		}
		writeAPIError(w, http.StatusInternalServerError, "internal_error", "failed to load user")
		return
	}

	var req updateMeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid_request", "invalid request body")
		return
	}

	displayName := strings.TrimSpace(current.DisplayName)
	if req.DisplayName != nil {
		nextDisplayName := strings.TrimSpace(*req.DisplayName)
		if nextDisplayName == "" {
			writeAPIError(w, http.StatusBadRequest, "invalid_request", "display_name cannot be empty")
			return
		}
		displayName = nextDisplayName
	}

	var preferredLanguage *string
	if req.PreferredLanguage != nil {
		lang := strings.ToLower(strings.TrimSpace(*req.PreferredLanguage))
		if lang != "" && lang != "en" && lang != "tr" {
			writeAPIError(w, http.StatusBadRequest, "invalid_request", "preferred_language must be en or tr")
			return
		}
		if lang != "" {
			preferredLanguage = &lang
		}
	} else {
		preferredLanguage = current.PreferredLanguage
	}

	updated, err := h.users.UpdateProfile(r.Context(), auth.UserID, displayName, preferredLanguage)
	if err != nil {
		if errors.Is(err, identity.ErrUserNotFound) {
			writeAPIError(w, http.StatusUnauthorized, "unauthorized", "user does not exist")
			return
		}
		writeAPIError(w, http.StatusInternalServerError, "internal_error", "failed to update profile")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"id":                 updated.ID.String(),
		"email":              updated.Email,
		"display_name":       updated.DisplayName,
		"preferred_language": updated.PreferredLanguage,
		"role":               string(updated.Role),
		"is_active":          updated.IsActive,
	})
}
