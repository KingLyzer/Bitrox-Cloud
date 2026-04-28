package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type AdminSettingsRepository interface {
	List(ctx context.Context) ([]AdminSettingRecord, error)
	Get(ctx context.Context, key string) (AdminSettingRecord, error)
	Upsert(ctx context.Context, key string, value any, updatedBy *uuid.UUID) (AdminSettingRecord, error)
}

type AdminSettingRecord struct {
	Key       string
	ValueJSON json.RawMessage
}

type AdminSettingsQuotaPolicyWriter interface {
	UpsertDefaultUserQuota(ctx context.Context, quotaBytes int64) error
}

type AdminStorageStats struct {
	TotalBytes int64 `json:"total_bytes"`
	UsedBytes  int64 `json:"used_bytes"`
	FreeBytes  int64 `json:"free_bytes"`
}

type AdminSettingsService struct {
	repo        AdminSettingsRepository
	quotaPolicy AdminSettingsQuotaPolicyWriter
	storageRoot string
}

func NewAdminSettingsService(
	repo AdminSettingsRepository,
	quotaPolicy AdminSettingsQuotaPolicyWriter,
	storageRoot string,
) *AdminSettingsService {
	return &AdminSettingsService{
		repo:        repo,
		quotaPolicy: quotaPolicy,
		storageRoot: storageRoot,
	}
}

func (s *AdminSettingsService) ListSections(ctx context.Context) (map[string]any, error) {
	response := defaultAdminSettingsSections()

	items, err := s.repo.List(ctx)
	if err != nil {
		return nil, err
	}
	for _, item := range items {
		var decoded map[string]any
		if err := json.Unmarshal(item.ValueJSON, &decoded); err != nil {
			continue
		}
		response[item.Key] = decoded
	}

	if otherSection, ok := response["other"].(map[string]any); ok {
		stats, err := readStorageStats(s.storageRoot)
		if err == nil {
			otherSection["system_storage_total_bytes"] = stats.TotalBytes
			otherSection["system_storage_used_bytes"] = stats.UsedBytes
			otherSection["system_storage_free_bytes"] = stats.FreeBytes
			otherSection["system_storage_checked_at"] = time.Now().UTC()
		}
	}

	return response, nil
}

func (s *AdminSettingsService) UpdateSection(
	ctx context.Context,
	section string,
	payload map[string]any,
	updatedBy *uuid.UUID,
) (map[string]any, error) {
	validated, err := validateSettingsSection(section, payload)
	if err != nil {
		return nil, err
	}
	item, err := s.repo.Upsert(ctx, section, validated, updatedBy)
	if err != nil {
		return nil, err
	}

	if section == "general" && s.quotaPolicy != nil {
		if value, ok := validated.(GeneralSettings); ok {
			if err := s.quotaPolicy.UpsertDefaultUserQuota(ctx, value.DefaultStorageQuotaBytes); err != nil {
				return nil, fmt.Errorf("sync default user quota policy: %w", err)
			}
		}
	}

	var decoded map[string]any
	if err := json.Unmarshal(item.ValueJSON, &decoded); err != nil {
		decoded = toMap(validated)
	}
	return decoded, nil
}

type AdminSettingsHandler struct {
	service *AdminSettingsService
}

func NewAdminSettingsHandler(service *AdminSettingsService) *AdminSettingsHandler {
	return &AdminSettingsHandler{service: service}
}

type GeneralSettings struct {
	SiteName                 string `json:"site_name"`
	SiteSubtitle             string `json:"site_subtitle"`
	BrowserTitle             string `json:"browser_title"`
	SiteLogoURL              string `json:"site_logo_url"`
	FaviconURL               string `json:"favicon_url"`
	BrandColor               string `json:"brand_color"`
	DefaultStorageQuotaBytes int64  `json:"default_storage_quota_bytes"`
	DefaultLanguage          string `json:"default_language"`
	DefaultTimezone          string `json:"default_timezone"`
	PublicBaseURL            string `json:"public_base_url"`
	MaintenanceMode          bool   `json:"maintenance_mode"`
}

type SharingSettings struct {
	PublicSharingEnabled        bool `json:"public_sharing_enabled"`
	AllowPasswordProtectedLinks bool `json:"allow_password_protected_links"`
	AllowExpiration             bool `json:"allow_expiration"`
	DefaultExpirationDays       int  `json:"default_expiration_days"`
	MaximumExpirationDays       int  `json:"maximum_expiration_days"`
	AllowPublicDownloads        bool `json:"allow_public_downloads"`
	AllowFolderSharing          bool `json:"allow_folder_sharing"`
	RequirePasswordForPublic    bool `json:"require_password_for_public_links"`
}

type SecuritySettings struct {
	MinimumPasswordLength    int    `json:"minimum_password_length"`
	RequireUppercase         bool   `json:"require_uppercase"`
	RequireLowercase         bool   `json:"require_lowercase"`
	RequireNumber            bool   `json:"require_number"`
	RequireSymbol            bool   `json:"require_symbol"`
	SessionLifetimeMinutes   int    `json:"session_lifetime_minutes"`
	RefreshTokenLifetimeMins int    `json:"refresh_token_lifetime_minutes"`
	TwoFactorRequired        bool   `json:"two_factor_required"`
	LoginRateLimitPerWindow  int    `json:"login_rate_limit_per_window"`
	LoginRateWindowSeconds   int    `json:"login_rate_window_seconds"`
	TrustedProxyNote         string `json:"trusted_proxy_note"`
}

type OtherSettings struct {
	StorageCleanupIntervalSeconds int    `json:"storage_cleanup_interval_seconds"`
	UploadMaxFileSizeBytes        int64  `json:"upload_max_file_size_bytes"`
	UploadMaxChunkSizeBytes       int    `json:"upload_max_chunk_size_bytes"`
	PreviewGenerationEnabled      bool   `json:"preview_generation_enabled"`
	LoggingLevel                  string `json:"logging_level"`
	RequiresRestartNote           string `json:"requires_restart_note"`
}

func (h *AdminSettingsHandler) GetSettings(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireAdminAuth(w, r); !ok {
		return
	}

	if h.service == nil {
		writeAPIError(w, http.StatusServiceUnavailable, "service_unavailable", "admin settings service is not configured")
		return
	}

	response, err := h.service.ListSections(r.Context())
	if err != nil {
		writeInternalOrSchemaError(w, err, "failed to list settings")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"settings": response})
}

func (h *AdminSettingsHandler) UpdateSection(w http.ResponseWriter, r *http.Request) {
	authValue, ok := requireAdminAuth(w, r)
	if !ok {
		return
	}
	section := strings.TrimSpace(strings.ToLower(chi.URLParam(r, "section")))
	if section == "" {
		writeAPIError(w, http.StatusBadRequest, "invalid_request", "section is required")
		return
	}

	var payload map[string]any
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid_request", "invalid request body")
		return
	}

	if h.service == nil {
		writeAPIError(w, http.StatusServiceUnavailable, "service_unavailable", "admin settings service is not configured")
		return
	}

	userID := authValue.UserID
	decoded, err := h.service.UpdateSection(r.Context(), section, payload, &userID)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unsupported settings section") ||
			strings.Contains(strings.ToLower(err.Error()), "must be") ||
			strings.Contains(strings.ToLower(err.Error()), "required") {
			writeAPIError(w, http.StatusBadRequest, "invalid_request", err.Error())
			return
		}
		writeInternalOrSchemaError(w, err, "failed to update settings")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"section": section,
		"value":   decoded,
	})
}

func validateSettingsSection(section string, payload map[string]any) (any, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("invalid payload")
	}

	switch section {
	case "general":
		var value GeneralSettings
		if err := json.Unmarshal(raw, &value); err != nil {
			return nil, fmt.Errorf("invalid general settings")
		}
		if strings.TrimSpace(value.SiteName) == "" {
			return nil, fmt.Errorf("site_name is required")
		}
		if strings.TrimSpace(value.SiteSubtitle) == "" {
			value.SiteSubtitle = "Private storage"
		}
		if strings.TrimSpace(value.BrowserTitle) == "" {
			value.BrowserTitle = value.SiteName
		}
		if value.DefaultStorageQuotaBytes <= 0 {
			return nil, fmt.Errorf("default_storage_quota_bytes must be greater than zero")
		}
		if err := validateOptionalHTTPURL("public_base_url", value.PublicBaseURL); err != nil {
			return nil, err
		}
		if err := validateOptionalHTTPURL("site_logo_url", value.SiteLogoURL); err != nil {
			return nil, err
		}
		if err := validateOptionalHTTPURL("favicon_url", value.FaviconURL); err != nil {
			return nil, err
		}
		if !hexColorPattern.MatchString(strings.TrimSpace(value.BrandColor)) {
			return nil, fmt.Errorf("brand_color must be a valid hex color")
		}
		return value, nil
	case "sharing":
		var value SharingSettings
		if err := json.Unmarshal(raw, &value); err != nil {
			return nil, fmt.Errorf("invalid sharing settings")
		}
		if value.DefaultExpirationDays < 0 {
			return nil, fmt.Errorf("default_expiration_days must be zero or positive")
		}
		if value.MaximumExpirationDays < 0 {
			return nil, fmt.Errorf("maximum_expiration_days must be zero or positive")
		}
		if value.MaximumExpirationDays > 0 && value.DefaultExpirationDays > value.MaximumExpirationDays {
			return nil, fmt.Errorf("default_expiration_days cannot exceed maximum_expiration_days")
		}
		return value, nil
	case "security":
		var value SecuritySettings
		if err := json.Unmarshal(raw, &value); err != nil {
			return nil, fmt.Errorf("invalid security settings")
		}
		if value.MinimumPasswordLength < 8 {
			return nil, fmt.Errorf("minimum_password_length must be at least 8")
		}
		if value.SessionLifetimeMinutes <= 0 {
			return nil, fmt.Errorf("session_lifetime_minutes must be greater than zero")
		}
		if value.RefreshTokenLifetimeMins <= value.SessionLifetimeMinutes {
			return nil, fmt.Errorf("refresh_token_lifetime_minutes must be greater than session_lifetime_minutes")
		}
		if value.LoginRateLimitPerWindow <= 0 || value.LoginRateWindowSeconds <= 0 {
			return nil, fmt.Errorf("login rate limit values must be greater than zero")
		}
		return value, nil
	case "other":
		var value OtherSettings
		if err := json.Unmarshal(raw, &value); err != nil {
			return nil, fmt.Errorf("invalid other settings")
		}
		if value.UploadMaxFileSizeBytes <= 0 {
			return nil, fmt.Errorf("upload_max_file_size_bytes must be greater than zero")
		}
		if value.UploadMaxChunkSizeBytes <= 0 {
			return nil, fmt.Errorf("upload_max_chunk_size_bytes must be greater than zero")
		}
		if int64(value.UploadMaxChunkSizeBytes) > value.UploadMaxFileSizeBytes {
			return nil, fmt.Errorf("upload_max_chunk_size_bytes cannot exceed upload_max_file_size_bytes")
		}
		if strings.TrimSpace(value.LoggingLevel) == "" {
			return nil, fmt.Errorf("logging_level is required")
		}
		return value, nil
	default:
		return nil, fmt.Errorf("unsupported settings section")
	}
}

var hexColorPattern = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

func validateOptionalHTTPURL(field string, raw string) error {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil
	}
	parsed, err := url.Parse(trimmed)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return fmt.Errorf("%s must be a valid URL", field)
	}
	switch strings.ToLower(parsed.Scheme) {
	case "http", "https":
		return nil
	default:
		return fmt.Errorf("%s must use http or https", field)
	}
}

func defaultAdminSettingsSections() map[string]any {
	return map[string]any{
		"general": toMap(GeneralSettings{
			SiteName:                 "BitroxCloud",
			SiteSubtitle:             "Private storage",
			BrowserTitle:             "BitroxCloud",
			SiteLogoURL:              "https://pb.dashboardicons.com/api/files/community_gallery/myyy4r7vdmreido/bitrocloud_ameyfihhth.png",
			FaviconURL:               "",
			BrandColor:               "#ff0000",
			DefaultStorageQuotaBytes: 21474836480,
			DefaultLanguage:          "en",
			DefaultTimezone:          "Europe/Istanbul",
			PublicBaseURL:            "",
			MaintenanceMode:          false,
		}),
		"sharing": toMap(SharingSettings{
			PublicSharingEnabled:        true,
			AllowPasswordProtectedLinks: true,
			AllowExpiration:             true,
			DefaultExpirationDays:       7,
			MaximumExpirationDays:       90,
			AllowPublicDownloads:        true,
			AllowFolderSharing:          false,
			RequirePasswordForPublic:    false,
		}),
		"security": toMap(SecuritySettings{
			MinimumPasswordLength:    12,
			RequireUppercase:         true,
			RequireLowercase:         true,
			RequireNumber:            true,
			RequireSymbol:            false,
			SessionLifetimeMinutes:   15,
			RefreshTokenLifetimeMins: 43200,
			TwoFactorRequired:        false,
			LoginRateLimitPerWindow:  10,
			LoginRateWindowSeconds:   60,
			TrustedProxyNote:         "Review APP_TRUSTED_PROXY_CIDRS and APP_COOKIE_SECURE settings carefully.",
		}),
		"other": toMap(OtherSettings{
			StorageCleanupIntervalSeconds: 60,
			UploadMaxFileSizeBytes:        107374182400,
			UploadMaxChunkSizeBytes:       16777216,
			PreviewGenerationEnabled:      false,
			LoggingLevel:                  "info",
			RequiresRestartNote:           "Some settings are environment-driven and require a service restart.",
		}),
	}
}

func toMap(value any) map[string]any {
	raw, err := json.Marshal(value)
	if err != nil {
		return map[string]any{}
	}
	out := map[string]any{}
	if err := json.Unmarshal(raw, &out); err != nil {
		return map[string]any{}
	}
	return out
}
