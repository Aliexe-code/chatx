package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"time"

	"websocket-demo/internal/client"
	"websocket-demo/internal/hub"
	"websocket-demo/internal/types"

	"github.com/coder/websocket"
	"github.com/labstack/echo/v4"
)

// HandleWebSocket handles individual WebSocket client connections
// It manages the lifecycle of each client connection including registration, message handling, and cleanup
func HandleWebSocket(hub *hub.Hub, c echo.Context) error {
	log.Printf("New WebSocket connection attempt from %s", c.RealIP())

	opts := &websocket.AcceptOptions{
		OriginPatterns: []string{"*"},
	}

	conn, err := websocket.Accept(c.Response(), c.Request(), opts)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return echo.NewHTTPError(400, "WebSocket upgrade failed")
	}
	log.Printf("WebSocket connection established successfully")

	defer func() {
		if conn != nil {
			conn.Close(websocket.StatusNormalClosure, "server shutting down")
		}
	}()

	// Generate a unique user name
	userName := fmt.Sprintf("User%d", rand.Intn(9000)+1000)
	newClient := client.NewClient(conn, userName)
	log.Printf("Created client with name: %s", userName)

	// Register the client
	hub.Register <- newClient
	log.Printf("Client %s queued for registration", userName)

	// Wait for this client's registration to complete with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	select {
	case <-newClient.Registered:
		log.Printf("Registration confirmed for %s", userName)
	case <-ctx.Done():
		log.Printf("Registration timeout for %s", userName)
		return echo.NewHTTPError(408, "Registration timeout")
	}

	// Send welcome message to the new client
	timestamp := time.Now().Format("15:04:05")
	welcomeMsg := []byte(fmt.Sprintf("[%s] Welcome to the chat! Your name is %s", timestamp, userName))
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
			hub.Unregister <- newClient
			break
		}

		log.Printf("Received message from %s: %s", userName, string(message))

		// Parse WebSocket message
		wsMsg, err := ParseWebSocketMessage(message)
		if err != nil {
			log.Printf("Error parsing WebSocket message from %s: %v", userName, err)
			errorMsg := []byte(fmt.Sprintf("Error parsing message: %v", err))
			newClient.Conn.Write(context.Background(), websocket.MessageText, errorMsg)
		} else if wsMsg != nil {
			log.Printf("Parsed WebSocket message type: %s", wsMsg.Type)
			err := HandleWebSocketMessage(hub, newClient, wsMsg)
			if err != nil {
				log.Printf("Error handling WebSocket message from %s: %v", userName, err)
				errorMsg := []byte(fmt.Sprintf("Error: %v", err))
				newClient.Conn.Write(context.Background(), websocket.MessageText, errorMsg)
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
			case hub.Broadcast <- types.Message{Content: formattedMsg, Sender: newClient, Type: types.MsgTypeChat}:
				log.Printf("Message from %s queued for broadcast", userName)
			case <-ctx.Done():
				log.Printf("Broadcast timeout for %s", userName)
				// Continue processing other messages
			}
		}
	}

	return nil
}

// HandleWebSocketMessage processes WebSocket messages and routes them appropriately
func HandleWebSocketMessage(hub *hub.Hub, client *client.Client, wsMsg *types.WebSocketMessage) error {
	switch wsMsg.Type {
	case types.MsgTypeChat:
		// Handle regular chat message
		timestamp := time.Now().Format("15:04:05")
		formattedMsg := []byte(fmt.Sprintf("[%s] %s: %s", timestamp, client.Name, wsMsg.Data.Content))
		hub.Broadcast <- types.Message{Content: formattedMsg, Sender: client, Type: types.MsgTypeChat}

	case types.MsgTypeRoomMessage:
		// Handle room-specific message
		currentRoom := client.GetCurrentRoom()

		if currentRoom != nil {
			timestamp := time.Now().Format("15:04:05")
			formattedMsg := []byte(fmt.Sprintf("[%s] %s: %s", timestamp, client.Name, wsMsg.Data.Content))
			hub.Broadcast <- types.Message{Content: formattedMsg, Sender: client, Type: types.MsgTypeRoomMessage, Room: currentRoom}
		}

	case types.MsgTypeCreateRoom:
		// Handle room creation
		newRoom, err := hub.CreateRoom(wsMsg.Data.Name, wsMsg.Data.Private, wsMsg.Data.Password, 100)
		if err != nil {
			// Send error message to client
			errorMsg := []byte(fmt.Sprintf("Error creating room: %v", err))
			client.Conn.Write(context.Background(), websocket.MessageText, errorMsg)
		} else {
			// Set the creator
			newRoom.SetCreator(client)
			// Add client to new room
			hub.JoinRoom(client, newRoom, "")
		}

	case types.MsgTypeJoinRoom:
		// Handle room joining
		targetRoom, exists := hub.GetRoom(wsMsg.Data.Name)

		if exists {
			err := hub.JoinRoom(client, targetRoom, wsMsg.Data.Password)
			if err != nil {
				// Send error message to client
				errorMsg := []byte(fmt.Sprintf("Error joining room: %v", err))
				client.Conn.Write(context.Background(), websocket.MessageText, errorMsg)
			}
		} else {
			// Send error message to client
			errorMsg := []byte(fmt.Sprintf("Room '%s' does not exist", wsMsg.Data.Name))
			client.Conn.Write(context.Background(), websocket.MessageText, errorMsg)
		}

	case types.MsgTypeLeaveRoom:
		// Handle room leaving
		hub.LeaveRoom(client)

	case types.MsgTypeListRooms:
		// Handle room listing with detailed info
		roomList := hub.GetRoomList(client)
		roomListJSON, _ := json.Marshal(roomList)
		listMsg := []byte(fmt.Sprintf("ROOMS_LIST:%s", string(roomListJSON)))
		client.Conn.Write(context.Background(), websocket.MessageText, listMsg)

	case types.MsgTypeDeleteRoom:
		// Handle room deletion
		err := hub.DeleteRoom(client, wsMsg.Data.Name)
		if err != nil {
			// Send error message to client
			errorMsg := []byte(fmt.Sprintf("Error deleting room: %v", err))
			client.Conn.Write(context.Background(), websocket.MessageText, errorMsg)
		} else {
			// Send success message
			successMsg := []byte(fmt.Sprintf("Room '%s' deleted successfully", wsMsg.Data.Name))
			client.Conn.Write(context.Background(), websocket.MessageText, successMsg)
		}

	default:
		// Unknown message type
		errorMsg := []byte(fmt.Sprintf("Unknown message type: %s", wsMsg.Type))
		client.Conn.Write(context.Background(), websocket.MessageText, errorMsg)
	}

	return nil
}

// ParseWebSocketMessage parses a WebSocket message from JSON
func ParseWebSocketMessage(message []byte) (*types.WebSocketMessage, error) {
	var wsMsg types.WebSocketMessage
	err := json.Unmarshal(message, &wsMsg)
	if err != nil {
		return nil, err
	}
	return &wsMsg, nil
}