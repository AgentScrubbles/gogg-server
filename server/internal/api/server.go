package api

import (
	"context"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/habedi/gogg/server/internal/api/handlers"
	"github.com/habedi/gogg/server/internal/repository"
	"github.com/rs/zerolog/log"
	"gorm.io/gorm"
)

// Config holds server configuration.
type Config struct {
	Port           string
	JWTSecret      string
	DownloadPath   string
	StaticPath     string // Path to frontend static files (optional)
	AllowedOrigins []string
}

// Server represents the HTTP API server.
type Server struct {
	config  Config
	router  *chi.Mux
	db      *gorm.DB
	handler *handlers.Handler
}

// NewServer creates a new API server instance.
func NewServer(cfg Config, db *gorm.DB) *Server {
	s := &Server{
		config: cfg,
		db:     db,
		router: chi.NewRouter(),
	}

	// Initialize repositories
	userRepo := repository.NewUserRepository(db)
	tokenRepo := repository.NewTokenRepository(db)
	gameRepo := repository.NewGameRepository(db)
	downloadRepo := repository.NewDownloadJobRepository(db)

	// Initialize handler with dependencies
	s.handler = handlers.NewHandler(handlers.HandlerConfig{
		JWTSecret:    cfg.JWTSecret,
		DownloadPath: cfg.DownloadPath,
		UserRepo:     userRepo,
		TokenRepo:    tokenRepo,
		GameRepo:     gameRepo,
		DownloadRepo: downloadRepo,
	})

	s.setupMiddleware()
	s.setupRoutes()

	return s
}

func (s *Server) setupMiddleware() {
	s.router.Use(middleware.RequestID)
	s.router.Use(middleware.RealIP)
	s.router.Use(middleware.Logger)
	s.router.Use(middleware.Recoverer)
	s.router.Use(middleware.Timeout(60 * time.Second))

	// CORS configuration
	s.router.Use(cors.Handler(cors.Options{
		AllowedOrigins:   s.config.AllowedOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))
}

func (s *Server) setupRoutes() {
	r := s.router

	// Health check
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	// API routes
	r.Route("/api", func(r chi.Router) {
		// Public routes
		r.Route("/auth", func(r chi.Router) {
			r.Get("/login-url", s.handler.GetLoginURL)
			r.Post("/exchange", s.handler.ExchangeCode)
		})

		// Image proxy (public, cached)
		r.Get("/images/{hash}", s.handler.GetImage)

		// Protected routes (require authentication)
		r.Group(func(r chi.Router) {
			r.Use(s.handler.AuthMiddleware)

			// Auth
			r.Post("/auth/logout", s.handler.Logout)
			r.Get("/auth/me", s.handler.GetCurrentUser)

			// Games
			r.Get("/games", s.handler.ListGames)
			r.Get("/games/search", s.handler.SearchGames)
			r.Get("/games/{gameID}", s.handler.GetGame)
			r.Post("/games/refresh", s.handler.RefreshCatalogue)

			// Downloads
			r.Get("/downloads", s.handler.ListDownloads)
			r.Post("/downloads", s.handler.CreateDownload)
			r.Get("/downloads/{downloadID}", s.handler.GetDownload)
			r.Delete("/downloads/{downloadID}", s.handler.CancelDownload)
		})

		// WebSocket route (auth handled inside)
		r.Get("/ws/downloads", s.handler.DownloadProgressWS)
	})

	// Serve frontend static files if configured
	if s.config.StaticPath != "" {
		s.serveStaticFiles(r)
	}
}

// serveStaticFiles serves the frontend SPA with proper fallback to index.html
func (s *Server) serveStaticFiles(r *chi.Mux) {
	staticPath := s.config.StaticPath

	// Check if static path exists
	if _, err := os.Stat(staticPath); os.IsNotExist(err) {
		log.Warn().Str("path", staticPath).Msg("Static path does not exist, skipping frontend serving")
		return
	}

	log.Info().Str("path", staticPath).Msg("Serving frontend static files")

	// Create file server
	fsys := os.DirFS(staticPath)

	// Serve static files and handle SPA routing
	r.Get("/*", func(w http.ResponseWriter, r *http.Request) {
		// Clean the path
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			path = "index.html"
		}

		// Try to open the file
		f, err := fsys.(fs.FS).Open(path)
		if err != nil {
			// File not found, serve index.html for SPA routing
			indexPath := filepath.Join(staticPath, "index.html")
			http.ServeFile(w, r, indexPath)
			return
		}
		f.Close()

		// Serve the actual file
		http.ServeFile(w, r, filepath.Join(staticPath, path))
	})
}

// Router returns the chi router for testing or embedding.
func (s *Server) Router() *chi.Mux {
	return s.router
}

// Start begins listening for HTTP requests.
func (s *Server) Start() error {
	addr := ":" + s.config.Port
	log.Info().Str("addr", addr).Msg("Starting HTTP server")
	return http.ListenAndServe(addr, s.router)
}

// StartWithContext starts the server with graceful shutdown support.
func (s *Server) StartWithContext(ctx context.Context) error {
	addr := ":" + s.config.Port
	srv := &http.Server{
		Addr:    addr,
		Handler: s.router,
	}

	go func() {
		<-ctx.Done()
		log.Info().Msg("Shutting down HTTP server...")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		srv.Shutdown(shutdownCtx)
	}()

	log.Info().Str("addr", addr).Msg("Starting HTTP server")
	return srv.ListenAndServe()
}
