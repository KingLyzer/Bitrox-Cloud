package config

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	defaultHTTPAddr                = ":8080"
	defaultEnvironment             = "development"
	defaultLogLevel                = "info"
	defaultLogFormat               = "json"
	defaultTrustedProxyCIDRsCSV    = "127.0.0.1/32,::1/128,10.0.0.0/8,172.16.0.0/12,192.168.0.0/16"
	defaultShutdownTimeout         = 15 * time.Second
	defaultAuthRateLimit           = 10
	defaultAuthRateWindow          = time.Minute
	defaultUploadSessionTTL        = 24 * time.Hour
	defaultUploadMaxChunkBytes     = 16 * 1024 * 1024
	defaultUploadMaxFileBytes      = int64(100 * 1024 * 1024 * 1024)
	defaultUploadMaxActiveSessions = 64
	defaultUploadMaxStagedBytes    = int64(200 * 1024 * 1024 * 1024)
	defaultUploadReaperInterval    = time.Minute
	defaultUploadReaperBatchSize   = 200
	defaultReminderScanInterval    = 30 * time.Second
	defaultReminderScanBatchSize   = 200
	defaultTrashPurgeInterval      = time.Hour
	defaultTrashRetention          = 30 * 24 * time.Hour
)

type Config struct {
	Environment             string
	HTTPAddr                string
	LogLevel                string
	LogFormat               string
	CORSOrigins             []string
	TrustedProxyCIDRs       []string
	DatabaseURL             string
	RedisURL                string
	StorageLocalRoot        string
	MasterKey               string
	JWTSigningKey           string
	JWTIssuer               string
	JWTAudience             string
	AccessTokenTTL          time.Duration
	RefreshTokenTTL         time.Duration
	CookieSecure            bool
	CookieDomain            string
	CookieSameSite          string
	BootstrapAdminEmail     string
	BootstrapAdminPassword  string
	BootstrapAdminName      string
	BootstrapAdminRole      string
	DefaultUserQuotaBytes   int64
	ShutdownTimeout         time.Duration
	AuthRateLimit           int
	AuthRateWindow          time.Duration
	UploadSessionTTL        time.Duration
	UploadMaxChunkBytes     int
	UploadMaxFileBytes      int64
	UploadMaxActiveSessions int
	UploadMaxStagedBytes    int64
	UploadReaperInterval    time.Duration
	UploadReaperBatchSize   int
	ReminderScanInterval    time.Duration
	ReminderScanBatchSize   int
	TrashPurgeInterval      time.Duration
	TrashRetention          time.Duration
}

func Load() (Config, error) {
	cfg := Config{
		Environment:            getEnv("APP_ENV", defaultEnvironment),
		HTTPAddr:               getEnv("APP_HTTP_ADDR", defaultHTTPAddr),
		LogLevel:               strings.ToLower(getEnv("APP_LOG_LEVEL", defaultLogLevel)),
		LogFormat:              strings.ToLower(getEnv("APP_LOG_FORMAT", defaultLogFormat)),
		CORSOrigins:            splitCSV(getEnv("APP_CORS_ORIGINS", "http://localhost:3000")),
		TrustedProxyCIDRs:      splitCSV(getEnv("APP_TRUSTED_PROXY_CIDRS", defaultTrustedProxyCIDRsCSV)),
		DatabaseURL:            getEnv("APP_DATABASE_URL", ""),
		RedisURL:               getEnv("APP_REDIS_URL", ""),
		StorageLocalRoot:       getEnv("APP_STORAGE_LOCAL_ROOT", "./data/storage"),
		MasterKey:              getEnv("APP_MASTER_KEY", ""),
		JWTSigningKey:          getFirstNonEmptyEnv([]string{"APP_JWT_SIGNING_KEY", "APP_JWT_SECRET"}, ""),
		JWTIssuer:              getEnv("APP_JWT_ISSUER", "cloud-api"),
		JWTAudience:            getEnv("APP_JWT_AUDIENCE", "cloud-web"),
		CookieDomain:           getEnv("APP_COOKIE_DOMAIN", ""),
		CookieSameSite:         strings.ToLower(getEnv("APP_COOKIE_SAMESITE", "lax")),
		BootstrapAdminEmail:    getEnv("APP_BOOTSTRAP_ADMIN_EMAIL", ""),
		BootstrapAdminPassword: getEnv("APP_BOOTSTRAP_ADMIN_PASSWORD", ""),
		BootstrapAdminName:     getEnv("APP_BOOTSTRAP_ADMIN_NAME", "Admin"),
		BootstrapAdminRole:     strings.ToLower(getEnv("APP_BOOTSTRAP_ADMIN_ROLE", "owner")),
	}

	shutdownTimeout, err := getDuration("APP_SHUTDOWN_TIMEOUT", defaultShutdownTimeout)
	if err != nil {
		return Config{}, fmt.Errorf("invalid APP_SHUTDOWN_TIMEOUT: %w", err)
	}
	cfg.ShutdownTimeout = shutdownTimeout

	authLimit, err := getInt("APP_AUTH_RATE_LIMIT", defaultAuthRateLimit)
	if err != nil {
		return Config{}, fmt.Errorf("invalid APP_AUTH_RATE_LIMIT: %w", err)
	}
	cfg.AuthRateLimit = authLimit

	authWindow, err := getDuration("APP_AUTH_RATE_WINDOW", defaultAuthRateWindow)
	if err != nil {
		return Config{}, fmt.Errorf("invalid APP_AUTH_RATE_WINDOW: %w", err)
	}
	cfg.AuthRateWindow = authWindow

	uploadSessionTTL, err := getDuration("APP_UPLOAD_SESSION_TTL", defaultUploadSessionTTL)
	if err != nil {
		return Config{}, fmt.Errorf("invalid APP_UPLOAD_SESSION_TTL: %w", err)
	}
	cfg.UploadSessionTTL = uploadSessionTTL

	uploadMaxChunkBytes, err := getInt("APP_UPLOAD_MAX_CHUNK_BYTES", defaultUploadMaxChunkBytes)
	if err != nil {
		return Config{}, fmt.Errorf("invalid APP_UPLOAD_MAX_CHUNK_BYTES: %w", err)
	}
	cfg.UploadMaxChunkBytes = uploadMaxChunkBytes

	uploadMaxFileBytes, err := getInt64("APP_UPLOAD_MAX_FILE_BYTES", defaultUploadMaxFileBytes)
	if err != nil {
		return Config{}, fmt.Errorf("invalid APP_UPLOAD_MAX_FILE_BYTES: %w", err)
	}
	cfg.UploadMaxFileBytes = uploadMaxFileBytes

	uploadMaxActiveSessions, err := getInt("APP_UPLOAD_MAX_ACTIVE_SESSIONS", defaultUploadMaxActiveSessions)
	if err != nil {
		return Config{}, fmt.Errorf("invalid APP_UPLOAD_MAX_ACTIVE_SESSIONS: %w", err)
	}
	cfg.UploadMaxActiveSessions = uploadMaxActiveSessions

	uploadMaxStagedBytes, err := getInt64("APP_UPLOAD_MAX_STAGED_BYTES", defaultUploadMaxStagedBytes)
	if err != nil {
		return Config{}, fmt.Errorf("invalid APP_UPLOAD_MAX_STAGED_BYTES: %w", err)
	}
	cfg.UploadMaxStagedBytes = uploadMaxStagedBytes

	uploadReaperInterval, err := getDuration("APP_UPLOAD_REAPER_INTERVAL", defaultUploadReaperInterval)
	if err != nil {
		return Config{}, fmt.Errorf("invalid APP_UPLOAD_REAPER_INTERVAL: %w", err)
	}
	cfg.UploadReaperInterval = uploadReaperInterval

	uploadReaperBatchSize, err := getInt("APP_UPLOAD_REAPER_BATCH_SIZE", defaultUploadReaperBatchSize)
	if err != nil {
		return Config{}, fmt.Errorf("invalid APP_UPLOAD_REAPER_BATCH_SIZE: %w", err)
	}
	cfg.UploadReaperBatchSize = uploadReaperBatchSize

	reminderScanInterval, err := getDuration("APP_REMINDER_SCAN_INTERVAL", defaultReminderScanInterval)
	if err != nil {
		return Config{}, fmt.Errorf("invalid APP_REMINDER_SCAN_INTERVAL: %w", err)
	}
	cfg.ReminderScanInterval = reminderScanInterval

	reminderScanBatchSize, err := getInt("APP_REMINDER_SCAN_BATCH_SIZE", defaultReminderScanBatchSize)
	if err != nil {
		return Config{}, fmt.Errorf("invalid APP_REMINDER_SCAN_BATCH_SIZE: %w", err)
	}
	cfg.ReminderScanBatchSize = reminderScanBatchSize

	trashPurgeInterval, err := getDuration("APP_TRASH_PURGE_INTERVAL", defaultTrashPurgeInterval)
	if err != nil {
		return Config{}, fmt.Errorf("invalid APP_TRASH_PURGE_INTERVAL: %w", err)
	}
	cfg.TrashPurgeInterval = trashPurgeInterval

	trashRetention, err := getDuration("APP_TRASH_RETENTION", defaultTrashRetention)
	if err != nil {
		return Config{}, fmt.Errorf("invalid APP_TRASH_RETENTION: %w", err)
	}
	cfg.TrashRetention = trashRetention

	accessTokenTTL, err := getDuration("APP_ACCESS_TOKEN_TTL", 15*time.Minute)
	if err != nil {
		return Config{}, fmt.Errorf("invalid APP_ACCESS_TOKEN_TTL: %w", err)
	}
	cfg.AccessTokenTTL = accessTokenTTL

	refreshTokenTTL, err := getDuration("APP_REFRESH_TOKEN_TTL", 720*time.Hour)
	if err != nil {
		return Config{}, fmt.Errorf("invalid APP_REFRESH_TOKEN_TTL: %w", err)
	}
	cfg.RefreshTokenTTL = refreshTokenTTL

	cookieSecure, err := getBool("APP_COOKIE_SECURE", cfg.Environment != "development")
	if err != nil {
		return Config{}, fmt.Errorf("invalid APP_COOKIE_SECURE: %w", err)
	}
	cfg.CookieSecure = cookieSecure

	defaultUserQuotaBytes, err := getInt64("APP_DEFAULT_USER_QUOTA_BYTES", 21474836480)
	if err != nil {
		return Config{}, fmt.Errorf("invalid APP_DEFAULT_USER_QUOTA_BYTES: %w", err)
	}
	cfg.DefaultUserQuotaBytes = defaultUserQuotaBytes

	if err := cfg.validate(); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func (c Config) validate() error {
	if c.DatabaseURL == "" {
		return fmt.Errorf("APP_DATABASE_URL is required")
	}
	if c.RedisURL == "" {
		return fmt.Errorf("APP_REDIS_URL is required")
	}
	if c.MasterKey == "" {
		return fmt.Errorf("APP_MASTER_KEY is required")
	}
	if len(c.MasterKey) < 32 {
		return fmt.Errorf("APP_MASTER_KEY must be at least 32 characters")
	}
	if c.JWTSigningKey == "" {
		return fmt.Errorf("APP_JWT_SIGNING_KEY is required")
	}
	if len(c.JWTSigningKey) < 32 {
		return fmt.Errorf("APP_JWT_SIGNING_KEY must be at least 32 characters")
	}
	if strings.TrimSpace(c.JWTAudience) == "" {
		return fmt.Errorf("APP_JWT_AUDIENCE is required")
	}

	switch c.LogFormat {
	case "json", "text":
	default:
		return fmt.Errorf("APP_LOG_FORMAT must be either 'json' or 'text'")
	}

	if c.AuthRateLimit <= 0 {
		return fmt.Errorf("APP_AUTH_RATE_LIMIT must be greater than zero")
	}
	if c.AuthRateWindow <= 0 {
		return fmt.Errorf("APP_AUTH_RATE_WINDOW must be greater than zero")
	}
	if c.UploadSessionTTL <= 0 {
		return fmt.Errorf("APP_UPLOAD_SESSION_TTL must be greater than zero")
	}
	if c.UploadMaxChunkBytes <= 0 {
		return fmt.Errorf("APP_UPLOAD_MAX_CHUNK_BYTES must be greater than zero")
	}
	if c.UploadMaxFileBytes <= 0 {
		return fmt.Errorf("APP_UPLOAD_MAX_FILE_BYTES must be greater than zero")
	}
	if int64(c.UploadMaxChunkBytes) > c.UploadMaxFileBytes {
		return fmt.Errorf("APP_UPLOAD_MAX_CHUNK_BYTES cannot exceed APP_UPLOAD_MAX_FILE_BYTES")
	}
	if c.UploadMaxActiveSessions <= 0 {
		return fmt.Errorf("APP_UPLOAD_MAX_ACTIVE_SESSIONS must be greater than zero")
	}
	if c.UploadMaxStagedBytes <= 0 {
		return fmt.Errorf("APP_UPLOAD_MAX_STAGED_BYTES must be greater than zero")
	}
	if c.UploadMaxStagedBytes < int64(c.UploadMaxChunkBytes) {
		return fmt.Errorf("APP_UPLOAD_MAX_STAGED_BYTES must be at least APP_UPLOAD_MAX_CHUNK_BYTES")
	}
	if c.UploadReaperInterval <= 0 {
		return fmt.Errorf("APP_UPLOAD_REAPER_INTERVAL must be greater than zero")
	}
	if c.UploadReaperBatchSize <= 0 {
		return fmt.Errorf("APP_UPLOAD_REAPER_BATCH_SIZE must be greater than zero")
	}
	if c.ReminderScanInterval <= 0 {
		return fmt.Errorf("APP_REMINDER_SCAN_INTERVAL must be greater than zero")
	}
	if c.ReminderScanBatchSize <= 0 {
		return fmt.Errorf("APP_REMINDER_SCAN_BATCH_SIZE must be greater than zero")
	}
	if c.TrashPurgeInterval <= 0 {
		return fmt.Errorf("APP_TRASH_PURGE_INTERVAL must be greater than zero")
	}
	if c.TrashRetention <= 0 {
		return fmt.Errorf("APP_TRASH_RETENTION must be greater than zero")
	}
	if len(c.CORSOrigins) == 0 {
		return fmt.Errorf("APP_CORS_ORIGINS must contain at least one origin")
	}
	if c.AccessTokenTTL <= 0 {
		return fmt.Errorf("APP_ACCESS_TOKEN_TTL must be greater than zero")
	}
	if c.RefreshTokenTTL <= 0 {
		return fmt.Errorf("APP_REFRESH_TOKEN_TTL must be greater than zero")
	}
	if c.RefreshTokenTTL <= c.AccessTokenTTL {
		return fmt.Errorf("APP_REFRESH_TOKEN_TTL must be greater than APP_ACCESS_TOKEN_TTL")
	}
	switch c.CookieSameSite {
	case "lax", "strict", "none":
	default:
		return fmt.Errorf("APP_COOKIE_SAMESITE must be one of: lax, strict, none")
	}
	if c.CookieSameSite == "none" && !c.CookieSecure {
		return fmt.Errorf("APP_COOKIE_SECURE must be true when APP_COOKIE_SAMESITE=none")
	}
	if strings.EqualFold(c.Environment, "production") && !c.CookieSecure {
		return fmt.Errorf("APP_COOKIE_SECURE must be true when APP_ENV=production")
	}
	if c.BootstrapAdminRole != "" && !isValidRole(c.BootstrapAdminRole) {
		return fmt.Errorf("APP_BOOTSTRAP_ADMIN_ROLE must be one of: owner, admin, user")
	}
	if c.DefaultUserQuotaBytes <= 0 {
		return fmt.Errorf("APP_DEFAULT_USER_QUOTA_BYTES must be greater than zero")
	}
	for _, cidr := range c.TrustedProxyCIDRs {
		if err := validateCIDROrIP(cidr); err != nil {
			return fmt.Errorf("invalid APP_TRUSTED_PROXY_CIDRS entry %q: %w", cidr, err)
		}
	}

	return nil
}

func getEnv(key, fallback string) string {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	return v
}

func getFirstNonEmptyEnv(keys []string, fallback string) string {
	for _, key := range keys {
		v := strings.TrimSpace(os.Getenv(strings.TrimSpace(key)))
		if v != "" {
			return v
		}
	}
	return fallback
}

func getInt(key string, fallback int) (int, error) {
	raw := getEnv(key, "")
	if raw == "" {
		return fallback, nil
	}
	return strconv.Atoi(raw)
}

func getDuration(key string, fallback time.Duration) (time.Duration, error) {
	raw := getEnv(key, "")
	if raw == "" {
		return fallback, nil
	}
	return time.ParseDuration(raw)
}

func getBool(key string, fallback bool) (bool, error) {
	raw := getEnv(key, "")
	if raw == "" {
		return fallback, nil
	}
	return strconv.ParseBool(raw)
}

func getInt64(key string, fallback int64) (int64, error) {
	raw := getEnv(key, "")
	if raw == "" {
		return fallback, nil
	}
	return strconv.ParseInt(raw, 10, 64)
}

func splitCSV(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return []string{}
	}

	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		val := strings.TrimSpace(part)
		if val != "" {
			out = append(out, val)
		}
	}
	return out
}

func isValidRole(role string) bool {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case "owner", "admin", "user":
		return true
	default:
		return false
	}
}

func validateCIDROrIP(raw string) error {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return fmt.Errorf("empty value")
	}
	if strings.Contains(trimmed, "/") {
		if _, _, err := net.ParseCIDR(trimmed); err != nil {
			return err
		}
		return nil
	}
	if net.ParseIP(trimmed) == nil {
		return fmt.Errorf("not a valid ip or cidr")
	}
	return nil
}
