package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/habedi/gogg/server/internal/api"
	"github.com/habedi/gogg/server/internal/jobs"
	"github.com/habedi/gogg/server/internal/models"
	"github.com/habedi/gogg/server/internal/repository"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func main() {
	// Configure logging
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	if os.Getenv("DEBUG") == "true" {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	} else {
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	}

	// Get configuration from environment
	config := loadConfig()

	// Connect to database
	db, err := connectDB(config.DatabaseURL)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to database")
	}

	// Run migrations
	if err := models.AutoMigrate(db); err != nil {
		log.Fatal().Err(err).Msg("Failed to run database migrations")
	}
	log.Info().Msg("Database migrations completed")

	// Create download worker
	worker := jobs.NewWorker(jobs.WorkerConfig{
		DownloadPath: config.DownloadPath,
		MaxWorkers:   config.MaxConcurrentDownloads,
		UserRepo:     repository.NewUserRepository(db),
		TokenRepo:    repository.NewTokenRepository(db),
		GameRepo:     repository.NewGameRepository(db),
		DownloadRepo: repository.NewDownloadJobRepository(db),
	})
	worker.Start()

	// Create and start HTTP server
	server := api.NewServer(api.Config{
		Port:           config.Port,
		JWTSecret:      config.JWTSecret,
		DownloadPath:   config.DownloadPath,
		StaticPath:     config.StaticPath,
		AllowedOrigins: config.AllowedOrigins,
	}, db)

	// Handle graceful shutdown
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		if err := server.StartWithContext(ctx); err != nil {
			log.Error().Err(err).Msg("Server error")
		}
	}()

	log.Info().Str("port", config.Port).Msg("Server started")

	// Wait for shutdown signal
	<-ctx.Done()
	log.Info().Msg("Shutting down...")

	// Stop worker
	worker.Stop()

	log.Info().Msg("Shutdown complete")
}

type serverConfig struct {
	Port                   string
	DatabaseURL            string
	JWTSecret              string
	DownloadPath           string
	StaticPath             string
	AllowedOrigins         []string
	MaxConcurrentDownloads int
}

func loadConfig() serverConfig {
	config := serverConfig{
		Port:                   getEnv("PORT", "8080"),
		DatabaseURL:            getEnv("DATABASE_URL", "postgres://gogg:gogg@localhost:5432/gogg?sslmode=disable"),
		JWTSecret:              getEnv("JWT_SECRET", "change-me-in-production"),
		DownloadPath:           getEnv("GOGG_DOWNLOAD_PATH", "/downloads/shared"),
		StaticPath:             getEnv("GOGG_STATIC_PATH", ""), // Empty = don't serve frontend
		MaxConcurrentDownloads: 3,
	}

	// Parse allowed origins
	originsStr := getEnv("ALLOWED_ORIGINS", "http://localhost:3000,http://localhost:5173")
	config.AllowedOrigins = splitAndTrim(originsStr, ",")

	return config
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func splitAndTrim(s, sep string) []string {
	if s == "" {
		return nil
	}
	parts := make([]string, 0)
	for _, part := range splitString(s, sep) {
		trimmed := trimString(part)
		if trimmed != "" {
			parts = append(parts, trimmed)
		}
	}
	return parts
}

func splitString(s, sep string) []string {
	var result []string
	start := 0
	for i := 0; i < len(s); i++ {
		if i+len(sep) <= len(s) && s[i:i+len(sep)] == sep {
			result = append(result, s[start:i])
			start = i + len(sep)
			i += len(sep) - 1
		}
	}
	result = append(result, s[start:])
	return result
}

func trimString(s string) string {
	start := 0
	end := len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t' || s[start] == '\n' || s[start] == '\r') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\n' || s[end-1] == '\r') {
		end--
	}
	return s[start:end]
}

func connectDB(dsn string) (*gorm.DB, error) {
	gormLogger := logger.Default.LogMode(logger.Silent)
	if os.Getenv("DEBUG") == "true" {
		gormLogger = logger.Default.LogMode(logger.Info)
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: gormLogger,
	})
	if err != nil {
		return nil, err
	}

	// Configure connection pool
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)

	return db, nil
}
