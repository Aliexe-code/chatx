package server

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strings"
	"time"

	"websocket-demo/internal/auth"
	"websocket-demo/internal/client"
	"websocket-demo/internal/hub"
	"websocket-demo/internal/repository"
	"websocket-demo/internal/types"
	"websocket-demo/internal/validator"

	"github.com/coder/websocket"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"golang.org/x/crypto/bcrypt"
)

type Server struct {
	hub        *hub.Hub
	echo       *echo.Echo
	csrf       *CSRFProtection
	repo       *repository.Repository
	jwtService *auth.JWTService
}

func NewServer(hub *hub.Hub, repo *repository.Repository) *Server {
	e := echo.New()

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		log.Fatal("JWT_SECRET environment variable is required")
	}

	jwtService, _ := auth.NewJWTService(jwtSecret, "24h")

	return &Server{
		hub:        hub,
		echo:       e,
		csrf:       NewCSRFProtection(),
		repo:       repo,
		jwtService: jwtService,
	}
}

func (s *Server) SetupRoutes() {
	s.echo.Use(middleware.Logger())
	s.echo.Use(middleware.Recover())
	s.echo.Use(middleware.CORS())

	s.echo.GET("/", func(c echo.Context) error {
		return c.String(http.StatusOK, "WebSocket Server is running")
	})

	api := s.echo.Group("/api")
	api.POST("/register", s.Register)
	api.POST("/login", s.Login)

	s.echo.GET("/ws", s.HandleWebSocket)
}

type RegisterRequest struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type AuthResponse struct {
	Token    string `json:"token"`
	Username string `json:"username"`
	UserID   string `json:"user_id"`
}

func (s *Server) Register(c echo.Context) error {
	var req RegisterRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request"})
	}

	// Validate input
	validationResult := validator.ValidateRegistration(req.Username, req.Email, req.Password)
	if !validationResult.Valid {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error":   "Validation failed",
			"details": validator.FormatValidationErrors(validationResult.Errors),
		})
	}

	// Check if user already exists
	ctx := c.Request().Context()
	_, err := s.repo.GetUserByEmail(ctx, req.Email)
	if err == nil {
		return c.JSON(http.StatusConflict, map[string]string{"error": "User already exists"})
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to hash password"})
	}

	// Create user
	_, err = s.repo.CreateUser(ctx, req.Username, req.Email, string(hashedPassword))
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to create user"})
	}

	return c.JSON(http.StatusCreated, map[string]string{"message": "User registered successfully"})
}

func (s *Server) Login(c echo.Context) error {
	var req LoginRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request"})
	}

	// Validate email format
	if err := validator.ValidateEmail(req.Email); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	if req.Password == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Password is required"})
	}

	// Get user by email
	ctx := c.Request().Context()
	user, err := s.repo.GetUserByEmail(ctx, req.Email)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Invalid credentials"})
	}

	// Verify password
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Invalid credentials"})
	}

	// Update last login
	s.repo.UpdateUserLastLogin(ctx, user.ID, pgtype.Timestamptz{Time: time.Now(), Valid: true})

	// Generate token
	token, err := s.jwtService.GenerateToken(uuid.UUID(user.ID.Bytes).String(), user.Username)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to generate token"})
	}

	return c.JSON(http.StatusOK, AuthResponse{
		Token:    token,
		Username: user.Username,
		UserID:   uuid.UUID(user.ID.Bytes).String(),
	})
}

func (s *Server) Start(addr string) error {
	return s.echo.Start(addr)
}

func (s *Server) Shutdown() error {
	return s.echo.Close()
}

// HandleWebSocket handles individual WebSocket client connections with JWT authentication
func (s *Server) HandleWebSocket(c echo.Context) error {
	log.Printf("New WebSocket connection attempt from %s", c.RealIP())

	// Extract and validate JWT token from Authorization header
	authHeader := c.Request().Header.Get("Authorization")
	if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
		return echo.NewHTTPError(401, "Authorization header with Bearer token required")
	}

	token := strings.TrimPrefix(authHeader, "Bearer ")
	claims, err := s.jwtService.ValidateToken(token)
	if err != nil {
		log.Printf("JWT validation failed: %v", err)
		return echo.NewHTTPError(401, "Invalid token")
	}

	opts := &websocket.AcceptOptions{
		OriginPatterns: []string{"*"},
	}

	conn, err := websocket.Accept(c.Response(), c.Request(), opts)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return echo.NewHTTPError(400, "WebSocket upgrade failed")
	}
	log.Printf("WebSocket connection established successfully")

	defer func() {
		if conn != nil {
			conn.Close(websocket.StatusNormalClosure, "server shutting down")
		}
	}()

	// Create client with proper authentication
	var userName string
	var userID string
	var authenticated bool

	if claims != nil {
		userName = claims.Username
		userID = claims.UserID
		authenticated = true
		log.Printf("Authenticated client: %s (ID: %s)", userName, userID)
	} else {
		// Fallback for unauthenticated connections
		userName = fmt.Sprintf("User%d", rand.Intn(9000)+1000)
	}

	newClient := client.NewClient(conn, userName)
	if authenticated {
		newClient.Authenticated = true
		newClient.UserID = userID
	} else {
		// Create anonymous user for database persistence
		ctx := context.Background()
		email := fmt.Sprintf("%s@anonymous.local", userName)
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(""), bcrypt.DefaultCost)
		if err != nil {
			log.Printf("Failed to hash empty password for anonymous user: %v", err)
		} else {
			user, err := s.repo.CreateUser(ctx, userName, email, string(hashedPassword))
			if err != nil {
				log.Printf("Failed to create anonymous user: %v", err)
			} else {
				newClient.UserID = uuid.UUID(user.ID.Bytes).String()
			}
		}
	}

	// Register the client
	s.hub.Register <- newClient
	log.Printf("Client %s queued for registration", userName)

	// Wait for this client's registration to complete with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	select {
	case <-newClient.Registered:
		log.Printf("Registration confirmed for %s", userName)
	case <-ctx.Done():
		log.Printf("Registration timeout for %s", userName)
		return echo.NewHTTPError(408, "Registration timeout")
	}

	// Welcome message removed

	// Get max message size limit
	maxMessageSize := validator.GetMaxMessageSize()
	log.Printf("WebSocket message size limit set to: %d bytes", maxMessageSize)

	for {
		_, message, err := conn.Read(context.Background())
		if err != nil {
			log.Printf("Read message error from %s: %v", userName, err)
			s.hub.Unregister <- newClient
			break
		}

		// Validate message size
		if err := validator.ValidateMessageSize(len(message), maxMessageSize); err != nil {
			log.Printf("Message size validation failed from %s: %v (size: %d)", userName, err, len(message))
			errorMsg := []byte(fmt.Sprintf("Message rejected: %v", err))
			newClient.Conn.Write(context.Background(), websocket.MessageText, errorMsg)
			continue // Skip processing this message
		}

		log.Printf("Received message from %s: %s (size: %d bytes)", userName, string(message), len(message))

		// Parse WebSocket message
		wsMsg, err := ParseWebSocketMessage(message)
		if err != nil {
			log.Printf("Error parsing WebSocket message from %s: %v", userName, err)
			errorMsg := []byte(fmt.Sprintf("Error parsing message: %v", err))
			newClient.Conn.Write(context.Background(), websocket.MessageText, errorMsg)
		} else if wsMsg != nil {
			log.Printf("Parsed WebSocket message type: %s", wsMsg.Type)
			err := HandleWebSocketMessage(s.hub, newClient, wsMsg)
			if err != nil {
				log.Printf("Error handling WebSocket message from %s: %v", userName, err)
				errorMsg := []byte(fmt.Sprintf("Error: %v", err))
				newClient.Conn.Write(context.Background(), websocket.MessageText, errorMsg)
			}
		} else {
			// Handle legacy chat messages
			timestamp := time.Now().Format("15:04:05")
			formattedMsg := []byte(fmt.Sprintf("[%s] %s: %s", timestamp, userName, string(message)))
			log.Printf("Attempting to send message from %s to broadcast channel", userName)

			// Send to broadcast channel with timeout
			ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
			defer cancel()

			select {
			case s.hub.Broadcast <- types.Message{Content: formattedMsg, Sender: newClient, Type: types.MsgTypeChat}:
				log.Printf("Message from %s queued for broadcast", userName)
			case <-ctx.Done():
				log.Printf("Broadcast timeout for %s", userName)
				// Continue processing other messages
			}
		}
	}

	return nil
}
