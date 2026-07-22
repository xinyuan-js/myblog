package httpapi

import (
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

func securityHeadersMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
		c.Header("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		c.Header("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'; base-uri 'none'")
		c.Next()
	}
}

type fixedWindowLimiter struct {
	mu      sync.Mutex
	limit   int
	window  time.Duration
	clients map[string]windowCounter
}

const maxRateLimitClients = 4096

type windowCounter struct {
	started time.Time
	count   int
}

func newRateLimiter(limit int, window time.Duration) gin.HandlerFunc {
	limiter := &fixedWindowLimiter{limit: limit, window: window, clients: make(map[string]windowCounter)}
	return func(c *gin.Context) {
		blocked, retry := limiter.record(time.Now(), c.ClientIP())
		if blocked {
			c.Header("Retry-After", strconv.Itoa(retry))
			writeError(c, http.StatusTooManyRequests, "RATE_LIMITED", "请求过于频繁，请稍后重试")
			return
		}
		c.Next()
	}
}

func (limiter *fixedWindowLimiter) record(now time.Time, key string) (blocked bool, retry int) {
	limiter.mu.Lock()
	defer limiter.mu.Unlock()
	// Keep the in-memory limiter bounded even when many distinct clients hit
	// the service in one window. Without eviction, a rotating set of source
	// addresses could grow this map until the process runs out of memory.
	if _, exists := limiter.clients[key]; !exists && len(limiter.clients) >= maxRateLimitClients {
		oldestKey := ""
		var oldest time.Time
		for client, value := range limiter.clients {
			if now.Sub(value.started) >= limiter.window {
				delete(limiter.clients, client)
				continue
			}
			if oldestKey == "" || value.started.Before(oldest) {
				oldestKey, oldest = client, value.started
			}
		}
		if len(limiter.clients) >= maxRateLimitClients && oldestKey != "" {
			delete(limiter.clients, oldestKey)
		}
	}
	entry := limiter.clients[key]
	if entry.started.IsZero() || now.Sub(entry.started) >= limiter.window {
		entry = windowCounter{started: now}
	}
	entry.count++
	limiter.clients[key] = entry
	blocked = entry.count > limiter.limit
	retry = int(entry.started.Add(limiter.window).Sub(now).Seconds())
	if retry < 1 {
		retry = 1
	}
	return blocked, retry
}
