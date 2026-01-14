package server

import (
	"sync"
	"time"

	"websocket-demo/internal/client"
)

// WebSocketRateLimiter manages per-client WebSocket message rate limiting
type WebSocketRateLimiter struct {
	clients map[string]*clientRateLimit
	mu      sync.RWMutex
}

// clientRateLimit tracks message rate for a specific client
type clientRateLimit struct {
	messages    []time.Time
	windowStart time.Time
	mu          sync.Mutex
}

const (
	// MaxMessagesPerSecond is the maximum number of messages allowed per second
	MaxMessagesPerSecond = 10
	// RateLimitWindow is the time window for rate limiting (1 second)
	RateLimitWindow = time.Second
)

// NewWebSocketRateLimiter creates a new WebSocket rate limiter
func NewWebSocketRateLimiter() *WebSocketRateLimiter {
	return &WebSocketRateLimiter{
		clients: make(map[string]*clientRateLimit),
	}
}

// CheckRateLimit checks if a client has exceeded the rate limit
// Returns true if rate limit exceeded, false otherwise
func (w *WebSocketRateLimiter) CheckRateLimit(client *client.Client) bool {
	clientID := client.UserID

	w.mu.RLock()
	limiter, exists := w.clients[clientID]
	w.mu.RUnlock()

	if !exists {
		w.mu.Lock()
		limiter = &clientRateLimit{
			messages:    make([]time.Time, 0),
			windowStart: time.Now(),
		}
		w.clients[clientID] = limiter
		w.mu.Unlock()
	}

	limiter.mu.Lock()
	defer limiter.mu.Unlock()

	now := time.Now()

	// Reset window if more than 1 second has passed
	if now.Sub(limiter.windowStart) >= RateLimitWindow {
		limiter.messages = limiter.messages[:0]
		limiter.windowStart = now
	}

	// Check if rate limit exceeded
	if len(limiter.messages) >= MaxMessagesPerSecond {
		return true
	}

	// Add current message timestamp
	limiter.messages = append(limiter.messages, now)

	return false
}

// RemoveClient removes a client from rate limiting when they disconnect
func (w *WebSocketRateLimiter) RemoveClient(clientID string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	delete(w.clients, clientID)
}

// GetMessageCount returns the number of messages sent by a client in the current window
func (w *WebSocketRateLimiter) GetMessageCount(clientID string) int {
	w.mu.RLock()
	limiter, exists := w.clients[clientID]
	w.mu.RUnlock()

	if !exists {
		return 0
	}

	limiter.mu.Lock()
	defer limiter.mu.Unlock()

	now := time.Now()

	// Clean up old messages outside the window
	validMessages := make([]time.Time, 0)
	for _, msgTime := range limiter.messages {
		if now.Sub(msgTime) < RateLimitWindow {
			validMessages = append(validMessages, msgTime)
		}
	}
	limiter.messages = validMessages

	return len(limiter.messages)
}

// CleanupExpiredClients removes clients that haven't sent messages recently
func (w *WebSocketRateLimiter) CleanupExpiredClients() {
	w.mu.Lock()
	defer w.mu.Unlock()

	now := time.Now()
	for clientID, limiter := range w.clients {
		limiter.mu.Lock()
		// Remove clients that haven't sent messages in the last 5 minutes
		if now.Sub(limiter.windowStart) > 5*time.Minute {
			delete(w.clients, clientID)
		}
		limiter.mu.Unlock()
	}
}