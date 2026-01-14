package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"
)

const (
	serverURL   = "http://localhost:8080"
	wsURL       = "ws://localhost:8080/ws"
	numClients  = 100 // Increased to 50 for higher concurrency
	numRooms    = 10  // 10 shared rooms to test concurrent access
	numMessages = 5   // Messages per client
)

// AuthResponse represents the login response
type AuthResponse struct {
	Token    string `json:"token"`
	Username string `json:"username"`
	UserID   string `json:"user_id"`
}

// TestStats tracks test statistics
type TestStats struct {
	TotalClients     int32
	SuccessfulLogins int32
	FailedLogins     int32
	RoomsCreated     int32
	RoomsJoined      int32
	MessagesSent     int32
	MessagesReceived int32
	RoomsLeft        int32
	Errors           int32
}

var stats TestStats

func main() {
	log.Println("Starting concurrency test...")
	log.Printf("Configuration: %d clients, %d rooms, %d messages per client", numClients, numRooms, numMessages)

	var wg sync.WaitGroup
	startTime := time.Now()

	// Create N clients concurrently
	for i := 0; i < numClients; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			runClient(id)
		}(i)
	}

	wg.Wait()
	duration := time.Since(startTime)

	// Print statistics
	printStats(duration)
}

func printStats(duration time.Duration) {
	log.Println("\n=== Concurrency Test Results ===")
	log.Printf("Duration: %v", duration)
	log.Printf("Total Clients: %d", atomic.LoadInt32(&stats.TotalClients))
	log.Printf("Successful Logins: %d", atomic.LoadInt32(&stats.SuccessfulLogins))
	log.Printf("Failed Logins: %d", atomic.LoadInt32(&stats.FailedLogins))
	log.Printf("Rooms Created: %d", atomic.LoadInt32(&stats.RoomsCreated))
	log.Printf("Rooms Joined: %d", atomic.LoadInt32(&stats.RoomsJoined))
	log.Printf("Messages Sent: %d", atomic.LoadInt32(&stats.MessagesSent))
	log.Printf("Messages Received: %d", atomic.LoadInt32(&stats.MessagesReceived))
	log.Printf("Rooms Left: %d", atomic.LoadInt32(&stats.RoomsLeft))
	log.Printf("Errors: %d", atomic.LoadInt32(&stats.Errors))

	// Calculate success rate
	totalOps := atomic.LoadInt32(&stats.SuccessfulLogins) + atomic.LoadInt32(&stats.RoomsCreated) +
		atomic.LoadInt32(&stats.RoomsJoined) + atomic.LoadInt32(&stats.MessagesSent) +
		atomic.LoadInt32(&stats.RoomsLeft)
	expectedOps := int32(numClients * (1 + 1 + 1 + numMessages + 1)) // login, create, join, messages, leave

	successRate := float64(totalOps) / float64(expectedOps) * 100
	log.Printf("Success Rate: %.2f%%", successRate)

	if successRate >= 95 {
		log.Println("✓ Test PASSED: High concurrency handling is working correctly")
	} else {
		log.Println("✗ Test FAILED: Concurrency issues detected")
	}
}

func runClient(id int) {
	atomic.AddInt32(&stats.TotalClients, 1)

	token, err := registerAndLogin(id)
	if err != nil {
		atomic.AddInt32(&stats.FailedLogins, 1)
		atomic.AddInt32(&stats.Errors, 1)
		log.Printf("[Client %d] Login failed: %v", id, err)
		return
	}
	atomic.AddInt32(&stats.SuccessfulLogins, 1)

	// Connect WS
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	conn, _, err := websocket.Dial(ctx, fmt.Sprintf("%s?token=%s", wsURL, token), nil)
	if err != nil {
		atomic.AddInt32(&stats.Errors, 1)
		log.Printf("[Client %d] WS Connect failed: %v", id, err)
		return
	}
	defer conn.Close(websocket.StatusNormalClosure, "bye")

	log.Printf("[Client %d] Connected", id)

	// Start message reader in goroutine
	var wg sync.WaitGroup
	wg.Add(1)
	readerDone := make(chan struct{})
	go func() {
		defer wg.Done()
		defer close(readerDone)
		
		// Create a timeout context for reading messages
		readCtx, readCancel := context.WithTimeout(ctx, 30*time.Second)
		defer readCancel()
		
		readMessages(id, conn, readCtx)
	}()

	// Channel to signal when room creation is complete
	roomCreated := make(chan error, 1)
	go func() {
		roomName := fmt.Sprintf("stressRoom%d", id)
		createMsg := map[string]interface{}{
			"type": "create_room",
			"data": map[string]interface{}{
				"name":     roomName,
				"private":  false,
				"password": "",
			},
		}
		roomCreated <- writeJSON(ctx, conn, createMsg)
	}()

	// Wait for room creation
	if err := <-roomCreated; err == nil {
		atomic.AddInt32(&stats.RoomsCreated, 1)
	} else {
		atomic.AddInt32(&stats.Errors, 1)
		log.Printf("[Client %d] Create room failed: %v", id, err)
	}

	// Channel to signal when shared room creation is complete
	sharedRoomCreated := make(chan error, 1)
	go func() {
		sharedRoomName := fmt.Sprintf("sharedRoom%d", id%numRooms)
		createSharedMsg := map[string]interface{}{
			"type": "create_room",
			"data": map[string]interface{}{
				"name":     sharedRoomName,
				"private":  false,
				"password": "",
			},
		}
		sharedRoomCreated <- writeJSON(ctx, conn, createSharedMsg)
	}()

	// Wait for shared room creation
	<-sharedRoomCreated

	// Channel to signal when room join is complete
	roomJoined := make(chan error, 1)
	go func() {
		sharedRoomName := fmt.Sprintf("sharedRoom%d", id%numRooms)
		joinMsg := map[string]interface{}{
			"type": "join_room",
			"data": map[string]interface{}{
				"name":     sharedRoomName,
				"password": "",
			},
		}
		roomJoined <- writeJSON(ctx, conn, joinMsg)
	}()

	// Wait for room join
	if err := <-roomJoined; err == nil {
		atomic.AddInt32(&stats.RoomsJoined, 1)
		log.Printf("[Client %d] Joined room %s", id, fmt.Sprintf("sharedRoom%d", id%numRooms))
	} else {
		atomic.AddInt32(&stats.Errors, 1)
		log.Printf("[Client %d] Join room failed: %v", id, err)
	}

	// Wait for server to process the join before sending messages
	time.Sleep(1 * time.Second)

	// Channel to signal all messages are sent
	messagesSent := make(chan struct{})
	go func() {
		defer close(messagesSent)
		
		// Test 3: Send messages concurrently
		var msgWg sync.WaitGroup
		for i := 0; i < numMessages; i++ {
			msgWg.Add(1)
			go func(msgNum int) {
				defer msgWg.Done()
				
				message := fmt.Sprintf("Message %d from client %d", msgNum+1, id)
				chatMsg := map[string]interface{}{
					"type": "room_message",
					"data": map[string]interface{}{
						"content": message,
					},
				}
				if err := writeJSON(ctx, conn, chatMsg); err == nil {
					atomic.AddInt32(&stats.MessagesSent, 1)
				} else {
					atomic.AddInt32(&stats.Errors, 1)
					log.Printf("[Client %d] Send message failed: %v", id, err)
				}
				
				// Small random delay between messages
				time.Sleep(time.Duration(rand.Intn(50)) * time.Millisecond)
			}(i)
		}
		msgWg.Wait()
	}()

	// Wait for all messages to be sent
	<-messagesSent

	// Wait for broadcasts to be received before leaving room
	time.Sleep(3 * time.Second)

	// Channel to signal when leave room is complete
	roomLeft := make(chan error, 1)
	go func() {
		leaveMsg := map[string]interface{}{
			"type": "leave_room",
			"data": map[string]interface{}{},
		}
		roomLeft <- writeJSON(ctx, conn, leaveMsg)
	}()

	// Wait for leave room
	if err := <-roomLeft; err == nil {
		atomic.AddInt32(&stats.RoomsLeft, 1)
	} else {
		atomic.AddInt32(&stats.Errors, 1)
		log.Printf("[Client %d] Leave room failed: %v", id, err)
	}

	// Wait a bit more for final messages to be received
	time.Sleep(1 * time.Second)

	// Cancel context to stop reader
	cancel()
	
	// Wait for reader to finish with timeout
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	
	select {
	case <-done:
		// Reader finished normally
	case <-time.After(5 * time.Second):
		// Timeout waiting for reader
		log.Printf("[Client %d] Reader timeout", id)
	}
	
	log.Printf("[Client %d] Done", id)
}

func readMessages(clientID int, conn *websocket.Conn, ctx context.Context) {
	for {
		_, message, err := conn.Read(ctx)
		if err != nil {
			return
		}

		atomic.AddInt32(&stats.MessagesReceived, 1)

		// Parse message to verify it's valid JSON
		var msg map[string]interface{}
		if err := json.Unmarshal(message, &msg); err == nil {
			// Successfully parsed message
			if msgType, ok := msg["type"].(string); ok {
				// Log important message types
				if msgType == "room_message" || msgType == "error" {
					log.Printf("[Client %d] Received: %s", clientID, msgType)
				}
			}
		}
	}
}

func registerAndLogin(id int) (string, error) {
	username := fmt.Sprintf("stressUser%d", id)
	email := fmt.Sprintf("stress%d@test.com", id)
	password := "password123"

	// Register
	regPayload := map[string]string{
		"username": username,
		"email":    email,
		"password": password,
	}
	regBody, _ := json.Marshal(regPayload)
	resp, err := http.Post(serverURL+"/api/register", "application/json", bytes.NewBuffer(regBody))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	// If 409, assume user exists and try login
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusConflict {
		return "", fmt.Errorf("register failed: %d", resp.StatusCode)
	}

	// Login
	loginPayload := map[string]string{
		"email":    email,
		"password": password,
	}
	loginBody, _ := json.Marshal(loginPayload)
	resp, err = http.Post(serverURL+"/api/login", "application/json", bytes.NewBuffer(loginBody))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("login failed: %d", resp.StatusCode)
	}

	var authResp AuthResponse
	if err := json.NewDecoder(resp.Body).Decode(&authResp); err != nil {
		return "", err
	}

	return authResp.Token, nil
}

func writeJSON(ctx context.Context, conn *websocket.Conn, v interface{}) error {
	w, err := conn.Writer(ctx, websocket.MessageText)
	if err != nil {
		return err
	}
	defer w.Close()
	return json.NewEncoder(w).Encode(v)
}
