package webserver

import (
	"net"
	"net/http"
	"sync"
	"time"
)

// SecurityHeaders sets response headers that are cheap defense-in-depth
// regardless of what TLS-terminating reverse proxy sits in front of this
// app: clickjacking, MIME-sniffing, and (once served over HTTPS) protocol
// downgrade protection.
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "same-origin")
		if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
			h.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}
		next.ServeHTTP(w, r)
	})
}

// LoginRateLimiter throttles login attempts per source IP to blunt online
// password guessing. It's a simple in-memory fixed-window limiter — fine for
// a single-instance deployment; would need a shared store (e.g. Redis) if
// this app is ever run behind a load balancer with multiple instances.
type LoginRateLimiter struct {
	mu       sync.Mutex
	attempts map[string][]time.Time
	limit    int
	window   time.Duration
}

func NewLoginRateLimiter(limit int, window time.Duration) *LoginRateLimiter {
	return &LoginRateLimiter{
		attempts: make(map[string][]time.Time),
		limit:    limit,
		window:   window,
	}
}

func (l *LoginRateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := clientIP(r)

		l.mu.Lock()
		now := time.Now()
		cutoff := now.Add(-l.window)
		attempts := l.attempts[key][:0]
		for _, t := range l.attempts[key] {
			if t.After(cutoff) {
				attempts = append(attempts, t)
			}
		}
		if len(attempts) >= l.limit {
			l.attempts[key] = attempts
			l.mu.Unlock()
			http.Error(w, "too many login attempts, please try again later", http.StatusTooManyRequests)
			return
		}
		l.attempts[key] = append(attempts, now)
		l.mu.Unlock()

		next.ServeHTTP(w, r)
	})
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
