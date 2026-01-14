package server

import (
	"crypto/rand"
	"encoding/hex"
	"log"
	"sync"
	"time"

	"websocket-demo/internal/auth"

	"github.com/labstack/echo/v4"
)

// CSRFToken represents a CSRF token with expiration
type CSRFToken struct {
	Token      string
	ExpiresAt  time.Time
	UserID     string
	IPAddress  string
	UserAgent  string
}

// CSRFProtection manages CSRF tokens
type CSRFProtection struct {
	tokens map[string]*CSRFToken
	mu     sync.RWMutex
}

// NewCSRFProtection creates a new CSRF protection instance
func NewCSRFProtection() *CSRFProtection {
	csrf := &CSRFProtection{
		tokens: make(map[string]*CSRFToken),
	}
	// Start cleanup goroutine
	go csrf.cleanupExpiredTokens()
	return csrf
}

// GenerateToken generates a new CSRF token for a user
func (c *CSRFProtection) GenerateToken(userID, ipAddress, userAgent string) string {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Generate random token
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		// Fallback to time-based token if crypto/rand fails
		return hex.EncodeToString([]byte(time.Now().String() + userID))
	}

	token := hex.EncodeToString(bytes)

	// Store token with expiration (1 hour)
	c.tokens[token] = &CSRFToken{
		Token:     token,
		ExpiresAt: time.Now().Add(time.Hour),
		UserID:    userID,
		IPAddress: ipAddress,
		UserAgent: userAgent,
	}

	return token
}

// ValidateToken validates a CSRF token
func (c *CSRFProtection) ValidateToken(token, userID, ipAddress, userAgent string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	storedToken, exists := c.tokens[token]
	if !exists {
		return false
	}

	// Check expiration
	if time.Now().After(storedToken.ExpiresAt) {
		return false
	}

	// Validate user ID
	if storedToken.UserID != userID {
		return false
	}

	// Validate IP address (optional, can be disabled for mobile users)
	// if storedToken.IPAddress != ipAddress {
	// 	return false
	// }

	// Validate user agent (optional, can be disabled)
	// if storedToken.UserAgent != userAgent {
	// 	return false
	// }

	return true
}

// RevokeToken revokes a CSRF token
func (c *CSRFProtection) RevokeToken(token string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.tokens, token)
}

// RevokeUserTokens revokes all tokens for a user
func (c *CSRFProtection) RevokeUserTokens(userID string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for token, csrfToken := range c.tokens {
		if csrfToken.UserID == userID {
			delete(c.tokens, token)
		}
	}
}

// cleanupExpiredTokens removes expired tokens
func (c *CSRFProtection) cleanupExpiredTokens() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		c.mu.Lock()
		now := time.Now()
		for token, csrfToken := range c.tokens {
			if now.After(csrfToken.ExpiresAt) {
				delete(c.tokens, token)
			}
		}
		c.mu.Unlock()
	}
}

// CSRFMiddleware validates CSRF tokens for state-changing operations
func (s *Server) CSRFMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		// Only validate CSRF for state-changing methods
		method := c.Request().Method
		path := c.Request().URL.Path
		
		log.Printf("CSRF middleware: method=%s, path=%s", method, path)
		
		if method != "POST" && method != "PUT" && method != "DELETE" && method != "PATCH" {
			log.Printf("CSRF middleware: skipping non-state-changing method %s", method)
			return next(c)
		}

		// Skip CSRF validation for WebSocket upgrade
		if c.Request().Header.Get("Upgrade") == "websocket" {
			return next(c)
		}

		// Skip CSRF validation for login and register endpoints
		if path == "/api/login" || path == "/api/register" {
			return next(c)
		}

		// Get user from context
		claims := c.Get("user")
		if claims == nil {
			return c.JSON(401, map[string]string{"error": "Not authenticated"})
		}

		// Type assertion with safety check
		authClaims, ok := claims.(*auth.Claims)
		if !ok {
			log.Printf("CSRF middleware: invalid claims type")
			return c.JSON(401, map[string]string{"error": "Invalid authentication"})
		}

		// Get CSRF token from header
		csrfToken := c.Request().Header.Get("X-CSRF-Token")
		if csrfToken == "" {
			return c.JSON(403, map[string]string{"error": "CSRF token required"})
		}

		// Validate token
		if !s.csrf.ValidateToken(csrfToken, authClaims.UserID, c.RealIP(), c.Request().UserAgent()) {
			return c.JSON(403, map[string]string{"error": "Invalid CSRF token"})
		}

		return next(c)
	}
}