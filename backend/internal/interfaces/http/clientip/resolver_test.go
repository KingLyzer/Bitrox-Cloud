package clientip

import (
	"net/http"
	"testing"
)

func TestResolveUsesSocketIPWhenProxyUntrusted(t *testing.T) {
	resolver, err := NewResolver([]string{"10.0.0.0/8"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	req := &http.Request{
		RemoteAddr: "203.0.113.5:49152",
		Header: http.Header{
			"X-Forwarded-For": []string{"1.2.3.4"},
		},
	}

	ip := resolver.Resolve(req)
	if ip != "203.0.113.5" {
		t.Fatalf("expected socket ip, got %s", ip)
	}
}

func TestResolveTrustsForwardedHeadersFromTrustedProxy(t *testing.T) {
	resolver, err := NewResolver([]string{"10.0.0.0/8"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	req := &http.Request{
		RemoteAddr: "10.1.2.3:12345",
		Header: http.Header{
			"X-Forwarded-For": []string{"198.51.100.9, 10.1.2.3"},
		},
	}

	ip := resolver.Resolve(req)
	if ip != "198.51.100.9" {
		t.Fatalf("expected x-forwarded-for client ip, got %s", ip)
	}
}

func TestResolveIgnoresSpoofedXFFFromUntrustedSource(t *testing.T) {
	resolver, err := NewResolver([]string{"10.0.0.0/8"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	req := &http.Request{
		RemoteAddr: "203.0.113.20:4040",
		Header: http.Header{
			"X-Forwarded-For": []string{"198.51.100.10"},
			"X-Real-IP":       []string{"198.51.100.11"},
		},
	}

	ip := resolver.Resolve(req)
	if ip != "203.0.113.20" {
		t.Fatalf("expected remote addr ip, got %s", ip)
	}
}

func TestResolveChoosesFirstUntrustedFromRight(t *testing.T) {
	resolver, err := NewResolver([]string{"10.0.0.0/8", "172.16.0.0/12"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	req := &http.Request{
		RemoteAddr: "10.1.2.3:12345",
		Header: http.Header{
			"X-Forwarded-For": []string{"198.51.100.9, 172.16.1.4, 10.1.2.3"},
		},
	}

	ip := resolver.Resolve(req)
	if ip != "198.51.100.9" {
		t.Fatalf("expected public client ip, got %s", ip)
	}
}

func TestResolveSupportsForwardedHeader(t *testing.T) {
	resolver, err := NewResolver([]string{"10.0.0.0/8"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	req := &http.Request{
		RemoteAddr: "10.2.3.4:8080",
		Header: http.Header{
			"Forwarded": []string{`for=198.51.100.123;proto=https;by=10.2.3.4`},
		},
	}

	ip := resolver.Resolve(req)
	if ip != "198.51.100.123" {
		t.Fatalf("expected forwarded for value, got %s", ip)
	}
}

func TestResolveNormalizesIPv6WithPort(t *testing.T) {
	resolver, err := NewResolver([]string{"fd00::/8"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	req := &http.Request{
		RemoteAddr: "[fd00::1]:8443",
		Header: http.Header{
			"X-Forwarded-For": []string{"[2001:db8::100]:443, [fd00::1]:8443"},
		},
	}

	ip := resolver.Resolve(req)
	if ip != "2001:db8::100" {
		t.Fatalf("expected normalized ipv6, got %s", ip)
	}
}
