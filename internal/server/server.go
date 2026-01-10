package server

import (
	"log"

	"websocket-demo/internal/hub"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

type Server struct {
	hub  *hub.Hub
	echo *echo.Echo
}

func NewServer(hub *hub.Hub) *Server {
	e := echo.New()

	// Middleware
	e.Use(middleware.RequestLogger())
	e.Use(middleware.Recover())

	return &Server{
		hub:  hub,
		echo: e,
	}
}

func (s *Server) SetupRoutes() {
	// Serve static files (CSS, JS)
	s.echo.GET("/styles.css", func(c echo.Context) error {
		return c.File("styles.css")
	})
	s.echo.GET("/app.js", func(c echo.Context) error {
		return c.File("app.js")
	})

	s.echo.GET("/ws", func(c echo.Context) error {
		return HandleWebSocket(s.hub, c)
	})

	s.echo.GET("/", func(c echo.Context) error {
		return c.File("index.html")
	})
}

func (s *Server) Start(address string) error {
	log.Printf("WebSocket server starting on %s", address)
	return s.echo.Start(address)
}

// Shutdown gracefully shuts down the server
func (s *Server) Shutdown() error {
	return s.echo.Close()
}
