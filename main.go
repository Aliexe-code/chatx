package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

// Message represents a message to be broadcast to all clients
type Message struct {
	Content   []byte
	Sender    *Client
	Type      string // "chat", "join", "leave"
	Room      *Room  // Target room (for room-specific messages)
	Timestamp time.Time
}

// Client represents a WebSocket client connection
type Client struct {
	conn        *websocket.Conn
	name        string
	registered  chan struct{} // Signal when this client is registered
	currentRoom *Room         // Track current room
	roomMutex   sync.RWMutex  // Thread safety for room tracking
}

// Room represents a chat room
type Room struct {
	name       string
	clients    map[*Client]bool
	mutex      sync.RWMutex
	created    time.Time
	private    bool
	password   string
	maxClients int
	active     bool
	creator    *Client
}

// Hub manages all WebSocket connections and broadcasts messages between clients
// Uses the Hub pattern for efficient client management
type Hub struct {
	clients     map[*Client]bool
	rooms       map[string]*Room  // Map of room name to room
	clientRooms map[*Client]*Room // Track which room each client is in
	broadcast   chan Message
	register    chan *Client
	unregister  chan *Client
	mutex       sync.RWMutex
	ctx         context.Context
	userCount   int
}

// newHub creates and initializes a new Hub instance
// The Hub is the central coordinator that manages all WebSocket connections
func newHub(ctx context.Context) *Hub {
	return &Hub{
		clients:     make(map[*Client]bool),
		rooms:       make(map[string]*Room),
		clientRooms: make(map[*Client]*Room),
		broadcast:   make(chan Message, 100), // Buffered channel to avoid blocking
		register:    make(chan *Client),
		unregister:  make(chan *Client),
		ctx:         ctx,
		userCount:   0,
	}
}

// createRoom creates a new room with the specified name and properties
func (h *Hub) createRoom(name string, private bool, password string, maxClients int) (*Room, error) {
	// Validate room name
	if name == "" || len(name) > 50 {
		return nil, errors.New("invalid room name")
	}

	// Check if room already exists
	h.mutex.RLock()
	if _, exists := h.rooms[name]; exists {
		h.mutex.RUnlock()
		return nil, errors.New("room already exists")
	}
	h.mutex.RUnlock()

	// Create new room
	room := &Room{
		name:       name,
		clients:    make(map[*Client]bool),
		created:    time.Now(),
		private:    private,
		password:   password,
		maxClients: maxClients,
		active:     true,
		creator:    nil, // Will be set when creator joins
	}

	// Add to hub's rooms map
	h.mutex.Lock()
	h.rooms[name] = room
	h.mutex.Unlock()

	return room, nil
}

// joinRoom adds a client to a room
func (h *Hub) joinRoom(client *Client, room *Room, password string) error {
	// Validate room is active
	if !room.active {
		return errors.New("room is not active")
	}

	// Check max clients
	room.mutex.RLock()
	if len(room.clients) >= room.maxClients {
		room.mutex.RUnlock()
		return errors.New("room is full")
	}
	room.mutex.RUnlock()

	// Validate password for private rooms
	if room.private {
		if !h.verifyPassword(password, room.password) {
			return errors.New("invalid password")
		}
	}

	// Remove client from any existing room
	h.leaveRoom(client)

	// Add client to new room
	room.mutex.Lock()
	room.clients[client] = true
	room.mutex.Unlock()

	// Update client's current room
	client.roomMutex.Lock()
	client.currentRoom = room
	client.roomMutex.Unlock()

	// Update hub's client-to-room mapping
	h.mutex.Lock()
	h.clientRooms[client] = room
	h.mutex.Unlock()

	// Broadcast room join notification
	timestamp := time.Now().Format("15:04:05")
	joinMsg := []byte(fmt.Sprintf("[%s] 👋 %s has joined the room", timestamp, client.name))
	h.broadcastToRoom(room, Message{Content: joinMsg, Sender: client, Type: "room_join"})

	// Send room welcome message
	welcomeMsg := []byte(fmt.Sprintf("[%s] 🎉 Welcome to room '%s'!", timestamp, room.name))
	if client.conn != nil {
		client.conn.Write(context.Background(), websocket.MessageText, welcomeMsg)
	}

	return nil
}

// leaveRoom removes a client from their current room
func (h *Hub) leaveRoom(client *Client) {
	client.roomMutex.RLock()
	currentRoom := client.currentRoom
	client.roomMutex.RUnlock()

	if currentRoom == nil {
		return // Not in any room
	}

	// Remove client from room
	currentRoom.mutex.Lock()
	delete(currentRoom.clients, client)
	currentRoom.mutex.Unlock()

	// Update client's current room
	client.roomMutex.Lock()
	client.currentRoom = nil
	client.roomMutex.Unlock()

	// Update hub's client-to-room mapping
	h.mutex.Lock()
	delete(h.clientRooms, client)
	h.mutex.Unlock()

	// Broadcast room leave notification
	timestamp := time.Now().Format("15:04:05")
	leaveMsg := []byte(fmt.Sprintf("[%s] 👋 %s has left the room", timestamp, client.name))
	h.broadcastToRoom(currentRoom, Message{Content: leaveMsg, Sender: nil, Type: "room_leave"})
}

// deleteRoom deletes a room and moves all clients to default room
func (h *Hub) deleteRoom(client *Client, roomName string) error {
	h.mutex.RLock()
	room, exists := h.rooms[roomName]
	h.mutex.RUnlock()

	if !exists {
		return errors.New("room does not exist")
	}

	// Check if client is the creator
	room.mutex.RLock()
	isCreator := room.creator == client
	room.mutex.RUnlock()

	if !isCreator {
		return errors.New("only the room creator can delete this room")
	}

	// Cannot delete default room
	if roomName == "default" {
		return errors.New("cannot delete default room")
	}

	// Move all clients to default room
	room.mutex.RLock()
	clients := make([]*Client, 0, len(room.clients))
	for c := range room.clients {
		clients = append(clients, c)
	}
	room.mutex.RUnlock()

	// Get default room
	h.mutex.RLock()
	defaultRoom, defaultExists := h.rooms["default"]
	h.mutex.RUnlock()

	if !defaultExists {
		defaultRoom, _ = h.createRoom("default", false, "", 1000)
	}

	// Move all clients to default room
	for _, c := range clients {
		h.joinRoom(c, defaultRoom, "")
	}

	// Remove room from hub
	h.mutex.Lock()
	delete(h.rooms, roomName)
	h.mutex.Unlock()

	// Broadcast room deletion notification
	timestamp := time.Now().Format("15:04:05")
	deleteMsg := []byte(fmt.Sprintf("[%s] 🗑️ Room '%s' has been deleted by %s", timestamp, roomName, client.name))
	h.broadcastToRoom(defaultRoom, Message{Content: deleteMsg, Sender: nil, Type: "room_delete"})

	return nil
}

// broadcastToRoom sends a message to all clients in a specific room
func (h *Hub) broadcastToRoom(room *Room, message Message) {
	room.mutex.RLock()
	clients := make([]*Client, 0, len(room.clients))
	for client := range room.clients {
		clients = append(clients, client)
	}
	room.mutex.RUnlock()

	// Send to all clients in room
	for _, client := range clients {
		if client.conn == nil {
			continue
		}

		// Format message with room prefix
		roomPrefix := fmt.Sprintf("[%s] ", room.name)
		formattedContent := append([]byte(roomPrefix), message.Content...)

		err := client.conn.Write(context.Background(), websocket.MessageText, formattedContent)
		if err != nil {
			// Handle write error - client likely disconnected
			h.unregister <- client
		}
	}
}

// verifyPassword checks if the provided password matches the correct password
func (h *Hub) verifyPassword(inputPassword, correctPassword string) bool {
	// For simplicity, using direct comparison
	// In production, use proper password hashing
	return inputPassword == correctPassword
}

const (
	MSG_TYPE_CHAT         = "chat"
	MSG_TYPE_JOIN         = "join"
	MSG_TYPE_LEAVE        = "leave"
	MSG_TYPE_ROOM_JOIN    = "room_join"
	MSG_TYPE_ROOM_LEAVE   = "room_leave"
	MSG_TYPE_CREATE_ROOM  = "create_room"
	MSG_TYPE_JOIN_ROOM    = "join_room"
	MSG_TYPE_LEAVE_ROOM   = "leave_room"
	MSG_TYPE_LIST_ROOMS   = "list_rooms"
	MSG_TYPE_ROOM_MESSAGE = "room_message"
	MSG_TYPE_DELETE_ROOM  = "delete_room"
)

// run starts the Hub's main loop that processes client connections and broadcasts messages
// This is the core of the Hub pattern implementation
func (h *Hub) run() {
	log.Println("Hub run() function started")
	for {
		select {
		case <-h.ctx.Done():
			// Context cancelled, close all connections and exit
			h.mutex.Lock()
			for client := range h.clients {
				if client != nil && client.conn != nil {
					client.conn.Close(websocket.StatusNormalClosure, "server shutting down")
				}
			}
			h.clients = make(map[*Client]bool)
			h.mutex.Unlock()
			return

		case client := <-h.register:
			if client != nil {
				h.mutex.Lock()
				h.clients[client] = true
				h.userCount++
				h.mutex.Unlock()
				log.Printf("Client %s connected. Total clients: %d", client.name, h.userCount)

				// Signal that this client's registration is complete FIRST
				close(client.registered)
				log.Printf("Registration signal sent for %s", client.name)

				// Create default room if it doesn't exist
				h.mutex.RLock()
				_, defaultExists := h.rooms["default"]
				h.mutex.RUnlock()

				if !defaultExists {
					h.createRoom("default", false, "", 1000)
				}

				// Then broadcast join notification to all other clients (non-blocking)
				go func() {
					timestamp := time.Now().Format("15:04:05")
					joinMsg := []byte(fmt.Sprintf("[%s] 👋 %s has joined the chat", timestamp, client.name))
					select {
					case h.broadcast <- Message{Content: joinMsg, Sender: client, Type: "join"}:
						log.Printf("Join notification queued for %s", client.name)
					case <-time.After(100 * time.Millisecond):
						log.Printf("Warning: Could not send join notification for %s (channel blocked)", client.name)
					}
				}()

				// Join client to default room
				h.mutex.RLock()
				defaultRoom := h.rooms["default"]
				h.mutex.RUnlock()

				if defaultRoom != nil {
					h.joinRoom(client, defaultRoom, "")
				}
			}

		case client := <-h.unregister:
			if client != nil {
				h.mutex.Lock()
				if _, ok := h.clients[client]; ok {
					delete(h.clients, client)
					h.userCount--
					if client.conn != nil {
						client.conn.Close(websocket.StatusNormalClosure, "")
					}
				}
				h.mutex.Unlock()
				log.Printf("Client %s disconnected. Total clients: %d", client.name, h.userCount)

				// Broadcast leave notification to all remaining clients
				timestamp := time.Now().Format("15:04:05")
				leaveMsg := []byte(fmt.Sprintf("[%s] 👋 %s has left the chat", timestamp, client.name))
				h.broadcast <- Message{Content: leaveMsg, Sender: nil, Type: "leave"}
			}

		case message := <-h.broadcast:
			// Handle room-specific broadcasts
			if message.Room != nil {
				h.broadcastToRoom(message.Room, message)
			} else {
				h.mutex.RLock()
				log.Printf("Broadcasting message of type '%s' to %d clients", message.Type, len(h.clients))
				sentCount := 0
				clientsToRemove := make([]*Client, 0)

				for client := range h.clients {
					// Don't send the message back to the sender (for chat messages)
					// But do send join/leave notifications to everyone including the sender
					if message.Type == "chat" && message.Sender != nil && client == message.Sender {
						log.Printf("Skipping sender %s for chat message", client.name)
						continue
					}

					// Check if client connection is nil before attempting to write
					if client.conn == nil {
						log.Printf("Skipping client %s with nil connection", client.name)
						continue
					}

					err := client.conn.Write(h.ctx, websocket.MessageText, message.Content)
					if err != nil {
						log.Printf("Error writing to client %s: %v", client.name, err)
						clientsToRemove = append(clientsToRemove, client)
					} else {
						sentCount++
						log.Printf("Message sent to client %s", client.name)
					}
				}
				h.mutex.RUnlock()

				// Remove failed clients with write lock
				if len(clientsToRemove) > 0 {
					h.mutex.Lock()
					for _, client := range clientsToRemove {
						if _, ok := h.clients[client]; ok {
							delete(h.clients, client)
							h.userCount--
							client.conn.Close(websocket.StatusInternalError, "write error")
							log.Printf("Removed failed client %s", client.name)
						}
					}
					h.mutex.Unlock()
				}

				log.Printf("Broadcast complete: sent to %d clients", sentCount)
			}
		}
	}
}

// handleWebSocket handles individual WebSocket client connections
// It manages the lifecycle of each client connection including registration, message handling, and cleanup
func handleWebSocket(hub *Hub, c echo.Context) error {
	log.Printf("New WebSocket connection attempt from %s", c.RealIP())

	opts := &websocket.AcceptOptions{
		OriginPatterns: []string{"*"},
	}

	conn, err := websocket.Accept(c.Response(), c.Request(), opts)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return echo.NewHTTPError(http.StatusBadRequest, "WebSocket upgrade failed")
	}
	log.Printf("WebSocket connection established successfully")

	defer func() {
		if conn != nil {
			conn.Close(websocket.StatusNormalClosure, "server shutting down")
		}
	}()

	// Generate a unique user name
	userName := fmt.Sprintf("User%d", rand.Intn(9000)+1000)
	client := &Client{
		conn:       conn,
		name:       userName,
		registered: make(chan struct{}),
	}
	log.Printf("Created client with name: %s", userName)

	// Register the client
	hub.register <- client
	log.Printf("Client %s queued for registration", userName)

	// Wait for this client's registration to complete with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	select {
	case <-client.registered:
		log.Printf("Registration confirmed for %s", userName)
	case <-ctx.Done():
		log.Printf("Registration timeout for %s", userName)
		return echo.NewHTTPError(http.StatusRequestTimeout, "Registration timeout")
	}

	// Send welcome message to the new client
	timestamp := time.Now().Format("15:04:05")
	welcomeMsg := []byte(fmt.Sprintf("[%s] 🎉 Welcome to the chat! Your name is %s", timestamp, userName))
	err = conn.Write(context.Background(), websocket.MessageText, welcomeMsg)
	if err != nil {
		log.Printf("Error sending welcome message to %s: %v", userName, err)
		// Don't return here as welcome message failure shouldn't close the connection
	} else {
		log.Printf("Welcome message sent to %s", userName)
	}

	for {
		_, message, err := conn.Read(context.Background())
		if err != nil {
			log.Printf("Read message error from %s: %v", userName, err)
			hub.unregister <- client
			break
		}

		log.Printf("Received message from %s: %s", userName, string(message))

		// Parse WebSocket message
		wsMsg, err := parseWebSocketMessage(message)
		if err != nil {
			log.Printf("Error parsing WebSocket message from %s: %v", userName, err)
			errorMsg := []byte(fmt.Sprintf("Error parsing message: %v", err))
			client.conn.Write(context.Background(), websocket.MessageText, errorMsg)
		} else if wsMsg != nil {
			log.Printf("Parsed WebSocket message type: %s", wsMsg.Type)
			err := handleWebSocketMessage(hub, client, wsMsg)
			if err != nil {
				log.Printf("Error handling WebSocket message from %s: %v", userName, err)
				errorMsg := []byte(fmt.Sprintf("Error: %v", err))
				client.conn.Write(context.Background(), websocket.MessageText, errorMsg)
			}
		} else {
			// Handle legacy chat messages
			timestamp := time.Now().Format("15:04:05")
			formattedMsg := []byte(fmt.Sprintf("[%s] %s: %s", timestamp, userName, string(message)))
			log.Printf("Attempting to send message from %s to broadcast channel", userName)

			// Send to broadcast channel with timeout
			ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
			defer cancel()

			select {
			case hub.broadcast <- Message{Content: formattedMsg, Sender: client, Type: "chat"}:
				log.Printf("Message from %s queued for broadcast", userName)
			case <-ctx.Done():
				log.Printf("Broadcast timeout for %s", userName)
				// Continue processing other messages
			}
		}
	}

	return nil
}

// handleWebSocketMessage processes WebSocket messages and routes them appropriately
func handleWebSocketMessage(hub *Hub, client *Client, wsMsg *WebSocketMessage) error {
	switch wsMsg.Type {
	case MSG_TYPE_CHAT:
		// Handle regular chat message
		timestamp := time.Now().Format("15:04:05")
		formattedMsg := []byte(fmt.Sprintf("[%s] %s: %s", timestamp, client.name, wsMsg.Data.Content))
		hub.broadcast <- Message{Content: formattedMsg, Sender: client, Type: "chat"}

	case MSG_TYPE_ROOM_MESSAGE:
		// Handle room-specific message
		client.roomMutex.RLock()
		currentRoom := client.currentRoom
		client.roomMutex.RUnlock()

		if currentRoom != nil {
			timestamp := time.Now().Format("15:04:05")
			formattedMsg := []byte(fmt.Sprintf("[%s] %s: %s", timestamp, client.name, wsMsg.Data.Content))
			hub.broadcast <- Message{Content: formattedMsg, Sender: client, Type: "room_message", Room: currentRoom}
		}

	case MSG_TYPE_CREATE_ROOM:
		// Handle room creation
		room, err := hub.createRoom(wsMsg.Data.Name, wsMsg.Data.Private, wsMsg.Data.Password, 100)
		if err != nil {
			// Send error message to client
			errorMsg := []byte(fmt.Sprintf("Error creating room: %v", err))
			client.conn.Write(context.Background(), websocket.MessageText, errorMsg)
		} else {
			// Set the creator
			room.mutex.Lock()
			room.creator = client
			room.mutex.Unlock()
			// Add client to new room
			hub.joinRoom(client, room, "")
		}

	case MSG_TYPE_JOIN_ROOM:
		// Handle room joining
		hub.mutex.RLock()
		room, exists := hub.rooms[wsMsg.Data.Name]
		hub.mutex.RUnlock()

		if exists {
			err := hub.joinRoom(client, room, wsMsg.Data.Password)
			if err != nil {
				// Send error message to client
				errorMsg := []byte(fmt.Sprintf("Error joining room: %v", err))
				client.conn.Write(context.Background(), websocket.MessageText, errorMsg)
			}
		} else {
			// Send error message to client
			errorMsg := []byte(fmt.Sprintf("Room '%s' does not exist", wsMsg.Data.Name))
			client.conn.Write(context.Background(), websocket.MessageText, errorMsg)
		}

	case MSG_TYPE_LEAVE_ROOM:
		// Handle room leaving
		hub.leaveRoom(client)

	case MSG_TYPE_LIST_ROOMS:
		// Handle room listing with detailed info
		hub.mutex.RLock()
		roomList := make([]map[string]interface{}, 0, len(hub.rooms))
		for name, room := range hub.rooms {
			room.mutex.RLock()
			clientCount := len(room.clients)
			isCreator := room.creator == client
			room.mutex.RUnlock()

			roomInfo := map[string]interface{}{
				"name":        name,
				"private":     room.private,
				"clientCount": clientCount,
				"isCreator":   isCreator,
			}
			roomList = append(roomList, roomInfo)
		}
		hub.mutex.RUnlock()

		roomListJSON, _ := json.Marshal(roomList)
		listMsg := []byte(fmt.Sprintf("ROOMS_LIST:%s", string(roomListJSON)))
		client.conn.Write(context.Background(), websocket.MessageText, listMsg)

	case MSG_TYPE_DELETE_ROOM:
		// Handle room deletion
		err := hub.deleteRoom(client, wsMsg.Data.Name)
		if err != nil {
			// Send error message to client
			errorMsg := []byte(fmt.Sprintf("Error deleting room: %v", err))
			client.conn.Write(context.Background(), websocket.MessageText, errorMsg)
		} else {
			// Send success message
			successMsg := []byte(fmt.Sprintf("Room '%s' deleted successfully", wsMsg.Data.Name))
			client.conn.Write(context.Background(), websocket.MessageText, successMsg)
		}

	default:
		// Unknown message type
		errorMsg := []byte(fmt.Sprintf("Unknown message type: %s", wsMsg.Type))
		client.conn.Write(context.Background(), websocket.MessageText, errorMsg)
	}

	return nil
}

// WebSocketMessage represents a WebSocket message
type WebSocketMessage struct {
	Type string `json:"type"`
	Data struct {
		Name     string `json:"name,omitempty"`
		Password string `json:"password,omitempty"`
		Content  string `json:"content,omitempty"`
		Private  bool   `json:"private,omitempty"`
	} `json:"data,omitempty"`
}

// parseWebSocketMessage parses a WebSocket message from JSON
func parseWebSocketMessage(message []byte) (*WebSocketMessage, error) {
	var wsMsg WebSocketMessage
	err := json.Unmarshal(message, &wsMsg)
	if err != nil {
		return nil, err
	}
	return &wsMsg, nil
}

// main initializes and starts the WebSocket chat server
// Sets up the Echo web framework, creates the Hub, and registers routes
func main() {
	e := echo.New()

	e.Use(middleware.RequestLogger())
	e.Use(middleware.Recover())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	hub := newHub(ctx)
	go hub.run()
	log.Println("Hub started running")

	e.GET("/ws", func(c echo.Context) error {
		return handleWebSocket(hub, c)
	})

	e.GET("/", func(c echo.Context) error {
		return c.File("index.html")
	})

	log.Println("WebSocket server starting on :8080")
	e.Logger.Fatal(e.Start(":8080"))
}
