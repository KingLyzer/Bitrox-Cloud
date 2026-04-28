package config

import (
	"os"
	"testing"
)

func TestLoadSuccess(t *testing.T) {
	t.Setenv("APP_DATABASE_URL", "postgres://postgres:postgres@localhost:5432/cloud?sslmode=disable")
	t.Setenv("APP_REDIS_URL", "redis://localhost:6379/0")
	t.Setenv("APP_MASTER_KEY", "0123456789abcdef0123456789abcdef")
	t.Setenv("APP_JWT_SIGNING_KEY", "0123456789abcdef0123456789abcdef")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if cfg.HTTPAddr == "" {
		t.Fatalf("expected default APP_HTTP_ADDR")
	}
	if cfg.AuthRateLimit <= 0 {
		t.Fatalf("expected positive auth rate limit")
	}
	if len(cfg.TrustedProxyCIDRs) == 0 {
		t.Fatalf("expected default trusted proxy cidrs to be populated")
	}
}

func TestLoadRequiresMasterKey(t *testing.T) {
	t.Setenv("APP_DATABASE_URL", "postgres://postgres:postgres@localhost:5432/cloud?sslmode=disable")
	t.Setenv("APP_REDIS_URL", "redis://localhost:6379/0")
	t.Setenv("APP_JWT_SIGNING_KEY", "0123456789abcdef0123456789abcdef")
	_ = os.Unsetenv("APP_MASTER_KEY")

	_, err := Load()
	if err == nil {
		t.Fatalf("expected an error when APP_MASTER_KEY is missing")
	}
}

func TestLoadRequiresJWTSigningKey(t *testing.T) {
	t.Setenv("APP_DATABASE_URL", "postgres://postgres:postgres@localhost:5432/cloud?sslmode=disable")
	t.Setenv("APP_REDIS_URL", "redis://localhost:6379/0")
	t.Setenv("APP_MASTER_KEY", "0123456789abcdef0123456789abcdef")
	_ = os.Unsetenv("APP_JWT_SIGNING_KEY")
	_ = os.Unsetenv("APP_JWT_SECRET")

	_, err := Load()
	if err == nil {
		t.Fatalf("expected an error when APP_JWT_SIGNING_KEY is missing")
	}
}

func TestLoadUsesLegacyJWTSecretFallback(t *testing.T) {
	t.Setenv("APP_DATABASE_URL", "postgres://postgres:postgres@localhost:5432/cloud?sslmode=disable")
	t.Setenv("APP_REDIS_URL", "redis://localhost:6379/0")
	t.Setenv("APP_MASTER_KEY", "0123456789abcdef0123456789abcdef")
	_ = os.Unsetenv("APP_JWT_SIGNING_KEY")
	t.Setenv("APP_JWT_SECRET", "abcdef0123456789abcdef0123456789")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if cfg.JWTSigningKey != "abcdef0123456789abcdef0123456789" {
		t.Fatalf("expected APP_JWT_SECRET fallback to be used")
	}
}

func TestLoadRejectsInsecureCookiesInProduction(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("APP_DATABASE_URL", "postgres://postgres:postgres@localhost:5432/cloud?sslmode=disable")
	t.Setenv("APP_REDIS_URL", "redis://localhost:6379/0")
	t.Setenv("APP_MASTER_KEY", "0123456789abcdef0123456789abcdef")
	t.Setenv("APP_JWT_SIGNING_KEY", "0123456789abcdef0123456789abcdef")
	t.Setenv("APP_COOKIE_SECURE", "false")

	_, err := Load()
	if err == nil {
		t.Fatalf("expected error for insecure production cookies")
	}
}

func TestLoadRejectsInvalidTrustedProxyCIDR(t *testing.T) {
	t.Setenv("APP_DATABASE_URL", "postgres://postgres:postgres@localhost:5432/cloud?sslmode=disable")
	t.Setenv("APP_REDIS_URL", "redis://localhost:6379/0")
	t.Setenv("APP_MASTER_KEY", "0123456789abcdef0123456789abcdef")
	t.Setenv("APP_JWT_SIGNING_KEY", "0123456789abcdef0123456789abcdef")
	t.Setenv("APP_TRUSTED_PROXY_CIDRS", "not-a-cidr")

	_, err := Load()
	if err == nil {
		t.Fatalf("expected error for invalid trusted proxy cidr")
	}
}

func TestLoadRejectsUploadChunkLargerThanMaxFile(t *testing.T) {
	t.Setenv("APP_DATABASE_URL", "postgres://postgres:postgres@localhost:5432/cloud?sslmode=disable")
	t.Setenv("APP_REDIS_URL", "redis://localhost:6379/0")
	t.Setenv("APP_MASTER_KEY", "0123456789abcdef0123456789abcdef")
	t.Setenv("APP_JWT_SIGNING_KEY", "0123456789abcdef0123456789abcdef")
	t.Setenv("APP_UPLOAD_MAX_CHUNK_BYTES", "2048")
	t.Setenv("APP_UPLOAD_MAX_FILE_BYTES", "1024")

	_, err := Load()
	if err == nil {
		t.Fatalf("expected error when chunk limit exceeds file limit")
	}
}

func TestLoadRejectsUploadStagedBelowChunkLimit(t *testing.T) {
	t.Setenv("APP_DATABASE_URL", "postgres://postgres:postgres@localhost:5432/cloud?sslmode=disable")
	t.Setenv("APP_REDIS_URL", "redis://localhost:6379/0")
	t.Setenv("APP_MASTER_KEY", "0123456789abcdef0123456789abcdef")
	t.Setenv("APP_JWT_SIGNING_KEY", "0123456789abcdef0123456789abcdef")
	t.Setenv("APP_UPLOAD_MAX_CHUNK_BYTES", "4096")
	t.Setenv("APP_UPLOAD_MAX_STAGED_BYTES", "2048")

	_, err := Load()
	if err == nil {
		t.Fatalf("expected error when staged limit is below chunk limit")
	}
}
