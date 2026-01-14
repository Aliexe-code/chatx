package hub

import (
	"context"
	"encoding/json"
	"testing"
)

func TestCreateAndListRoom(t *testing.T) {
	// Create a hub without repo (in-memory only)
	h := NewHub(context.Background(), nil)

	// Create a room
	_, err := h.CreateRoom("TestRoom", false, "", 10)
	if err != nil {
		t.Fatalf("Failed to create room: %v", err)
	}

	// Verify it's in the map
	if _, exists := h.Rooms["TestRoom"]; !exists {
		t.Errorf("Room not found in map")
	}

	// Get list
	list := h.GetRoomList(nil)
	found := false
	for _, r := range list {
		if r.Name == "TestRoom" {
			found = true
			break
		}
	}

	if !found {
		t.Errorf("Room not found in GetRoomList")
	}

	// Test Marshalling (simulate handler)
	roomListJSON, err := json.Marshal(list)
	if err != nil {
		t.Fatalf("Failed to marshal room list: %v", err)
	}

	msgStr := string(roomListJSON)
	if msgStr == "" {
		t.Errorf("Marshalled JSON is empty")
	}
	t.Logf("JSON: %s", msgStr)
}
