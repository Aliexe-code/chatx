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
	} `json:"data,omitempty"`
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
)