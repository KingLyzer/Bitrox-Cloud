package auth

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"cloud/backend/internal/domain/identity"

	"github.com/google/uuid"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrInvalidSession     = errors.New("invalid session")
	ErrInactiveUser       = errors.New("inactive user")
)

type UserRepository interface {
	GetByEmail(ctx context.Context, email string) (identity.User, error)
	GetByID(ctx context.Context, id uuid.UUID) (identity.User, error)
}

type SessionRepository interface {
	Create(ctx context.Context, session identity.Session) error
	GetByID(ctx context.Context, id uuid.UUID) (identity.Session, error)
	GetByRefreshTokenHash(ctx context.Context, refreshTokenHash string) (identity.Session, error)
	Rotate(ctx context.Context, oldSessionID uuid.UUID, newSession identity.Session, now time.Time) error
	RevokeByRefreshTokenHash(ctx context.Context, refreshTokenHash string, now time.Time, reason string) (bool, error)
	RevokeByID(ctx context.Context, sessionID uuid.UUID, now time.Time, reason string) (bool, error)
	RevokeFamily(ctx context.Context, familyID uuid.UUID, now time.Time, reason string) error
}

type PasswordHasher interface {
	Hash(password string) (string, error)
	Verify(encodedHash string, password string) bool
}

type TokenManager interface {
	IssueAccessToken(input AccessTokenInput) (string, time.Time, error)
	VerifyAccessToken(accessToken string) (AccessTokenClaims, error)
	GenerateRefreshToken() (string, string, error)
	HashOpaqueToken(rawToken string) string
}

type Clock interface {
	Now() time.Time
}

type AccessTokenInput struct {
	UserID    uuid.UUID
	SessionID uuid.UUID
	Role      identity.Role
	Now       time.Time
}

type AccessTokenClaims struct {
	UserID    uuid.UUID
	SessionID uuid.UUID
	Role      identity.Role
	ExpiresAt time.Time
}

type LoginInput struct {
	Email     string
	Password  string
	UserAgent string
	IP        string
}

type RefreshInput struct {
	RefreshToken string
	UserAgent    string
	IP           string
}

type AuthResult struct {
	User               identity.User
	AccessToken        string
	AccessTokenExpires time.Time
	RefreshToken       string
	RefreshTokenExpiry time.Time
}

type SecurityEvent struct {
	EventType string
	Severity  string
	UserID    *uuid.UUID
	SessionID *uuid.UUID
	IP        string
	UserAgent string
	Metadata  map[string]any
	CreatedAt time.Time
}

type AuditRecorder interface {
	RecordSecurityEvent(ctx context.Context, event SecurityEvent) error
}

type Service struct {
	users      UserRepository
	sessions   SessionRepository
	tokenMgr   TokenManager
	hasher     PasswordHasher
	clock      Clock
	refreshTTL time.Duration
	log        *slog.Logger
	audit      AuditRecorder
}

func NewService(
	users UserRepository,
	sessions SessionRepository,
	tokenMgr TokenManager,
	hasher PasswordHasher,
	clock Clock,
	refreshTTL time.Duration,
	log *slog.Logger,
	audit AuditRecorder,
) *Service {
	if log == nil {
		log = slog.Default()
	}
	return &Service{
		users:      users,
		sessions:   sessions,
		tokenMgr:   tokenMgr,
		hasher:     hasher,
		clock:      clock,
		refreshTTL: refreshTTL,
		log:        log,
		audit:      audit,
	}
}

func (s *Service) Login(ctx context.Context, input LoginInput) (AuthResult, error) {
	email := identity.NormalizeEmail(input.Email)
	if email == "" || strings.TrimSpace(input.Password) == "" {
		return AuthResult{}, ErrInvalidCredentials
	}

	user, err := s.users.GetByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, identity.ErrUserNotFound) {
			return AuthResult{}, ErrInvalidCredentials
		}
		return AuthResult{}, fmt.Errorf("get user by email: %w", err)
	}

	if !user.IsActive {
		return AuthResult{}, ErrInactiveUser
	}

	if !s.hasher.Verify(user.PasswordHash, input.Password) {
		return AuthResult{}, ErrInvalidCredentials
	}

	now := s.clock.Now().UTC()
	refreshRaw, refreshHash, err := s.tokenMgr.GenerateRefreshToken()
	if err != nil {
		return AuthResult{}, fmt.Errorf("generate refresh token: %w", err)
	}

	session := identity.Session{
		ID:               uuid.New(),
		UserID:           user.ID,
		FamilyID:         uuid.New(),
		RefreshTokenHash: refreshHash,
		UserAgent:        input.UserAgent,
		IP:               input.IP,
		ExpiresAt:        now.Add(s.refreshTTL),
	}

	if err := s.sessions.Create(ctx, session); err != nil {
		return AuthResult{}, fmt.Errorf("create session: %w", err)
	}

	accessToken, accessExpiry, err := s.tokenMgr.IssueAccessToken(AccessTokenInput{
		UserID:    user.ID,
		SessionID: session.ID,
		Role:      user.Role,
		Now:       now,
	})
	if err != nil {
		return AuthResult{}, fmt.Errorf("issue access token: %w", err)
	}

	s.recordSecurityEvent(ctx, SecurityEvent{
		EventType: "auth.login",
		Severity:  "info",
		UserID:    &user.ID,
		SessionID: &session.ID,
		IP:        input.IP,
		UserAgent: input.UserAgent,
		Metadata: map[string]any{
			"email": user.Email,
		},
	})

	return AuthResult{
		User:               user,
		AccessToken:        accessToken,
		AccessTokenExpires: accessExpiry,
		RefreshToken:       refreshRaw,
		RefreshTokenExpiry: session.ExpiresAt,
	}, nil
}

func (s *Service) Refresh(ctx context.Context, input RefreshInput) (AuthResult, error) {
	if strings.TrimSpace(input.RefreshToken) == "" {
		return AuthResult{}, ErrInvalidSession
	}

	now := s.clock.Now().UTC()
	tokenHash := s.tokenMgr.HashOpaqueToken(input.RefreshToken)
	session, err := s.sessions.GetByRefreshTokenHash(ctx, tokenHash)
	if err != nil {
		if errors.Is(err, identity.ErrSessionNotFound) {
			return AuthResult{}, ErrInvalidSession
		}
		return AuthResult{}, fmt.Errorf("get session by refresh token hash: %w", err)
	}

	if session.IsRevoked() {
		if session.ReplacedBySessionID != nil {
			if err := s.sessions.RevokeFamily(ctx, session.FamilyID, now, "refresh_reuse_detected"); err != nil {
				s.log.Error("refresh replay detected but family revocation failed",
					slog.Any("error", err),
					slog.String("user_id", session.UserID.String()),
					slog.String("session_id", session.ID.String()),
				)
				s.recordSecurityEvent(ctx, SecurityEvent{
					EventType: "auth.refresh_replay_detected",
					Severity:  "critical",
					UserID:    &session.UserID,
					SessionID: &session.ID,
					IP:        input.IP,
					UserAgent: input.UserAgent,
					Metadata: map[string]any{
						"family_id":               session.FamilyID.String(),
						"family_revocation_error": err.Error(),
					},
				})
				return AuthResult{}, ErrInvalidSession
			}
			s.log.Warn("refresh replay detected; family revoked",
				slog.String("user_id", session.UserID.String()),
				slog.String("session_id", session.ID.String()),
			)
			s.recordSecurityEvent(ctx, SecurityEvent{
				EventType: "auth.refresh_replay_detected",
				Severity:  "high",
				UserID:    &session.UserID,
				SessionID: &session.ID,
				IP:        input.IP,
				UserAgent: input.UserAgent,
				Metadata: map[string]any{
					"family_id": session.FamilyID.String(),
				},
			})
		}
		return AuthResult{}, ErrInvalidSession
	}

	if !now.Before(session.ExpiresAt) {
		_, _ = s.sessions.RevokeByRefreshTokenHash(ctx, tokenHash, now, "expired")
		return AuthResult{}, ErrInvalidSession
	}

	user, err := s.users.GetByID(ctx, session.UserID)
	if err != nil {
		if errors.Is(err, identity.ErrUserNotFound) {
			return AuthResult{}, ErrInvalidSession
		}
		return AuthResult{}, fmt.Errorf("get user by id: %w", err)
	}

	if !user.IsActive {
		return AuthResult{}, ErrInactiveUser
	}

	newRefreshRaw, newRefreshHash, err := s.tokenMgr.GenerateRefreshToken()
	if err != nil {
		return AuthResult{}, fmt.Errorf("generate refresh token: %w", err)
	}

	newSession := identity.Session{
		ID:               uuid.New(),
		UserID:           session.UserID,
		FamilyID:         session.FamilyID,
		RefreshTokenHash: newRefreshHash,
		UserAgent:        input.UserAgent,
		IP:               input.IP,
		ExpiresAt:        now.Add(s.refreshTTL),
	}

	if err := s.sessions.Rotate(ctx, session.ID, newSession, now); err != nil {
		if errors.Is(err, identity.ErrSessionAlreadyRotated) {
			if revokeErr := s.sessions.RevokeFamily(ctx, session.FamilyID, now, "refresh_reuse_detected"); revokeErr != nil {
				s.log.Error("refresh token replay race detected but family revocation failed",
					slog.Any("error", revokeErr),
					slog.String("user_id", session.UserID.String()),
					slog.String("session_id", session.ID.String()),
				)
				s.recordSecurityEvent(ctx, SecurityEvent{
					EventType: "auth.refresh_replay_race",
					Severity:  "critical",
					UserID:    &session.UserID,
					SessionID: &session.ID,
					IP:        input.IP,
					UserAgent: input.UserAgent,
					Metadata: map[string]any{
						"family_id":               session.FamilyID.String(),
						"family_revocation_error": revokeErr.Error(),
					},
				})
				return AuthResult{}, ErrInvalidSession
			}
			s.recordSecurityEvent(ctx, SecurityEvent{
				EventType: "auth.refresh_replay_race",
				Severity:  "high",
				UserID:    &session.UserID,
				SessionID: &session.ID,
				IP:        input.IP,
				UserAgent: input.UserAgent,
				Metadata: map[string]any{
					"family_id": session.FamilyID.String(),
				},
			})
			return AuthResult{}, ErrInvalidSession
		}
		return AuthResult{}, fmt.Errorf("rotate session: %w", err)
	}

	accessToken, accessExpiry, err := s.tokenMgr.IssueAccessToken(AccessTokenInput{
		UserID:    user.ID,
		SessionID: newSession.ID,
		Role:      user.Role,
		Now:       now,
	})
	if err != nil {
		return AuthResult{}, fmt.Errorf("issue access token: %w", err)
	}

	return AuthResult{
		User:               user,
		AccessToken:        accessToken,
		AccessTokenExpires: accessExpiry,
		RefreshToken:       newRefreshRaw,
		RefreshTokenExpiry: newSession.ExpiresAt,
	}, nil
}

type LogoutInput struct {
	SessionID    uuid.UUID
	RefreshToken string
}

func (s *Service) Logout(ctx context.Context, input LogoutInput) error {
	now := s.clock.Now().UTC()
	if input.SessionID != uuid.Nil {
		if _, err := s.sessions.RevokeByID(ctx, input.SessionID, now, "logout"); err != nil {
			return fmt.Errorf("revoke session by id: %w", err)
		}
		sid := input.SessionID
		s.recordSecurityEvent(ctx, SecurityEvent{
			EventType: "auth.logout",
			Severity:  "info",
			SessionID: &sid,
			Metadata: map[string]any{
				"logout_scope": "session_or_family",
			},
		})
	}

	refreshToken := strings.TrimSpace(input.RefreshToken)
	if refreshToken != "" {
		tokenHash := s.tokenMgr.HashOpaqueToken(refreshToken)
		session, err := s.sessions.GetByRefreshTokenHash(ctx, tokenHash)
		if err != nil && !errors.Is(err, identity.ErrSessionNotFound) {
			return fmt.Errorf("get session by refresh token hash during logout: %w", err)
		}
		if err == nil {
			if err := s.sessions.RevokeFamily(ctx, session.FamilyID, now, "logout"); err != nil {
				return fmt.Errorf("revoke session family by refresh token during logout: %w", err)
			}
		}
	}

	return nil
}

func (s *Service) VerifyAccessToken(accessToken string) (AccessTokenClaims, error) {
	return s.tokenMgr.VerifyAccessToken(accessToken)
}

func (s *Service) AuthenticateAccessToken(ctx context.Context, accessToken string) (AccessTokenClaims, error) {
	claims, err := s.VerifyAccessToken(accessToken)
	if err != nil {
		return AccessTokenClaims{}, ErrInvalidSession
	}

	session, err := s.sessions.GetByID(ctx, claims.SessionID)
	if err != nil {
		return AccessTokenClaims{}, ErrInvalidSession
	}

	now := s.clock.Now().UTC()
	if session.IsRevoked() || !now.Before(session.ExpiresAt) {
		return AccessTokenClaims{}, ErrInvalidSession
	}
	if session.UserID != claims.UserID {
		return AccessTokenClaims{}, ErrInvalidSession
	}

	user, err := s.users.GetByID(ctx, claims.UserID)
	if err != nil {
		return AccessTokenClaims{}, ErrInvalidSession
	}
	if !user.IsActive {
		return AccessTokenClaims{}, ErrInvalidSession
	}

	return claims, nil
}

func (s *Service) recordSecurityEvent(ctx context.Context, event SecurityEvent) {
	if s.audit == nil {
		return
	}
	if err := s.audit.RecordSecurityEvent(ctx, event); err != nil {
		s.log.Error("failed to persist security audit event",
			slog.Any("error", err),
			slog.String("event_type", event.EventType),
		)
	}
}

type RealClock struct{}

func (RealClock) Now() time.Time {
	return time.Now()
}
