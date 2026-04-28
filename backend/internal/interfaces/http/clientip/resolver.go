package clientip

import (
	"net"
	"net/http"
	"strings"
)

type Resolver struct {
	trustedCIDRs []*net.IPNet
}

func NewResolver(trustedProxyCIDRs []string) (*Resolver, error) {
	resolver := &Resolver{
		trustedCIDRs: make([]*net.IPNet, 0, len(trustedProxyCIDRs)),
	}

	for _, raw := range trustedProxyCIDRs {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			continue
		}
		if strings.Contains(trimmed, "/") {
			_, network, err := net.ParseCIDR(trimmed)
			if err != nil {
				return nil, err
			}
			resolver.trustedCIDRs = append(resolver.trustedCIDRs, network)
			continue
		}

		ip := net.ParseIP(trimmed)
		if ip == nil {
			return nil, &net.ParseError{Type: "IP address", Text: trimmed}
		}

		maskBits := 32
		if ip.To4() == nil {
			maskBits = 128
		}
		network := &net.IPNet{
			IP:   ip,
			Mask: net.CIDRMask(maskBits, maskBits),
		}
		resolver.trustedCIDRs = append(resolver.trustedCIDRs, network)
	}

	return resolver, nil
}

func (r *Resolver) Resolve(req *http.Request) string {
	remoteIP := extractRemoteIP(req.RemoteAddr)
	if remoteIP == "" {
		return ""
	}

	if !r.isTrustedProxy(remoteIP) {
		return remoteIP
	}

	forwardedChain := parseForwardedHeader(req.Header.Values("Forwarded"))
	if len(forwardedChain) == 0 {
		forwardedChain = parseXForwardedFor(req.Header.Values("X-Forwarded-For"))
	}
	if len(forwardedChain) > 0 {
		if forwardedClient := r.pickClientFromChain(forwardedChain); forwardedClient != "" {
			return forwardedClient
		}
	}

	for _, headerName := range []string{"CF-Connecting-IP", "True-Client-IP", "X-Client-IP"} {
		if headerIP := parseIP(req.Header.Get(headerName)); headerIP != "" {
			return headerIP
		}
	}

	if realIP := parseIP(req.Header.Get("X-Real-IP")); realIP != "" {
		return realIP
	}

	return remoteIP
}

func (r *Resolver) isTrustedProxy(ipRaw string) bool {
	if len(r.trustedCIDRs) == 0 {
		return false
	}

	ip := net.ParseIP(ipRaw)
	if ip == nil {
		return false
	}
	for _, network := range r.trustedCIDRs {
		if network.Contains(ip) {
			return true
		}
	}
	return false
}

func extractRemoteIP(remoteAddr string) string {
	host, _, err := net.SplitHostPort(strings.TrimSpace(remoteAddr))
	if err != nil {
		return parseIP(remoteAddr)
	}
	return parseIP(host)
}

func parseIP(raw string) string {
	trimmed := strings.TrimSpace(raw)
	trimmed = strings.Trim(trimmed, "\"")
	trimmed = strings.TrimPrefix(trimmed, "for=")

	if strings.HasPrefix(trimmed, "[") {
		if host, _, err := net.SplitHostPort(trimmed); err == nil {
			trimmed = host
		}
	} else if strings.Count(trimmed, ":") == 1 {
		if host, _, err := net.SplitHostPort(trimmed); err == nil {
			trimmed = host
		}
	}
	trimmed = strings.Trim(trimmed, "[]")

	ip := net.ParseIP(strings.TrimSpace(trimmed))
	if ip == nil {
		return ""
	}
	return ip.String()
}

func (r *Resolver) pickClientFromChain(chain []string) string {
	if len(chain) == 0 {
		return ""
	}

	// Walk right-to-left, skipping trusted proxies.
	for i := len(chain) - 1; i >= 0; i-- {
		candidate := chain[i]
		if !r.isTrustedProxy(candidate) {
			return candidate
		}
	}

	// If every value in chain is trusted, fallback to more explicit headers.
	return ""
}

func parseXForwardedFor(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		parts := strings.Split(value, ",")
		for _, part := range parts {
			if ip := parseIP(part); ip != "" {
				out = append(out, ip)
			}
		}
	}
	return out
}

func parseForwardedHeader(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		parts := strings.Split(value, ",")
		for _, part := range parts {
			segments := strings.Split(part, ";")
			for _, segment := range segments {
				kv := strings.SplitN(strings.TrimSpace(segment), "=", 2)
				if len(kv) != 2 {
					continue
				}
				if !strings.EqualFold(strings.TrimSpace(kv[0]), "for") {
					continue
				}
				if ip := parseIP(kv[1]); ip != "" {
					out = append(out, ip)
				}
			}
		}
	}
	return out
}
