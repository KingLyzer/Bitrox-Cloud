package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	appauth "cloud/backend/internal/application/auth"
	"cloud/backend/internal/domain/identity"
	"cloud/backend/internal/interfaces/http/authctx"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type AdminUserRepository interface {
	List(ctx context.Context) ([]identity.User, error)
	GetByID(ctx context.Context, id uuid.UUID) (identity.User, error)
	GetByEmail(ctx context.Context, email string) (identity.User, error)
	Create(ctx context.Context, input identity.CreateUserInput) (identity.User, error)
	Update(ctx context.Context, input identity.UpdateUserInput) (identity.User, error)
	ReactivateByID(ctx context.Context, input identity.UpdateUserInput, passwordHash *string) (identity.User, error)
	UpdatePasswordHash(ctx context.Context, userID uuid.UUID, passwordHash string) (identity.User, error)
	SoftDeleteByID(ctx context.Context, userID uuid.UUID) (bool, error)
	HardDeleteByID(ctx context.Context, userID uuid.UUID) (bool, error)
	CountActiveByRole(ctx context.Context, role identity.Role) (int, error)
}

type AdminPasswordHasher interface {
	Hash(password string) (string, error)
}

type AdminQuotaPolicyReader interface {
	GetEffectiveUserQuota(ctx context.Context, userID uuid.UUID) (int64, error)
}

type AdminUsageReader interface {
	GetUserUsedBytes(ctx context.Context, userID uuid.UUID) (int64, error)
}

type AdminOwnershipReader interface {
	CountActiveNodesByOwner(ctx context.Context, ownerUserID uuid.UUID) (int, error)
}

type AdminAuditReader interface {
	ListSecurityEvents(ctx context.Context, query AdminAuditQuery) (AdminAuditQueryResult, error)
}

type AdminAuditQuery struct {
	Limit      int
	Page       int
	EventTypes []string
	Search     string
}

type AdminAuditEntry struct {
	Event     appauth.SecurityEvent
	UserEmail *string
}

type AdminAuditQueryResult struct {
	Events []AdminAuditEntry
	Total  int
	Page   int
	Limit  int
}

type adminUserStatusFilter string

const (
	adminUserStatusFilterAll      adminUserStatusFilter = "all"
	adminUserStatusFilterActive   adminUserStatusFilter = "active"
	adminUserStatusFilterInactive adminUserStatusFilter = "inactive"
	adminUserStatusFilterDeleted  adminUserStatusFilter = "deleted"
)

type AdminUsersHandler struct {
	log         *slog.Logger
	users       AdminUserRepository
	hasher      AdminPasswordHasher
	quotas      AdminQuotaPolicyReader
	usage       AdminUsageReader
	ownership   AdminOwnershipReader
	audit       AdminAuditReader
	settings    AdminSettingsRepository
	storageRoot string
}

func NewAdminUsersHandler(
	log *slog.Logger,
	users AdminUserRepository,
	hasher AdminPasswordHasher,
	quotas AdminQuotaPolicyReader,
	usage AdminUsageReader,
	ownership AdminOwnershipReader,
	audit AdminAuditReader,
	settings AdminSettingsRepository,
	storageRoot string,
) *AdminUsersHandler {
	if log == nil {
		log = slog.Default()
	}
	return &AdminUsersHandler{
		log:         log,
		users:       users,
		hasher:      hasher,
		quotas:      quotas,
		usage:       usage,
		ownership:   ownership,
		audit:       audit,
		settings:    settings,
		storageRoot: storageRoot,
	}
}

func (h *AdminUsersHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireAdminAuth(w, r); !ok {
		return
	}

	statusFilter, err := parseAdminUserStatusFilter(r.URL.Query().Get("status"))
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid_request", "invalid status filter")
		return
	}

	users, err := h.users.List(r.Context())
	if err != nil {
		h.log.Error("list users failed", slog.Any("error", err))
		writeAPIError(w, http.StatusInternalServerError, "internal_error", "failed to list users")
		return
	}

	resp := make([]map[string]any, 0, len(users))
	for _, user := range users {
		if statusFilter != adminUserStatusFilterAll && adminUserStatusOf(user) != statusFilter {
			continue
		}

		item := adminUserToResponse(user)
		if h.quotas != nil {
			limitBytes, err := h.quotas.GetEffectiveUserQuota(r.Context(), user.ID)
			if err != nil {
				h.log.Error("load user effective quota failed", slog.Any("error", err), slog.String("user_id", user.ID.String()))
				writeAPIError(w, http.StatusInternalServerError, "internal_error", "failed to load quota")
				return
			}
			item["limit_bytes"] = limitBytes
		}
		if h.usage != nil {
			usedBytes, err := h.usage.GetUserUsedBytes(r.Context(), user.ID)
			if err != nil {
				h.log.Error("load user usage failed", slog.Any("error", err), slog.String("user_id", user.ID.String()))
				writeAPIError(w, http.StatusInternalServerError, "internal_error", "failed to load usage")
				return
			}
			item["used_bytes"] = usedBytes
		}
		resp = append(resp, item)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"users": resp,
	})
}

type createAdminUserRequest struct {
	Email              string `json:"email"`
	DisplayName        string `json:"display_name"`
	Role               string `json:"role"`
	Password           string `json:"password"`
	QuotaBytes         *int64 `json:"quota_bytes"`
	IsActive           *bool  `json:"is_active"`
	ReactivateExisting bool   `json:"reactivate_existing"`
}

func (h *AdminUsersHandler) CreateUser(w http.ResponseWriter, r *http.Request) {
	authValue, ok := requireAdminAuth(w, r)
	if !ok {
		return
	}

	var req createAdminUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid_request", "invalid request body")
		return
	}

	email := identity.NormalizeEmail(req.Email)
	displayName := strings.TrimSpace(req.DisplayName)
	password := strings.TrimSpace(req.Password)
	role, err := parseAdminRoleInput(req.Role)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid_request", "invalid role")
		return
	}

	if email == "" || displayName == "" || password == "" {
		writeAPIError(w, http.StatusBadRequest, "invalid_request", "email, display_name and password are required")
		return
	}
	securityPolicy := h.loadSecuritySettings(r.Context())
	if err := validatePasswordByPolicy(password, securityPolicy); err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	if req.QuotaBytes != nil && *req.QuotaBytes <= 0 {
		writeAPIError(w, http.StatusBadRequest, "invalid_request", "quota_bytes must be greater than zero")
		return
	}
	if req.QuotaBytes != nil {
		if err := h.validateQuotaAgainstServerCapacity(*req.QuotaBytes); err != nil {
			writeAPIError(w, http.StatusBadRequest, "invalid_request", err.Error())
			return
		}
	}
	if authValue.Role != identity.RoleOwner && role != identity.RoleUser {
		writeAPIError(w, http.StatusForbidden, "forbidden", "admin can create only user role")
		return
	}

	existingUser, existingErr := h.users.GetByEmail(r.Context(), email)
	if existingErr == nil {
		existingStatus := adminUserStatusOf(existingUser)
		if existingStatus == adminUserStatusFilterActive {
			writeAPIError(w, http.StatusConflict, "already_exists", "user already exists")
			return
		}

		if !req.ReactivateExisting {
			writeJSON(w, http.StatusConflict, map[string]any{
				"error": map[string]any{
					"code":    "existing_inactive_user",
					"message": "A disabled/inactive user already exists with this email. Do you want to reactivate it?",
				},
				"user": adminUserToResponse(existingUser),
			})
			return
		}
	} else if !errors.Is(existingErr, identity.ErrUserNotFound) {
		h.log.Error("check existing user failed", slog.Any("error", existingErr))
		writeAPIError(w, http.StatusInternalServerError, "internal_error", "failed to validate user")
		return
	}

	passwordHash, err := h.hasher.Hash(password)
	if err != nil {
		h.log.Error("hash user password failed", slog.Any("error", err))
		writeAPIError(w, http.StatusInternalServerError, "internal_error", "failed to create user")
		return
	}

	isActive := true
	if req.IsActive != nil {
		isActive = *req.IsActive
	}

	var user identity.User
	if existingErr == nil {
		if !canManageTargetUser(authValue, existingUser) {
			writeAPIError(w, http.StatusForbidden, "forbidden", "insufficient permission for target user")
			return
		}

		user, err = h.users.ReactivateByID(r.Context(), identity.UpdateUserInput{
			ID:          existingUser.ID,
			DisplayName: displayName,
			Role:        role,
			QuotaBytes:  req.QuotaBytes,
			IsActive:    true,
		}, &passwordHash)
		if err != nil {
			if errors.Is(err, identity.ErrUserNotFound) {
				writeAPIError(w, http.StatusNotFound, "user_not_found", "user not found")
				return
			}
			h.log.Error("reactivate user failed", slog.Any("error", err))
			writeAPIError(w, http.StatusInternalServerError, "internal_error", "failed to reactivate user")
			return
		}
	} else {
		user, err = h.users.Create(r.Context(), identity.CreateUserInput{
			Email:        email,
			DisplayName:  displayName,
			Role:         role,
			PasswordHash: passwordHash,
			QuotaBytes:   req.QuotaBytes,
			IsActive:     isActive,
		})
	}
	if err != nil {
		h.log.Error("create user failed", slog.Any("error", err))
		if strings.Contains(strings.ToLower(err.Error()), "users_email_lower_uniq") {
			writeAPIError(w, http.StatusConflict, "already_exists", "user already exists")
			return
		}
		writeAPIError(w, http.StatusInternalServerError, "internal_error", "failed to create user")
		return
	}

	eventType := "admin.user.created"
	if existingErr == nil {
		eventType = "admin.user.reactivated"
	}
	h.recordAdminAudit(r, authValue, eventType, "info", map[string]any{
		"target_user_id": user.ID.String(),
		"target_email":   user.Email,
		"target_role":    normalizeRoleForAdminResponse(user.Role),
	})

	status := http.StatusCreated
	if existingErr == nil {
		status = http.StatusOK
	}
	writeJSON(w, status, map[string]any{
		"user": adminUserToResponse(user),
	})
}

type updateAdminUserRequest struct {
	DisplayName     *string `json:"display_name"`
	Role            *string `json:"role"`
	QuotaBytes      *int64  `json:"quota_bytes"`
	UseDefaultQuota *bool   `json:"use_default_quota"`
	IsActive        *bool   `json:"is_active"`
}

func (h *AdminUsersHandler) UpdateUser(w http.ResponseWriter, r *http.Request) {
	authValue, ok := requireAdminAuth(w, r)
	if !ok {
		return
	}

	userID, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "userID")))
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid_request", "invalid user id")
		return
	}

	var req updateAdminUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid_request", "invalid request body")
		return
	}

	user, err := h.users.GetByID(r.Context(), userID)
	if err != nil {
		if errors.Is(err, identity.ErrUserNotFound) {
			writeAPIError(w, http.StatusNotFound, "user_not_found", "user not found")
			return
		}
		h.log.Error("load user for update failed", slog.Any("error", err))
		writeAPIError(w, http.StatusInternalServerError, "internal_error", "failed to load user")
		return
	}
	if !canManageTargetUser(authValue, user) {
		writeAPIError(w, http.StatusForbidden, "forbidden", "insufficient permission for target user")
		return
	}

	if req.DisplayName != nil {
		nextName := strings.TrimSpace(*req.DisplayName)
		if nextName == "" {
			writeAPIError(w, http.StatusBadRequest, "invalid_request", "display_name cannot be empty")
			return
		}
		user.DisplayName = nextName
	}

	if req.Role != nil {
		nextRole, err := parseAdminRoleInput(*req.Role)
		if err != nil {
			writeAPIError(w, http.StatusBadRequest, "invalid_request", "invalid role")
			return
		}
		if authValue.UserID == user.ID && authValue.Role == identity.RoleOwner && nextRole != identity.RoleOwner {
			writeAPIError(w, http.StatusBadRequest, "invalid_request", "owner cannot demote itself")
			return
		}
		if authValue.Role != identity.RoleOwner && nextRole != identity.RoleUser {
			writeAPIError(w, http.StatusForbidden, "forbidden", "admin can assign only user role")
			return
		}
		if user.Role == identity.RoleOwner && user.IsActive && nextRole != identity.RoleOwner {
			ownerCount, err := h.users.CountActiveByRole(r.Context(), identity.RoleOwner)
			if err != nil {
				h.log.Error("count active owners failed", slog.Any("error", err))
				writeAPIError(w, http.StatusInternalServerError, "internal_error", "failed to validate owner count")
				return
			}
			if ownerCount <= 1 {
				writeAPIError(w, http.StatusConflict, "last_owner", "cannot demote last active owner")
				return
			}
		}
		user.Role = nextRole
	}

	if req.UseDefaultQuota != nil && *req.UseDefaultQuota {
		user.QuotaBytes = nil
	} else if req.QuotaBytes != nil {
		if *req.QuotaBytes <= 0 {
			writeAPIError(w, http.StatusBadRequest, "invalid_request", "quota_bytes must be greater than zero")
			return
		}
		if err := h.validateQuotaAgainstServerCapacity(*req.QuotaBytes); err != nil {
			writeAPIError(w, http.StatusBadRequest, "invalid_request", err.Error())
			return
		}
		value := *req.QuotaBytes
		user.QuotaBytes = &value
	}

	shouldReactivateDeleted := false
	if req.IsActive != nil {
		if authValue.UserID == user.ID && !*req.IsActive {
			writeAPIError(w, http.StatusBadRequest, "invalid_request", "cannot deactivate current session user")
			return
		}
		if user.Role == identity.RoleOwner && user.IsActive && !*req.IsActive {
			ownerCount, err := h.users.CountActiveByRole(r.Context(), identity.RoleOwner)
			if err != nil {
				h.log.Error("count active owners failed", slog.Any("error", err))
				writeAPIError(w, http.StatusInternalServerError, "internal_error", "failed to validate owner count")
				return
			}
			if ownerCount <= 1 {
				writeAPIError(w, http.StatusConflict, "last_owner", "cannot deactivate last active owner")
				return
			}
		}
		user.IsActive = *req.IsActive
		if *req.IsActive && user.DeletedAt != nil {
			shouldReactivateDeleted = true
		}
	} else if user.DeletedAt != nil {
		writeAPIError(w, http.StatusConflict, "user_already_disabled", "user is already disabled")
		return
	}

	var updated identity.User
	if shouldReactivateDeleted {
		updated, err = h.users.ReactivateByID(r.Context(), identity.UpdateUserInput{
			ID:          user.ID,
			DisplayName: user.DisplayName,
			Role:        user.Role,
			QuotaBytes:  user.QuotaBytes,
			IsActive:    true,
		}, nil)
	} else {
		updated, err = h.users.Update(r.Context(), identity.UpdateUserInput{
			ID:          user.ID,
			DisplayName: user.DisplayName,
			Role:        user.Role,
			QuotaBytes:  user.QuotaBytes,
			IsActive:    user.IsActive,
		})
	}
	if err != nil {
		if errors.Is(err, identity.ErrUserNotFound) {
			writeAPIError(w, http.StatusNotFound, "user_not_found", "user not found")
			return
		}
		h.log.Error("update user failed", slog.Any("error", err))
		writeAPIError(w, http.StatusInternalServerError, "internal_error", "failed to update user")
		return
	}

	h.recordAdminAudit(r, authValue, "admin.user.updated", "info", map[string]any{
		"target_user_id": updated.ID.String(),
		"target_email":   updated.Email,
		"target_role":    normalizeRoleForAdminResponse(updated.Role),
		"is_active":      updated.IsActive,
	})

	writeJSON(w, http.StatusOK, map[string]any{
		"user": adminUserToResponse(updated),
	})
}

func (h *AdminUsersHandler) DeleteUser(w http.ResponseWriter, r *http.Request) {
	authValue, ok := requireAdminAuth(w, r)
	if !ok {
		return
	}

	userID, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "userID")))
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid_request", "invalid user id")
		return
	}
	if authValue.UserID == userID {
		writeAPIError(w, http.StatusBadRequest, "invalid_request", "cannot delete current session user")
		return
	}

	user, err := h.users.GetByID(r.Context(), userID)
	if err != nil {
		if errors.Is(err, identity.ErrUserNotFound) {
			writeAPIError(w, http.StatusNotFound, "user_not_found", "user not found")
			return
		}
		h.log.Error("load user for delete failed", slog.Any("error", err))
		writeAPIError(w, http.StatusInternalServerError, "internal_error", "failed to load user")
		return
	}

	if !canManageTargetUser(authValue, user) {
		writeAPIError(w, http.StatusForbidden, "forbidden", "insufficient permission for target user")
		return
	}

	permanentDelete := parseBooleanQueryParam(r, "permanent")
	if permanentDelete {
		if user.DeletedAt == nil {
			writeAPIError(w, http.StatusConflict, "user_not_deleted", "disable user before permanent deletion")
			return
		}

		deleted, err := h.users.HardDeleteByID(r.Context(), userID)
		if err != nil {
			h.log.Error("hard delete user failed", slog.Any("error", err))
			writeAPIError(w, http.StatusInternalServerError, "internal_error", "failed to permanently delete user")
			return
		}
		if !deleted {
			writeAPIError(w, http.StatusNotFound, "user_not_found", "user not found")
			return
		}

		h.recordAdminAudit(r, authValue, "admin.user.permanently_deleted", "high", map[string]any{
			"target_user_id": userID.String(),
			"target_email":   user.Email,
			"target_role":    normalizeRoleForAdminResponse(user.Role),
		})
		w.WriteHeader(http.StatusNoContent)
		return
	}

	if user.Role == identity.RoleOwner && user.IsActive {
		ownerCount, err := h.users.CountActiveByRole(r.Context(), identity.RoleOwner)
		if err != nil {
			h.log.Error("count active owners failed", slog.Any("error", err))
			writeAPIError(w, http.StatusInternalServerError, "internal_error", "failed to validate owner count")
			return
		}
		if ownerCount <= 1 {
			writeAPIError(w, http.StatusConflict, "last_owner", "cannot delete last active owner")
			return
		}
	}

	deleted, err := h.users.SoftDeleteByID(r.Context(), userID)
	if err != nil {
		h.log.Error("soft delete user failed", slog.Any("error", err))
		writeAPIError(w, http.StatusInternalServerError, "internal_error", "failed to disable user")
		return
	}
	if !deleted {
		writeAPIError(w, http.StatusNotFound, "user_not_found", "user not found")
		return
	}

	h.recordAdminAudit(r, authValue, "admin.user.soft_deleted", "high", map[string]any{
		"target_user_id": userID.String(),
		"target_email":   user.Email,
		"target_role":    normalizeRoleForAdminResponse(user.Role),
	})
	w.WriteHeader(http.StatusNoContent)
}

type updatePasswordRequest struct {
	NewPassword string `json:"new_password"`
}

func (h *AdminUsersHandler) UpdateUserPassword(w http.ResponseWriter, r *http.Request) {
	authValue, ok := requireAdminAuth(w, r)
	if !ok {
		return
	}

	userID, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "userID")))
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid_request", "invalid user id")
		return
	}

	user, err := h.users.GetByID(r.Context(), userID)
	if err != nil {
		if errors.Is(err, identity.ErrUserNotFound) {
			writeAPIError(w, http.StatusNotFound, "user_not_found", "user not found")
			return
		}
		h.log.Error("load user for password update failed", slog.Any("error", err))
		writeAPIError(w, http.StatusInternalServerError, "internal_error", "failed to load user")
		return
	}
	if authValue.UserID != user.ID && !canManageTargetUser(authValue, user) {
		writeAPIError(w, http.StatusForbidden, "forbidden", "insufficient permission for target user")
		return
	}

	var req updatePasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid_request", "invalid request body")
		return
	}

	newPassword := strings.TrimSpace(req.NewPassword)
	securityPolicy := h.loadSecuritySettings(r.Context())
	if err := validatePasswordByPolicy(newPassword, securityPolicy); err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	passwordHash, err := h.hasher.Hash(newPassword)
	if err != nil {
		h.log.Error("hash new password failed", slog.Any("error", err))
		writeAPIError(w, http.StatusInternalServerError, "internal_error", "failed to update password")
		return
	}

	updated, err := h.users.UpdatePasswordHash(r.Context(), userID, passwordHash)
	if err != nil {
		if errors.Is(err, identity.ErrUserNotFound) {
			writeAPIError(w, http.StatusNotFound, "user_not_found", "user not found")
			return
		}
		h.log.Error("update user password failed", slog.Any("error", err))
		writeAPIError(w, http.StatusInternalServerError, "internal_error", "failed to update password")
		return
	}

	h.recordAdminAudit(r, authValue, "admin.user.password_updated", "high", map[string]any{
		"target_user_id": updated.ID.String(),
		"target_email":   updated.Email,
	})

	writeJSON(w, http.StatusOK, map[string]any{
		"user": adminUserToResponse(updated),
	})
}

func (h *AdminUsersHandler) loadSecuritySettings(ctx context.Context) SecuritySettings {
	settings := defaultAdminSettingsSections()
	section, ok := settings["security"].(map[string]any)
	if !ok {
		return SecuritySettings{
			MinimumPasswordLength: 12,
			RequireUppercase:      true,
			RequireLowercase:      true,
			RequireNumber:         true,
			RequireSymbol:         false,
		}
	}
	defaultRaw, _ := json.Marshal(section)
	var policy SecuritySettings
	_ = json.Unmarshal(defaultRaw, &policy)

	if h.settings == nil {
		return policy
	}
	item, err := h.settings.Get(ctx, "security")
	if err != nil || len(item.ValueJSON) == 0 {
		return policy
	}
	var persisted SecuritySettings
	if err := json.Unmarshal(item.ValueJSON, &persisted); err != nil {
		return policy
	}
	if persisted.MinimumPasswordLength > 0 {
		policy.MinimumPasswordLength = persisted.MinimumPasswordLength
	}
	policy.RequireUppercase = persisted.RequireUppercase
	policy.RequireLowercase = persisted.RequireLowercase
	policy.RequireNumber = persisted.RequireNumber
	policy.RequireSymbol = persisted.RequireSymbol
	return policy
}

func validatePasswordByPolicy(password string, policy SecuritySettings) error {
	length := policy.MinimumPasswordLength
	if length < 8 {
		length = 8
	}
	if len(password) < length {
		return fmt.Errorf("password must be at least %d characters", length)
	}
	if policy.RequireUppercase && !containsAny(password, "ABCDEFGHIJKLMNOPQRSTUVWXYZ") {
		return fmt.Errorf("password must contain at least one uppercase letter")
	}
	if policy.RequireLowercase && !containsAny(password, "abcdefghijklmnopqrstuvwxyz") {
		return fmt.Errorf("password must contain at least one lowercase letter")
	}
	if policy.RequireNumber && !containsAny(password, "0123456789") {
		return fmt.Errorf("password must contain at least one number")
	}
	if policy.RequireSymbol && !containsAny(password, `!"#$%&'()*+,-./:;<=>?@[\]^_{|}~`) {
		return fmt.Errorf("password must contain at least one symbol")
	}
	return nil
}

func containsAny(input string, candidates string) bool {
	for _, r := range input {
		if strings.ContainsRune(candidates, r) {
			return true
		}
	}
	return false
}

func (h *AdminUsersHandler) validateQuotaAgainstServerCapacity(quotaBytes int64) error {
	if quotaBytes <= 0 || h.storageRoot == "" {
		return nil
	}
	stats, err := readStorageStats(h.storageRoot)
	if err != nil || stats.TotalBytes <= 0 {
		return nil
	}
	if quotaBytes > stats.TotalBytes {
		return fmt.Errorf("quota_bytes cannot exceed server storage capacity")
	}
	return nil
}

func (h *AdminUsersHandler) ListAudit(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireAdminAuth(w, r); !ok {
		return
	}
	if h.audit == nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"events": []any{},
			"pagination": map[string]any{
				"page":        1,
				"limit":       20,
				"total":       0,
				"total_pages": 0,
			},
		})
		return
	}

	limit := 20
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value <= 0 {
			writeAPIError(w, http.StatusBadRequest, "invalid_request", "invalid limit")
			return
		}
		if value > 100 {
			value = 100
		}
		limit = value
	}

	page := 1
	if raw := strings.TrimSpace(r.URL.Query().Get("page")); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value <= 0 {
			writeAPIError(w, http.StatusBadRequest, "invalid_request", "invalid page")
			return
		}
		page = value
	}

	eventTypes := collectAuditEventTypes(r)
	hasExplicitEventFilter := len(r.URL.Query()["event_type"]) > 0 || strings.TrimSpace(r.URL.Query().Get("event_types")) != ""
	if !hasExplicitEventFilter {
		eventTypes = []string{
			"auth.login",
			"auth.logout",
			"files.upload.finalized",
			"files.node.deleted",
			"admin.user.created",
			"admin.user.updated",
			"admin.user.password_updated",
			"admin.user.soft_deleted",
			"admin.user.permanently_deleted",
		}
	}

	search := strings.TrimSpace(r.URL.Query().Get("search"))

	result, err := h.audit.ListSecurityEvents(r.Context(), AdminAuditQuery{
		Limit:      limit,
		Page:       page,
		EventTypes: eventTypes,
		Search:     search,
	})
	if err != nil {
		h.log.Error("list audit events failed", slog.Any("error", err))
		writeAPIError(w, http.StatusInternalServerError, "internal_error", "failed to list audit events")
		return
	}

	response := make([]map[string]any, 0, len(result.Events))
	for _, entry := range result.Events {
		event := entry.Event
		normalizedIP := normalizeAuditIP(event.IP)
		item := map[string]any{
			"event_type":       event.EventType,
			"severity":         event.Severity,
			"ip":               normalizedIP,
			"user_agent":       event.UserAgent,
			"user_agent_short": summarizeAuditUserAgent(event.UserAgent),
			"device_type":      detectAuditDeviceType(event.UserAgent),
			"browser":          detectAuditBrowser(event.UserAgent),
			"metadata":         event.Metadata,
			"created_at":       event.CreatedAt,
			"created_at_human": event.CreatedAt.Local().Format("02 Jan 2006 15:04:05"),
		}
		if event.UserID != nil {
			item["user_id"] = event.UserID.String()
		} else {
			item["user_id"] = nil
		}
		if event.SessionID != nil {
			item["session_id"] = event.SessionID.String()
		} else {
			item["session_id"] = nil
		}
		if entry.UserEmail != nil {
			item["user_email"] = *entry.UserEmail
		} else {
			item["user_email"] = nil
		}
		response = append(response, item)
	}

	totalPages := 0
	if result.Total > 0 {
		totalPages = (result.Total + result.Limit - 1) / result.Limit
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"events": response,
		"pagination": map[string]any{
			"page":        result.Page,
			"limit":       result.Limit,
			"total":       result.Total,
			"total_pages": totalPages,
		},
	})
}

func collectAuditEventTypes(r *http.Request) []string {
	query := r.URL.Query()
	combined := make([]string, 0, 8)

	for _, value := range query["event_type"] {
		combined = append(combined, value)
	}
	if raw := strings.TrimSpace(query.Get("event_types")); raw != "" {
		combined = append(combined, strings.Split(raw, ",")...)
	}

	normalized := make([]string, 0, len(combined))
	seen := map[string]struct{}{}
	for _, raw := range combined {
		eventType := strings.TrimSpace(raw)
		if eventType == "" {
			continue
		}
		if strings.EqualFold(eventType, "all") {
			return []string{}
		}
		if _, ok := seen[eventType]; ok {
			continue
		}
		seen[eventType] = struct{}{}
		normalized = append(normalized, eventType)
	}
	return normalized
}

func canManageTargetUser(actor authctx.Context, target identity.User) bool {
	if actor.Role == identity.RoleOwner {
		return true
	}
	if actor.Role == identity.RoleAdmin {
		return target.Role == identity.RoleUser
	}
	return false
}

func requireAdminAuth(w http.ResponseWriter, r *http.Request) (authctx.Context, bool) {
	authValue, ok := authctx.FromContext(r.Context())
	if !ok {
		writeAPIError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return authctx.Context{}, false
	}
	if authValue.Role != identity.RoleOwner && authValue.Role != identity.RoleAdmin {
		writeAPIError(w, http.StatusForbidden, "forbidden", "admin role required")
		return authctx.Context{}, false
	}
	return authValue, true
}

func parseAdminRoleInput(raw string) (identity.Role, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "owner":
		return identity.RoleOwner, nil
	case "admin":
		return identity.RoleAdmin, nil
	case "user":
		return identity.RoleUser, nil
	default:
		return "", errors.New("invalid role")
	}
}

func normalizeRoleForAdminResponse(role identity.Role) string {
	switch role {
	case identity.RoleOwner:
		return "owner"
	case identity.RoleAdmin:
		return "admin"
	case identity.RoleUser:
		return "user"
	default:
		return "user"
	}
}

func adminUserToResponse(user identity.User) map[string]any {
	resp := map[string]any{
		"id":                 user.ID.String(),
		"email":              user.Email,
		"display_name":       user.DisplayName,
		"preferred_language": user.PreferredLanguage,
		"role":               normalizeRoleForAdminResponse(user.Role),
		"is_active":          user.IsActive,
		"status":             string(adminUserStatusOf(user)),
		"created_at":         user.CreatedAt,
		"updated_at":         user.UpdatedAt,
	}
	if user.QuotaBytes != nil {
		resp["quota_bytes"] = *user.QuotaBytes
	} else {
		resp["quota_bytes"] = nil
	}
	if user.DeletedAt != nil {
		resp["deleted_at"] = *user.DeletedAt
	} else {
		resp["deleted_at"] = nil
	}
	return resp
}

func adminUserStatusOf(user identity.User) adminUserStatusFilter {
	if user.DeletedAt != nil {
		return adminUserStatusFilterDeleted
	}
	if !user.IsActive {
		return adminUserStatusFilterInactive
	}
	return adminUserStatusFilterActive
}

func parseAdminUserStatusFilter(raw string) (adminUserStatusFilter, error) {
	normalized := strings.ToLower(strings.TrimSpace(raw))
	if normalized == "" || normalized == "all" || normalized == "tum" || normalized == "tumu" {
		return adminUserStatusFilterAll, nil
	}
	switch normalized {
	case "active", "aktif":
		return adminUserStatusFilterActive, nil
	case "inactive", "pasif", "disabled":
		return adminUserStatusFilterInactive, nil
	case "deleted", "silinmis", "soft_deleted":
		return adminUserStatusFilterDeleted, nil
	default:
		return "", errors.New("invalid status filter")
	}
}

func (h *AdminUsersHandler) recordAdminAudit(r *http.Request, actor authctx.Context, eventType string, severity string, metadata map[string]any) {
	if h.audit == nil {
		return
	}
	userID := actor.UserID
	sessionID := actor.SessionID
	if recorder, ok := h.audit.(appauth.AuditRecorder); ok {
		if err := recorder.RecordSecurityEvent(r.Context(), appauth.SecurityEvent{
			EventType: eventType,
			Severity:  severity,
			UserID:    &userID,
			SessionID: &sessionID,
			IP:        requestAuditIP(r),
			UserAgent: strings.TrimSpace(r.UserAgent()),
			Metadata:  metadata,
		}); err != nil {
			h.log.Warn("record admin audit failed", slog.Any("error", err), slog.String("event_type", eventType))
		}
	}
}

func detectAuditDeviceType(userAgent string) string {
	ua := strings.ToLower(strings.TrimSpace(userAgent))
	if ua == "" {
		return "Unknown"
	}
	if strings.Contains(ua, "ipad") || strings.Contains(ua, "tablet") {
		return "Tablet"
	}
	if strings.Contains(ua, "mobile") || strings.Contains(ua, "android") || strings.Contains(ua, "iphone") {
		return "Mobile"
	}
	if strings.Contains(ua, "windows") || strings.Contains(ua, "macintosh") || strings.Contains(ua, "linux") {
		return "Desktop"
	}
	return "Unknown"
}

func detectAuditBrowser(userAgent string) string {
	ua := strings.ToLower(strings.TrimSpace(userAgent))
	switch {
	case ua == "":
		return "Unknown"
	case strings.Contains(ua, "edg/"):
		return "Edge"
	case strings.Contains(ua, "opr/") || strings.Contains(ua, "opera"):
		return "Opera"
	case strings.Contains(ua, "chrome/") && !strings.Contains(ua, "edg/"):
		return "Chrome"
	case strings.Contains(ua, "firefox/"):
		return "Firefox"
	case strings.Contains(ua, "safari/") && !strings.Contains(ua, "chrome/"):
		return "Safari"
	default:
		return "Unknown"
	}
}

func summarizeAuditUserAgent(userAgent string) string {
	trimmed := strings.TrimSpace(userAgent)
	if trimmed == "" {
		return "Unknown client"
	}
	if len(trimmed) <= 80 {
		return trimmed
	}
	return trimmed[:77] + "..."
}

func parseBooleanQueryParam(r *http.Request, name string) bool {
	raw := strings.TrimSpace(strings.ToLower(r.URL.Query().Get(name)))
	switch raw {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}
