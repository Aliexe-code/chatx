package client

import (
	"fmt"
	"sync"
	"testing"

	"github.com/coder/websocket"
	"github.com/stretchr/testify/assert"
)

func TestNewClient(t *testing.T) {
	conn := &websocket.Conn{}
	client := NewClient(conn, "TestUser")

	assert.NotNil(t, client)
	assert.Equal(t, conn, client.Conn)
	assert.Equal(t, "TestUser", client.Name)
	assert.NotNil(t, client.Registered)
	assert.Nil(t, client.GetCurrentRoom())
}

func TestSetCurrentRoom(t *testing.T) {
	client := NewClient(nil, "TestUser")

	room := "test-room"
	client.SetCurrentRoom(room)

	assert.Equal(t, room, client.GetCurrentRoom())
}

func TestConcurrentRoomAccess(t *testing.T) {
	client := NewClient(nil, "TestUser")

	var wg sync.WaitGroup
	const numGoroutines = 100

	// Concurrently set and get current room
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			room := fmt.Sprintf("room-%d", id)
			client.SetCurrentRoom(room)
			client.GetCurrentRoom()
		}(i)
	}

	wg.Wait()
}