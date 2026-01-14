# ChatX - Real-Time WebSocket Chat Application

A high-performance, concurrent-safe real-time chat application built with Go, WebSockets, and the Echo framework. Features public/private rooms, password protection, and a hub-based architecture for scalable WebSocket connection management.

![Go Version](https://img.shields.io/badge/Go-1.25.5+-00ADD8?style=flat&logo=go)
![License](https://img.shields.io/badge/license-MIT-blue.svg)
![WebSocket](https://img.shields.io/badge/WebSocket-RFC--6455-010101?style=flat)


## 🚀 Features

### 📡 WebSocket Features
- **Real-time Messaging**: Instant message delivery in chat rooms
- **Room Management**: Create, join, leave, delete with password protection
- **Private Rooms**: Password-protected rooms with secure authentication
- **Public Rooms**: Open-access rooms for general discussions
- **Message History**: Paginated message retrieval with filtering
- **User Presence**: Track online users and room membership in real-time
- **Broadcast System**: Efficient multi-client message delivery
- **Connection Management**: Graceful client connection handling with cleanup
- **Leave Notifications**: User feedback and room member notifications
- **Message Size Limits**: Configurable limits to prevent DoS attacks



### 🛡️ Security & Authentication
- **JWT Authentication**: Secure token-based user authentication with bcrypt password hashing
- **CSRF Protection**: Cross-site request forgery prevention with token validation
- **Input Validation**: Comprehensive validation for usernames, emails, passwords, and messages
- **Rate Limiting**: Protection against message flooding and API abuse with configurable limits
- **Password Hashing**: bcrypt hashing for both user and room passwords
- **Audit Logging**: Security event tracking and monitoring for compliance
- **Environment Variables**: Secure configuration management without hardcoded secrets

### 🗄️ Database Integration
- **PostgreSQL Integration**: Full SQLC-powered type-safe database operations
- **Repository Pattern**: Clean data access abstraction with proper error handling
- **Migration System**: Database schema evolution management with versioning
- **Connection Pooling**: Configurable database connections for optimal performance
- **Persistent Storage**: Users, rooms, messages, and room memberships
- **ACID Compliance**: Transaction-safe database operations

## 🛠️ Technology Stack

- **Backend**: Go 1.21+ with Echo framework and gorilla/websocket
- **Database**: PostgreSQL 13+ with SQLC for type-safe queries
- **Authentication**: JWT tokens with bcrypt password hashing
- **Security**: CSRF protection, rate limiting, input validation
- **Testing**: Comprehensive test suite with race detection



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

## 🚧 Roadmap

### Phase 1: Core Features ✅
- [x] WebSocket connection management
- [x] Public and private rooms
- [x] Room management (create, join, leave, delete)
- [x] Message broadcasting
- [x] Concurrent-safe architecture
- [x] Comprehensive testing

### Phase 2: Authentication & Persistence
- [X]JWT authentication
- [X] PostgreSQL integration
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

