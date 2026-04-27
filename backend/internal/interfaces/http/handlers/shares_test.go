package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"cloud/backend/internal/domain/identity"
	"cloud/backend/internal/interfaces/http/authctx"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type fakeShareRepo struct {
	node    ShareNodeSummaryDTO
	nodeErr error
	shares  []ShareDTO
}

func (f *fakeShareRepo) Create(context.Context, ShareInputDTO) (ShareDTO, error) {
	return ShareDTO{}, nil
}
func (f *fakeShareRepo) ListByOwner(context.Context, uuid.UUID, *uuid.UUID) ([]ShareDTO, error) {
	return f.shares, nil
}
func (f *fakeShareRepo) GetByIDForOwner(context.Context, uuid.UUID, uuid.UUID) (ShareDTO, error) {
	return ShareDTO{}, nil
}
func (f *fakeShareRepo) UpdateForOwner(context.Context, uuid.UUID, uuid.UUID, *time.Time, *int, *bool, *string, bool) (ShareDTO, error) {
	return ShareDTO{}, nil
}
func (f *fakeShareRepo) RevokeForOwner(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	return false, nil
}
func (f *fakeShareRepo) GetPublicByTokenHash(context.Context, string) (PublicShareDTO, error) {
	return PublicShareDTO{}, nil
}
func (f *fakeShareRepo) IncrementDownloadCount(context.Context, uuid.UUID) (ShareDTO, error) {
	return ShareDTO{}, nil
}
func (f *fakeShareRepo) CreateAccessGrant(context.Context, uuid.UUID, string, time.Time) error {
	return nil
}
func (f *fakeShareRepo) ValidateAccessGrant(context.Context, uuid.UUID, string, time.Time) (bool, error) {
	return false, nil
}
func (f *fakeShareRepo) GetNodeSummaryForOwner(context.Context, uuid.UUID, uuid.UUID) (ShareNodeSummaryDTO, error) {
	if f.nodeErr != nil {
		return ShareNodeSummaryDTO{}, f.nodeErr
	}
	return f.node, nil
}

type fakeShareHasher struct{}

func (fakeShareHasher) Hash(password string) (string, error) { return "hash:" + password, nil }
func (fakeShareHasher) Verify(encodedHash string, password string) bool {
	return encodedHash == "hash:"+password
}

type staticSettingsReader struct {
	item AdminSettingRecord
}

func (s staticSettingsReader) Get(context.Context, string) (AdminSettingRecord, error) {
	return s.item, nil
}

type fakeDownloadService struct{}

func (fakeDownloadService) DownloadNode(context.Context, uuid.UUID, uuid.UUID) (ShareDownloadResult, error) {
	return ShareDownloadResult{}, nil
}

func TestCreateShareBlockedWhenPublicSharingDisabled(t *testing.T) {
	repo := &fakeShareRepo{
		node: ShareNodeSummaryDTO{
			ID:          uuid.New(),
			OwnerUserID: uuid.New(),
			Type:        "file",
			Name:        "doc.txt",
			SizeBytes:   12,
			UpdatedAt:   time.Now().UTC(),
		},
	}
	handler := NewSharesHandler(
		repo,
		fakeShareHasher{},
		staticSettingsReader{item: AdminSettingRecord{
			Key: "sharing",
			ValueJSON: []byte(`{
				"public_sharing_enabled": false,
				"allow_password_protected_links": true,
				"allow_expiration": true,
				"default_expiration_days": 7,
				"maximum_expiration_days": 90,
				"allow_public_downloads": true,
				"allow_folder_sharing": false,
				"require_password_for_public_links": false
			}`),
		}},
		fakeDownloadService{},
	)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/shares", strings.NewReader(`{"node_id":"`+repo.node.ID.String()+`"}`))
	req = req.WithContext(authctx.WithContext(req.Context(), authctx.Context{
		UserID: uuid.New(),
		Role:   identity.RoleUser,
	}))
	rec := httptest.NewRecorder()

	handler.CreateShare(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 when public sharing disabled, got %d", rec.Code)
	}
}

func TestValidateShareAccessRules(t *testing.T) {
	now := time.Now().UTC()

	active := ShareDTO{}
	if ok, _ := validateShareAccess(active); !ok {
		t.Fatalf("expected active share to be accessible")
	}

	expiredAt := now.Add(-time.Minute)
	expired := ShareDTO{ExpiresAt: &expiredAt}
	if ok, reason := validateShareAccess(expired); ok || reason != "share expired" {
		t.Fatalf("expected expired share denial, got ok=%v reason=%q", ok, reason)
	}

	revokedAt := now
	revoked := ShareDTO{RevokedAt: &revokedAt}
	if ok, reason := validateShareAccess(revoked); ok || reason != "share revoked" {
		t.Fatalf("expected revoked share denial, got ok=%v reason=%q", ok, reason)
	}

	maxDownloads := 2
	limited := ShareDTO{MaxDownloads: &maxDownloads, DownloadCount: 2}
	if ok, reason := validateShareAccess(limited); ok || reason != "maximum downloads reached" {
		t.Fatalf("expected max downloads denial, got ok=%v reason=%q", ok, reason)
	}
}

func TestListSharesNodeValidation(t *testing.T) {
	actorID := uuid.New()
	nodeID := uuid.New()

	repo := &fakeShareRepo{
		nodeErr: pgx.ErrNoRows,
	}
	handler := NewSharesHandler(repo, fakeShareHasher{}, staticSettingsReader{}, fakeDownloadService{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/shares?node_id="+nodeID.String(), nil)
	req = req.WithContext(authctx.WithContext(req.Context(), authctx.Context{
		UserID: actorID,
		Role:   identity.RoleUser,
	}))
	rec := httptest.NewRecorder()
	handler.ListShares(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for missing node, got %d", rec.Code)
	}

	repo.nodeErr = errors.New(`relation "shares" does not exist`)
	rec = httptest.NewRecorder()
	handler.ListShares(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 for missing schema, got %d", rec.Code)
	}
}

func TestListSharesIncludesNodeMetadata(t *testing.T) {
	actorID := uuid.New()
	nodeID := uuid.New()
	now := time.Now().UTC()

	repo := &fakeShareRepo{
		node: ShareNodeSummaryDTO{
			ID:          nodeID,
			OwnerUserID: actorID,
			Type:        "file",
			Name:        "invoice.pdf",
			SizeBytes:   3210,
			UpdatedAt:   now,
		},
		shares: []ShareDTO{
			{
				ID:          uuid.New(),
				NodeID:      nodeID,
				OwnerUserID: actorID,
				CreatedAt:   now,
				UpdatedAt:   now,
			},
		},
	}
	handler := NewSharesHandler(repo, fakeShareHasher{}, staticSettingsReader{}, fakeDownloadService{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/shares", nil)
	req = req.WithContext(authctx.WithContext(req.Context(), authctx.Context{
		UserID: actorID,
		Role:   identity.RoleUser,
	}))
	rec := httptest.NewRecorder()
	handler.ListShares(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var payload struct {
		Shares []map[string]any `json:"shares"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(payload.Shares) != 1 {
		t.Fatalf("expected one share, got %d", len(payload.Shares))
	}
	first := payload.Shares[0]
	if first["node_name"] != "invoice.pdf" {
		t.Fatalf("expected node_name invoice.pdf, got %#v", first["node_name"])
	}
	if first["node_type"] != "file" {
		t.Fatalf("expected node_type file, got %#v", first["node_type"])
	}
}
