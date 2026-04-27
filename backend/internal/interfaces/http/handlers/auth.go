package handlers

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"cloud/backend/internal/application/auth"
	"cloud/backend/internal/interfaces/http/authctx"
)

const (
	legacyAccessCookieName  = "cloud_access_token"
	legacyRefreshCookieName = "cloud_refresh_token"
	legacyCSRFCookieName    = "cloud_csrf_token"

	hostAccessCookieName  = "__Host-cloud_access_token"
	hostRefreshCookieName = "__Host-cloud_refresh_token"
	hostCSRFCookieName    = "__Host-cloud_csrf_token"

	csrfHeaderName = "X-CSRF-Token"
)

type IPResolver interface {
	Resolve(req *http.Request) string
}

type AuthCookieConfig struct {
	Domain         string
	Secure         bool
	SameSite       http.SameSite
	AccessTTL      time.Duration
	RefreshTTL     time.Duration
	AllowedOrigins []string
}

type cookieNames struct {
	access  string
	refresh string
	csrf    string
}

type AuthHandler struct {
	log            *slog.Logger
	service        *auth.Service
	cookieCfg      AuthCookieConfig
	originAllowSet map[string]struct{}
	originAllowAny bool
	ipResolver     IPResolver
}

func NewAuthHandler(
	log *slog.Logger,
	service *auth.Service,
	cookieCfg AuthCookieConfig,
	ipResolver IPResolver,
) *AuthHandler {
	if log == nil {
		log = slog.Default()
	}
	origins := map[string]struct{}{}
	allowAny := false
	for _, origin := range cookieCfg.AllowedOrigins {
		trimmed := strings.TrimSpace(origin)
		if trimmed == "" {
			continue
		}
		if trimmed == "*" {
			allowAny = true
			continue
		}
		normalized := normalizeOrigin(trimmed)
		if normalized != "" {
			origins[normalized] = struct{}{}
		}
	}
	return &AuthHandler{
		log:            log,
		service:        service,
		cookieCfg:      cookieCfg,
		originAllowSet: origins,
		originAllowAny: allowAny,
		ipResolver:     ipResolver,
	}
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type authUserResponse struct {
	ID          string `json:"id"`
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
	Role        string `json:"role"`
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid_request", "invalid request body")
		return
	}

	result, err := h.service.Login(r.Context(), auth.LoginInput{
		Email:     req.Email,
		Password:  req.Password,
		UserAgent: strings.TrimSpace(r.UserAgent()),
		IP:        h.resolveClientIP(r),
	})
	if err != nil {
		if errors.Is(err, auth.ErrInvalidCredentials) || errors.Is(err, auth.ErrInactiveUser) {
			// Keep error signal generic to reduce account enumeration.
			h.log.Warn("login rejected", slog.Any("error", err), slog.String("ip", h.resolveClientIP(r)))
			writeAPIError(w, http.StatusUnauthorized, "invalid_credentials", "invalid credentials")
			return
		}
		h.log.Error("login failed", slog.Any("error", err))
		writeAPIError(w, http.StatusInternalServerError, "internal_error", "failed to login")
		return
	}

	csrfToken, err := generateCSRFToken()
	if err != nil {
		h.log.Error("failed to generate csrf token", slog.Any("error", err))
		writeAPIError(w, http.StatusInternalServerError, "internal_error", "failed to login")
		return
	}

	h.setAuthCookies(w, result.AccessToken, result.RefreshToken, csrfToken)

	writeJSON(w, http.StatusOK, map[string]any{
		"user": authUserResponse{
			ID:          result.User.ID.String(),
			Email:       result.User.Email,
			DisplayName: result.User.DisplayName,
			Role:        string(result.User.Role),
		},
	})
}

func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	if err := h.validateBrowserCSRF(r); err != nil {
		writeAPIError(w, http.StatusForbidden, "csrf_validation_failed", err.Error())
		return
	}

	refreshToken, err := h.readRefreshTokenCookie(r)
	if err != nil {
		writeAPIError(w, http.StatusUnauthorized, "invalid_session", "missing refresh token")
		return
	}

	result, err := h.service.Refresh(r.Context(), auth.RefreshInput{
		RefreshToken: refreshToken,
		UserAgent:    strings.TrimSpace(r.UserAgent()),
		IP:           h.resolveClientIP(r),
	})
	if err != nil {
		if errors.Is(err, auth.ErrInvalidSession) {
			writeAPIError(w, http.StatusUnauthorized, "invalid_session", "session is invalid")
			return
		}
		if errors.Is(err, auth.ErrInactiveUser) {
			writeAPIError(w, http.StatusUnauthorized, "invalid_session", "session is invalid")
			return
		}
		h.log.Error("refresh failed", slog.Any("error", err))
		writeAPIError(w, http.StatusInternalServerError, "internal_error", "failed to refresh session")
		return
	}

	csrfToken, err := generateCSRFToken()
	if err != nil {
		h.log.Error("failed to generate csrf token", slog.Any("error", err))
		writeAPIError(w, http.StatusInternalServerError, "internal_error", "failed to refresh session")
		return
	}
	h.setAuthCookies(w, result.AccessToken, result.RefreshToken, csrfToken)

	writeJSON(w, http.StatusOK, map[string]any{
		"user": authUserResponse{
			ID:          result.User.ID.String(),
			Email:       result.User.Email,
			DisplayName: result.User.DisplayName,
			Role:        string(result.User.Role),
		},
	})
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	authValue, ok := authctx.FromContext(r.Context())
	if !ok {
		writeAPIError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}

	requiresCSRF := h.hasAccessCookie(r) || h.hasRefreshCookie(r)
	if requiresCSRF {
		if err := h.validateBrowserCSRF(r); err != nil {
			writeAPIError(w, http.StatusForbidden, "csrf_validation_failed", err.Error())
			return
		}
	}

	refreshToken, _ := h.readRefreshTokenCookie(r)
	if err := h.service.Logout(r.Context(), auth.LogoutInput{
		SessionID:    authValue.SessionID,
		RefreshToken: refreshToken,
	}); err != nil {
		h.log.Error("logout failed", slog.Any("error", err))
		writeAPIError(w, http.StatusInternalServerError, "internal_error", "failed to logout")
		return
	}

	h.clearAuthCookies(w)
	w.WriteHeader(http.StatusNoContent)
}

func (h *AuthHandler) setAuthCookies(w http.ResponseWriter, accessToken, refreshToken, csrfToken string) {
	names := h.currentCookieNames()

	http.SetCookie(w, &http.Cookie{
		Name:     names.access,
		Value:    accessToken,
		Path:     "/",
		Domain:   h.cookieCfg.Domain,
		HttpOnly: true,
		Secure:   h.cookieCfg.Secure,
		SameSite: h.cookieCfg.SameSite,
		MaxAge:   int(h.cookieCfg.AccessTTL.Seconds()),
	})

	http.SetCookie(w, &http.Cookie{
		Name:     names.refresh,
		Value:    refreshToken,
		Path:     "/",
		Domain:   h.cookieCfg.Domain,
		HttpOnly: true,
		Secure:   h.cookieCfg.Secure,
		SameSite: h.cookieCfg.SameSite,
		MaxAge:   int(h.cookieCfg.RefreshTTL.Seconds()),
	})

	// CSRF cookie is readable by browser JS and must be echoed in X-CSRF-Token header.
	http.SetCookie(w, &http.Cookie{
		Name:     names.csrf,
		Value:    csrfToken,
		Path:     "/",
		Domain:   h.cookieCfg.Domain,
		HttpOnly: false,
		Secure:   h.cookieCfg.Secure,
		SameSite: h.cookieCfg.SameSite,
		MaxAge:   int(h.cookieCfg.RefreshTTL.Seconds()),
	})
}

func (h *AuthHandler) clearAuthCookies(w http.ResponseWriter) {
	names := h.currentCookieNames()

	for _, name := range []string{
		names.access, names.refresh, names.csrf,
		legacyAccessCookieName, legacyRefreshCookieName, legacyCSRFCookieName,
		hostAccessCookieName, hostRefreshCookieName, hostCSRFCookieName,
	} {
		http.SetCookie(w, &http.Cookie{
			Name:     name,
			Value:    "",
			Path:     "/",
			Domain:   h.cookieCfg.Domain,
			HttpOnly: name != names.csrf && name != legacyCSRFCookieName && name != hostCSRFCookieName,
			Secure:   h.cookieCfg.Secure,
			SameSite: h.cookieCfg.SameSite,
			MaxAge:   -1,
		})
	}
}

func (h *AuthHandler) currentCookieNames() cookieNames {
	if h.cookieCfg.Secure && strings.TrimSpace(h.cookieCfg.Domain) == "" {
		return cookieNames{
			access:  hostAccessCookieName,
			refresh: hostRefreshCookieName,
			csrf:    hostCSRFCookieName,
		}
	}
	return cookieNames{
		access:  legacyAccessCookieName,
		refresh: legacyRefreshCookieName,
		csrf:    legacyCSRFCookieName,
	}
}

func (h *AuthHandler) readRefreshTokenCookie(r *http.Request) (string, error) {
	names := []string{hostRefreshCookieName, legacyRefreshCookieName}
	for _, name := range names {
		c, err := r.Cookie(name)
		if err == nil && strings.TrimSpace(c.Value) != "" {
			return strings.TrimSpace(c.Value), nil
		}
	}
	return "", http.ErrNoCookie
}

func (h *AuthHandler) hasAccessCookie(r *http.Request) bool {
	for _, name := range []string{hostAccessCookieName, legacyAccessCookieName} {
		if c, err := r.Cookie(name); err == nil && strings.TrimSpace(c.Value) != "" {
			return true
		}
	}
	return false
}

func (h *AuthHandler) hasRefreshCookie(r *http.Request) bool {
	for _, name := range []string{hostRefreshCookieName, legacyRefreshCookieName} {
		if c, err := r.Cookie(name); err == nil && strings.TrimSpace(c.Value) != "" {
			return true
		}
	}
	return false
}

func (h *AuthHandler) readCSRFCookie(r *http.Request) (string, error) {
	for _, name := range []string{hostCSRFCookieName, legacyCSRFCookieName} {
		if c, err := r.Cookie(name); err == nil && strings.TrimSpace(c.Value) != "" {
			return strings.TrimSpace(c.Value), nil
		}
	}
	return "", http.ErrNoCookie
}

func (h *AuthHandler) validateBrowserCSRF(r *http.Request) error {
	origin := normalizeOrigin(r.Header.Get("Origin"))
	if origin == "" {
		return errors.New("missing origin header")
	}
	if !h.originAllowAny {
		if _, ok := h.originAllowSet[origin]; !ok {
			return errors.New("origin is not allowed")
		}
	}

	csrfCookie, err := h.readCSRFCookie(r)
	if err != nil {
		return errors.New("missing csrf cookie")
	}
	csrfHeader := strings.TrimSpace(r.Header.Get(csrfHeaderName))
	if csrfHeader == "" {
		return errors.New("missing csrf header")
	}
	if subtle.ConstantTimeCompare([]byte(csrfCookie), []byte(csrfHeader)) != 1 {
		return errors.New("csrf token mismatch")
	}
	return nil
}

func normalizeOrigin(raw string) string {
	trimmed := strings.TrimRight(strings.TrimSpace(raw), "/")
	if trimmed == "" {
		return ""
	}

	parsed, err := url.Parse(trimmed)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return strings.ToLower(trimmed)
	}

	return strings.ToLower(parsed.Scheme) + "://" + strings.ToLower(parsed.Host)
}

func (h *AuthHandler) resolveClientIP(r *http.Request) string {
	if h.ipResolver == nil {
		return ""
	}
	return h.ipResolver.Resolve(r)
}

func generateCSRFToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
