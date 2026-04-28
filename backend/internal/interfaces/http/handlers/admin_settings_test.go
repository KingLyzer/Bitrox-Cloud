package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"cloud/backend/internal/domain/identity"
	"cloud/backend/internal/interfaces/http/authctx"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type fakeAdminSettingsRepo struct {
	listItems []AdminSettingRecord
	listErr   error
	upsertErr error
}

func (f fakeAdminSettingsRepo) List(context.Context) ([]AdminSettingRecord, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	return f.listItems, nil
}

func (f fakeAdminSettingsRepo) Get(context.Context, string) (AdminSettingRecord, error) {
	return AdminSettingRecord{}, nil
}

func (f fakeAdminSettingsRepo) Upsert(context.Context, string, any, *uuid.UUID) (AdminSettingRecord, error) {
	if f.upsertErr != nil {
		return AdminSettingRecord{}, f.upsertErr
	}
	return AdminSettingRecord{}, nil
}

type capturingAdminSettingsRepo struct {
	lastKey   string
	lastValue any
}

func newAdminSettingsTestHandler(repo AdminSettingsRepository) *AdminSettingsHandler {
	return NewAdminSettingsHandler(NewAdminSettingsService(repo, nil, "."))
}

func (c *capturingAdminSettingsRepo) List(context.Context) ([]AdminSettingRecord, error) {
	return nil, nil
}

func (c *capturingAdminSettingsRepo) Get(context.Context, string) (AdminSettingRecord, error) {
	return AdminSettingRecord{}, nil
}

func (c *capturingAdminSettingsRepo) Upsert(_ context.Context, key string, value any, _ *uuid.UUID) (AdminSettingRecord, error) {
	c.lastKey = key
	c.lastValue = value
	raw, _ := json.Marshal(value)
	return AdminSettingRecord{
		Key:       key,
		ValueJSON: raw,
	}, nil
}

func TestAdminSettingsGetRequiresAdminRole(t *testing.T) {
	h := newAdminSettingsTestHandler(fakeAdminSettingsRepo{})

	noAuthReq := httptest.NewRequest(http.MethodGet, "/api/v1/admin/settings", nil)
	noAuthRec := httptest.NewRecorder()
	h.GetSettings(noAuthRec, noAuthReq)
	if noAuthRec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for missing auth context, got %d", noAuthRec.Code)
	}

	userReq := httptest.NewRequest(http.MethodGet, "/api/v1/admin/settings", nil)
	userReq = userReq.WithContext(authctx.WithContext(userReq.Context(), authctx.Context{
		UserID: uuid.New(),
		Role:   identity.RoleUser,
	}))
	userRec := httptest.NewRecorder()
	h.GetSettings(userRec, userReq)
	if userRec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for non-admin role, got %d", userRec.Code)
	}
}

func TestAdminSettingsGetReturnsDecodedSettings(t *testing.T) {
	h := newAdminSettingsTestHandler(fakeAdminSettingsRepo{
		listItems: []AdminSettingRecord{
			{
				Key:       "general",
				ValueJSON: json.RawMessage(`{"site_name":"BitroxCloud"}`),
			},
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/settings", nil)
	req = req.WithContext(authctx.WithContext(req.Context(), authctx.Context{
		UserID: uuid.New(),
		Role:   identity.RoleAdmin,
	}))
	rec := httptest.NewRecorder()

	h.GetSettings(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var payload map[string]map[string]map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["settings"]["general"]["site_name"] != "BitroxCloud" {
		t.Fatalf("expected general.site_name to be BitroxCloud, got %#v", payload)
	}
}

func TestAdminSettingsUpdateRejectsUnsupportedSection(t *testing.T) {
	h := newAdminSettingsTestHandler(fakeAdminSettingsRepo{})

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/admin/settings/unknown", strings.NewReader(`{"a":1}`))
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("section", "unknown")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx))
	req = req.WithContext(authctx.WithContext(req.Context(), authctx.Context{
		UserID: uuid.New(),
		Role:   identity.RoleOwner,
	}))
	rec := httptest.NewRecorder()

	h.UpdateSection(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for unsupported section, got %d", rec.Code)
	}
}

func TestAdminSettingsGetReturnsSchemaNotReadyWhenTableMissing(t *testing.T) {
	h := newAdminSettingsTestHandler(fakeAdminSettingsRepo{
		listErr: errors.New(`relation "app_settings" does not exist`),
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/settings", nil)
	req = req.WithContext(authctx.WithContext(req.Context(), authctx.Context{
		UserID: uuid.New(),
		Role:   identity.RoleAdmin,
	}))
	rec := httptest.NewRecorder()

	h.GetSettings(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rec.Code)
	}
}

func TestAdminSettingsGetSkipsMalformedStoredSection(t *testing.T) {
	h := newAdminSettingsTestHandler(fakeAdminSettingsRepo{
		listItems: []AdminSettingRecord{
			{
				Key:       "general",
				ValueJSON: json.RawMessage(`{"site_name":"BitroxCloud"}`),
			},
			{
				Key:       "sharing",
				ValueJSON: json.RawMessage(`{bad-json`),
			},
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/settings", nil)
	req = req.WithContext(authctx.WithContext(req.Context(), authctx.Context{
		UserID: uuid.New(),
		Role:   identity.RoleAdmin,
	}))
	rec := httptest.NewRecorder()

	h.GetSettings(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var payload map[string]map[string]map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["settings"]["general"]["site_name"] != "BitroxCloud" {
		t.Fatalf("expected general section to survive malformed settings payload")
	}
}

func TestAdminSettingsUpdateGeneralPersistsBrandingFields(t *testing.T) {
	repo := &capturingAdminSettingsRepo{}
	h := newAdminSettingsTestHandler(repo)

	body := `{
		"site_name":"Bitrox Cloud",
		"site_logo_url":"https://cdn.example.com/logo.png",
		"favicon_url":"https://cdn.example.com/favicon.ico",
		"brand_color":"#2563eb",
		"default_storage_quota_bytes":21474836480,
		"default_language":"tr",
		"default_timezone":"Europe/Istanbul",
		"public_base_url":"https://cloud.example.com",
		"maintenance_mode":false
	}`

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/admin/settings/general", strings.NewReader(body))
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("section", "general")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx))
	req = req.WithContext(authctx.WithContext(req.Context(), authctx.Context{
		UserID: uuid.New(),
		Role:   identity.RoleOwner,
	}))
	rec := httptest.NewRecorder()

	h.UpdateSection(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if repo.lastKey != "general" {
		t.Fatalf("expected upsert section general, got %q", repo.lastKey)
	}

	value, ok := repo.lastValue.(GeneralSettings)
	if !ok {
		t.Fatalf("expected GeneralSettings payload, got %T", repo.lastValue)
	}
	if value.SiteName != "Bitrox Cloud" || value.SiteLogoURL == "" || value.FaviconURL == "" || value.BrandColor == "" {
		t.Fatalf("expected branding fields to persist, got %#v", value)
	}
}

func TestAdminSettingsUpdateGeneralRejectsUnsafeLogoURL(t *testing.T) {
	repo := &capturingAdminSettingsRepo{}
	h := newAdminSettingsTestHandler(repo)

	body := `{
		"site_name":"Bitrox Cloud",
		"site_logo_url":"javascript:alert(1)",
		"favicon_url":"https://cdn.example.com/favicon.ico",
		"brand_color":"#2563eb",
		"default_storage_quota_bytes":21474836480,
		"default_language":"tr",
		"default_timezone":"Europe/Istanbul",
		"public_base_url":"https://cloud.example.com",
		"maintenance_mode":false
	}`

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/admin/settings/general", strings.NewReader(body))
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("section", "general")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx))
	req = req.WithContext(authctx.WithContext(req.Context(), authctx.Context{
		UserID: uuid.New(),
		Role:   identity.RoleOwner,
	}))
	rec := httptest.NewRecorder()

	h.UpdateSection(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}
