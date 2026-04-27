package token

import (
	"testing"
	"time"

	"cloud/backend/internal/application/auth"
	"cloud/backend/internal/domain/identity"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

func TestJWTManagerIssueAndVerifyAccessToken(t *testing.T) {
	manager := NewJWTManager("0123456789abcdef0123456789abcdef", "cloud-test", "cloud-web", 15*time.Minute)
	now := time.Now().UTC()
	userID := uuid.New()
	sessionID := uuid.New()

	token, _, err := manager.IssueAccessToken(auth.AccessTokenInput{
		UserID:    userID,
		SessionID: sessionID,
		Role:      identity.RoleAdmin,
		Now:       now,
	})
	if err != nil {
		t.Fatalf("expected no issue error, got %v", err)
	}

	claims, err := manager.VerifyAccessToken(token)
	if err != nil {
		t.Fatalf("expected verify success, got %v", err)
	}

	if claims.UserID != userID {
		t.Fatalf("expected user id %s, got %s", userID, claims.UserID)
	}
	if claims.SessionID != sessionID {
		t.Fatalf("expected session id %s, got %s", sessionID, claims.SessionID)
	}
	if claims.Role != identity.RoleAdmin {
		t.Fatalf("expected role admin, got %s", claims.Role)
	}
}

func TestJWTManagerGenerateRefreshToken(t *testing.T) {
	manager := NewJWTManager("0123456789abcdef0123456789abcdef", "cloud-test", "cloud-web", 15*time.Minute)
	raw, hash, err := manager.GenerateRefreshToken()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if raw == "" || hash == "" {
		t.Fatalf("expected non-empty token and hash")
	}
	if manager.HashOpaqueToken(raw) != hash {
		t.Fatalf("expected consistent token hash")
	}
}

func TestJWTManagerRejectsWrongAudience(t *testing.T) {
	issuer := "cloud-test"
	key := "0123456789abcdef0123456789abcdef"
	managerA := NewJWTManager(key, issuer, "aud-A", 15*time.Minute)
	managerB := NewJWTManager(key, issuer, "aud-B", 15*time.Minute)
	now := time.Now().UTC()

	token, _, err := managerA.IssueAccessToken(auth.AccessTokenInput{
		UserID:    uuid.New(),
		SessionID: uuid.New(),
		Role:      identity.RoleUser,
		Now:       now,
	})
	if err != nil {
		t.Fatalf("expected no issue error, got %v", err)
	}

	if _, err := managerB.VerifyAccessToken(token); err == nil {
		t.Fatalf("expected wrong audience token verification to fail")
	}
}

func TestJWTManagerRejectsWrongTokenUse(t *testing.T) {
	manager := NewJWTManager("0123456789abcdef0123456789abcdef", "cloud-test", "cloud-web", 15*time.Minute)
	now := time.Now().UTC()
	claims := customClaims{
		Role:      string(identity.RoleAdmin),
		SessionID: uuid.New().String(),
		TokenUse:  "refresh",
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "cloud-test",
			Subject:   uuid.New().String(),
			Audience:  jwt.ClaimStrings{"cloud-web"},
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(15 * time.Minute)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		t.Fatalf("expected no sign error, got %v", err)
	}

	if _, err := manager.VerifyAccessToken(signed); err == nil {
		t.Fatalf("expected wrong token_use token verification to fail")
	}
}
