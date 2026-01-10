package room

import (
	"fmt"
	"sync"
	"testing"

	"websocket-demo/internal/client"

	"github.com/stretchr/testify/assert"
)

func TestNewRoom(t *testing.T) {
	room := NewRoom("test-room", false, "", 100)

	assert.NotNil(t, room)
	assert.Equal(t, "test-room", room.Name)
	assert.False(t, room.Private)
	assert.Equal(t, "", room.Password)
	assert.Equal(t, 100, room.MaxClients)
	assert.True(t, room.Active)
	assert.Nil(t, room.Creator)
	assert.Equal(t, 0, room.GetClientCount())
}

func TestAddClient(t *testing.T) {
	room := NewRoom("test-room", false, "", 2)

	client1 := &client.Client{Name: "Client1"}
	client2 := &client.Client{Name: "Client2"}
	client3 := &client.Client{Name: "Client3"}

	// Add first client
	assert.True(t, room.AddClient(client1))
	assert.Equal(t, 1, room.GetClientCount())

	// Add second client
	assert.True(t, room.AddClient(client2))
	assert.Equal(t, 2, room.GetClientCount())

	// Try to add third client (should fail due to max clients)
	assert.False(t, room.AddClient(client3))
	assert.Equal(t, 2, room.GetClientCount())
}

func TestRemoveClient(t *testing.T) {
	room := NewRoom("test-room", false, "", 100)

	client1 := &client.Client{Name: "Client1"}
	client2 := &client.Client{Name: "Client2"}

	// Add clients
	room.AddClient(client1)
	room.AddClient(client2)
	assert.Equal(t, 2, room.GetClientCount())

	// Remove one client
	room.RemoveClient(client1)
	assert.Equal(t, 1, room.GetClientCount())

	// Remove non-existent client (should not panic)
	room.RemoveClient(client1)
	assert.Equal(t, 1, room.GetClientCount())
}

func TestGetClients(t *testing.T) {
	room := NewRoom("test-room", false, "", 100)

	client1 := &client.Client{Name: "Client1"}
	client2 := &client.Client{Name: "Client2"}

	// Add clients
	room.AddClient(client1)
	room.AddClient(client2)

	// Get clients
	clients := room.GetClients()
	assert.Equal(t, 2, len(clients))

	// Verify returned list is a copy (modifying it shouldn't affect room)
	clients = append(clients, &client.Client{Name: "Client3"})
	assert.Equal(t, 2, room.GetClientCount())
}

func TestIsCreator(t *testing.T) {
	room := NewRoom("test-room", false, "", 100)

	creator := &client.Client{Name: "Creator"}
	otherClient := &client.Client{Name: "Other"}

	// Initially no creator
	assert.False(t, room.IsCreator(creator))

	// Set creator
	room.SetCreator(creator)
	assert.True(t, room.IsCreator(creator))
	assert.False(t, room.IsCreator(otherClient))
}

func TestConcurrentAccess(t *testing.T) {
	room := NewRoom("test-room", false, "", 1000)

	var wg sync.WaitGroup
	const numGoroutines = 100

	// Concurrently add and remove clients
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			client := &client.Client{Name: fmt.Sprintf("Client%d", id)}
			room.AddClient(client)
			room.RemoveClient(client)
		}(i)
	}

	wg.Wait()
	assert.Equal(t, 0, room.GetClientCount())
}