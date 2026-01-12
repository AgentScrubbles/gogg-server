package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/habedi/gogg/server/internal/repository"
)

// HandlerConfig holds dependencies for handlers.
type HandlerConfig struct {
	JWTSecret    string
	DownloadPath string
	UserRepo     repository.UserRepository
	TokenRepo    repository.TokenRepository
	GameRepo     repository.GameRepository
	DownloadRepo repository.DownloadJobRepository
}

// Handler contains all HTTP handlers and their dependencies.
type Handler struct {
	config       HandlerConfig
	userRepo     repository.UserRepository
	tokenRepo    repository.TokenRepository
	gameRepo     repository.GameRepository
	downloadRepo repository.DownloadJobRepository
}

// NewHandler creates a new Handler with the given configuration.
func NewHandler(cfg HandlerConfig) *Handler {
	return &Handler{
		config:       cfg,
		userRepo:     cfg.UserRepo,
		tokenRepo:    cfg.TokenRepo,
		gameRepo:     cfg.GameRepo,
		downloadRepo: cfg.DownloadRepo,
	}
}

// --- JWT Claims ---

type JWTClaims struct {
	UserID   uint   `json:"user_id"`
	Username string `json:"username"`
	jwt.RegisteredClaims
}

// --- Context Keys ---

type contextKey string

const userContextKey contextKey = "user"

// UserFromContext retrieves the authenticated user from context.
func UserFromContext(ctx context.Context) *JWTClaims {
	claims, _ := ctx.Value(userContextKey).(*JWTClaims)
	return claims
}

// --- Response Helpers ---

type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type PaginatedResponse struct {
	Data   interface{} `json:"data"`
	Total  int64       `json:"total"`
	Limit  int         `json:"limit"`
	Offset int         `json:"offset"`
}

func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func respondError(w http.ResponseWriter, status int, code, message string) {
	respondJSON(w, status, APIError{Code: code, Message: message})
}

// --- Parameter Parsing ---

func parseInt(s string, defaultVal int) int {
	if s == "" {
		return defaultVal
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return defaultVal
	}
	return v
}

func parseUint(s string) (uint, error) {
	v, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return 0, err
	}
	return uint(v), nil
}

// --- JWT Helpers ---

func (h *Handler) generateJWT(userID uint, username string) (string, error) {
	claims := JWTClaims{
		UserID:   userID,
		Username: username,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(h.config.JWTSecret))
}

func (h *Handler) parseJWT(tokenString string) (*JWTClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		return []byte(h.config.JWTSecret), nil
	})
	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*JWTClaims)
	if !ok || !token.Valid {
		return nil, jwt.ErrSignatureInvalid
	}

	return claims, nil
}
