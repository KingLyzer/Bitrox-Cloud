package auth

import (
	"context"
	"testing"
	"time"

	"cloud/backend/internal/domain/identity"

	"github.com/google/uuid"
)

type fakeAuthUserRepo struct {
	user identity.User
}

func (f *fakeAuthUserRepo) GetByEmail(_ context.Context, email string) (identity.User, error) {
	if identity.NormalizeEmail(email) != identity.NormalizeEmail(f.user.Email) {
		return identity.User{}, identity.ErrUserNotFound
	}
	return f.user, nil
}

func (f *fakeAuthUserRepo) GetByID(_ context.Context, id uuid.UUID) (identity.User, error) {
	if id != f.user.ID {
		return identity.User{}, identity.ErrUserNotFound
	}
	return f.user, nil
}

type fakeAuthSessionRepo struct{}

func (fakeAuthSessionRepo) Create(context.Context, identity.Session) error { return nil }
func (fakeAuthSessionRepo) GetByID(context.Context, uuid.UUID) (identity.Session, error) {
	return identity.Session{}, identity.ErrSessionNotFound
}
func (fakeAuthSessionRepo) GetByRefreshTokenHash(context.Context, string) (identity.Session, error) {
	return identity.Session{}, identity.ErrSessionNotFound
}
func (fakeAuthSessionRepo) Rotate(context.Context, uuid.UUID, identity.Session, time.Time) error {
	return nil
}
func (fakeAuthSessionRepo) RevokeByRefreshTokenHash(context.Context, string, time.Time, string) (bool, error) {
	return true, nil
}
func (fakeAuthSessionRepo) RevokeByID(context.Context, uuid.UUID, time.Time, string) (bool, error) {
	return true, nil
}
func (fakeAuthSessionRepo) RevokeFamily(context.Context, uuid.UUID, time.Time, string) error {
	return nil
}

type fakeAuthHasher struct{}

func (fakeAuthHasher) Hash(password string) (string, error) { return "hashed:" + password, nil }
func (fakeAuthHasher) Verify(encodedHash string, password string) bool {
	return encodedHash == "hashed:"+password
}

type fakeAuthTokenManager struct{}

func (fakeAuthTokenManager) IssueAccessToken(input AccessTokenInput) (string, time.Time, error) {
	return "access-token", input.Now.Add(15 * time.Minute), nil
}
func (fakeAuthTokenManager) VerifyAccessToken(string) (AccessTokenClaims, error) {
	return AccessTokenClaims{}, nil
}
func (fakeAuthTokenManager) GenerateRefreshToken() (string, string, error) {
	return "refresh-token", "refresh-hash", nil
}
func (fakeAuthTokenManager) HashOpaqueToken(rawToken string) string { return rawToken }

type lifecycleFixedClock struct {
	now time.Time
}

func (c lifecycleFixedClock) Now() time.Time { return c.now }

func TestLoginBlockedWhileInactiveAndAllowedAfterReactivation(t *testing.T) {
	now := time.Date(2026, time.April, 27, 10, 0, 0, 0, time.UTC)
	repo := &fakeAuthUserRepo{
		user: identity.User{
			ID:           uuid.New(),
			Email:        "inactive@example.com",
			DisplayName:  "Inactive User",
			Role:         identity.RoleUser,
			PasswordHash: "hashed:StrongPassword123!",
			IsActive:     false,
			CreatedAt:    now,
			UpdatedAt:    now,
		},
	}

	service := NewService(
		repo,
		fakeAuthSessionRepo{},
		fakeAuthTokenManager{},
		fakeAuthHasher{},
		lifecycleFixedClock{now: now},
		24*time.Hour,
		nil,
		nil,
	)

	_, err := service.Login(context.Background(), LoginInput{
		Email:    "inactive@example.com",
		Password: "StrongPassword123!",
		IP:       "127.0.0.1",
	})
	if err != ErrInactiveUser {
		t.Fatalf("expected ErrInactiveUser, got %v", err)
	}

	repo.user.IsActive = true
	result, err := service.Login(context.Background(), LoginInput{
		Email:    "inactive@example.com",
		Password: "StrongPassword123!",
		IP:       "127.0.0.1",
	})
	if err != nil {
		t.Fatalf("expected successful login after reactivation, got %v", err)
	}
	if result.User.ID != repo.user.ID {
		t.Fatalf("expected same user id, got %s", result.User.ID)
	}
}
