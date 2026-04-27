package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	appauth "cloud/backend/internal/application/auth"
	"cloud/backend/internal/domain/identity"
	"cloud/backend/internal/interfaces/http/authctx"

	"github.com/google/uuid"
)

type fakeAdminAuditReader struct {
	result AdminAuditQueryResult
	err    error
}

func (f fakeAdminAuditReader) ListSecurityEvents(context.Context, AdminAuditQuery) (AdminAuditQueryResult, error) {
	if f.err != nil {
		return AdminAuditQueryResult{}, f.err
	}
	return f.result, nil
}

func TestAdminUsersListAuditIncludesClientFields(t *testing.T) {
	userID := uuid.New()
	userEmail := "admin@example.com"
	sessionID := uuid.New()
	now := time.Date(2026, time.April, 24, 12, 30, 0, 0, time.UTC)

	h := NewAdminUsersHandler(nil, nil, nil, nil, nil, nil, fakeAdminAuditReader{
		result: AdminAuditQueryResult{
			Events: []AdminAuditEntry{
				{
					Event: appauth.SecurityEvent{
						EventType: "auth.login",
						Severity:  "info",
						UserID:    &userID,
						SessionID: &sessionID,
						IP:        "128.1.1.199:54321",
						UserAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) Chrome/124.0.0.0 Safari/537.36",
						CreatedAt: now,
						Metadata:  map[string]any{},
					},
					UserEmail: &userEmail,
				},
			},
			Total: 1,
			Page:  1,
			Limit: 20,
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/audit", nil)
	req = req.WithContext(authctx.WithContext(req.Context(), authctx.Context{
		UserID: uuid.New(),
		Role:   identity.RoleAdmin,
	}))
	rec := httptest.NewRecorder()

	h.ListAudit(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var payload struct {
		Events []map[string]any `json:"events"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(payload.Events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(payload.Events))
	}

	item := payload.Events[0]
	if got := item["ip"]; got != "128.1.1.199" {
		t.Fatalf("expected normalized ip 128.1.1.199, got %#v", got)
	}
	if got := item["device_type"]; got != "Desktop" {
		t.Fatalf("expected device_type Desktop, got %#v", got)
	}
	if got := item["browser"]; got != "Chrome" {
		t.Fatalf("expected browser Chrome, got %#v", got)
	}
}

func TestNormalizeAuditIP(t *testing.T) {
	if got := normalizeAuditIP("127.0.0.1:8080"); got != "127.0.0.1" {
		t.Fatalf("expected stripped host, got %q", got)
	}
	if got := normalizeAuditIP("[::1]:8080"); got != "::1" {
		t.Fatalf("expected ipv6 host, got %q", got)
	}
	if got := normalizeAuditIP(""); got != "unknown" {
		t.Fatalf("expected unknown for empty input, got %q", got)
	}
}
