package client

import (
	"sync"

	"github.com/coder/websocket"
)

// Client represents a WebSocket client connection
type Client struct {
	Conn        *websocket.Conn
	Name        string
	Registered  chan struct{} // Signal when this client is registered
	CurrentRoom interface{}   // Track current room (will be *room.Room)
	RoomMutex   sync.RWMutex  // Thread safety for room tracking
}

// NewClient creates a new client instance
func NewClient(conn *websocket.Conn, name string) *Client {
	return &Client{
		Conn:       conn,
		Name:       name,
		Registered: make(chan struct{}),
	}
}

// GetCurrentRoom returns the current room for the client
func (c *Client) GetCurrentRoom() interface{} {
	c.RoomMutex.RLock()
	defer c.RoomMutex.RUnlock()
	return c.CurrentRoom
}

// SetCurrentRoom sets the current room for the client
func (c *Client) SetCurrentRoom(room interface{}) {
	c.RoomMutex.Lock()
	defer c.RoomMutex.Unlock()
	c.CurrentRoom = room
}