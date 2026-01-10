package hub

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"websocket-demo/internal/client"
	"websocket-demo/internal/room"
	"websocket-demo/internal/types"

	"github.com/coder/websocket"
)

// Hub manages all WebSocket connections and broadcasts messages between clients
// Uses the Hub pattern for efficient client management
type Hub struct {
	Clients     map[*client.Client]bool
	Rooms       map[string]*room.Room
	ClientRooms map[*client.Client]*room.Room
	Broadcast   chan types.Message
	Register    chan *client.Client
	Unregister  chan *client.Client
	Mutex       sync.RWMutex
	Ctx         context.Context
	UserCount   int
	roomOpMutex sync.Mutex // Prevents concurrent room operations on the same client
}

// NewHub creates and initializes a new Hub instance
func NewHub(ctx context.Context) *Hub {
	return &Hub{
		Clients:     make(map[*client.Client]bool),
		Rooms:       make(map[string]*room.Room),
		ClientRooms: make(map[*client.Client]*room.Room),
		Broadcast:   make(chan types.Message, 100), // Buffered channel to avoid blocking
		Register:    make(chan *client.Client),
		Unregister:  make(chan *client.Client),
		Ctx:         ctx,
		UserCount:   0,
	}
}

// CreateRoom creates a new room with the specified name and properties
func (h *Hub) CreateRoom(name string, private bool, password string, maxClients int) (*room.Room, error) {
	// Validate room name
	if name == "" || len(name) > 50 {
		return nil, errors.New("invalid room name")
	}

	// Hold write lock during entire check-and-create operation to prevent race condition
	h.Mutex.Lock()
	defer h.Mutex.Unlock()

	// Check if room already exists
	if _, exists := h.Rooms[name]; exists {
		return nil, errors.New("room already exists")
	}

	// Create new room
	newRoom := room.NewRoom(name, private, password, maxClients)

	// Add to hub's rooms map
	h.Rooms[name] = newRoom

	return newRoom, nil
}

// JoinRoom adds a client to a room
func (h *Hub) JoinRoom(client *client.Client, targetRoom *room.Room, password string) error {
	// Lock to prevent concurrent room operations on the same client
	h.roomOpMutex.Lock()
	defer h.roomOpMutex.Unlock()

	// Set the creator if this is the first client
	if targetRoom.Creator == nil {
		targetRoom.SetCreator(client)
	}
	// Validate room is active
	if !targetRoom.Active {
		return errors.New("room is not active")
	}

	// Check max clients
	if !targetRoom.AddClient(client) {
		return errors.New("room is full")
	}

	// Validate password for private rooms
	if targetRoom.Private {
		if !h.VerifyPassword(password, targetRoom.Password) {
			return errors.New("invalid password")
		}
	}

	// Remove client from any existing room
	h.leaveRoomInternal(client)

	// Add client to new room
	targetRoom.AddClient(client)

	// Update client's current room
	client.SetCurrentRoom(targetRoom)

	// Update hub's client-to-room mapping
	h.Mutex.Lock()
	h.ClientRooms[client] = targetRoom
	h.Mutex.Unlock()

	// Broadcast room join notification
	timestamp := time.Now().Format("15:04:05")
	joinMsg := []byte(fmt.Sprintf("[%s] %s has joined the room", timestamp, client.Name))
	h.BroadcastToRoom(targetRoom, types.Message{Content: joinMsg, Sender: client, Type: types.MsgTypeRoomJoin})

	// Send room welcome message
	welcomeMsg := []byte(fmt.Sprintf("[%s] Welcome to room '%s'!", timestamp, targetRoom.Name))
	if client.Conn != nil {
		client.Conn.Write(h.Ctx, websocket.MessageText, welcomeMsg)
	}

	return nil
}

// leaveRoomInternal removes a client from their current room (internal use, assumes lock is held)
func (h *Hub) leaveRoomInternal(client *client.Client) {
	currentRoom := client.GetCurrentRoom()
	if currentRoom == nil {
		return // Not in any room
	}

	// Type assertion
	room, ok := currentRoom.(*room.Room)
	if !ok {
		return
	}

	// Remove client from room
	room.RemoveClient(client)

	// Update client's current room
	client.SetCurrentRoom(nil)

	// Update hub's client-to-room mapping
	h.Mutex.Lock()
	delete(h.ClientRooms, client)
	h.Mutex.Unlock()

	// Broadcast room leave notification
	timestamp := time.Now().Format("15:04:05")
	leaveMsg := []byte(fmt.Sprintf("[%s] %s has left the room", timestamp, client.Name))
	h.BroadcastToRoom(room, types.Message{Content: leaveMsg, Sender: nil, Type: types.MsgTypeRoomLeave})
}

// LeaveRoom removes a client from their current room
func (h *Hub) LeaveRoom(client *client.Client) {
	// Lock to prevent concurrent room operations on the same client
	h.roomOpMutex.Lock()
	defer h.roomOpMutex.Unlock()
	h.leaveRoomInternal(client)
}

// DeleteRoom deletes a room and moves all clients to default room
func (h *Hub) DeleteRoom(client *client.Client, roomName string) error {
	h.Mutex.RLock()
	targetRoom, exists := h.Rooms[roomName]
	h.Mutex.RUnlock()

	if !exists {
		return errors.New("room does not exist")
	}

	// Check if client is the creator
	if !targetRoom.IsCreator(client) {
		return errors.New("only the room creator can delete this room")
	}

	// Cannot delete default room
	if roomName == "default" {
		return errors.New("cannot delete default room")
	}

	// Move all clients to default room
	clients := targetRoom.GetClients()

	// Get default room
	h.Mutex.RLock()
	defaultRoom, defaultExists := h.Rooms["default"]
	h.Mutex.RUnlock()

	if !defaultExists {
		defaultRoom, _ = h.CreateRoom("default", false, "", 1000)
	}

	// Move all clients to default room
	for _, c := range clients {
		h.JoinRoom(c, defaultRoom, "")
	}

	// Remove room from hub
	h.Mutex.Lock()
	delete(h.Rooms, roomName)
	h.Mutex.Unlock()

	// Broadcast room deletion notification
	timestamp := time.Now().Format("15:04:05")
	deleteMsg := []byte(fmt.Sprintf("[%s] Room '%s' has been deleted by %s", timestamp, roomName, client.Name))
	h.BroadcastToRoom(defaultRoom, types.Message{Content: deleteMsg, Sender: nil, Type: types.MsgTypeDeleteRoom})

	return nil
}

// BroadcastToRoom sends a message to all clients in a specific room
func (h *Hub) BroadcastToRoom(targetRoom *room.Room, message types.Message) {
	clients := targetRoom.GetClients()

	// Send to all clients in room
	for _, client := range clients {
		if client.Conn == nil {
			continue
		}

		// Format message with room prefix
		roomPrefix := fmt.Sprintf("[%s] ", targetRoom.Name)
		formattedContent := append([]byte(roomPrefix), message.Content...)

		err := client.Conn.Write(h.Ctx, websocket.MessageText, formattedContent)
		if err != nil {
			// Handle write error - client likely disconnected
			h.Unregister <- client
		}
	}
}

// VerifyPassword checks if the provided password matches the correct password
func (h *Hub) VerifyPassword(inputPassword, correctPassword string) bool {
	// For simplicity, using direct comparison
	// In production, use proper password hashing
	return inputPassword == correctPassword
}

// Run starts the Hub's main loop that processes client connections and broadcasts messages
// This is the core of the Hub pattern implementation
func (h *Hub) Run() {
	log.Println("Hub Run() function started")
	for {
		select {
		case <-h.Ctx.Done():
			// Context cancelled, close all connections and exit
			h.Mutex.Lock()
			for client := range h.Clients {
				if client != nil && client.Conn != nil {
					client.Conn.Close(websocket.StatusNormalClosure, "server shutting down")
				}
			}
			h.Clients = make(map[*client.Client]bool)
			h.Mutex.Unlock()
			return

		case client := <-h.Register:
			if client != nil {
				h.Mutex.Lock()
				h.Clients[client] = true
				h.UserCount++
				h.Mutex.Unlock()
				log.Printf("Client %s connected. Total clients: %d", client.Name, h.UserCount)

				// Signal that this client's registration is complete FIRST
				close(client.Registered)
				log.Printf("Registration signal sent for %s", client.Name)

				// Create default room if it doesn't exist
				h.Mutex.RLock()
				_, defaultExists := h.Rooms["default"]
				h.Mutex.RUnlock()

				if !defaultExists {
					h.CreateRoom("default", false, "", 1000)
				}

				// Then broadcast join notification to all other clients (non-blocking)
				go func() {
					timestamp := time.Now().Format("15:04:05")
					joinMsg := []byte(fmt.Sprintf("[%s] %s has joined the chat", timestamp, client.Name))
					select {
					case h.Broadcast <- types.Message{Content: joinMsg, Sender: client, Type: types.MsgTypeJoin}:
						log.Printf("Join notification queued for %s", client.Name)
					case <-time.After(100 * time.Millisecond):
						log.Printf("Warning: Could not send join notification for %s (channel blocked)", client.Name)
					}
				}()

				// Join client to default room
				h.Mutex.RLock()
				defaultRoom := h.Rooms["default"]
				h.Mutex.RUnlock()

				if defaultRoom != nil {
					h.JoinRoom(client, defaultRoom, "")
				}
			}

		case client := <-h.Unregister:
			if client != nil {
				h.Mutex.Lock()
				if _, ok := h.Clients[client]; ok {
					delete(h.Clients, client)
					h.UserCount--
					if client.Conn != nil {
						client.Conn.Close(websocket.StatusNormalClosure, "")
					}
				}
				h.Mutex.Unlock()
				log.Printf("Client %s disconnected. Total clients: %d", client.Name, h.UserCount)

				// Broadcast leave notification to all remaining clients
				timestamp := time.Now().Format("15:04:05")
				leaveMsg := []byte(fmt.Sprintf("[%s] %s has left the chat", timestamp, client.Name))
				h.Broadcast <- types.Message{Content: leaveMsg, Sender: nil, Type: types.MsgTypeLeave}
			}

		case message := <-h.Broadcast:
			// Handle room-specific broadcasts
			if message.Room != nil {
				// Type assertion
				if room, ok := message.Room.(*room.Room); ok {
					h.BroadcastToRoom(room, message)
				}
			} else {
				h.Mutex.RLock()
				log.Printf("Broadcasting message of type '%s' to %d clients", message.Type, len(h.Clients))
				sentCount := 0
				clientsToRemove := make([]*client.Client, 0)

				for client := range h.Clients {
					// Don't send the message back to the sender (for chat messages)
					// But do send join/leave notifications to everyone including the sender
					if message.Type == types.MsgTypeChat && message.Sender != nil && client == message.Sender {
						log.Printf("Skipping sender %s for chat message", client.Name)
						continue
					}

					// Check if client connection is nil before attempting to write
					if client.Conn == nil {
						log.Printf("Skipping client %s with nil connection", client.Name)
						continue
					}

					err := client.Conn.Write(h.Ctx, websocket.MessageText, message.Content)
					if err != nil {
						log.Printf("Error writing to client %s: %v", client.Name, err)
						clientsToRemove = append(clientsToRemove, client)
					} else {
						sentCount++
						log.Printf("Message sent to client %s", client.Name)
					}
				}
				h.Mutex.RUnlock()

				// Remove failed clients with write lock
				if len(clientsToRemove) > 0 {
					h.Mutex.Lock()
					for _, client := range clientsToRemove {
						if _, ok := h.Clients[client]; ok {
							delete(h.Clients, client)
							h.UserCount--
							client.Conn.Close(websocket.StatusInternalError, "write error")
							log.Printf("Removed failed client %s", client.Name)
						}
					}
					h.Mutex.Unlock()
				}

				log.Printf("Broadcast complete: sent to %d clients", sentCount)
			}
		}
	}
}

// GetRoom returns a room by name
func (h *Hub) GetRoom(name string) (*room.Room, bool) {
	h.Mutex.RLock()
	defer h.Mutex.RUnlock()
	room, exists := h.Rooms[name]
	return room, exists
}

// GetRoomList returns a list of all rooms with their information
func (h *Hub) GetRoomList(client *client.Client) []map[string]interface{} {
	h.Mutex.RLock()
	defer h.Mutex.RUnlock()

	roomList := make([]map[string]interface{}, 0, len(h.Rooms))
	for name, room := range h.Rooms {
		room.Mutex.RLock()
		clientCount := len(room.Clients)
		isCreator := room.Creator == client
		room.Mutex.RUnlock()

		roomInfo := map[string]interface{}{
			"name":        name,
			"private":     room.Private,
			"clientCount": clientCount,
			"isCreator":   isCreator,
		}
		roomList = append(roomList, roomInfo)
	}
	return roomList
}