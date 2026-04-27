package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type staticIPResolver struct {
	ip string
}

func (s staticIPResolver) Resolve(*http.Request) string {
	return s.ip
}

func TestSetAuthCookiesUsesHostPrefixWhenEligible(t *testing.T) {
	handler := NewAuthHandler(nil, nil, AuthCookieConfig{
		Domain:         "",
		Secure:         true,
		SameSite:       http.SameSiteLaxMode,
		AccessTTL:      15 * time.Minute,
		RefreshTTL:     24 * time.Hour,
		AllowedOrigins: []string{"https://cloud.example.com"},
	}, staticIPResolver{ip: "127.0.0.1"})

	rec := httptest.NewRecorder()
	handler.setAuthCookies(rec, "access", "refresh", "csrf-token")

	headers := rec.Header()["Set-Cookie"]
	joined := strings.Join(headers, "\n")
	if !strings.Contains(joined, "__Host-cloud_access_token=") {
		t.Fatalf("expected __Host access cookie, got headers: %v", headers)
	}
	if !strings.Contains(joined, "__Host-cloud_refresh_token=") {
		t.Fatalf("expected __Host refresh cookie, got headers: %v", headers)
	}
	if !strings.Contains(joined, "__Host-cloud_csrf_token=") {
		t.Fatalf("expected __Host csrf cookie, got headers: %v", headers)
	}
}

func TestValidateBrowserCSRFFailsForDisallowedOrigin(t *testing.T) {
	handler := NewAuthHandler(nil, nil, AuthCookieConfig{
		Domain:         "",
		Secure:         true,
		SameSite:       http.SameSiteLaxMode,
		AccessTTL:      15 * time.Minute,
		RefreshTTL:     24 * time.Hour,
		AllowedOrigins: []string{"https://allowed.example.com"},
	}, staticIPResolver{ip: "127.0.0.1"})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", nil)
	req.Header.Set("Origin", "https://blocked.example.com")
	req.Header.Set(csrfHeaderName, "token-a")
	req.AddCookie(&http.Cookie{Name: hostCSRFCookieName, Value: "token-a"})

	err := handler.validateBrowserCSRF(req)
	if err == nil {
		t.Fatalf("expected disallowed origin to fail csrf validation")
	}
}
