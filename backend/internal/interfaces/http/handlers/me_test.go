package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"cloud/backend/internal/domain/identity"
	"cloud/backend/internal/interfaces/http/authctx"

	"github.com/google/uuid"
)

type stubMeRepo struct {
	user identity.User
}

func (s *stubMeRepo) GetByID(_ context.Context, id uuid.UUID) (identity.User, error) {
	if s.user.ID != id {
		return identity.User{}, identity.ErrUserNotFound
	}
	return s.user, nil
}

func (s *stubMeRepo) UpdateProfile(_ context.Context, userID uuid.UUID, displayName string, preferredLanguage *string) (identity.User, error) {
	if s.user.ID != userID {
		return identity.User{}, identity.ErrUserNotFound
	}
	s.user.DisplayName = displayName
	s.user.PreferredLanguage = preferredLanguage
	s.user.UpdatedAt = time.Now().UTC()
	return s.user, nil
}

func TestMeUpdatePersistsDisplayNameAndLanguage(t *testing.T) {
	userID := uuid.New()
	repo := &stubMeRepo{
		user: identity.User{
			ID:          userID,
			Email:       "admin@example.com",
			DisplayName: "Admin",
			Role:        identity.RoleOwner,
			IsActive:    true,
			CreatedAt:   time.Now().UTC(),
			UpdatedAt:   time.Now().UTC(),
		},
	}
	h := NewMeHandler(repo)

	body := []byte(`{"display_name":"Esat Erhan","preferred_language":"tr"}`)
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/me", bytes.NewReader(body))
	req = req.WithContext(authctx.WithContext(req.Context(), authctx.Context{
		UserID: userID,
		Role:   identity.RoleOwner,
	}))
	rec := httptest.NewRecorder()

	h.UpdateMe(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var payload map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if payload["display_name"] != "Esat Erhan" {
		t.Fatalf("expected display_name updated, got %#v", payload["display_name"])
	}
	if payload["preferred_language"] != "tr" {
		t.Fatalf("expected preferred_language tr, got %#v", payload["preferred_language"])
	}
}
