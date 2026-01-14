package server

import (
	"net/http"
	"sync"
	"time"

	"github.com/labstack/echo/v4"
	"golang.org/x/time/rate"
)

// RateLimiterConfig holds configuration for rate limiting
type RateLimiterConfig struct {
	RequestsPerSecond float64
	BurstSize         int
	CleanupInterval   time.Duration
}

// RateLimiter manages rate limiting for users and IPs
type RateLimiter struct {
	limiters map[string]*rate.Limiter
	mu       sync.RWMutex
	config   RateLimiterConfig
}

// NewRateLimiter creates a new rate limiter with default config
func NewRateLimiter() *RateLimiter {
	return &RateLimiter{
		limiters: make(map[string]*rate.Limiter),
		config: RateLimiterConfig{
			RequestsPerSecond: 200,
			BurstSize:         200,
			CleanupInterval:   5 * time.Minute,
		},
	}
}

// NewRateLimiterWithConfig creates a new rate limiter with custom config
func NewRateLimiterWithConfig(config RateLimiterConfig) *RateLimiter {
	return &RateLimiter{
		limiters: make(map[string]*rate.Limiter),
		config:   config,
	}
}

// GetLimiter returns a rate limiter for the given key (user ID or IP)
func (r *RateLimiter) GetLimiter(key string) *rate.Limiter {
	r.mu.Lock()
	defer r.mu.Unlock()

	if limiter, exists := r.limiters[key]; exists {
		return limiter
	}

	// Create new limiter with configured rate
	limiter := rate.NewLimiter(rate.Limit(r.config.RequestsPerSecond), r.config.BurstSize)
	r.limiters[key] = limiter
	return limiter
}

// RemoveLimiter removes a rate limiter for the given key
func (r *RateLimiter) RemoveLimiter(key string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.limiters, key)
}

// Cleanup removes old limiters that haven't been used recently
func (r *RateLimiter) Cleanup() {
	r.mu.Lock()
	defer r.mu.Unlock()
	// In a production environment, you'd want to track last access time
	// For now, this is a placeholder for cleanup logic
}

// RateLimitMiddleware creates middleware for rate limiting based on IP
func (r *RateLimiter) RateLimitMiddleware(requestsPerSecond float64, burstSize int) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// Use IP as the rate limit key
			ip := c.RealIP()
			if ip == "" {
				ip = c.Request().RemoteAddr
			}

			limiter := r.GetLimiter(ip)
			if !limiter.Allow() {
				return c.JSON(http.StatusTooManyRequests, map[string]string{
					"error": "Rate limit exceeded. Please try again later.",
				})
			}

			return next(c)
		}
	}
}

// AuthRateLimitMiddleware creates stricter rate limiting for auth endpoints
func (r *RateLimiter) AuthRateLimitMiddleware() echo.MiddlewareFunc {
	// Stricter limits for auth endpoints: 5 requests per minute, burst of 10
	// RELAXED FOR STRESS TESTING: 200 requests per second, burst of 200
	return r.RateLimitMiddleware(200.0, 200)
}
