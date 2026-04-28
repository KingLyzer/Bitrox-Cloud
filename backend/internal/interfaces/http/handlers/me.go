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
	UpdatePasswordHash(ctx context.Context, userID uuid.UUID, passwordHash string) (identity.User, error)
}

type UserPasswordHasher interface {
	Hash(password string) (string, error)
	Verify(encodedHash string, password string) bool
}

type MeHandler struct {
	users    UserReader
	hasher   UserPasswordHasher
	settings AdminSettingsRepository
}

func NewMeHandler(users UserReader, hasher UserPasswordHasher, settings AdminSettingsRepository) *MeHandler {
	return &MeHandler{
		users:    users,
		hasher:   hasher,
		settings: settings,
	}
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

type updateMePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

func (h *MeHandler) UpdateMyPassword(w http.ResponseWriter, r *http.Request) {
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

	var req updateMePasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid_request", "invalid request body")
		return
	}

	currentPassword := strings.TrimSpace(req.CurrentPassword)
	newPassword := strings.TrimSpace(req.NewPassword)
	if currentPassword == "" || newPassword == "" {
		writeAPIError(w, http.StatusBadRequest, "invalid_request", "current_password and new_password are required")
		return
	}
	policy := defaultAdminSettingsSections()["security"]
	securitySettings := SecuritySettings{
		MinimumPasswordLength: 12,
		RequireUppercase:      true,
		RequireLowercase:      true,
		RequireNumber:         true,
		RequireSymbol:         false,
	}
	if policyMap, ok := policy.(map[string]any); ok {
		raw, _ := json.Marshal(policyMap)
		_ = json.Unmarshal(raw, &securitySettings)
	}
	if h.settings != nil {
		if item, err := h.settings.Get(r.Context(), "security"); err == nil && len(item.ValueJSON) > 0 {
			var persisted SecuritySettings
			if err := json.Unmarshal(item.ValueJSON, &persisted); err == nil {
				if persisted.MinimumPasswordLength > 0 {
					securitySettings.MinimumPasswordLength = persisted.MinimumPasswordLength
				}
				securitySettings.RequireUppercase = persisted.RequireUppercase
				securitySettings.RequireLowercase = persisted.RequireLowercase
				securitySettings.RequireNumber = persisted.RequireNumber
				securitySettings.RequireSymbol = persisted.RequireSymbol
			}
		}
	}
	if err := validatePasswordByPolicy(newPassword, securitySettings); err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	if h.hasher == nil {
		writeAPIError(w, http.StatusInternalServerError, "internal_error", "password hasher is not configured")
		return
	}
	if !h.hasher.Verify(current.PasswordHash, currentPassword) {
		writeAPIError(w, http.StatusUnauthorized, "invalid_credentials", "current password is invalid")
		return
	}

	passwordHash, err := h.hasher.Hash(newPassword)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "internal_error", "failed to hash password")
		return
	}
	if _, err := h.users.UpdatePasswordHash(r.Context(), auth.UserID, passwordHash); err != nil {
		if errors.Is(err, identity.ErrUserNotFound) {
			writeAPIError(w, http.StatusUnauthorized, "unauthorized", "user does not exist")
			return
		}
		writeAPIError(w, http.StatusInternalServerError, "internal_error", "failed to update password")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
