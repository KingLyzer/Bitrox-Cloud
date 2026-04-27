package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"cloud/backend/internal/domain/identity"
	"cloud/backend/internal/interfaces/http/authctx"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type stubAdminUserRepo struct {
	usersByID    map[uuid.UUID]identity.User
	usersByEmail map[string]uuid.UUID
}

func newStubAdminUserRepo(users ...identity.User) *stubAdminUserRepo {
	repo := &stubAdminUserRepo{
		usersByID:    map[uuid.UUID]identity.User{},
		usersByEmail: map[string]uuid.UUID{},
	}
	for _, user := range users {
		repo.usersByID[user.ID] = user
		repo.usersByEmail[identity.NormalizeEmail(user.Email)] = user.ID
	}
	return repo
}

func (s *stubAdminUserRepo) List(context.Context) ([]identity.User, error) {
	out := make([]identity.User, 0, len(s.usersByID))
	for _, user := range s.usersByID {
		out = append(out, user)
	}
	return out, nil
}

func (s *stubAdminUserRepo) GetByID(_ context.Context, id uuid.UUID) (identity.User, error) {
	user, ok := s.usersByID[id]
	if !ok {
		return identity.User{}, identity.ErrUserNotFound
	}
	return user, nil
}

func (s *stubAdminUserRepo) GetByEmail(_ context.Context, email string) (identity.User, error) {
	id, ok := s.usersByEmail[identity.NormalizeEmail(email)]
	if !ok {
		return identity.User{}, identity.ErrUserNotFound
	}
	return s.usersByID[id], nil
}

func (s *stubAdminUserRepo) Create(_ context.Context, input identity.CreateUserInput) (identity.User, error) {
	id := uuid.New()
	user := identity.User{
		ID:           id,
		Email:        identity.NormalizeEmail(input.Email),
		DisplayName:  input.DisplayName,
		Role:         input.Role,
		PasswordHash: input.PasswordHash,
		QuotaBytes:   input.QuotaBytes,
		IsActive:     input.IsActive,
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}
	s.usersByID[id] = user
	s.usersByEmail[user.Email] = id
	return user, nil
}

func (s *stubAdminUserRepo) Update(_ context.Context, input identity.UpdateUserInput) (identity.User, error) {
	user, ok := s.usersByID[input.ID]
	if !ok {
		return identity.User{}, identity.ErrUserNotFound
	}
	user.DisplayName = input.DisplayName
	user.Role = input.Role
	user.QuotaBytes = input.QuotaBytes
	user.IsActive = input.IsActive
	user.UpdatedAt = time.Now().UTC()
	s.usersByID[input.ID] = user
	return user, nil
}

func (s *stubAdminUserRepo) ReactivateByID(_ context.Context, input identity.UpdateUserInput, passwordHash *string) (identity.User, error) {
	user, ok := s.usersByID[input.ID]
	if !ok {
		return identity.User{}, identity.ErrUserNotFound
	}
	user.DisplayName = input.DisplayName
	user.Role = input.Role
	user.QuotaBytes = input.QuotaBytes
	user.IsActive = true
	user.DeletedAt = nil
	if passwordHash != nil {
		user.PasswordHash = *passwordHash
	}
	user.UpdatedAt = time.Now().UTC()
	s.usersByID[input.ID] = user
	return user, nil
}

func (s *stubAdminUserRepo) UpdatePasswordHash(_ context.Context, userID uuid.UUID, passwordHash string) (identity.User, error) {
	user, ok := s.usersByID[userID]
	if !ok {
		return identity.User{}, identity.ErrUserNotFound
	}
	user.PasswordHash = passwordHash
	user.UpdatedAt = time.Now().UTC()
	s.usersByID[userID] = user
	return user, nil
}

func (s *stubAdminUserRepo) SoftDeleteByID(_ context.Context, userID uuid.UUID) (bool, error) {
	user, ok := s.usersByID[userID]
	if !ok {
		return false, nil
	}
	if user.DeletedAt != nil {
		return false, nil
	}
	now := time.Now().UTC()
	user.IsActive = false
	user.DeletedAt = &now
	user.UpdatedAt = now
	s.usersByID[userID] = user
	return true, nil
}

func (s *stubAdminUserRepo) HardDeleteByID(_ context.Context, userID uuid.UUID) (bool, error) {
	user, ok := s.usersByID[userID]
	if !ok {
		return false, nil
	}
	if user.DeletedAt == nil {
		return false, nil
	}
	delete(s.usersByID, userID)
	delete(s.usersByEmail, identity.NormalizeEmail(user.Email))
	return true, nil
}

func (s *stubAdminUserRepo) CountActiveByRole(_ context.Context, role identity.Role) (int, error) {
	count := 0
	for _, user := range s.usersByID {
		if user.Role == role && user.IsActive && user.DeletedAt == nil {
			count++
		}
	}
	return count, nil
}

type stubAdminHasher struct{}

func (stubAdminHasher) Hash(password string) (string, error) {
	if password == "" {
		return "", errors.New("empty password")
	}
	return "hashed:" + password, nil
}

func withAdminAuth(req *http.Request) *http.Request {
	return req.WithContext(authctx.WithContext(req.Context(), authctx.Context{
		UserID: uuid.New(),
		Role:   identity.RoleOwner,
	}))
}

func TestAdminListUsersIncludesInactiveAndDeleted(t *testing.T) {
	now := time.Now().UTC()
	deletedAt := now.Add(-time.Hour)

	repo := newStubAdminUserRepo(
		identity.User{ID: uuid.New(), Email: "active@example.com", DisplayName: "Active", Role: identity.RoleUser, IsActive: true, CreatedAt: now, UpdatedAt: now},
		identity.User{ID: uuid.New(), Email: "inactive@example.com", DisplayName: "Inactive", Role: identity.RoleUser, IsActive: false, CreatedAt: now, UpdatedAt: now},
		identity.User{ID: uuid.New(), Email: "deleted@example.com", DisplayName: "Deleted", Role: identity.RoleUser, IsActive: false, DeletedAt: &deletedAt, CreatedAt: now, UpdatedAt: now},
	)
	h := NewAdminUsersHandler(nil, repo, stubAdminHasher{}, nil, nil, nil, nil)

	req := withAdminAuth(httptest.NewRequest(http.MethodGet, "/api/v1/admin/users?status=all", nil))
	rec := httptest.NewRecorder()
	h.ListUsers(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var payload struct {
		Users []map[string]any `json:"users"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(payload.Users) != 3 {
		t.Fatalf("expected 3 users, got %d", len(payload.Users))
	}
}

func TestAdminCreateUserConflictsForExistingInactiveEmail(t *testing.T) {
	now := time.Now().UTC()
	deletedAt := now.Add(-time.Hour)
	existing := identity.User{
		ID:          uuid.New(),
		Email:       "pasif@example.com",
		DisplayName: "Pasif Kullanici",
		Role:        identity.RoleUser,
		IsActive:    false,
		DeletedAt:   &deletedAt,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	repo := newStubAdminUserRepo(existing)
	h := NewAdminUsersHandler(nil, repo, stubAdminHasher{}, nil, nil, nil, nil)

	body := []byte(`{
		"email":"pasif@example.com",
		"display_name":"Yeni Ad",
		"role":"user",
		"password":"VeryStrong123!",
		"is_active":true
	}`)
	req := withAdminAuth(httptest.NewRequest(http.MethodPost, "/api/v1/admin/users", bytes.NewReader(body)))
	rec := httptest.NewRecorder()
	h.CreateUser(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", rec.Code)
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte(`"existing_inactive_user"`)) {
		t.Fatalf("expected existing_inactive_user code, got %s", rec.Body.String())
	}
}

func TestAdminCreateUserCanReactivateExistingInactiveEmail(t *testing.T) {
	now := time.Now().UTC()
	deletedAt := now.Add(-time.Hour)
	existing := identity.User{
		ID:          uuid.New(),
		Email:       "pasif@example.com",
		DisplayName: "Pasif Kullanici",
		Role:        identity.RoleUser,
		IsActive:    false,
		DeletedAt:   &deletedAt,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	repo := newStubAdminUserRepo(existing)
	h := NewAdminUsersHandler(nil, repo, stubAdminHasher{}, nil, nil, nil, nil)

	body := []byte(`{
		"email":"pasif@example.com",
		"display_name":"Aktif Kullanici",
		"role":"user",
		"password":"VeryStrong123!",
		"is_active":true,
		"reactivate_existing":true
	}`)
	req := withAdminAuth(httptest.NewRequest(http.MethodPost, "/api/v1/admin/users", bytes.NewReader(body)))
	rec := httptest.NewRecorder()
	h.CreateUser(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	reactivated, err := repo.GetByEmail(context.Background(), "pasif@example.com")
	if err != nil {
		t.Fatalf("expected user, got err: %v", err)
	}
	if !reactivated.IsActive || reactivated.DeletedAt != nil {
		t.Fatalf("expected user to be active and restored, got %+v", reactivated)
	}
}

func TestAdminDeleteUserPermanentRequiresSoftDeletedUser(t *testing.T) {
	now := time.Now().UTC()
	user := identity.User{
		ID:          uuid.New(),
		Email:       "active@example.com",
		DisplayName: "Active User",
		Role:        identity.RoleUser,
		IsActive:    true,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	repo := newStubAdminUserRepo(user)
	h := NewAdminUsersHandler(nil, repo, stubAdminHasher{}, nil, nil, nil, nil)

	req := withAdminAuth(httptest.NewRequest(http.MethodDelete, "/api/v1/admin/users/"+user.ID.String()+"?permanent=true", nil))
	req = withURLParam(req, "userID", user.ID.String())
	rec := httptest.NewRecorder()
	h.DeleteUser(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", rec.Code)
	}
}

func TestAdminDeleteUserPermanentDeletesSoftDeletedUser(t *testing.T) {
	now := time.Now().UTC()
	deletedAt := now.Add(-time.Hour)
	user := identity.User{
		ID:          uuid.New(),
		Email:       "deleted@example.com",
		DisplayName: "Deleted User",
		Role:        identity.RoleUser,
		IsActive:    false,
		DeletedAt:   &deletedAt,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	repo := newStubAdminUserRepo(user)
	h := NewAdminUsersHandler(nil, repo, stubAdminHasher{}, nil, nil, nil, nil)

	req := withAdminAuth(httptest.NewRequest(http.MethodDelete, "/api/v1/admin/users/"+user.ID.String()+"?permanent=true", nil))
	req = withURLParam(req, "userID", user.ID.String())
	rec := httptest.NewRecorder()
	h.DeleteUser(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rec.Code)
	}
	if _, err := repo.GetByID(context.Background(), user.ID); !errors.Is(err, identity.ErrUserNotFound) {
		t.Fatalf("expected user to be removed, got err=%v", err)
	}
}

func TestAdminUpdateUserPasswordResetsPasswordHash(t *testing.T) {
	now := time.Now().UTC()
	user := identity.User{
		ID:           uuid.New(),
		Email:        "member@example.com",
		DisplayName:  "Member",
		Role:         identity.RoleUser,
		PasswordHash: "hashed:old-password",
		IsActive:     true,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	repo := newStubAdminUserRepo(user)
	h := NewAdminUsersHandler(nil, repo, stubAdminHasher{}, nil, nil, nil, nil)

	body := []byte(`{"new_password":"BrandNewPassword123!"}`)
	req := withAdminAuth(httptest.NewRequest(http.MethodPost, "/api/v1/admin/users/"+user.ID.String()+"/password", bytes.NewReader(body)))
	req = withURLParam(req, "userID", user.ID.String())
	rec := httptest.NewRecorder()

	h.UpdateUserPassword(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	updated, err := repo.GetByID(context.Background(), user.ID)
	if err != nil {
		t.Fatalf("get updated user: %v", err)
	}
	if updated.PasswordHash != "hashed:BrandNewPassword123!" {
		t.Fatalf("expected updated password hash, got %q", updated.PasswordHash)
	}
}

func withURLParam(req *http.Request, key, value string) *http.Request {
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, &chi.Context{
		URLParams: chi.RouteParams{
			Keys:   []string{key},
			Values: []string{value},
		},
	})
	return req.WithContext(ctx)
}
