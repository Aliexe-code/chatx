# ChatX - Real-Time WebSocket Chat Application

A high-performance, concurrent-safe real-time chat application built with Go, WebSockets, and the Echo framework. Features public/private rooms, password protection, and a hub-based architecture for scalable WebSocket connection management.

![Go Version](https://img.shields.io/badge/Go-1.25.5+-00ADD8?style=flat&logo=go)
![License](https://img.shields.io/badge/license-MIT-blue.svg)
![WebSocket](https://img.shields.io/badge/WebSocket-RFC--6455-010101?style=flat)

## 🌟 Features

### Core Functionality
- **Real-time Messaging** - Instant message delivery using WebSocket protocol
- **Room Management** - Create, join, leave, and delete chat rooms
- **Public & Private Rooms** - Support for both public and password-protected private rooms
- **User Presence** - Track online users and room membership
- **Broadcast Messaging** - Efficient message broadcasting to all room members
- **Graceful Shutdown** - Clean connection handling on server shutdown

### Technical Highlights
- **Concurrent-Safe Architecture** - Thread-safe operations using mutexes and channels
- **Hub Pattern** - Centralized connection management for scalability
- **Race Condition Free** - All code verified with Go's race detector
- **Comprehensive Testing** - Unit tests with high coverage
- **Clean Architecture** - Separation of concerns with internal packages

## 🚀 Quick Start

### Prerequisites
- Go 1.25.5 or higher
- Modern web browser with WebSocket support

### Installation

1. Clone the repository:
```bash
git clone https://github.com/Aliexe-code/chatx.git
cd chatx
```

2. Install dependencies:
```bash
go mod download
```

3. Run the server:
```bash
make run
```

4. Open your browser and navigate to:
```
http://localhost:8080
```




### Architecture Diagram
```
┌─────────────────────────────────────────────────────────────┐
│                        Client (Browser)                      │
│                    (HTML/CSS/JavaScript)                     │
└────────────────────────┬────────────────────────────────────┘
                         │ WebSocket
                         ▼
┌─────────────────────────────────────────────────────────────┐
│                      Echo HTTP Server                        │
│                         (handler.go)                         │
└────────────────────────┬────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────────┐
│                           Hub                                │
│  ┌──────────────────────────────────────────────────────┐  │
│  │  Clients Map  │  Rooms Map  │  ClientRooms Map      │  │
│  │  (RWMutex)    │  (RWMutex)  │  (RWMutex)            │  │
│  └──────────────────────────────────────────────────────┘  │
│  ┌──────────────────────────────────────────────────────┐  │
│  │  Register Ch  │  Unregister Ch  │  Broadcast Ch      │  │
│  └──────────────────────────────────────────────────────┘  │
└────────────────────────┬────────────────────────────────────┘
                         │
         ┌───────────────┼───────────────┐
         ▼               ▼               ▼
    ┌─────────┐    ┌─────────┐    ┌─────────┐
    │ Room 1  │    │ Room 2  │    │ Room 3  │
    └─────────┘    └─────────┘    └─────────┘
         │               │               │
         └───────────────┼───────────────┘
                         ▼
                    ┌─────────┐
                    │ Clients │
                    └─────────┘
```


## 🔒 Concurrent Safety

This project implements comprehensive concurrent safety measures:

### Thread-Safe Operations
- **Hub Management** - All client and room operations protected with `sync.RWMutex`
- **Room Operations** - Room state protected with atomic operations
- **Client State** - Client room tracking protected with mutexes
- **Message Broadcasting** - Channel-based communication for safe concurrent access

### Race Condition Prevention
- **Atomic Room Creation** - Single lock during check-and-create operations
- **Serialized Room Operations** - `roomOpMutex` prevents concurrent join/leave operations
- **Channel Communication** - All inter-goroutine communication uses channels
- **Verified Safety** - All code passes Go's race detector (`go test -race`)



## 🛠️ Technology Stack

### Backend
- **Go 1.25.5+** - Core programming language
- **Echo v4** - High-performance HTTP framework
- **WebSocket (coder/websocket)** - Real-time communication protocol
- **sync** - Concurrent programming primitives

### Frontend
- **HTML5** - Structure
- **CSS3** - Styling
- **JavaScript (ES6+)** - Client-side logic
- **WebSocket API** - Browser WebSocket support

### Testing
- **testing** - Go's built-in testing framework
- **testify** - Assertion library
- **race detector** - Race condition detection

## 📊 Performance

- **Concurrent Connections** - Supports hundreds of concurrent WebSocket connections
- **Low Latency** - Sub-millisecond message delivery
- **Efficient Broadcasting** - Channel-based message distribution
- **Memory Efficient** - Minimal memory footprint per connection

## 🚧 Roadmap

### Phase 1: Core Features ✅
- [x] WebSocket connection management
- [x] Public and private rooms
- [x] Room management (create, join, leave, delete)
- [x] Message broadcasting
- [x] Concurrent-safe architecture
- [x] Comprehensive testing

### Phase 2: Authentication & Persistence
- [ ] JWT authentication
- [ ] PostgreSQL integration
- [ ] Redis for session management
- [ ] Message persistence
- [ ] User profiles

### Phase 3: Advanced Features
- [ ] Direct messages (1-on-1 chats)
- [ ] File sharing
- [ ] Message reactions
- [ ] Typing indicators
- [ ] Read receipts
- [ ] Message search


## 👤 Author

**Aliexe-code**

- GitHub: [@Aliexe-code](https://github.com/Aliexe-code)
- LinkedIn: [LinkedIn Profile](https://www.linkedin.com/in/ali-mohammed-4685ba318/)

## 🙏 Acknowledgments

- [Echo Framework](https://echo.labstack.com/) - High-performance HTTP framework
- [WebSocket Library](https://github.com/coder/websocket) - WebSocket implementation
- [Go Community](https://golang.org/) - Excellent documentation and community support

**Built with ❤️ using Go and WebSockets**