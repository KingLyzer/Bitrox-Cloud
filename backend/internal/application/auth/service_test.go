package auth

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"testing"
	"time"

	"cloud/backend/internal/domain/identity"

	"github.com/google/uuid"
)

type fakeUsersRepo struct {
	byEmail map[string]identity.User
	byID    map[uuid.UUID]identity.User
}

func (f *fakeUsersRepo) GetByEmail(_ context.Context, email string) (identity.User, error) {
	user, ok := f.byEmail[email]
	if !ok {
		return identity.User{}, identity.ErrUserNotFound
	}
	return user, nil
}

func (f *fakeUsersRepo) GetByID(_ context.Context, id uuid.UUID) (identity.User, error) {
	user, ok := f.byID[id]
	if !ok {
		return identity.User{}, identity.ErrUserNotFound
	}
	return user, nil
}

type fakeSessionsRepo struct {
	byHash           map[string]identity.Session
	byID             map[uuid.UUID]identity.Session
	revokedFamilies  map[uuid.UUID]bool
	failRevokeFamily bool
	revokedByID      map[uuid.UUID]bool
}

func (f *fakeSessionsRepo) Create(_ context.Context, session identity.Session) error {
	if f.byHash == nil {
		f.byHash = map[string]identity.Session{}
	}
	if f.byID == nil {
		f.byID = map[uuid.UUID]identity.Session{}
	}
	f.byHash[session.RefreshTokenHash] = session
	f.byID[session.ID] = session
	return nil
}

func (f *fakeSessionsRepo) GetByRefreshTokenHash(_ context.Context, refreshTokenHash string) (identity.Session, error) {
	s, ok := f.byHash[refreshTokenHash]
	if !ok {
		return identity.Session{}, identity.ErrSessionNotFound
	}
	return s, nil
}

func (f *fakeSessionsRepo) GetByID(_ context.Context, sessionID uuid.UUID) (identity.Session, error) {
	s, ok := f.byID[sessionID]
	if !ok {
		return identity.Session{}, identity.ErrSessionNotFound
	}
	return s, nil
}

func (f *fakeSessionsRepo) Rotate(_ context.Context, oldSessionID uuid.UUID, newSession identity.Session, now time.Time) error {
	old, ok := f.byID[oldSessionID]
	if !ok {
		return identity.ErrSessionNotFound
	}
	if old.RevokedAt != nil {
		return identity.ErrSessionAlreadyRotated
	}
	old.RevokedAt = &now
	old.ReplacedBySessionID = &newSession.ID
	f.byID[oldSessionID] = old
	f.byHash[old.RefreshTokenHash] = old

	f.byID[newSession.ID] = newSession
	f.byHash[newSession.RefreshTokenHash] = newSession
	return nil
}

func (f *fakeSessionsRepo) RevokeByRefreshTokenHash(_ context.Context, refreshTokenHash string, now time.Time, reason string) (bool, error) {
	s, ok := f.byHash[refreshTokenHash]
	if !ok {
		return false, nil
	}
	s.RevokedAt = &now
	s.RevocationReason = &reason
	f.byHash[refreshTokenHash] = s
	f.byID[s.ID] = s
	return true, nil
}

func (f *fakeSessionsRepo) RevokeFamily(_ context.Context, familyID uuid.UUID, _ time.Time, _ string) error {
	if f.failRevokeFamily {
		return errors.New("forced revoke family failure")
	}
	if f.revokedFamilies == nil {
		f.revokedFamilies = map[uuid.UUID]bool{}
	}
	f.revokedFamilies[familyID] = true
	return nil
}

func (f *fakeSessionsRepo) RevokeByID(_ context.Context, sessionID uuid.UUID, _ time.Time, _ string) (bool, error) {
	if f.revokedByID == nil {
		f.revokedByID = map[uuid.UUID]bool{}
	}
	_, ok := f.byID[sessionID]
	if !ok {
		return false, nil
	}
	f.revokedByID[sessionID] = true
	return true, nil
}

type fakeHasher struct{}

func (fakeHasher) Hash(password string) (string, error) {
	return "hashed-" + password, nil
}

func (fakeHasher) Verify(encodedHash string, password string) bool {
	return encodedHash == "hashed-"+password
}

type fakeTokenManager struct {
	refreshSeed  int
	verifyClaims AccessTokenClaims
	verifyErr    error
}

func (f *fakeTokenManager) IssueAccessToken(input AccessTokenInput) (string, time.Time, error) {
	return "access-" + input.UserID.String(), input.Now.Add(15 * time.Minute), nil
}

func (f *fakeTokenManager) VerifyAccessToken(_ string) (AccessTokenClaims, error) {
	if f.verifyErr != nil {
		return AccessTokenClaims{}, f.verifyErr
	}
	return f.verifyClaims, nil
}

func (f *fakeTokenManager) GenerateRefreshToken() (string, string, error) {
	f.refreshSeed++
	raw := fmt.Sprintf("refresh-token-%d", f.refreshSeed)
	return raw, f.HashOpaqueToken(raw), nil
}

func (f *fakeTokenManager) HashOpaqueToken(rawToken string) string {
	return "hash-" + rawToken
}

type fixedClock struct {
	now time.Time
}

type fakeAuditRecorder struct {
	events []SecurityEvent
}

func (f *fakeAuditRecorder) RecordSecurityEvent(_ context.Context, event SecurityEvent) error {
	f.events = append(f.events, event)
	return nil
}

func (f fixedClock) Now() time.Time {
	return f.now
}

func TestLoginSuccess(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	user := identity.User{
		ID:           userID,
		Email:        "admin@example.com",
		Role:         identity.RoleAdmin,
		PasswordHash: "hashed-secret",
		IsActive:     true,
	}

	usersRepo := &fakeUsersRepo{
		byEmail: map[string]identity.User{"admin@example.com": user},
		byID:    map[uuid.UUID]identity.User{userID: user},
	}
	sessionsRepo := &fakeSessionsRepo{
		byHash: map[string]identity.Session{},
		byID:   map[uuid.UUID]identity.Session{},
	}
	tokenMgr := &fakeTokenManager{
		verifyClaims: AccessTokenClaims{
			UserID:    userID,
			SessionID: uuid.New(),
			Role:      identity.RoleUser,
			ExpiresAt: time.Now().UTC().Add(15 * time.Minute),
		},
	}
	svc := NewService(usersRepo, sessionsRepo, tokenMgr, fakeHasher{}, fixedClock{now: time.Now().UTC()}, 24*time.Hour, slog.Default(), &fakeAuditRecorder{})

	result, err := svc.Login(ctx, LoginInput{
		Email:    "admin@example.com",
		Password: "secret",
	})
	if err != nil {
		t.Fatalf("expected login success, got %v", err)
	}
	if result.AccessToken == "" || result.RefreshToken == "" {
		t.Fatalf("expected non-empty access and refresh token")
	}
}

func TestLoginInvalidPassword(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	user := identity.User{
		ID:           userID,
		Email:        "admin@example.com",
		Role:         identity.RoleAdmin,
		PasswordHash: "hashed-secret",
		IsActive:     true,
	}

	usersRepo := &fakeUsersRepo{
		byEmail: map[string]identity.User{"admin@example.com": user},
		byID:    map[uuid.UUID]identity.User{userID: user},
	}
	sessionsRepo := &fakeSessionsRepo{
		byHash: map[string]identity.Session{},
		byID:   map[uuid.UUID]identity.Session{},
	}
	tokenMgr := &fakeTokenManager{}
	svc := NewService(usersRepo, sessionsRepo, tokenMgr, fakeHasher{}, fixedClock{now: time.Now().UTC()}, 24*time.Hour, slog.Default(), &fakeAuditRecorder{})

	_, err := svc.Login(ctx, LoginInput{
		Email:    "admin@example.com",
		Password: "wrong",
	})
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("expected invalid credentials error, got %v", err)
	}
}

func TestRefreshReuseDetectionRevokesFamily(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC()
	userID := uuid.New()
	familyID := uuid.New()
	sessionID := uuid.New()
	oldRaw := "old-token"
	oldHash := "hash-" + oldRaw

	user := identity.User{
		ID:           userID,
		Email:        "admin@example.com",
		Role:         identity.RoleAdmin,
		PasswordHash: "hashed-secret",
		IsActive:     true,
	}

	oldSession := identity.Session{
		ID:               sessionID,
		UserID:           userID,
		FamilyID:         familyID,
		RefreshTokenHash: oldHash,
		ExpiresAt:        now.Add(24 * time.Hour),
	}

	usersRepo := &fakeUsersRepo{
		byEmail: map[string]identity.User{"admin@example.com": user},
		byID:    map[uuid.UUID]identity.User{userID: user},
	}
	sessionsRepo := &fakeSessionsRepo{
		byHash: map[string]identity.Session{oldHash: oldSession},
		byID:   map[uuid.UUID]identity.Session{sessionID: oldSession},
	}
	tokenMgr := &fakeTokenManager{}
	audit := &fakeAuditRecorder{}
	svc := NewService(usersRepo, sessionsRepo, tokenMgr, fakeHasher{}, fixedClock{now: now}, 24*time.Hour, slog.Default(), audit)

	_, err := svc.Refresh(ctx, RefreshInput{RefreshToken: oldRaw})
	if err != nil {
		t.Fatalf("expected first refresh success, got %v", err)
	}

	_, err = svc.Refresh(ctx, RefreshInput{RefreshToken: oldRaw})
	if !errors.Is(err, ErrInvalidSession) {
		t.Fatalf("expected invalid session on token reuse, got %v", err)
	}

	if !sessionsRepo.revokedFamilies[familyID] {
		t.Fatalf("expected token reuse to revoke session family")
	}
	if len(audit.events) == 0 {
		t.Fatalf("expected security audit event on replay")
	}
}

func TestRefreshReplayFailClosedWhenFamilyRevocationFails(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC()
	userID := uuid.New()
	familyID := uuid.New()
	sessionID := uuid.New()
	oldRaw := "old-token"
	oldHash := "hash-" + oldRaw
	revokedAt := now.Add(-1 * time.Minute)
	replacementID := uuid.New()

	user := identity.User{
		ID:           userID,
		Email:        "admin@example.com",
		Role:         identity.RoleAdmin,
		PasswordHash: "hashed-secret",
		IsActive:     true,
	}

	oldSession := identity.Session{
		ID:                  sessionID,
		UserID:              userID,
		FamilyID:            familyID,
		RefreshTokenHash:    oldHash,
		ExpiresAt:           now.Add(24 * time.Hour),
		RevokedAt:           &revokedAt,
		ReplacedBySessionID: &replacementID,
	}

	usersRepo := &fakeUsersRepo{
		byEmail: map[string]identity.User{"admin@example.com": user},
		byID:    map[uuid.UUID]identity.User{userID: user},
	}
	sessionsRepo := &fakeSessionsRepo{
		byHash:           map[string]identity.Session{oldHash: oldSession},
		byID:             map[uuid.UUID]identity.Session{sessionID: oldSession},
		failRevokeFamily: true,
		revokedFamilies:  map[uuid.UUID]bool{},
	}
	tokenMgr := &fakeTokenManager{
		verifyClaims: AccessTokenClaims{
			UserID:    userID,
			SessionID: sessionID,
			Role:      identity.RoleUser,
			ExpiresAt: now.Add(15 * time.Minute),
		},
	}
	audit := &fakeAuditRecorder{}
	svc := NewService(usersRepo, sessionsRepo, tokenMgr, fakeHasher{}, fixedClock{now: now}, 24*time.Hour, slog.Default(), audit)

	_, err := svc.Refresh(ctx, RefreshInput{RefreshToken: oldRaw})
	if !errors.Is(err, ErrInvalidSession) {
		t.Fatalf("expected invalid session on failed family revocation, got %v", err)
	}
	if len(audit.events) == 0 {
		t.Fatalf("expected security event to be audited")
	}
}

func TestAuthenticateAccessTokenRejectsRevokedSession(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC()
	userID := uuid.New()
	sessionID := uuid.New()
	revokedAt := now.Add(-1 * time.Minute)

	user := identity.User{
		ID:           userID,
		Email:        "user@example.com",
		Role:         identity.RoleUser,
		PasswordHash: "hashed-secret",
		IsActive:     true,
	}
	session := identity.Session{
		ID:               sessionID,
		UserID:           userID,
		FamilyID:         uuid.New(),
		RefreshTokenHash: "hash-refresh",
		ExpiresAt:        now.Add(time.Hour),
		RevokedAt:        &revokedAt,
	}

	usersRepo := &fakeUsersRepo{
		byEmail: map[string]identity.User{"user@example.com": user},
		byID:    map[uuid.UUID]identity.User{userID: user},
	}
	sessionsRepo := &fakeSessionsRepo{
		byHash: map[string]identity.Session{},
		byID:   map[uuid.UUID]identity.Session{sessionID: session},
	}
	tokenMgr := &fakeTokenManager{}
	audit := &fakeAuditRecorder{}
	svc := NewService(usersRepo, sessionsRepo, tokenMgr, fakeHasher{}, fixedClock{now: now}, 24*time.Hour, slog.Default(), audit)

	_, err := svc.AuthenticateAccessToken(ctx, "any-token")
	if !errors.Is(err, ErrInvalidSession) {
		t.Fatalf("expected invalid session error, got %v", err)
	}
}

func TestLogoutRevokesCurrentSession(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC()
	userID := uuid.New()
	sessionID := uuid.New()

	user := identity.User{
		ID:           userID,
		Email:        "user@example.com",
		Role:         identity.RoleUser,
		PasswordHash: "hashed-secret",
		IsActive:     true,
	}
	session := identity.Session{
		ID:               sessionID,
		UserID:           userID,
		FamilyID:         uuid.New(),
		RefreshTokenHash: "hash-refresh",
		ExpiresAt:        now.Add(time.Hour),
	}

	usersRepo := &fakeUsersRepo{
		byEmail: map[string]identity.User{"user@example.com": user},
		byID:    map[uuid.UUID]identity.User{userID: user},
	}
	sessionsRepo := &fakeSessionsRepo{
		byHash: map[string]identity.Session{"hash-refresh-token": session},
		byID:   map[uuid.UUID]identity.Session{sessionID: session},
	}
	tokenMgr := &fakeTokenManager{}
	audit := &fakeAuditRecorder{}
	svc := NewService(usersRepo, sessionsRepo, tokenMgr, fakeHasher{}, fixedClock{now: now}, 24*time.Hour, slog.Default(), audit)

	err := svc.Logout(ctx, LogoutInput{
		SessionID:    sessionID,
		RefreshToken: "refresh-token",
	})
	if err != nil {
		t.Fatalf("expected no logout error, got %v", err)
	}
	if !sessionsRepo.revokedByID[sessionID] {
		t.Fatalf("expected session id to be revoked")
	}
	if !sessionsRepo.revokedFamilies[session.FamilyID] {
		t.Fatalf("expected refresh token family to be revoked")
	}
}

func TestAuthenticateAccessTokenRejectsInactiveUser(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC()
	userID := uuid.New()
	sessionID := uuid.New()

	user := identity.User{
		ID:           userID,
		Email:        "user@example.com",
		Role:         identity.RoleUser,
		PasswordHash: "hashed-secret",
		IsActive:     false,
	}
	session := identity.Session{
		ID:               sessionID,
		UserID:           userID,
		FamilyID:         uuid.New(),
		RefreshTokenHash: "hash-refresh",
		ExpiresAt:        now.Add(time.Hour),
	}

	usersRepo := &fakeUsersRepo{
		byEmail: map[string]identity.User{"user@example.com": user},
		byID:    map[uuid.UUID]identity.User{userID: user},
	}
	sessionsRepo := &fakeSessionsRepo{
		byHash: map[string]identity.Session{},
		byID:   map[uuid.UUID]identity.Session{sessionID: session},
	}
	tokenMgr := &fakeTokenManager{
		verifyClaims: AccessTokenClaims{
			UserID:    userID,
			SessionID: sessionID,
			Role:      identity.RoleUser,
			ExpiresAt: now.Add(15 * time.Minute),
		},
	}
	svc := NewService(usersRepo, sessionsRepo, tokenMgr, fakeHasher{}, fixedClock{now: now}, 24*time.Hour, slog.Default(), &fakeAuditRecorder{})

	_, err := svc.AuthenticateAccessToken(ctx, "any-token")
	if !errors.Is(err, ErrInvalidSession) {
		t.Fatalf("expected invalid session for inactive user, got %v", err)
	}
}
