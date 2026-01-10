package hub

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"websocket-demo/internal/client"
	"websocket-demo/internal/room"
	"websocket-demo/internal/types"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewHub(t *testing.T) {
	ctx := context.Background()
	hub := NewHub(ctx)

	assert.NotNil(t, hub)
	assert.Empty(t, hub.Clients)
	assert.NotNil(t, hub.Broadcast)
	assert.NotNil(t, hub.Register)
	assert.NotNil(t, hub.Unregister)
	assert.Equal(t, ctx, hub.Ctx)
}

func TestHubRun(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	hub := NewHub(ctx)
	go hub.Run()

	select {
	case <-hub.Register:
		t.Error("register channel should not have data")
	default:
	}

	select {
	case <-hub.Unregister:
		t.Error("unregister channel should not have data")
	default:
	}

	select {
	case <-hub.Broadcast:
		t.Error("broadcast channel should not have data")
	default:
	}

	cancel()
	time.Sleep(10 * time.Millisecond)
}

// TestRaceConditions tests for race conditions
func TestRaceConditions(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping race condition test in short mode")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	hub := NewHub(ctx)
	go hub.Run()

	var wg sync.WaitGroup
	const numClients = 50
	const numMessages = 100

	// Concurrently register clients
	clients := make([]*client.Client, numClients)
	for i := 0; i < numClients; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			newClient := &client.Client{
				Name:       fmt.Sprintf("RaceClient%d", id),
				Registered: make(chan struct{}),
			}
			clients[id] = newClient
			hub.Register <- newClient
		}(i)
	}

	wg.Wait()
	time.Sleep(100 * time.Millisecond)

	// Concurrently send messages
	for i := 0; i < numMessages; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			msg := types.Message{
				Content: []byte(fmt.Sprintf("Race test message %d", id)),
				Type:    types.MsgTypeChat,
			}
			hub.Broadcast <- msg
		}(i)
	}

	// Concurrently unregister
	for i := 0; i < numClients; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			if clients[id] != nil {
				hub.Unregister <- clients[id]
			}
		}(i)
	}

	wg.Wait()
	time.Sleep(100 * time.Millisecond)
}

// TestHighConcurrencyStress tests high concurrency stress
func TestHighConcurrencyStress(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	hub := NewHub(ctx)
	go hub.Run()

	var wg sync.WaitGroup
	const numClients = 100
	const numMessages = 200

	// Create many concurrent clients
	clients := make([]*client.Client, numClients)
	for i := 0; i < numClients; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			newClient := &client.Client{
				Name:       fmt.Sprintf("StressClient%d", id),
				Registered: make(chan struct{}),
			}
			clients[id] = newClient
			hub.Register <- newClient
		}(i)
	}

	wg.Wait()
	time.Sleep(200 * time.Millisecond)

	// Send many concurrent messages
	for i := 0; i < numMessages; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			msg := types.Message{
				Content: []byte(fmt.Sprintf("Stress message %d", id)),
				Type:    types.MsgTypeChat,
			}
			hub.Broadcast <- msg
		}(i)
	}

	// Concurrently unregister
	for i := 0; i < numClients; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			if clients[id] != nil {
				hub.Unregister <- clients[id]
			}
		}(i)
	}

	wg.Wait()
	time.Sleep(200 * time.Millisecond)
}

// TestHubShutdown tests hub shutdown
func TestHubShutdown(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	hub := NewHub(ctx)
	go hub.Run()

	// Register some clients
	clients := make([]*client.Client, 5)
	for i := 0; i < 5; i++ {
		newClient := &client.Client{
			Name:       fmt.Sprintf("ShutdownClient%d", i),
			Registered: make(chan struct{}),
		}
		clients[i] = newClient
		hub.Register <- newClient
		<-newClient.Registered
	}

	// Send some messages
	for i := 0; i < 3; i++ {
		hub.Broadcast <- types.Message{
			Content: []byte(fmt.Sprintf("Shutdown message %d", i)),
			Type:    types.MsgTypeChat,
		}
	}

	// Cancel context to shut down hub
	cancel()
	time.Sleep(100 * time.Millisecond)
}

// TestConcurrentMapAccess tests concurrent map access
func TestConcurrentMapAccess(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	hub := NewHub(ctx)
	go hub.Run()

	var wg sync.WaitGroup
	const numGoroutines = 50

	// Concurrently access hub's clients map
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			newClient := &client.Client{
				Name:       fmt.Sprintf("MapClient%d", id),
				Registered: make(chan struct{}),
			}

			// Register
			hub.Register <- newClient
			time.Sleep(time.Millisecond * time.Duration(id%5))

			// Unregister
			hub.Unregister <- newClient
		}(i)
	}

	wg.Wait()
	time.Sleep(100 * time.Millisecond)
}

// TestCreateRoom tests room creation
func TestCreateRoom(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	hub := NewHub(ctx)
	go hub.Run()

	// Test creating a valid room
	newRoom, err := hub.CreateRoom("test-room", false, "", 100)
	require.NoError(t, err)
	assert.NotNil(t, newRoom)
	assert.Equal(t, "test-room", newRoom.Name)

	// Test creating duplicate room
	_, err = hub.CreateRoom("test-room", false, "", 100)
	assert.Error(t, err)

	// Test creating room with invalid name
	_, err = hub.CreateRoom("", false, "", 100)
	assert.Error(t, err)

	cancel()
}

// TestJoinRoom tests joining rooms
func TestJoinRoom(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	hub := NewHub(ctx)
	go hub.Run()

	// Create a test room
	testRoom, _ := hub.CreateRoom("test-room", false, "", 100)

	// Create a test client
	testClient := &client.Client{
		Name:       "TestClient",
		Registered: make(chan struct{}),
	}

	// Join the room
	err := hub.JoinRoom(testClient, testRoom, "")
	require.NoError(t, err)

	// Verify client is in the room
	assert.Equal(t, 1, testRoom.GetClientCount())

	cancel()
}

// TestBroadcastToRoom tests broadcasting to a room
func TestBroadcastToRoom(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	hub := NewHub(ctx)
	go hub.Run()

	// Create a test room
	testRoom := room.NewRoom("test-room", false, "", 100)

	// Create test clients
	client1 := &client.Client{
		Name:       "Client1",
		Registered: make(chan struct{}),
	}
	client2 := &client.Client{
		Name:       "Client2",
		Registered: make(chan struct{}),
	}

	// Add clients to room
	testRoom.AddClient(client1)
	testRoom.AddClient(client2)

	// Broadcast message
	msg := types.Message{
		Content: []byte("Test message"),
		Type:    types.MsgTypeChat,
	}
	hub.BroadcastToRoom(testRoom, msg)

	// Verify clients are still in room
	assert.Equal(t, 2, testRoom.GetClientCount())

	cancel()
}