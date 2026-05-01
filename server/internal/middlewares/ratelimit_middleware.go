package middlewares

import (
	"net/http"
	"sync"
	"time"

	"github.com/your-username/go-mux-backend-template/server/internal/utils"
)

// rateLimitEntry tracks the request count and window start time for a single IP.
type rateLimitEntry struct {
	count     int
	windowEnd time.Time
}

// RateLimiter is a simple in-memory, per-IP sliding-window rate limiter.
// For production use, replace this with a Redis-backed implementation.
type RateLimiter struct {
	mu      sync.Mutex
	entries map[string]*rateLimitEntry
	limit   int           // max requests per window
	window  time.Duration // window duration
}

// NewRateLimiter creates a RateLimiter with the given request limit and time window.
// Example: NewRateLimiter(100, time.Minute) allows 100 requests per minute per IP.
func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	rl := &RateLimiter{
		entries: make(map[string]*rateLimitEntry),
		limit:   limit,
		window:  window,
	}
	// Background goroutine to evict stale entries and prevent unbounded memory growth
	go rl.cleanup()
	return rl
}

// Middleware returns an http.Handler middleware that enforces the rate limit.
func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := utils.GetIP(r)

		rl.mu.Lock()
		entry, ok := rl.entries[ip]
		now := time.Now()

		if !ok || now.After(entry.windowEnd) {
			rl.entries[ip] = &rateLimitEntry{count: 1, windowEnd: now.Add(rl.window)}
			rl.mu.Unlock()
			next.ServeHTTP(w, r)
			return
		}

		entry.count++
		if entry.count > rl.limit {
			rl.mu.Unlock()
			utils.NewHttpWriter(w, r).Status(http.StatusTooManyRequests).JSON(utils.M{
				"success": false,
				"message": "rate limit exceeded, please slow down",
			})
			return
		}
		rl.mu.Unlock()

		next.ServeHTTP(w, r)
	})
}

// cleanup periodically removes expired entries from the map.
func (rl *RateLimiter) cleanup() {
	ticker := time.NewTicker(rl.window)
	defer ticker.Stop()
	for range ticker.C {
		rl.mu.Lock()
		now := time.Now()
		for ip, e := range rl.entries {
			if now.After(e.windowEnd) {
				delete(rl.entries, ip)
			}
		}
		rl.mu.Unlock()
	}
}
