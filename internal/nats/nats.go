package nats

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"websocket-demo/internal/types"

	"github.com/nats-io/nats.go"
)

// NATSMessage is a simplified message structure that can be marshaled to JSON
type NATSMessage struct {
	MessageID  string    `json:"message_id"`  // Unique ID to prevent re-broadcasting
	Content    []byte    `json:"content"`
	Type       string    `json:"type"`
	SenderID   string    `json:"sender_id"`
	SenderName string    `json:"sender_name"`
	RoomName   string    `json:"room_name,omitempty"`
	ServerID   string    `json:"server_id"`   // ID of the server that sent the message
	Timestamp  time.Time `json:"timestamp"`
}

// Client wraps NATS connection and provides messaging functionality
type Client struct {
	conn          *nats.Conn
	js            nats.JetStreamContext
	connected     bool
	serverID      string
	mu            sync.RWMutex
	ctx           context.Context
	cancel        context.CancelFunc
	reconnectChan chan struct{}
}

// Config holds NATS connection configuration
type Config struct {
	URL            string
	MaxReconnects  int
	ReconnectWait  time.Duration
	Timeout        time.Duration
	EnableJetStream bool
}

// NewClient creates a new NATS client with the given configuration
func NewClient(cfg Config) (*Client, error) {
	if cfg.URL == "" {
		cfg.URL = nats.DefaultURL
	}
	if cfg.MaxReconnects == 0 {
		cfg.MaxReconnects = 10
	}
	if cfg.ReconnectWait == 0 {
		cfg.ReconnectWait = 2 * time.Second
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 10 * time.Second
	}

	ctx, cancel := context.WithCancel(context.Background())

	// Generate a unique server ID
	serverID := fmt.Sprintf("server-%d", time.Now().UnixNano())

	client := &Client{
		ctx:           ctx,
		cancel:        cancel,
		reconnectChan: make(chan struct{}, 1),
		serverID:      serverID,
	}

	opts := []nats.Option{
		nats.Name("websocket-chat"),
		nats.MaxReconnects(cfg.MaxReconnects),
		nats.ReconnectWait(cfg.ReconnectWait),
		nats.Timeout(cfg.Timeout),
		nats.PingInterval(20 * time.Second),
		nats.MaxPingsOutstanding(5),
		nats.ReconnectHandler(func(nc *nats.Conn) {
			log.Printf("NATS reconnected to %s", nc.ConnectedUrl())
			client.mu.Lock()
			client.connected = true
			client.mu.Unlock()
			select {
			case client.reconnectChan <- struct{}{}:
			default:
			}
		}),
		nats.DisconnectErrHandler(func(nc *nats.Conn, err error) {
			log.Printf("NATS disconnected: %v", err)
			client.mu.Lock()
			client.connected = false
			client.mu.Unlock()
		}),
		nats.ClosedHandler(func(nc *nats.Conn) {
			log.Printf("NATS connection closed")
			client.mu.Lock()
			client.connected = false
			client.mu.Unlock()
		}),
		nats.ErrorHandler(func(nc *nats.Conn, sub *nats.Subscription, err error) {
			log.Printf("NATS error: %v", err)
		}),
	}

	// Connect to NATS
	conn, err := nats.Connect(cfg.URL, opts...)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to connect to NATS: %w", err)
	}

	client.conn = conn
	client.mu.Lock()
	client.connected = true
	client.mu.Unlock()

	log.Printf("Connected to NATS at %s", cfg.URL)

	// Initialize JetStream if enabled
	if cfg.EnableJetStream {
		js, err := conn.JetStream()
		if err != nil {
			client.Close()
			return nil, fmt.Errorf("failed to initialize JetStream: %w", err)
		}
		client.js = js
		log.Println("NATS JetStream initialized")
	}

	return client, nil
}

// Publish publishes a message to a NATS subject
func (c *Client) Publish(subject string, msg types.Message) error {
	c.mu.RLock()
	if !c.connected || c.conn == nil {
		c.mu.RUnlock()
		return fmt.Errorf("NATS not connected")
	}
	c.mu.RUnlock()

	// Generate unique message ID to prevent infinite loops
	messageID := fmt.Sprintf("%d-%s", time.Now().UnixNano(), msg.Type)

	// Convert types.Message to NATSMessage for JSON serialization
	natsMsg := NATSMessage{
		MessageID: messageID,
		Content:   msg.Content,
		Type:      msg.Type,
		Timestamp: msg.Timestamp,
		ServerID:  c.GetServerID(), // Add server ID to track origin
	}

	// Extract sender information if available
	if msg.Sender != nil {
		// Try to get sender ID and name from the sender interface
		if sender, ok := msg.Sender.(interface{ GetID() string }); ok {
			natsMsg.SenderID = sender.GetID()
		}
		if sender, ok := msg.Sender.(interface{ GetName() string }); ok {
			natsMsg.SenderName = sender.GetName()
		}
	}

	// Extract room name if available
	if msg.Room != nil {
		if room, ok := msg.Room.(interface{ GetName() string }); ok {
			natsMsg.RoomName = room.GetName()
		}
	}

	// Serialize message to JSON
	data, err := json.Marshal(natsMsg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	// Publish message
	if err := c.conn.Publish(subject, data); err != nil {
		return fmt.Errorf("failed to publish message: %w", err)
	}

	return nil
}

// Subscribe subscribes to a NATS subject and calls the handler for each message
func (c *Client) Subscribe(subject string, handler func(msg types.Message)) (*nats.Subscription, error) {
	c.mu.RLock()
	if !c.connected || c.conn == nil {
		c.mu.RUnlock()
		return nil, fmt.Errorf("NATS not connected")
	}
	c.mu.RUnlock()

	sub, err := c.conn.Subscribe(subject, func(m *nats.Msg) {
		var natsMsg NATSMessage
		if err := json.Unmarshal(m.Data, &natsMsg); err != nil {
			log.Printf("Failed to unmarshal NATS message: %v", err)
			return
		}

		// Convert NATSMessage back to types.Message
		msg := types.Message{
			MessageID: natsMsg.MessageID, // Pass message ID to prevent re-broadcasting
			Content:   natsMsg.Content,
			Type:      natsMsg.Type,
			Timestamp: natsMsg.Timestamp,
			Sender:    nil, // Sender is not available across servers
			Room:      nil, // Room is not available across servers
			ServerID:  natsMsg.ServerID, // Pass server ID to identify origin
		}

		handler(msg)
	})
	if err != nil {
		return nil, fmt.Errorf("failed to subscribe: %w", err)
	}

	return sub, nil
}

// SubscribeQueue creates a queue subscription for load balancing
func (c *Client) SubscribeQueue(subject, queue string, handler func(msg types.Message)) (*nats.Subscription, error) {
	c.mu.RLock()
	if !c.connected || c.conn == nil {
		c.mu.RUnlock()
		return nil, fmt.Errorf("NATS not connected")
	}
	c.mu.RUnlock()

	sub, err := c.conn.QueueSubscribe(subject, queue, func(m *nats.Msg) {
		var natsMsg NATSMessage
		if err := json.Unmarshal(m.Data, &natsMsg); err != nil {
			log.Printf("Failed to unmarshal NATS message: %v", err)
			return
		}

		// Convert NATSMessage back to types.Message
		msg := types.Message{
			MessageID: natsMsg.MessageID, // Pass message ID to prevent re-broadcasting
			Content:   natsMsg.Content,
			Type:      natsMsg.Type,
			Timestamp: natsMsg.Timestamp,
			Sender:    nil, // Sender is not available across servers
			Room:      nil, // Room is not available across servers
		}

		handler(msg)
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create queue subscription: %w", err)
	}

	return sub, nil
}

// IsConnected returns whether the client is connected to NATS
func (c *Client) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.connected
}

// Close closes the NATS connection
func (c *Client) Close() {
	c.cancel()
	c.mu.Lock()
	if c.conn != nil {
		c.conn.Close()
		c.conn = nil
		c.connected = false
	}
	c.mu.Unlock()
}

// ReconnectChan returns a channel that signals when reconnection occurs
func (c *Client) ReconnectChan() <-chan struct{} {
	return c.reconnectChan
}

// GetConn returns the underlying NATS connection
func (c *Client) GetConn() *nats.Conn {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.conn
}

// GetJetStream returns the JetStream context if enabled
func (c *Client) GetJetStream() (nats.JetStreamContext, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.js == nil {
		return nil, fmt.Errorf("JetStream not enabled")
	}
	return c.js, nil
}

// GetServerID returns the unique server ID
func (c *Client) GetServerID() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.serverID
}

// Subject constants for NATS messaging
const (
	SubjectGlobalChat     = "chat.global"
	SubjectRoomPrefix     = "chat.room"
	SubjectPresencePrefix = "presence"
	SubjectRoomSync       = "room.sync"  // For room synchronization across servers
)

// RoomSubject returns the NATS subject for a specific room
func RoomSubject(roomName string) string {
	return fmt.Sprintf("%s.%s", SubjectRoomPrefix, roomName)
}

// PresenceSubject returns the NATS subject for presence updates
func PresenceSubject(roomName string) string {
	return fmt.Sprintf("%s.%s", SubjectPresencePrefix, roomName)
}