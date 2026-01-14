package server

import (
	"context"
	"log"
	"time"

	"github.com/labstack/echo/v4"
	"websocket-demo/internal/db"
)

// AuditEventType represents the type of audit event

type AuditEventType string



const (

	AuditEventLoginSuccess   AuditEventType = "login_success"

	AuditEventLoginFailure   AuditEventType = "login_failure"

	AuditEventLogout         AuditEventType = "logout"

	AuditEventRegister       AuditEventType = "register"

	AuditEventPasswordChange AuditEventType = "password_change"

	AuditEventUsernameChange AuditEventType = "username_change"

	AuditEventRoomCreate     AuditEventType = "room_create"

	AuditEventRoomDelete     AuditEventType = "room_delete"

	AuditEventTokenRefresh   AuditEventType = "token_refresh"

)

// AuditEvent represents an audit log entry
type AuditEvent struct {
	ID        string
	UserID    string
	Username  string
	EventType AuditEventType
	IPAddress string
	UserAgent string
	Details   map[string]interface{}
	Timestamp time.Time
}

// AuditLogger handles audit logging
type AuditLogger struct {
	pool *db.Queries
}

// NewAuditLogger creates a new audit logger
func NewAuditLogger(pool *db.Queries) *AuditLogger {
	return &AuditLogger{
		pool: pool,
	}
}

// LogEvent logs an audit event
func (a *AuditLogger) LogEvent(ctx context.Context, event AuditEvent) {
	// Log to stdout for now (can be extended to write to database or file)
	log.Printf("[AUDIT] %s | User: %s (%s) | IP: %s | Type: %s | Details: %v",
		event.Timestamp.Format(time.RFC3339),
		event.Username,
		event.UserID,
		event.IPAddress,
		event.EventType,
		event.Details,
	)

	// TODO: Store audit events in database when audit_logs table is created
	// This would require creating a migrations file and updating queries
}

// LogLoginSuccess logs a successful login
func (a *AuditLogger) LogLoginSuccess(ctx context.Context, userID, username, ipAddress, userAgent string) {
	a.LogEvent(ctx, AuditEvent{
		UserID:    userID,
		Username:  username,
		EventType: AuditEventLoginSuccess,
		IPAddress: ipAddress,
		UserAgent: userAgent,
		Details:   map[string]interface{}{},
		Timestamp: time.Now(),
	})
}

// LogLoginFailure logs a failed login attempt
func (a *AuditLogger) LogLoginFailure(ctx context.Context, email, ipAddress, userAgent string, reason string) {
	a.LogEvent(ctx, AuditEvent{
		UserID:    "",
		Username:  email,
		EventType: AuditEventLoginFailure,
		IPAddress: ipAddress,
		UserAgent: userAgent,
		Details:   map[string]interface{}{"email": email, "reason": reason},
		Timestamp: time.Now(),
	})
}

// LogLogout logs a logout event
func (a *AuditLogger) LogLogout(ctx context.Context, userID, username, ipAddress, userAgent string) {
	a.LogEvent(ctx, AuditEvent{
		UserID:    userID,
		Username:  username,
		EventType: AuditEventLogout,
		IPAddress: ipAddress,
		UserAgent: userAgent,
		Details:   map[string]interface{}{},
		Timestamp: time.Now(),
	})
}

// LogRegister logs a user registration
func (a *AuditLogger) LogRegister(ctx context.Context, userID, username, email, ipAddress, userAgent string) {
	a.LogEvent(ctx, AuditEvent{
		UserID:    userID,
		Username:  username,
		EventType: AuditEventRegister,
		IPAddress: ipAddress,
		UserAgent: userAgent,
		Details:   map[string]interface{}{"email": email},
		Timestamp: time.Now(),
	})
}

// LogPasswordChange logs a password change
func (a *AuditLogger) LogPasswordChange(ctx context.Context, userID, username, ipAddress, userAgent string) {
	a.LogEvent(ctx, AuditEvent{
		UserID:    userID,
		Username:  username,
		EventType: AuditEventPasswordChange,
		IPAddress: ipAddress,
		UserAgent: userAgent,
		Details:   map[string]interface{}{},
		Timestamp: time.Now(),
	})
}

// LogUsernameChange logs a username change
func (a *AuditLogger) LogUsernameChange(ctx context.Context, userID, username, oldUsername, newUsername, ipAddress, userAgent string) {
	a.LogEvent(ctx, AuditEvent{
		UserID:    userID,
		Username:  username,
		EventType: AuditEventUsernameChange,
		IPAddress: ipAddress,
		UserAgent: userAgent,
		Details:   map[string]interface{}{"old_username": oldUsername, "new_username": newUsername},
		Timestamp: time.Now(),
	})
}

// LogRoomCreate logs a room creation
func (a *AuditLogger) LogRoomCreate(ctx context.Context, userID, username, roomName, ipAddress, userAgent string) {
	a.LogEvent(ctx, AuditEvent{
		UserID:    userID,
		Username:  username,
		EventType: AuditEventRoomCreate,
		IPAddress: ipAddress,
		UserAgent: userAgent,
		Details:   map[string]interface{}{"room_name": roomName},
		Timestamp: time.Now(),
	})
}

// LogRoomDelete logs a room deletion
func (a *AuditLogger) LogRoomDelete(ctx context.Context, userID, username, roomName, ipAddress, userAgent string) {
	a.LogEvent(ctx, AuditEvent{
		UserID:    userID,
		Username:  username,
		EventType: AuditEventRoomDelete,
		IPAddress: ipAddress,
		UserAgent: userAgent,
		Details:   map[string]interface{}{"room_name": roomName},
		Timestamp: time.Now(),
	})
}

// LogTokenRefresh logs a token refresh
func (a *AuditLogger) LogTokenRefresh(ctx context.Context, userID, username, ipAddress, userAgent string) {
	a.LogEvent(ctx, AuditEvent{
		UserID:    userID,
		Username:  username,
		EventType: AuditEventTokenRefresh,
		IPAddress: ipAddress,
		UserAgent: userAgent,
		Details:   map[string]interface{}{},
		Timestamp: time.Now(),
	})
}

// Helper function to get client IP address
func GetClientIP(c echo.Context) string {
	ip := c.RealIP()
	if ip == "" {
		ip = c.Request().RemoteAddr
	}
	return ip
}

// Helper function to get user agent
func GetUserAgent(c echo.Context) string {
	return c.Request().UserAgent()
}
