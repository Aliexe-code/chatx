package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var (
	ErrInvalidToken = errors.New("invalid token")
	ErrExpiredToken = errors.New("token has expired")
)

// Claims represents the JWT claims structure
type Claims struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	jwt.RegisteredClaims
}

// JWTService handles JWT token generation and validation
type JWTService struct {
	secretKey      []byte
	expiryDuration time.Duration
}

// NewJWTService creates a new JWT service instance
func NewJWTService(secret string, expiry string) (*JWTService, error) {
	if len(secret) < 32 {
		return nil, errors.New("JWT secret must be at least 32 characters")
	}

	duration, err := time.ParseDuration(expiry)
	if err != nil {
		// Default to 24 hours if parsing fails
		duration = 24 * time.Hour
	}

	return &JWTService{
		secretKey:      []byte(secret),
		expiryDuration: duration,
	}, nil
}

// GenerateToken generates a new JWT token for a user
func (j *JWTService) GenerateToken(userID, username string) (string, error) {
	now := time.Now()
	claims := Claims{
		UserID:   userID,
		Username: username,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(j.expiryDuration)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(j.secretKey)
	if err != nil {
		return "", err
	}

	return tokenString, nil
}

// ValidateToken validates a JWT token and returns the claims
func (j *JWTService) ValidateToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		// Validate signing method
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidToken
		}
		return j.secretKey, nil
	})

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrExpiredToken
		}
		return nil, ErrInvalidToken
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, ErrInvalidToken
	}

	return claims, nil
}

// RefreshToken generates a new token from an existing valid token
func (j *JWTService) RefreshToken(tokenString string) (string, error) {
	claims, err := j.ValidateToken(tokenString)
	if err != nil {
		return "", err
	}

	// Generate new token with same user info
	return j.GenerateToken(claims.UserID, claims.Username)
}

// GetUserID extracts user ID from token without full validation (for performance)
// Note: This should only be used after ValidateToken has been called
func GetUserIDFromClaims(claims *Claims) string {
	return claims.UserID
}

// GetUsername extracts username from token without full validation (for performance)
// Note: This should only be used after ValidateToken has been called
func GetUsernameFromClaims(claims *Claims) string {
	return claims.Username
}
