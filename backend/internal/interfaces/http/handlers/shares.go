package handlers

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"cloud/backend/internal/interfaces/http/authctx"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type ShareRepository interface {
	Create(ctx context.Context, input ShareInputDTO) (ShareDTO, error)
	ListByOwner(ctx context.Context, ownerUserID uuid.UUID, nodeID *uuid.UUID) ([]ShareDTO, error)
	GetByIDForOwner(ctx context.Context, ownerUserID, shareID uuid.UUID) (ShareDTO, error)
	UpdateForOwner(ctx context.Context, ownerUserID, shareID uuid.UUID, expiresAt *time.Time, maxDownloads *int, allowDownload *bool, passwordHash *string, clearPassword bool) (ShareDTO, error)
	RevokeForOwner(ctx context.Context, ownerUserID, shareID uuid.UUID) (bool, error)
	GetPublicByTokenHash(ctx context.Context, tokenHash string) (PublicShareDTO, error)
	IncrementDownloadCount(ctx context.Context, shareID uuid.UUID) (ShareDTO, error)
	CreateAccessGrant(ctx context.Context, shareID uuid.UUID, grantHash string, expiresAt time.Time) error
	ValidateAccessGrant(ctx context.Context, shareID uuid.UUID, grantHash string, now time.Time) (bool, error)
	GetNodeSummaryForOwner(ctx context.Context, ownerUserID, nodeID uuid.UUID) (ShareNodeSummaryDTO, error)
}

type ShareInputDTO struct {
	NodeID        uuid.UUID
	OwnerUserID   uuid.UUID
	TokenHash     string
	PasswordHash  *string
	ExpiresAt     *time.Time
	MaxDownloads  *int
	AllowDownload bool
	AllowPreview  bool
}

type ShareDTO struct {
	ID            uuid.UUID
	NodeID        uuid.UUID
	OwnerUserID   uuid.UUID
	TokenHash     string
	PasswordHash  *string
	ExpiresAt     *time.Time
	MaxDownloads  *int
	DownloadCount int
	AllowDownload bool
	AllowPreview  bool
	CreatedAt     time.Time
	UpdatedAt     time.Time
	RevokedAt     *time.Time
}

type PublicShareDTO struct {
	Share ShareDTO
	Node  ShareNodeSummaryDTO
}

type ShareNodeSummaryDTO struct {
	ID          uuid.UUID
	OwnerUserID uuid.UUID
	Type        string
	Name        string
	SizeBytes   int64
	MIMEType    *string
	UpdatedAt   time.Time
	DeletedAt   *time.Time
}

type SharePasswordHasher interface {
	Hash(password string) (string, error)
	Verify(encodedHash string, password string) bool
}

type ShareSettingsReader interface {
	Get(ctx context.Context, key string) (AdminSettingRecord, error)
}

type ShareDownloadService interface {
	DownloadNode(ctx context.Context, ownerUserID uuid.UUID, nodeID uuid.UUID) (ShareDownloadResult, error)
}

type ShareDownloadResult struct {
	FileName  string
	SizeBytes int64
	MIMEType  string
	Reader    io.ReadCloser
}

type SharesHandler struct {
	repo          ShareRepository
	hasher        SharePasswordHasher
	settings      ShareSettingsReader
	downloads     ShareDownloadService
	shareBasePath string
}

func NewSharesHandler(repo ShareRepository, hasher SharePasswordHasher, settings ShareSettingsReader, downloads ShareDownloadService) *SharesHandler {
	return &SharesHandler{
		repo:      repo,
		hasher:    hasher,
		settings:  settings,
		downloads: downloads,
	}
}

type createShareRequest struct {
	NodeID        string  `json:"node_id"`
	Password      string  `json:"password"`
	ExpiresAt     *string `json:"expires_at"`
	AllowDownload *bool   `json:"allow_download"`
	MaxDownloads  *int    `json:"max_downloads"`
}

type updateShareRequest struct {
	Password      *string `json:"password"`
	ClearPassword bool    `json:"clear_password"`
	ExpiresAt     *string `json:"expires_at"`
	AllowDownload *bool   `json:"allow_download"`
	MaxDownloads  *int    `json:"max_downloads"`
}

type unlockShareRequest struct {
	Password string `json:"password"`
}

func (h *SharesHandler) ListShares(w http.ResponseWriter, r *http.Request) {
	authValue, ok := authctx.FromContext(r.Context())
	if !ok {
		writeAPIError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}

	var nodeID *uuid.UUID
	if rawNodeID := strings.TrimSpace(r.URL.Query().Get("node_id")); rawNodeID != "" {
		parsed, err := uuid.Parse(rawNodeID)
		if err != nil {
			writeAPIError(w, http.StatusBadRequest, "invalid_request", "invalid node_id")
			return
		}
		nodeID = &parsed

		if _, err := h.repo.GetNodeSummaryForOwner(r.Context(), authValue.UserID, parsed); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				writeAPIError(w, http.StatusNotFound, "node_not_found", "node not found")
				return
			}
			writeInternalOrSchemaError(w, err, "failed to validate node")
			return
		}
	}

	items, err := h.repo.ListByOwner(r.Context(), authValue.UserID, nodeID)
	if err != nil {
		writeInternalOrSchemaError(w, err, "failed to list shares")
		return
	}

	nodeSummaryByID := make(map[uuid.UUID]ShareNodeSummaryDTO, len(items))
	for _, item := range items {
		if _, exists := nodeSummaryByID[item.NodeID]; exists {
			continue
		}
		summary, summaryErr := h.repo.GetNodeSummaryForOwner(r.Context(), authValue.UserID, item.NodeID)
		if summaryErr != nil {
			if errors.Is(summaryErr, pgx.ErrNoRows) {
				continue
			}
			writeInternalOrSchemaError(w, summaryErr, "failed to list share node metadata")
			return
		}
		nodeSummaryByID[item.NodeID] = summary
	}

	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		var summary *ShareNodeSummaryDTO
		if loaded, exists := nodeSummaryByID[item.NodeID]; exists {
			summary = &loaded
		}
		out = append(out, shareToResponse(item, "", summary))
	}
	writeJSON(w, http.StatusOK, map[string]any{"shares": out})
}

func (h *SharesHandler) CreateShare(w http.ResponseWriter, r *http.Request) {
	authValue, ok := authctx.FromContext(r.Context())
	if !ok {
		writeAPIError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}

	var req createShareRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid_request", "invalid request body")
		return
	}

	nodeID, err := uuid.Parse(strings.TrimSpace(req.NodeID))
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid_request", "invalid node_id")
		return
	}

	node, err := h.repo.GetNodeSummaryForOwner(r.Context(), authValue.UserID, nodeID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeAPIError(w, http.StatusNotFound, "node_not_found", "node not found")
			return
		}
		writeInternalOrSchemaError(w, err, "failed to load node")
		return
	}
	if node.DeletedAt != nil {
		writeAPIError(w, http.StatusNotFound, "node_not_found", "node not found")
		return
	}

	sharingSettings, err := h.readSharingSettings(r.Context())
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "internal_error", "failed to load sharing settings")
		return
	}
	if !sharingSettings.PublicSharingEnabled {
		writeAPIError(w, http.StatusForbidden, "sharing_disabled", "public sharing is disabled by admin")
		return
	}
	if node.Type == "folder" && !sharingSettings.AllowFolderSharing {
		writeAPIError(w, http.StatusBadRequest, "invalid_request", "folder sharing is not enabled")
		return
	}

	password := strings.TrimSpace(req.Password)
	if sharingSettings.RequirePasswordForPublic && password == "" {
		writeAPIError(w, http.StatusBadRequest, "password_required", "password is required by sharing policy")
		return
	}
	if password != "" && !sharingSettings.AllowPasswordProtectedLinks {
		writeAPIError(w, http.StatusBadRequest, "invalid_request", "password protected shares are disabled")
		return
	}

	var expiresAt *time.Time
	if req.ExpiresAt != nil {
		value, err := time.Parse(time.RFC3339, strings.TrimSpace(*req.ExpiresAt))
		if err != nil {
			writeAPIError(w, http.StatusBadRequest, "invalid_request", "invalid expires_at")
			return
		}
		value = value.UTC()
		expiresAt = &value
	}
	if expiresAt == nil && sharingSettings.AllowExpiration && sharingSettings.DefaultExpirationDays > 0 {
		value := time.Now().UTC().Add(time.Duration(sharingSettings.DefaultExpirationDays) * 24 * time.Hour)
		expiresAt = &value
	}
	if expiresAt != nil && !sharingSettings.AllowExpiration {
		writeAPIError(w, http.StatusBadRequest, "invalid_request", "expiration is disabled by admin")
		return
	}
	if expiresAt != nil && sharingSettings.MaximumExpirationDays > 0 {
		maxAllowed := time.Now().UTC().Add(time.Duration(sharingSettings.MaximumExpirationDays) * 24 * time.Hour)
		if expiresAt.After(maxAllowed) {
			writeAPIError(w, http.StatusBadRequest, "invalid_request", "expires_at exceeds maximum expiration")
			return
		}
	}

	allowDownload := sharingSettings.AllowPublicDownloads
	if req.AllowDownload != nil {
		allowDownload = *req.AllowDownload
	}
	if !sharingSettings.AllowPublicDownloads {
		allowDownload = false
	}

	if req.MaxDownloads != nil && *req.MaxDownloads <= 0 {
		writeAPIError(w, http.StatusBadRequest, "invalid_request", "max_downloads must be greater than zero")
		return
	}

	rawToken, tokenHash, err := generateShareTokenPair()
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "internal_error", "failed to generate share token")
		return
	}

	var passwordHash *string
	if password != "" {
		hash, err := h.hasher.Hash(password)
		if err != nil {
			writeAPIError(w, http.StatusInternalServerError, "internal_error", "failed to protect share password")
			return
		}
		passwordHash = &hash
	}

	share, err := h.repo.Create(r.Context(), ShareInputDTO{
		NodeID:        nodeID,
		OwnerUserID:   authValue.UserID,
		TokenHash:     tokenHash,
		PasswordHash:  passwordHash,
		ExpiresAt:     expiresAt,
		MaxDownloads:  req.MaxDownloads,
		AllowDownload: allowDownload,
		AllowPreview:  true,
	})
	if err != nil {
		writeInternalOrSchemaError(w, err, "failed to create share")
		return
	}

	publicURL := buildPublicShareURL(r, rawToken)
	response := shareToResponse(share, publicURL, &node)
	response["token"] = rawToken
	writeJSON(w, http.StatusCreated, map[string]any{"share": response})
}

func (h *SharesHandler) UpdateShare(w http.ResponseWriter, r *http.Request) {
	authValue, ok := authctx.FromContext(r.Context())
	if !ok {
		writeAPIError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}
	shareID, err := parseURLParamUUID(r, "shareID")
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid_request", "invalid share id")
		return
	}

	var req updateShareRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid_request", "invalid request body")
		return
	}

	var passwordHash *string
	if req.Password != nil {
		password := strings.TrimSpace(*req.Password)
		if password != "" {
			hash, err := h.hasher.Hash(password)
			if err != nil {
				writeAPIError(w, http.StatusInternalServerError, "internal_error", "failed to hash share password")
				return
			}
			passwordHash = &hash
		}
	}

	var expiresAt *time.Time
	if req.ExpiresAt != nil {
		trimmed := strings.TrimSpace(*req.ExpiresAt)
		if trimmed == "" {
			expiresAt = nil
		} else {
			parsed, err := time.Parse(time.RFC3339, trimmed)
			if err != nil {
				writeAPIError(w, http.StatusBadRequest, "invalid_request", "invalid expires_at")
				return
			}
			parsed = parsed.UTC()
			expiresAt = &parsed
		}
	}

	share, err := h.repo.UpdateForOwner(r.Context(), authValue.UserID, shareID, expiresAt, req.MaxDownloads, req.AllowDownload, passwordHash, req.ClearPassword)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeAPIError(w, http.StatusNotFound, "share_not_found", "share not found")
			return
		}
		writeInternalOrSchemaError(w, err, "failed to update share")
		return
	}

	node, nodeErr := h.repo.GetNodeSummaryForOwner(r.Context(), authValue.UserID, share.NodeID)
	if nodeErr != nil && !errors.Is(nodeErr, pgx.ErrNoRows) {
		writeInternalOrSchemaError(w, nodeErr, "failed to load node metadata")
		return
	}

	var nodeSummary *ShareNodeSummaryDTO
	if nodeErr == nil {
		nodeSummary = &node
	}

	writeJSON(w, http.StatusOK, map[string]any{"share": shareToResponse(share, "", nodeSummary)})
}

func (h *SharesHandler) DeleteShare(w http.ResponseWriter, r *http.Request) {
	authValue, ok := authctx.FromContext(r.Context())
	if !ok {
		writeAPIError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}
	shareID, err := parseURLParamUUID(r, "shareID")
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid_request", "invalid share id")
		return
	}

	deleted, err := h.repo.RevokeForOwner(r.Context(), authValue.UserID, shareID)
	if err != nil {
		writeInternalOrSchemaError(w, err, "failed to revoke share")
		return
	}
	if !deleted {
		writeAPIError(w, http.StatusNotFound, "share_not_found", "share not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *SharesHandler) PublicLanding(w http.ResponseWriter, r *http.Request) {
	token := strings.TrimSpace(chi.URLParam(r, "token"))
	if token == "" {
		http.NotFound(w, r)
		return
	}
	http.Redirect(w, r, "/public/share/"+token, http.StatusFound)
}

func (h *SharesHandler) PublicGetShare(w http.ResponseWriter, r *http.Request) {
	token := strings.TrimSpace(chi.URLParam(r, "token"))
	shareData, err := h.loadPublicShareByToken(r.Context(), token)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeAPIError(w, http.StatusNotFound, "share_not_found", "share not found")
			return
		}
		writeInternalOrSchemaError(w, err, "failed to load share")
		return
	}

	if ok, reason := validateShareAccess(shareData.Share); !ok {
		writeAPIError(w, http.StatusGone, "share_unavailable", reason)
		return
	}

	grant := readShareGrant(r)
	locked := shareData.Share.PasswordHash != nil && !h.validateGrantForShare(r.Context(), shareData.Share.ID, grant)
	if locked {
		writeJSON(w, http.StatusUnauthorized, map[string]any{
			"error": map[string]any{
				"code":    "share_locked",
				"message": "password required",
			},
			"needs_password": true,
			"share": map[string]any{
				"node_name":      shareData.Node.Name,
				"node_type":      shareData.Node.Type,
				"size_bytes":     shareData.Node.SizeBytes,
				"allow_download": shareData.Share.AllowDownload,
			},
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"share": map[string]any{
			"node_id":        shareData.Node.ID.String(),
			"node_name":      shareData.Node.Name,
			"node_type":      shareData.Node.Type,
			"size_bytes":     shareData.Node.SizeBytes,
			"mime_type":      shareData.Node.MIMEType,
			"updated_at":     shareData.Node.UpdatedAt,
			"allow_download": shareData.Share.AllowDownload,
			"expires_at":     shareData.Share.ExpiresAt,
			"download_count": shareData.Share.DownloadCount,
			"max_downloads":  shareData.Share.MaxDownloads,
			"needs_password": false,
		},
	})
}

func (h *SharesHandler) PublicUnlockShare(w http.ResponseWriter, r *http.Request) {
	token := strings.TrimSpace(chi.URLParam(r, "token"))
	shareData, err := h.loadPublicShareByToken(r.Context(), token)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeAPIError(w, http.StatusNotFound, "share_not_found", "share not found")
			return
		}
		writeInternalOrSchemaError(w, err, "failed to load share")
		return
	}

	if ok, reason := validateShareAccess(shareData.Share); !ok {
		writeAPIError(w, http.StatusGone, "share_unavailable", reason)
		return
	}

	if shareData.Share.PasswordHash == nil {
		writeJSON(w, http.StatusOK, map[string]any{"unlocked": true})
		return
	}

	var req unlockShareRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid_request", "invalid request body")
		return
	}
	if !h.hasher.Verify(*shareData.Share.PasswordHash, strings.TrimSpace(req.Password)) {
		writeAPIError(w, http.StatusUnauthorized, "invalid_password", "invalid password")
		return
	}

	rawGrant, hashGrant, err := generateShareGrantPair()
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "internal_error", "failed to generate access grant")
		return
	}
	expiresAt := time.Now().UTC().Add(24 * time.Hour)
	if shareData.Share.ExpiresAt != nil && shareData.Share.ExpiresAt.Before(expiresAt) {
		expiresAt = *shareData.Share.ExpiresAt
	}
	if err := h.repo.CreateAccessGrant(r.Context(), shareData.Share.ID, hashGrant, expiresAt); err != nil {
		writeInternalOrSchemaError(w, err, "failed to create access grant")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"unlocked":     true,
		"access_grant": rawGrant,
		"expires_at":   expiresAt,
	})
}

func (h *SharesHandler) PublicDownloadShare(w http.ResponseWriter, r *http.Request) {
	token := strings.TrimSpace(chi.URLParam(r, "token"))
	shareData, err := h.loadPublicShareByToken(r.Context(), token)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeAPIError(w, http.StatusNotFound, "share_not_found", "share not found")
			return
		}
		writeInternalOrSchemaError(w, err, "failed to load share")
		return
	}
	if ok, reason := validateShareAccess(shareData.Share); !ok {
		writeAPIError(w, http.StatusGone, "share_unavailable", reason)
		return
	}
	if !shareData.Share.AllowDownload {
		writeAPIError(w, http.StatusForbidden, "download_disabled", "download is disabled for this share")
		return
	}
	if shareData.Node.Type != "file" {
		writeAPIError(w, http.StatusBadRequest, "invalid_share", "only file download is supported")
		return
	}

	if shareData.Share.PasswordHash != nil {
		grant := readShareGrant(r)
		if !h.validateGrantForShare(r.Context(), shareData.Share.ID, grant) {
			writeAPIError(w, http.StatusUnauthorized, "share_locked", "password required")
			return
		}
	}

	result, err := h.downloads.DownloadNode(r.Context(), shareData.Share.OwnerUserID, shareData.Share.NodeID)
	if err != nil {
		writeInternalOrSchemaError(w, err, "failed to stream shared file")
		return
	}
	defer result.Reader.Close()

	updated, err := h.repo.IncrementDownloadCount(r.Context(), shareData.Share.ID)
	if err == nil {
		shareData.Share = updated
	}

	w.Header().Set("Content-Type", result.MIMEType)
	w.Header().Set("Content-Length", strconv.FormatInt(result.SizeBytes, 10))
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", result.FileName))
	_, _ = io.Copy(w, result.Reader)
}

func (h *SharesHandler) loadPublicShareByToken(ctx context.Context, token string) (PublicShareDTO, error) {
	tokenHash := hashShareSecret(token)
	return h.repo.GetPublicByTokenHash(ctx, tokenHash)
}

func (h *SharesHandler) validateGrantForShare(ctx context.Context, shareID uuid.UUID, rawGrant string) bool {
	grant := strings.TrimSpace(rawGrant)
	if grant == "" {
		return false
	}
	ok, err := h.repo.ValidateAccessGrant(ctx, shareID, hashShareSecret(grant), time.Now().UTC())
	if err != nil {
		return false
	}
	return ok
}

func (h *SharesHandler) readSharingSettings(ctx context.Context) (SharingSettings, error) {
	defaultValue := SharingSettings{
		PublicSharingEnabled:        true,
		AllowPasswordProtectedLinks: true,
		AllowExpiration:             true,
		DefaultExpirationDays:       7,
		MaximumExpirationDays:       90,
		AllowPublicDownloads:        true,
		AllowFolderSharing:          false,
		RequirePasswordForPublic:    false,
	}
	if h.settings == nil {
		return defaultValue, nil
	}

	item, err := h.settings.Get(ctx, "sharing")
	if err != nil {
		return defaultValue, nil
	}
	if len(item.ValueJSON) == 0 {
		return defaultValue, nil
	}
	if err := json.Unmarshal(item.ValueJSON, &defaultValue); err != nil {
		return SharingSettings{}, err
	}
	return defaultValue, nil
}

func validateShareAccess(item ShareDTO) (bool, string) {
	now := time.Now().UTC()
	if item.RevokedAt != nil {
		return false, "share revoked"
	}
	if item.ExpiresAt != nil && !item.ExpiresAt.After(now) {
		return false, "share expired"
	}
	if item.MaxDownloads != nil && item.DownloadCount >= *item.MaxDownloads {
		return false, "maximum downloads reached"
	}
	return true, ""
}

func buildPublicShareURL(r *http.Request, token string) string {
	scheme := "http"
	if strings.EqualFold(strings.TrimSpace(r.Header.Get("X-Forwarded-Proto")), "https") || r.TLS != nil {
		scheme = "https"
	}
	host := strings.TrimSpace(r.Header.Get("X-Forwarded-Host"))
	if host == "" {
		host = r.Host
	}
	return fmt.Sprintf("%s://%s/s/%s", scheme, host, token)
}

func generateShareTokenPair() (string, string, error) {
	raw, err := randomBase64Token(32)
	if err != nil {
		return "", "", err
	}
	return raw, hashShareSecret(raw), nil
}

func generateShareGrantPair() (string, string, error) {
	raw, err := randomBase64Token(32)
	if err != nil {
		return "", "", err
	}
	return raw, hashShareSecret(raw), nil
}

func randomBase64Token(length int) (string, error) {
	buf := make([]byte, length)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func hashShareSecret(secret string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(secret)))
	return hex.EncodeToString(sum[:])
}

func readShareGrant(r *http.Request) string {
	if raw := strings.TrimSpace(r.URL.Query().Get("grant")); raw != "" {
		return raw
	}
	return strings.TrimSpace(r.Header.Get("X-Share-Grant"))
}

func shareToResponse(item ShareDTO, publicURL string, node *ShareNodeSummaryDTO) map[string]any {
	response := map[string]any{
		"id":             item.ID.String(),
		"node_id":        item.NodeID.String(),
		"owner_user_id":  item.OwnerUserID.String(),
		"expires_at":     item.ExpiresAt,
		"max_downloads":  item.MaxDownloads,
		"download_count": item.DownloadCount,
		"allow_download": item.AllowDownload,
		"allow_preview":  item.AllowPreview,
		"created_at":     item.CreatedAt,
		"updated_at":     item.UpdatedAt,
		"revoked_at":     item.RevokedAt,
		"needs_password": item.PasswordHash != nil,
	}
	if publicURL != "" {
		response["public_url"] = publicURL
	}
	if node != nil {
		response["node_name"] = node.Name
		response["node_type"] = node.Type
		response["size"] = node.SizeBytes
		response["size_bytes"] = node.SizeBytes
		response["mime_type"] = node.MIMEType
		response["node_updated_at"] = node.UpdatedAt
		response["node_deleted"] = node.DeletedAt != nil
	}
	return response
}
