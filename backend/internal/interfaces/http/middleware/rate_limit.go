package middleware

import (
	"net"
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

type RateLimiter struct {
	mu      sync.Mutex
	clients map[string]*clientLimiter
	rate    rate.Limit
	burst   int
	ttl     time.Duration
	ipr     ClientIPResolver
}

type clientLimiter struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

type ClientIPResolver interface {
	Resolve(req *http.Request) string
}

func NewRateLimiter(limit int, window time.Duration, resolver ClientIPResolver) *RateLimiter {
	if limit < 1 {
		limit = 1
	}
	if window <= 0 {
		window = time.Minute
	}

	r := &RateLimiter{
		clients: make(map[string]*clientLimiter),
		rate:    rate.Every(window / time.Duration(limit)),
		burst:   limit,
		ttl:     10 * time.Minute,
		ipr:     resolver,
	}

	go r.cleanup()
	return r
}

func (r *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		key := r.resolveClientIP(req)
		limiter := r.getLimiter(key)
		if !limiter.Allow() {
			http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
			return
		}

		next.ServeHTTP(w, req)
	})
}

func (r *RateLimiter) getLimiter(key string) *rate.Limiter {
	r.mu.Lock()
	defer r.mu.Unlock()

	cl, ok := r.clients[key]
	if !ok {
		lim := rate.NewLimiter(r.rate, r.burst)
		r.clients[key] = &clientLimiter{limiter: lim, lastSeen: time.Now()}
		return lim
	}

	cl.lastSeen = time.Now()
	return cl.limiter
}

func (r *RateLimiter) cleanup() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	for now := range ticker.C {
		r.mu.Lock()
		for key, cl := range r.clients {
			if now.Sub(cl.lastSeen) > r.ttl {
				delete(r.clients, key)
			}
		}
		r.mu.Unlock()
	}
}

func (r *RateLimiter) resolveClientIP(req *http.Request) string {
	if r.ipr == nil {
		host, _, err := net.SplitHostPort(req.RemoteAddr)
		if err != nil {
			return req.RemoteAddr
		}
		return host
	}
	ip := r.ipr.Resolve(req)
	if ip == "" {
		host, _, err := net.SplitHostPort(req.RemoteAddr)
		if err != nil {
			return req.RemoteAddr
		}
		return host
	}
	return ip
}
