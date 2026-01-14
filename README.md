# WebSocket Chat Server

A high-performance, WebSocket chat application with comprehensive security, database persistence, and monitoring capabilities.

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

## 📦 Installation

### Prerequisites
- Go 1.21 or higher
- PostgreSQL 13 or higher
- Git


### WebSocket Responses

#### System Messages
- **Join Notification**: `[15:30:45] john_doe has joined the room`
- **Leave Notification**: `[15:32:10] john_doe has left the room`
- **Leave Confirmation**: `You have left the room "general-chat"`
- **Success Response**: `ROOM_LEAVE_SUCCESS:You have successfully left the room`
- **Room List**: `ROOMS_LIST:[{"name":"general","private":false,"id":"uuid"}]`

#### Error Messages
- **Validation Error**: `"Message rejected: message too large (max 65536 bytes)"`
- **Authentication Error**: `"Authorization header with Bearer token required"`
- **Room Not Found**: `"Error: room does not exist"`
- **Invalid Password**: `"Error: incorrect room password"`
- **Rate Limit Error**: `"Error: rate limit exceeded"`


## 🔒 Security Features

### Input Validation Rules
- **Username**: 3-30 characters, alphanumeric + `_` + `-` (no reserved names)
- **Email**: Valid email format with RFC compliance
- **Password**: Minimum 8 characters, no common passwords
- **Room Name**: 2-50 characters, alphanumeric + spaces + `_` + `-`
- **Message Size**: Configurable limit (default 64KB, maximum 1MB)

### Authentication Flow
1. **Register**: Create account with validation and password hashing
2. **Login**: Authenticate with JWT token response
3. **WebSocket Connect**: Use JWT in Authorization header
4. **Token Validation**: Verify JWT on each WebSocket connection
5. **Session Management**: Secure token-based sessions

### Rate Limiting
- **HTTP Endpoints**: Configurable requests per minute per IP
- **WebSocket Messages**: Message rate limits per user
- **Sliding Window**: Advanced rate limiting with in-memory storage
- **IP-based Protection**: Per-client IP rate limiting
- **User-based Protection**: Per-authenticated-user rate limiting

