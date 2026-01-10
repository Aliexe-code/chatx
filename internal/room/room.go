package room

import (
	"sync"
	"time"

	"websocket-demo/internal/client"
)

// Room represents a chat room
type Room struct {
	Name       string
	Clients    map[*client.Client]bool
	Mutex      sync.RWMutex
	Created    time.Time
	Private    bool
	Password   string
	MaxClients int
	Active     bool
	Creator    *client.Client
}

// NewRoom creates a new room instance
func NewRoom(name string, private bool, password string, maxClients int) *Room {
	return &Room{
		Name:       name,
		Clients:    make(map[*client.Client]bool),
		Created:    time.Now(),
		Private:    private,
		Password:   password,
		MaxClients: maxClients,
		Active:     true,
	}
}

// AddClient adds a client to the room
func (r *Room) AddClient(client *client.Client) bool {
	r.Mutex.Lock()
	defer r.Mutex.Unlock()

	if len(r.Clients) >= r.MaxClients {
		return false
	}

	r.Clients[client] = true
	return true
}

// RemoveClient removes a client from the room
func (r *Room) RemoveClient(client *client.Client) {
	r.Mutex.Lock()
	defer r.Mutex.Unlock()
	delete(r.Clients, client)
}

// GetClientCount returns the number of clients in the room
func (r *Room) GetClientCount() int {
	r.Mutex.RLock()
	defer r.Mutex.RUnlock()
	return len(r.Clients)
}

// GetClients returns a copy of the clients map
func (r *Room) GetClients() []*client.Client {
	r.Mutex.RLock()
	defer r.Mutex.RUnlock()

	clients := make([]*client.Client, 0, len(r.Clients))
	for client := range r.Clients {
		clients = append(clients, client)
	}
	return clients
}

// IsCreator checks if the given client is the creator of the room
func (r *Room) IsCreator(client *client.Client) bool {
	r.Mutex.RLock()
	defer r.Mutex.RUnlock()
	return r.Creator == client
}

// SetCreator sets the creator of the room
func (r *Room) SetCreator(client *client.Client) {
	r.Mutex.Lock()
	defer r.Mutex.Unlock()
	r.Creator = client
}