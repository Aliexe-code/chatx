package server

import (
	"net/http"
	"strings"

	"websocket-demo/internal/auth"

	"github.com/labstack/echo/v4"
)

// JWTMiddleware validates JWT tokens and adds user claims to context
func (s *Server) JWTMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		// Get Authorization header
		authHeader := c.Request().Header.Get("Authorization")
		if authHeader == "" {
			return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Authorization header required"})
		}

		// Check Bearer token format
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Invalid authorization header format"})
		}

		// Validate JWT token
		claims, err := s.jwtService.ValidateToken(parts[1])
		if err != nil {
			if err == auth.ErrExpiredToken {
				return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Token has expired"})
			}
			return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Invalid token"})
		}

		// Add user claims to context
		c.Set("user_id", claims.UserID)
		c.Set("username", claims.Username)
		c.Set("claims", claims)

		return next(c)
	}
}

// GetUserID retrieves user ID from context (must be used after JWTMiddleware)
func GetUserID(c echo.Context) string {
	if userID, ok := c.Get("user_id").(string); ok {
		return userID
	}
	return ""
}

// GetUsername retrieves username from context (must be used after JWTMiddleware)
func GetUsername(c echo.Context) string {
	if username, ok := c.Get("username").(string); ok {
		return username
	}
	return ""
}

// GetClaims retrieves claims from context (must be used after JWTMiddleware)
func GetClaims(c echo.Context) *auth.Claims {
	if claims, ok := c.Get("claims").(*auth.Claims); ok {
		return claims
	}
	return nil
}
