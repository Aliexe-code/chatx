package types

import (
	"time"
)

// Message represents a message to be broadcast to all clients
type Message struct {
	Content   []byte
	Sender    interface{} // Can be *client.Client
	Type      string      // "chat", "join", "leave"
	Room      interface{} // Can be *room.Room
	Timestamp time.Time
}

// WebSocketMessage represents a WebSocket message structure
type WebSocketMessage struct {
	Type string `json:"type"`
	Data struct {
		Name     string `json:"name,omitempty"`
		Password string `json:"password,omitempty"`
		Content  string `json:"content,omitempty"`
		Private  bool   `json:"private,omitempty"`
		Limit    int    `json:"limit,omitempty"`
		Offset   int    `json:"offset,omitempty"`
	} `json:"data,omitempty"`
}

// ChatMessage represents a chat message in JSON format
type ChatMessage struct {
	Type      string `json:"type"`
	Timestamp string `json:"timestamp"`
	Sender    string `json:"sender"`
	Content   string `json:"content"`
	Room      string `json:"room,omitempty"`
}

// RoomDTO represents a room information sent to clients
type RoomDTO struct {
	Name        string `json:"name"`
	Private     bool   `json:"private"`
	ClientCount int    `json:"clientCount"`
	IsCreator   bool   `json:"isCreator"`
}

// Message type constants
const (
	MsgTypeChat        = "chat"
	MsgTypeJoin        = "join"
	MsgTypeLeave       = "leave"
	MsgTypeRoomJoin    = "room_join"
	MsgTypeRoomLeave   = "room_leave"
	MsgTypeCreateRoom  = "create_room"
	MsgTypeJoinRoom    = "join_room"
	MsgTypeLeaveRoom   = "leave_room"
	MsgTypeListRooms   = "list_rooms"
	MsgTypeRoomMessage = "room_message"
	MsgTypeDeleteRoom  = "delete_room"
	MsgTypeGetMessages = "get_messages"
)
