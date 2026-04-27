package token

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"time"

	"cloud/backend/internal/application/auth"
	"cloud/backend/internal/domain/identity"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type JWTManager struct {
	signingKey []byte
	issuer     string
	audience   string
	accessTTL  time.Duration
}

func NewJWTManager(signingKey string, issuer string, audience string, accessTTL time.Duration) *JWTManager {
	return &JWTManager{
		signingKey: []byte(signingKey),
		issuer:     issuer,
		audience:   audience,
		accessTTL:  accessTTL,
	}
}

type customClaims struct {
	Role      string `json:"role"`
	SessionID string `json:"sid"`
	TokenUse  string `json:"token_use"`
	jwt.RegisteredClaims
}

func (m *JWTManager) IssueAccessToken(input auth.AccessTokenInput) (string, time.Time, error) {
	expiresAt := input.Now.Add(m.accessTTL)
	claims := customClaims{
		Role:      string(input.Role),
		SessionID: input.SessionID.String(),
		TokenUse:  "access",
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    m.issuer,
			Subject:   input.UserID.String(),
			Audience:  jwt.ClaimStrings{m.audience},
			IssuedAt:  jwt.NewNumericDate(input.Now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(m.signingKey)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("sign access token: %w", err)
	}
	return signed, expiresAt, nil
}

func (m *JWTManager) VerifyAccessToken(accessToken string) (auth.AccessTokenClaims, error) {
	claims := &customClaims{}
	parsedToken, err := jwt.ParseWithClaims(accessToken, claims, func(token *jwt.Token) (any, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("unexpected signing method: %s", token.Method.Alg())
		}
		return m.signingKey, nil
	}, jwt.WithIssuer(m.issuer), jwt.WithAudience(m.audience))
	if err != nil {
		return auth.AccessTokenClaims{}, fmt.Errorf("parse access token: %w", err)
	}
	if !parsedToken.Valid {
		return auth.AccessTokenClaims{}, fmt.Errorf("invalid access token")
	}

	userID, err := uuid.Parse(claims.Subject)
	if err != nil {
		return auth.AccessTokenClaims{}, fmt.Errorf("invalid user id claim: %w", err)
	}
	sessionID, err := uuid.Parse(claims.SessionID)
	if err != nil {
		return auth.AccessTokenClaims{}, fmt.Errorf("invalid session id claim: %w", err)
	}
	if !identity.IsValidRole(identity.Role(claims.Role)) {
		return auth.AccessTokenClaims{}, fmt.Errorf("invalid role claim")
	}
	if claims.TokenUse != "access" {
		return auth.AccessTokenClaims{}, fmt.Errorf("invalid token_use claim")
	}
	if claims.ExpiresAt == nil {
		return auth.AccessTokenClaims{}, fmt.Errorf("missing expiry claim")
	}

	return auth.AccessTokenClaims{
		UserID:    userID,
		SessionID: sessionID,
		Role:      identity.Role(claims.Role),
		ExpiresAt: claims.ExpiresAt.Time,
	}, nil
}

func (m *JWTManager) GenerateRefreshToken() (string, string, error) {
	b := make([]byte, 48)
	if _, err := rand.Read(b); err != nil {
		return "", "", fmt.Errorf("generate refresh token bytes: %w", err)
	}

	raw := base64.RawURLEncoding.EncodeToString(b)
	return raw, m.HashOpaqueToken(raw), nil
}

func (m *JWTManager) HashOpaqueToken(rawToken string) string {
	digest := sha256.Sum256([]byte(rawToken))
	return hex.EncodeToString(digest[:])
}
