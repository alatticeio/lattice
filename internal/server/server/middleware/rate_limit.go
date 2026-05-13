package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

type rateLimiterEntry struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

type IPRateLimiter struct {
	mu       sync.RWMutex
	limiters map[string]*rateLimiterEntry
	stopCh   chan struct{}
}

func NewIPRateLimiter() *IPRateLimiter {
	rl := &IPRateLimiter{
		limiters: make(map[string]*rateLimiterEntry),
		stopCh:   make(chan struct{}),
	}
	go rl.cleanup(10 * time.Minute)
	return rl
}

func (rl *IPRateLimiter) Shutdown() {
	close(rl.stopCh)
}

func (rl *IPRateLimiter) Allow(key string, r rate.Limit, burst int) bool {
	rl.mu.Lock()
	entry, exists := rl.limiters[key]
	if !exists {
		entry = &rateLimiterEntry{limiter: rate.NewLimiter(r, burst)}
		rl.limiters[key] = entry
	}
	entry.lastSeen = time.Now()
	rl.mu.Unlock()
	return entry.limiter.Allow()
}

func (rl *IPRateLimiter) cleanup(ttl time.Duration) {
	ticker := time.NewTicker(ttl)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			rl.mu.Lock()
			for k, v := range rl.limiters {
				if time.Since(v.lastSeen) > ttl {
					delete(rl.limiters, k)
				}
			}
			rl.mu.Unlock()
		case <-rl.stopCh:
			return
		}
	}
}

func (rl *IPRateLimiter) Middleware(r rate.Limit, burst int) gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()
		if !rl.Allow(ip, r, burst) {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"code":    429,
				"message": "Too many requests, please try again later",
			})
			return
		}
		c.Next()
	}
}
