package hub

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"

	clientpkg "websocket-demo/internal/client"
	natsclient "websocket-demo/internal/nats"
	"websocket-demo/internal/repository"
	"websocket-demo/internal/room"
	"websocket-demo/internal/types"
	"websocket-demo/internal/validator"

	"github.com/coder/websocket"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nats-io/nats.go"
)

// Hub manages all WebSocket connections and broadcasts messages between clients
// Uses the Hub pattern for efficient client management
type Hub struct {
	Clients     map[*clientpkg.Client]bool
	Rooms       map[string]*room.Room
	ClientRooms map[*clientpkg.Client]*room.Room
	Broadcast   chan types.Message
	Register    chan *clientpkg.Client
	Unregister  chan *clientpkg.Client
	Repo        *repository.Repository
	Mutex       sync.RWMutex
	Ctx         context.Context
	UserCount   int
	roomOpMutex sync.Mutex // Prevents concurrent room operations on the same client
	NATS        *natsclient.Client
	NATSEnabled bool
}

// NewHub creates and initializes a new Hub instance
func NewHub(ctx context.Context, repo *repository.Repository, natsClient *natsclient.Client) *Hub {
	natsEnabled := natsClient != nil && natsClient.IsConnected()
	return &Hub{
		Clients:     make(map[*clientpkg.Client]bool),
		Rooms:       make(map[string]*room.Room),
		ClientRooms: make(map[*clientpkg.Client]*room.Room),
		Broadcast:   make(chan types.Message, 100),  // Buffered channel to avoid blocking
		Register:    make(chan *clientpkg.Client, 100), // Buffered to prevent deadlocks
		Unregister:  make(chan *clientpkg.Client, 100), // Buffered to prevent deadlocks
		Repo:        repo,
		Ctx:         ctx,
		UserCount:   0,
		NATS:        natsClient,
		NATSEnabled: natsEnabled,
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

	// Check if room already exists in database
	if h.Repo != nil {
		ctx := context.Background()
		_, err := h.Repo.GetRoomByName(ctx, name)
		if err == nil {
			return nil, errors.New("room already exists")
		}
	}

	// Check if room already exists in memory
	if _, exists := h.Rooms[name]; exists {
		return nil, errors.New("room already exists")
	}

	// Create new room
	newRoom := room.NewRoom(name, private, password, maxClients)

	// Add to hub's rooms map
	h.Rooms[name] = newRoom

	// Persist room to database if repository is available
	if h.Repo != nil {
		ctx := context.Background()
		passwordHash := pgtype.Text{Valid: false}
		if private && password != "" {
			// Hash the password using bcrypt
			hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
			if err != nil {
				log.Printf("Failed to hash room password: %v", err)
				return nil, fmt.Errorf("failed to hash password: %w", err)
			}
			passwordHash = pgtype.Text{String: string(hashedPassword), Valid: true}
		}

		creatorID := pgtype.UUID{Valid: false}
		if newRoom.Creator != nil && newRoom.Creator.UserID != "" {
			creatorID.Scan(newRoom.Creator.UserID)
		}

		dbRoom, err := h.Repo.CreateRoom(ctx, name, pgtype.Bool{Bool: private, Valid: true}, passwordHash, creatorID)
		if err != nil {
			log.Printf("Failed to persist room %s to database: %v", name, err)
			// Continue with in-memory room for now
		} else {
			// Store database ID in room for future reference
			newRoom.ID = uuid.UUID(dbRoom.ID.Bytes).String()
			log.Printf("Room %s persisted to database with ID %s", name, newRoom.ID)
		}
	}

	// Publish room creation to NATS for synchronization across servers
	if h.NATSEnabled && h.NATS != nil {
		roomData := map[string]interface{}{
			"name":     name,
			"private":  private,
			"password": password,
			"maxClients": maxClients,
		}
		roomDataJSON, _ := json.Marshal(roomData)

		syncMsg := types.Message{
			Content: roomDataJSON,
			Type:    types.MsgTypeRoomSync,
		}

		if err := h.NATS.Publish(natsclient.SubjectRoomSync, syncMsg); err != nil {
			log.Printf("Failed to publish room sync to NATS: %v", err)
		} else {
			log.Printf("Published room sync to NATS: %s", name)
		}
	}

	return newRoom, nil
}

// JoinRoom adds a client to a room
func (h *Hub) JoinRoom(client *clientpkg.Client, targetRoom *room.Room, password string) error {
	// Acquire locks in consistent order: h.Mutex first, then roomOpMutex
	h.Mutex.Lock()
	h.roomOpMutex.Lock()

	// Set the creator if this is the first client
	if targetRoom.Creator == nil {
		targetRoom.SetCreator(client)
	}
	// Validate room is active
	if !targetRoom.Active {
		h.roomOpMutex.Unlock()
		h.Mutex.Unlock()
		return errors.New("room is not active")
	}

	// Check max clients
	if !targetRoom.AddClient(client) {
		h.roomOpMutex.Unlock()
		h.Mutex.Unlock()
		return errors.New("room is full")
	}

	// Validate password for private rooms
	if targetRoom.Private {
		if !h.VerifyPassword(password, targetRoom.Password) {
			targetRoom.RemoveClient(client) // Rollback
			h.roomOpMutex.Unlock()
			h.Mutex.Unlock()
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
	h.ClientRooms[client] = targetRoom

	// Persist room membership to database if repository is available
	if h.Repo != nil && client.UserID != "" {
		ctx := context.Background()
		roomID := pgtype.UUID{}
		userID := pgtype.UUID{}

		if err := roomID.Scan(targetRoom.ID); err == nil {
			if err := userID.Scan(client.UserID); err == nil {
				if err := h.Repo.AddRoomMember(ctx, roomID, userID); err != nil {
					log.Printf("Failed to persist room membership for user %s in room %s: %v", client.UserID, targetRoom.Name, err)
				}
			}
		}
	}

	h.roomOpMutex.Unlock()
	h.Mutex.Unlock()

	// Subscribe to room-specific NATS subject if enabled
	if h.NATSEnabled && h.NATS != nil {
		subject := natsclient.RoomSubject(targetRoom.Name)
		// Use regular subscription (not queue) so ALL servers receive every message
		// Queue subscriptions are for load balancing (one consumer gets the message),
		// but we need pub/sub (all consumers get the message) for cross-server distribution
		_, err := h.NATS.Subscribe(subject, func(msg types.Message) {
			// Skip messages that originated from this server to prevent duplicate delivery
			if msg.ServerID != "" && msg.ServerID == h.NATS.GetServerID() {
				log.Printf("Skipping message from own server %s", msg.ServerID)
				return
			}
			// Forward NATS messages to BroadcastToRoom for consistent handling
			// BroadcastToRoom will handle delivery to local clients
			h.BroadcastToRoom(targetRoom, msg)
		})
		if err != nil {
			log.Printf("Failed to subscribe to room NATS subject %s: %v", subject, err)
		} else {
			log.Printf("Subscribed to room NATS subject: %s", subject)
		}
	}

	// Broadcast room join notification
	timestamp := time.Now().Format("15:04:05")
	joinMsg := []byte(fmt.Sprintf("[%s] %s has joined the room", timestamp, client.Name))
	// Add MessageID to prevent duplicate broadcasting via NATS
	joinMessageID := fmt.Sprintf("join-%d-%s", time.Now().UnixNano(), client.UserID)
	h.BroadcastToRoom(targetRoom, types.Message{MessageID: joinMessageID, Content: joinMsg, Sender: client, Type: types.MsgTypeRoomJoin})

	// Send room welcome message
	welcomeMsg := []byte(fmt.Sprintf("[%s] Welcome to room '%s'!", timestamp, targetRoom.Name))
	if client.Conn != nil {
		client.Conn.Write(h.Ctx, websocket.MessageText, welcomeMsg)
	}

	return nil
}

// leaveRoomInternal removes a client from their current room (internal use, assumes h.Mutex and roomOpMutex are held)
func (h *Hub) leaveRoomInternal(client *clientpkg.Client) {
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
	delete(h.ClientRooms, client)

	// Remove room membership from database if repository is available
	if h.Repo != nil && client.UserID != "" {
		ctx := context.Background()
		roomID := pgtype.UUID{}
		userID := pgtype.UUID{}

		if err := roomID.Scan(room.ID); err == nil {
			if err := userID.Scan(client.UserID); err == nil {
				if err := h.Repo.RemoveRoomMember(ctx, roomID, userID); err != nil {
					log.Printf("Failed to remove room membership for user %s from room %s: %v", client.UserID, room.Name, err)
				}
			}
		}
	}

	// Send confirmation to the leaving user
	leaveConfirmMsg := []byte(fmt.Sprintf("You have left the room \"%s\"", room.Name))
	if err := client.Conn.Write(context.Background(), websocket.MessageText, leaveConfirmMsg); err != nil {
		log.Printf("Failed to send leave confirmation to %s: %v", client.Name, err)
	}

	// Broadcast room leave notification to remaining room members
	timestamp := time.Now().Format("15:04:05")
	leaveMsg := []byte(fmt.Sprintf("[%s] %s has left the room", timestamp, client.Name))
	h.BroadcastToRoom(room, types.Message{Content: leaveMsg, Sender: nil, Type: types.MsgTypeRoomLeave})
}

// LeaveRoom removes a client from their current room
func (h *Hub) LeaveRoom(client *clientpkg.Client) {
	// Acquire locks in consistent order: h.Mutex first, then roomOpMutex
	h.Mutex.Lock()
	h.roomOpMutex.Lock()
	h.leaveRoomInternal(client)
	h.roomOpMutex.Unlock()
	h.Mutex.Unlock()
}

// DeleteRoom deletes a room
func (h *Hub) DeleteRoom(client *clientpkg.Client, roomName string) error {
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

	// Broadcast room deletion notification globally
	timestamp := time.Now().Format("15:04:05")
	deleteMsg := []byte(fmt.Sprintf("[%s] Room '%s' has been deleted by %s", timestamp, roomName, client.Name))
	h.Broadcast <- types.Message{Content: deleteMsg, Sender: nil, Type: types.MsgTypeDeleteRoom}

	// Remove room from hub
	h.Mutex.Lock()
	delete(h.Rooms, roomName)
	h.Mutex.Unlock()

	return nil
}

// BroadcastToRoom sends a message to all clients in a specific room
func (h *Hub) BroadcastToRoom(targetRoom *room.Room, message types.Message) {
	clients := targetRoom.GetClients()
	log.Printf("BroadcastToRoom: Room '%s', Message type '%s', MessageID: '%s', Total clients in room: %d", targetRoom.Name, message.Type, message.MessageID, len(clients))

	// Skip publishing to NATS if this message already has a MessageID (meaning it came from NATS)
	// This prevents the infinite loop: NATS → BroadcastToRoom → NATS → BroadcastToRoom → ...
	if h.NATSEnabled && h.NATS != nil && message.MessageID == "" {
		subject := natsclient.RoomSubject(targetRoom.Name)
		if err := h.NATS.Publish(subject, message); err != nil {
			log.Printf("Failed to publish message to NATS subject %s: %v", subject, err)
		} else {
			log.Printf("Published message to NATS subject %s", subject)
		}
	}

	clientsToRemove := make([]*clientpkg.Client, 0)
	// Send to all clients in room
	for _, client := range clients {
		if client.Conn == nil {
			log.Printf("BroadcastToRoom: Skipping client %s (nil connection)", client.Name)
			continue
		}

		// Don't send the message back to the sender (for room messages)
		if message.Type == types.MsgTypeRoomMessage && message.Sender != nil && client == message.Sender {
			log.Printf("BroadcastToRoom: Skipping sender %s", client.Name)
			continue
		}

		// Validate message size before broadcasting
		maxSize := validator.GetMaxMessageSize()
		if err := validator.ValidateMessageSize(len(message.Content), maxSize); err != nil {
			log.Printf("BroadcastToRoom: Skipping message to %s due to size validation: %v", client.Name, err)
			return // Skip this message
		}

		// Format message with room prefix
		roomPrefix := fmt.Sprintf("[%s] ", targetRoom.Name)
		formattedContent := append([]byte(roomPrefix), message.Content...)

		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		err := client.Conn.Write(ctx, websocket.MessageText, formattedContent)
		cancel()
		if err != nil {
			// Handle write error - client likely disconnected
			log.Printf("BroadcastToRoom: Error writing to client %s: %v", client.Name, err)
			clientsToRemove = append(clientsToRemove, client)
		} else {
			log.Printf("BroadcastToRoom: Sent message to client %s: %s", client.Name, string(formattedContent))
		}
	}
	// Unregister failed clients
	for _, c := range clientsToRemove {
		h.Unregister <- c
	}
}

// VerifyPassword checks if the provided password matches the correct password
func (h *Hub) VerifyPassword(inputPassword, correctPassword string) bool {
	// Compare using bcrypt to verify hashed passwords
	err := bcrypt.CompareHashAndPassword([]byte(correctPassword), []byte(inputPassword))
	return err == nil
}

// Run starts the Hub's main loop that processes client connections and broadcasts messages
// This is the core of the Hub pattern implementation
func (h *Hub) Run() {
	log.Println("Hub Run() function started")

	// Set up NATS subscriptions if enabled
	var globalChatSub *nats.Subscription
	var roomSyncSub *nats.Subscription
	if h.NATSEnabled && h.NATS != nil {
		// Subscribe to global chat
		sub, err := h.NATS.Subscribe(natsclient.SubjectGlobalChat, func(msg types.Message) {
			// Skip messages that originated from this server
			if msg.ServerID != "" && msg.ServerID == h.NATS.GetServerID() {
				log.Printf("Skipping global message from own server %s", msg.ServerID)
				return
			}
			// Only process messages from other servers
			h.Broadcast <- msg
		})
		if err != nil {
			log.Printf("Failed to subscribe to global chat: %v", err)
		} else {
			globalChatSub = sub
			log.Println("Subscribed to NATS global chat subject")
		}

		// Subscribe to room sync for cross-server room creation
		roomSyncSub, err = h.NATS.Subscribe(natsclient.SubjectRoomSync, func(msg types.Message) {
			// Parse room data
			var roomData map[string]interface{}
			if err := json.Unmarshal(msg.Content, &roomData); err != nil {
				log.Printf("Failed to unmarshal room sync data: %v", err)
				return
			}

			name, _ := roomData["name"].(string)
			private, _ := roomData["private"].(bool)
			password, _ := roomData["password"].(string)
			maxClients := 100
			if mc, ok := roomData["maxClients"].(float64); ok {
				maxClients = int(mc)
			}

			// Check if room already exists
			h.Mutex.Lock()
			if _, exists := h.Rooms[name]; !exists {
				// Create room from sync data
				newRoom := room.NewRoom(name, private, password, maxClients)
				h.Rooms[name] = newRoom
				log.Printf("Room %s synced from NATS", name)

				// Try to load from database to get ID
				if h.Repo != nil {
					ctx := context.Background()
					dbRoom, err := h.Repo.GetRoomByName(ctx, name)
					if err == nil {
						newRoom.ID = uuid.UUID(dbRoom.ID.Bytes).String()
					}
				}
			}
			h.Mutex.Unlock()
		})
		if err != nil {
			log.Printf("Failed to subscribe to room sync: %v", err)
		} else {
			log.Println("Subscribed to NATS room sync subject")
		}
	}

	defer func() {
		if globalChatSub != nil {
			globalChatSub.Unsubscribe()
		}
		if roomSyncSub != nil {
			roomSyncSub.Unsubscribe()
		}
	}()

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
			h.Clients = make(map[*clientpkg.Client]bool)
			h.Mutex.Unlock()
			if h.NATS != nil {
				h.NATS.Close()
			}
			return

		case client := <-h.Register:
			if client != nil {
				h.Mutex.Lock()
				h.Clients[client] = true
				h.UserCount++
				h.Mutex.Unlock()
				log.Printf("Client %s connected. Total clients: %d", client.Name, h.UserCount)

				// Signal that this client's registration is complete FIRST
				client.RegisteredOnce.Do(func() {
					close(client.Registered)
				})
				log.Printf("Registration signal sent for %s", client.Name)

				// Join notification removed
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
			// Save chat messages to database
			if (message.Type == types.MsgTypeChat || message.Type == types.MsgTypeRoomMessage) && message.Sender != nil {
				if sender, ok := message.Sender.(*clientpkg.Client); ok && sender.Authenticated && sender.UserID != "" {
					// Parse the message content to get chat content
					var chatMsg types.ChatMessage
					if err := json.Unmarshal(message.Content, &chatMsg); err == nil {
						// Save to database if repository is available
						if h.Repo != nil {
							var senderUUID pgtype.UUID
							if err := senderUUID.Scan(sender.UserID); err == nil {
								ctx := context.Background()
								// Save message to database (use null UUID for global chat - no room)
								_, err := h.Repo.CreateMessage(ctx, pgtype.UUID{Valid: false}, senderUUID, chatMsg.Content)
								if err != nil {
									log.Printf("Failed to save chat message to database: %v", err)
								}
							}
						}
					}
				}
			}

			// Publish to NATS for global messages if enabled and message doesn't have a MessageID
			if h.NATSEnabled && h.NATS != nil && message.Room == nil && message.MessageID == "" {
				if err := h.NATS.Publish(natsclient.SubjectGlobalChat, message); err != nil {
					log.Printf("Failed to publish global message to NATS: %v", err)
				}
			}

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
				clientsToRemove := make([]*clientpkg.Client, 0)

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
func (h *Hub) GetRoomList(client *clientpkg.Client) []types.RoomDTO {
	h.Mutex.RLock()
	rooms := make(map[string]*room.Room)
	for name, r := range h.Rooms {
		rooms[name] = r
	}
	h.Mutex.RUnlock()

	roomList := make([]types.RoomDTO, 0, len(rooms))
	for name, room := range rooms {
		room.Mutex.RLock()
		clientCount := len(room.Clients)
		isCreator := room.Creator == client
		room.Mutex.RUnlock()

		roomInfo := types.RoomDTO{
			Name:        name,
			Private:     room.Private,
			ClientCount: clientCount,
			IsCreator:   isCreator,
		}
		roomList = append(roomList, roomInfo)
	}
	return roomList
}

// LoadRoomsFromDB loads all rooms from the database into memory
func (h *Hub) LoadRoomsFromDB() {
	if h.Repo == nil {
		return
	}

	ctx := context.Background()
	dbRooms, err := h.Repo.GetAllRooms(ctx)
	if err != nil {
		log.Printf("Failed to load rooms from DB: %v", err)
		return
	}

	h.Mutex.Lock()
	for _, dbRoom := range dbRooms {
		room := room.NewRoom(dbRoom.Name, dbRoom.Private.Bool, dbRoom.PasswordHash.String, 100)
		room.ID = uuid.UUID(dbRoom.ID.Bytes).String()
		// Creator not loaded, set to nil
		h.Rooms[dbRoom.Name] = room
	}
	h.Mutex.Unlock()

	log.Printf("Loaded %d rooms from database", len(dbRooms))
}
