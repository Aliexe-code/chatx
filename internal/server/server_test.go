package server

import (
	"context"
	"fmt"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
	"time"

	"websocket-demo/internal/hub"

	"github.com/coder/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewServer(t *testing.T) {
	ctx := context.Background()
	hub := hub.NewHub(ctx)

	server := NewServer(hub)

	assert.NotNil(t, server)
	assert.NotNil(t, server.echo)
	assert.Equal(t, hub, server.hub)
}

func TestSetupRoutes(t *testing.T) {
	ctx := context.Background()
	hub := hub.NewHub(ctx)
	server := NewServer(hub)

	server.SetupRoutes()

	// Verify routes are registered
	routes := server.echo.Routes()
	assert.Greater(t, len(routes), 0)

	// Check for WebSocket route
	hasWSRoute := false
	hasRootRoute := false
	for _, route := range routes {
		if route.Path == "/ws" && route.Method == "GET" {
			hasWSRoute = true
		}
		if route.Path == "/" && route.Method == "GET" {
			hasRootRoute = true
		}
	}

	assert.True(t, hasWSRoute, "WebSocket route should be registered")
	assert.True(t, hasRootRoute, "Root route should be registered")
}

func TestServerStart(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	hub := hub.NewHub(ctx)
	go hub.Run()

	server := NewServer(hub)
	server.SetupRoutes()

	// Start test server
	testServer := httptest.NewServer(server.echo)
	defer testServer.Close()

	// Verify server is accessible by checking WebSocket route
	u, _ := url.Parse(testServer.URL)
	u.Scheme = "ws"
	u.Path = "/ws"

	conn, _, err := websocket.Dial(context.Background(), u.String(), nil)
	require.NoError(t, err)
	defer conn.Close(websocket.StatusNormalClosure, "")

	// Verify we can connect
	assert.NotNil(t, conn)
}

func TestWebSocketConnection(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	hub := hub.NewHub(ctx)
	go hub.Run()

	server := NewServer(hub)
	server.SetupRoutes()

	// Start test server
	testServer := httptest.NewServer(server.echo)
	defer testServer.Close()

	// Connect via WebSocket
	u, _ := url.Parse(testServer.URL)
	u.Scheme = "ws"
	u.Path = "/ws"

	conn, _, err := websocket.Dial(context.Background(), u.String(), nil)
	require.NoError(t, err)
	defer conn.Close(websocket.StatusNormalClosure, "")

	// Verify we can read at least one message (welcome message)
	readCtx, readCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer readCancel()

	_, msg, err := conn.Read(readCtx)
	require.NoError(t, err, "Should be able to read welcome message")
	assert.NotEmpty(t, msg, "Welcome message should not be empty")
}

func TestMultipleWebSocketConnections(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	hub := hub.NewHub(ctx)
	go hub.Run()

	server := NewServer(hub)
	server.SetupRoutes()

	// Start test server
	testServer := httptest.NewServer(server.echo)
	defer testServer.Close()

	// Connect multiple clients concurrently
	const numClients = 10
	var wg sync.WaitGroup
	connections := make([]*websocket.Conn, numClients)

	u, _ := url.Parse(testServer.URL)
	u.Scheme = "ws"
	u.Path = "/ws"

	for i := 0; i < numClients; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			conn, _, err := websocket.Dial(context.Background(), u.String(), nil)
			require.NoError(t, err)
			connections[id] = conn

			// Read welcome message
			_, _, err = conn.Read(context.Background())
			assert.NoError(t, err)
		}(i)
	}

	wg.Wait()

	// Close all connections
	for _, conn := range connections {
		if conn != nil {
			conn.Close(websocket.StatusNormalClosure, "")
		}
	}
}

func TestWebSocketMessageSend(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	hub := hub.NewHub(ctx)
	go hub.Run()

	server := NewServer(hub)
	server.SetupRoutes()

	// Start test server
	testServer := httptest.NewServer(server.echo)
	defer testServer.Close()

	// Connect via WebSocket
	u, _ := url.Parse(testServer.URL)
	u.Scheme = "ws"
	u.Path = "/ws"

	conn, _, err := websocket.Dial(context.Background(), u.String(), nil)
	require.NoError(t, err)
	defer conn.Close(websocket.StatusNormalClosure, "")

	// Read welcome message
	_, _, err = conn.Read(context.Background())
	require.NoError(t, err)

	// Send a chat message
	testMsg := `{"type":"chat","data":{"content":"Hello, World!"}}`
	err = conn.Write(context.Background(), websocket.MessageText, []byte(testMsg))
	require.NoError(t, err)

	// Give time for message to be processed
	time.Sleep(100 * time.Millisecond)
}

func TestWebSocketRoomOperations(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	hub := hub.NewHub(ctx)
	go hub.Run()

	server := NewServer(hub)
	server.SetupRoutes()

	// Start test server
	testServer := httptest.NewServer(server.echo)
	defer testServer.Close()

	// Connect via WebSocket
	u, _ := url.Parse(testServer.URL)
	u.Scheme = "ws"
	u.Path = "/ws"

	conn, _, err := websocket.Dial(context.Background(), u.String(), nil)
	require.NoError(t, err)
	defer conn.Close(websocket.StatusNormalClosure, "")

	// Read welcome message and join notification
	for i := 0; i < 2; i++ {
		_, _, err = conn.Read(context.Background())
		require.NoError(t, err)
	}

	// Create a room
	createRoomMsg := `{"type":"create_room","data":{"name":"test-room","private":false,"password":""}}`
	err = conn.Write(context.Background(), websocket.MessageText, []byte(createRoomMsg))
	require.NoError(t, err)

	time.Sleep(100 * time.Millisecond)

	// List rooms
	listRoomsMsg := `{"type":"list_rooms"}`
	err = conn.Write(context.Background(), websocket.MessageText, []byte(listRoomsMsg))
	require.NoError(t, err)

	// Read room list response (keep reading until we find ROOMS_LIST or timeout)
	readCtx, readCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer readCancel()

	found := false
	for {
		_, msg, err := conn.Read(readCtx)
		if err != nil {
			break
		}
		if contains(string(msg), "ROOMS_LIST") {
			found = true
			break
		}
	}
	assert.True(t, found, "Should receive ROOMS_LIST response")
}

// Helper function to check if string contains substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestWebSocketConnectionRaceConditions(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping race condition test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	hub := hub.NewHub(ctx)
	go hub.Run()

	server := NewServer(hub)
	server.SetupRoutes()

	// Start test server
	testServer := httptest.NewServer(server.echo)
	defer testServer.Close()

	u, _ := url.Parse(testServer.URL)
	u.Scheme = "ws"
	u.Path = "/ws"

	var wg sync.WaitGroup
	const numConnections = 20

	// Concurrently connect and disconnect clients
	for i := 0; i < numConnections; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			conn, _, err := websocket.Dial(ctx, u.String(), nil)
			if err != nil {
				return
			}

			// Send some messages
			for j := 0; j < 5; j++ {
				msg := `{"type":"chat","data":{"content":"Test message"}}`
				conn.Write(ctx, websocket.MessageText, []byte(msg))
				time.Sleep(10 * time.Millisecond)
			}

			conn.Close(websocket.StatusNormalClosure, "")
		}(i)
	}

	wg.Wait()
}

func TestServerGracefulShutdown(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	hub := hub.NewHub(ctx)
	go hub.Run()

	server := NewServer(hub)
	server.SetupRoutes()

	// Start test server
	testServer := httptest.NewServer(server.echo)

	// Connect a client
	u, _ := url.Parse(testServer.URL)
	u.Scheme = "ws"
	u.Path = "/ws"

	conn, _, err := websocket.Dial(context.Background(), u.String(), nil)
	require.NoError(t, err)

	// Read welcome message
	_, _, err = conn.Read(context.Background())
	require.NoError(t, err)

	// Trigger graceful shutdown
	cancel()
	server.Shutdown()
	testServer.Close()

	// Close client connection
	conn.Close(websocket.StatusNormalClosure, "")

	// Give time for cleanup
	time.Sleep(100 * time.Millisecond)
}

func TestConcurrentServerOperations(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping concurrent operations test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	hub := hub.NewHub(ctx)
	go hub.Run()

	server := NewServer(hub)
	server.SetupRoutes()

	// Start test server
	testServer := httptest.NewServer(server.echo)
	defer testServer.Close()

	u, _ := url.Parse(testServer.URL)
	u.Scheme = "ws"
	u.Path = "/ws"

	var wg sync.WaitGroup
	const numGoroutines = 30

	// Concurrently perform various operations
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			conn, _, err := websocket.Dial(ctx, u.String(), nil)
			if err != nil {
				return
			}
			defer conn.Close(websocket.StatusNormalClosure, "")

			// Read welcome
			conn.Read(ctx)

			// Perform room operations
			operations := []string{
				`{"type":"create_room","data":{"name":"room-%d","private":false,"password":""}}`,
				`{"type":"list_rooms"}`,
				`{"type":"chat","data":{"content":"Message %d"}}`,
			}

			for j, op := range operations {
				msg := op
				if j == 0 {
					msg = fmt.Sprintf(op, id)
				} else if j == 2 {
					msg = fmt.Sprintf(op, id)
				}
				conn.Write(ctx, websocket.MessageText, []byte(msg))
				time.Sleep(20 * time.Millisecond)
			}
		}(i)
	}

	wg.Wait()
}