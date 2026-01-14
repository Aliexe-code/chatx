package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"websocket-demo/internal/client"
	"websocket-demo/internal/hub"
	"websocket-demo/internal/room"
	"websocket-demo/internal/types"

	"github.com/coder/websocket"
	"github.com/jackc/pgx/v5/pgtype"
)

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
			// Save message to database if client is authenticated
			if client.Authenticated && client.UserID != "" && hub.Repo != nil {
				if room, ok := currentRoom.(*room.Room); ok && room.ID != "" {
					ctx := context.Background()
					var senderUUID pgtype.UUID
					var roomUUID pgtype.UUID
					if err := senderUUID.Scan(client.UserID); err == nil {
						if err := roomUUID.Scan(room.ID); err == nil {
							_, err := hub.Repo.CreateMessage(ctx, roomUUID, senderUUID, wsMsg.Data.Content)
							if err != nil {
								log.Printf("Failed to save room message to database: %v", err)
							}
						}
					}
				}
			}

			timestamp := time.Now().Format("15:04:05")
			formattedMsg := []byte(fmt.Sprintf("[%s] %s: %s", timestamp, client.Name, wsMsg.Data.Content))
			hub.Broadcast <- types.Message{Content: formattedMsg, Sender: client, Type: types.MsgTypeRoomMessage, Room: currentRoom}
			// Send success message to sender
			successMsg := []byte("Message sent to room")
			client.Conn.Write(context.Background(), websocket.MessageText, successMsg)
		} else {
			// Send error message if not in a room
			errorMsg := []byte("You are not in a room")
			client.Conn.Write(context.Background(), websocket.MessageText, errorMsg)
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
			// Send success message
			successMsg := []byte(fmt.Sprintf("Room '%s' created successfully", wsMsg.Data.Name))
			client.Conn.Write(context.Background(), websocket.MessageText, successMsg)
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

		// Send leave confirmation response
		leaveResponse := []byte("ROOM_LEAVE_SUCCESS:You have successfully left the room")
		if err := client.Conn.Write(context.Background(), websocket.MessageText, leaveResponse); err != nil {
			log.Printf("Failed to send leave response to client %s: %v", client.Name, err)
		}

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

	case types.MsgTypeGetMessages:
		// Handle getting messages for a room
		// Check if user is joined to the requested room
		currentRoomInterface := client.GetCurrentRoom()
		if currentRoomInterface != nil {
			currentRoom := currentRoomInterface.(*room.Room)
			if currentRoom.Name == wsMsg.Data.Name {
				// User is in the requested room, fetch messages
				ctx := context.Background()

				// Get room ID as pgtype.UUID
				var roomUUID pgtype.UUID
				if err := roomUUID.Scan(currentRoom.ID); err != nil {
					errorMsg := []byte(fmt.Sprintf("Error parsing room ID: %v", err))
					client.Conn.Write(context.Background(), websocket.MessageText, errorMsg)
					break
				}

				// Set default values for limit and offset
				limit := int32(wsMsg.Data.Limit)
				offset := int32(wsMsg.Data.Offset)
				if limit <= 0 {
					limit = 50 // default limit
				}
				if offset < 0 {
					offset = 0 // default offset
				}

				// Fetch messages from database
				messages, err := hub.Repo.ListMessagesByRoom(ctx, roomUUID, limit, offset)
				if err != nil {
					errorMsg := []byte(fmt.Sprintf("Error fetching messages: %v", err))
					client.Conn.Write(context.Background(), websocket.MessageText, errorMsg)
					break
				}

				// Format messages as JSON
				type MessageResponse struct {
					Username  string `json:"username"`
					Content   string `json:"content"`
					Timestamp string `json:"timestamp"`
				}

				var messageResponses []MessageResponse
				for _, msg := range messages {
					messageResponses = append(messageResponses, MessageResponse{
						Username:  msg.Username,
						Content:   msg.Content,
						Timestamp: msg.CreatedAt.Time.Format(time.RFC3339),
					})
				}

				// Send messages back to client
				messagesJSON, _ := json.Marshal(messageResponses)
				responseMsg := []byte(fmt.Sprintf("MESSAGES:%s", string(messagesJSON)))
				client.Conn.Write(context.Background(), websocket.MessageText, responseMsg)
			} else {
				// User is in a different room
				errorMsg := []byte("You can only get messages from the room you have joined")
				client.Conn.Write(context.Background(), websocket.MessageText, errorMsg)
			}
		} else {
			// User is not in any room
			errorMsg := []byte("You must join a room first to get messages")
			client.Conn.Write(context.Background(), websocket.MessageText, errorMsg)
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
