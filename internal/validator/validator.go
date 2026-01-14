package validator

import (
	"fmt"
	"log"
	"os"
	"regexp"
	"strconv"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

var (
	// Email regex pattern
	emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)

	// Username regex - alphanumeric, underscores, hyphens, 3-30 characters
	usernameRegex = regexp.MustCompile(`^[a-zA-Z0-9_-]{3,30}$`)

	// Password requirements
	minPasswordLength = 8
)

// ValidationError represents a validation error
type ValidationError struct {
	Field   string
	Message string
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// ValidationResult contains validation errors
type ValidationResult struct {
	Errors []ValidationError
	Valid  bool
}

// ValidateEmail validates email format
func ValidateEmail(email string) error {
	email = strings.TrimSpace(strings.ToLower(email))

	if email == "" {
		return ValidationError{Field: "email", Message: "email is required"}
	}

	if len(email) > 255 {
		return ValidationError{Field: "email", Message: "email must be less than 255 characters"}
	}

	if !emailRegex.MatchString(email) {
		return ValidationError{Field: "email", Message: "invalid email format"}
	}

	return nil
}

// ValidateUsername validates username format
func ValidateUsername(username string) error {
	username = strings.TrimSpace(username)

	if username == "" {
		return ValidationError{Field: "username", Message: "username is required"}
	}

	if len(username) < 3 {
		return ValidationError{Field: "username", Message: "username must be at least 3 characters"}
	}

	if len(username) > 30 {
		return ValidationError{Field: "username", Message: "username must be less than 30 characters"}
	}

	if !usernameRegex.MatchString(username) {
		return ValidationError{Field: "username", Message: "username can only contain letters, numbers, underscores, and hyphens"}
	}

	// Check for reserved names
	reservedNames := []string{"admin", "root", "system", "api", "www", "mail", "support", "info", "about"}
	for _, reserved := range reservedNames {
		if strings.ToLower(username) == reserved {
			return ValidationError{Field: "username", Message: "username is reserved"}
		}
	}

	return nil
}

// ValidatePassword validates password strength
func ValidatePassword(password string) error {
	if password == "" {
		return ValidationError{Field: "password", Message: "password is required"}
	}

	if len(password) < minPasswordLength {
		return ValidationError{Field: "password", Message: fmt.Sprintf("password must be at least %d characters", minPasswordLength)}
	}

	if len(password) > 128 {
		return ValidationError{Field: "password", Message: "password must be less than 128 characters"}
	}

	// Check for common weak passwords
	weakPasswords := []string{"password", "123456", "qwerty", "abc123", "password123", "admin123"}
	for _, weak := range weakPasswords {
		if strings.ToLower(password) == weak {
			return ValidationError{Field: "password", Message: "password is too common"}
		}
	}

	return nil
}

// ValidateRoomName validates room name
func ValidateRoomName(name string) error {
	name = strings.TrimSpace(name)

	if name == "" {
		return ValidationError{Field: "name", Message: "room name is required"}
	}

	if len(name) < 2 {
		return ValidationError{Field: "name", Message: "room name must be at least 2 characters"}
	}

	if len(name) > 50 {
		return ValidationError{Field: "name", Message: "room name must be less than 50 characters"}
	}

	// Allow letters, numbers, spaces, hyphens, underscores
	roomNameRegex := regexp.MustCompile(`^[a-zA-Z0-9\s_-]{2,50}$`)
	if !roomNameRegex.MatchString(name) {
		return ValidationError{Field: "name", Message: "room name can only contain letters, numbers, spaces, hyphens, and underscores"}
	}

	return nil
}

// ValidateRoomPassword validates room password
func ValidateRoomPassword(password string) error {
	if password == "" {
		return nil // Empty password is allowed for public rooms
	}

	if len(password) < 4 {
		return ValidationError{Field: "password", Message: "room password must be at least 4 characters"}
	}

	if len(password) > 64 {
		return ValidationError{Field: "password", Message: "room password must be less than 64 characters"}
	}

	return nil
}

// SanitizeInput removes potentially dangerous characters
func SanitizeInput(input string) string {
	// Remove HTML tags and scripts
	htmlTagRegex := regexp.MustCompile(`<[^>]*>`)
	sanitized := htmlTagRegex.ReplaceAllString(input, "")

	// Trim whitespace
	sanitized = strings.TrimSpace(sanitized)

	return sanitized
}

// ValidateRegistration validates user registration data
func ValidateRegistration(username, email, password string) *ValidationResult {
	result := &ValidationResult{Valid: true}

	if err := ValidateUsername(username); err != nil {
		result.Errors = append(result.Errors, err.(ValidationError))
		result.Valid = false
	}

	if err := ValidateEmail(email); err != nil {
		result.Errors = append(result.Errors, err.(ValidationError))
		result.Valid = false
	}

	if err := ValidatePassword(password); err != nil {
		result.Errors = append(result.Errors, err.(ValidationError))
		result.Valid = false
	}

	return result
}

// ValidateRoomCreation validates room creation data
func ValidateRoomCreation(name, password string, isPrivate bool) *ValidationResult {
	result := &ValidationResult{Valid: true}

	if err := ValidateRoomName(name); err != nil {
		result.Errors = append(result.Errors, err.(ValidationError))
		result.Valid = false
	}

	if isPrivate {
		if err := ValidateRoomPassword(password); err != nil {
			result.Errors = append(result.Errors, err.(ValidationError))
			result.Valid = false
		}
	}

	return result
}

// HashPassword creates a bcrypt hash of the password
func HashPassword(password string) (string, error) {
	if err := ValidatePassword(password); err != nil {
		return "", err
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("failed to hash password: %w", err)
	}

	return string(hash), nil
}

// Message size validation
const (
	MaxMessageSizeDefault = 64 * 1024 // 64KB default
	MinMessageSize        = 1
	MaxMessageSize        = 1024 * 1024 // 1MB maximum
)

// ValidateMessageSize checks if message size is within acceptable limits
func ValidateMessageSize(size int, maxSize int) error {
	if size < MinMessageSize {
		return ValidationError{
			Field:   "message",
			Message: "message cannot be empty",
		}
	}

	if size > maxSize {
		return ValidationError{
			Field:   "message",
			Message: fmt.Sprintf("message too large (max %d bytes)", maxSize),
		}
	}

	if size > MaxMessageSize {
		return ValidationError{
			Field:   "message",
			Message: fmt.Sprintf("message exceeds maximum allowed size (%d bytes)", MaxMessageSize),
		}
	}

	return nil
}

// GetMaxMessageSize reads message size limit from environment or returns default
func GetMaxMessageSize() int {
	if value := os.Getenv("WS_MAX_MESSAGE_SIZE"); value != "" {
		if size, err := strconv.Atoi(value); err == nil && size > 0 {
			if size > MaxMessageSize {
				log.Printf("WS_MAX_MESSAGE_SIZE (%d) exceeds maximum, using %d", size, MaxMessageSize)
				return MaxMessageSize
			}
			return size
		}
		log.Printf("Invalid WS_MAX_MESSAGE_SIZE, using default: %d", MaxMessageSizeDefault)
	}
	return MaxMessageSizeDefault
}

// FormatValidationErrors converts validation errors to a user-friendly message
func FormatValidationErrors(errors []ValidationError) string {
	if len(errors) == 0 {
		return ""
	}

	var messages []string
	for _, err := range errors {
		messages = append(messages, err.Message)
	}

	return strings.Join(messages, "; ")
}
