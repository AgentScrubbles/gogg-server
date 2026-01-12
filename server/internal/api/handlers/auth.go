package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/habedi/gogg/client"
	"github.com/habedi/gogg/server/internal/models"
	"github.com/habedi/gogg/server/internal/repository"
	"github.com/rs/zerolog/log"
)

// GOGLoginURL is the OAuth authorization URL for GOG.
const GOGLoginURL = "https://auth.gog.com/auth?client_id=46899977096215655" +
	"&redirect_uri=https%3A%2F%2Fembed.gog.com%2Fon_login_success%3Forigin%3Dclient" +
	"&response_type=code&layout=client2"

// GetLoginURLResponse is the response for the login URL endpoint.
type GetLoginURLResponse struct {
	URL string `json:"url"`
}

// GetLoginURL returns the GOG OAuth authorization URL.
func (h *Handler) GetLoginURL(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, GetLoginURLResponse{URL: GOGLoginURL})
}

// ExchangeCodeRequest is the request body for code exchange.
type ExchangeCodeRequest struct {
	Code     string `json:"code"`
	Username string `json:"username,omitempty"` // Optional: display name for user
}

// ExchangeCodeResponse is the response for successful code exchange.
type ExchangeCodeResponse struct {
	Token string       `json:"token"`
	User  *models.User `json:"user"`
}

// ExchangeCode exchanges an OAuth code for tokens and creates a session.
func (h *Handler) ExchangeCode(w http.ResponseWriter, r *http.Request) {
	var req ExchangeCodeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
		return
	}

	if req.Code == "" {
		respondError(w, http.StatusBadRequest, "invalid_request", "Code is required")
		return
	}

	// Exchange code for tokens using existing client
	gogClient := &client.GogClient{TokenURL: "https://auth.gog.com/token"}
	accessToken, refreshToken, expiresAtStr, err := gogClient.ExchangeCodeForToken(req.Code)
	if err != nil {
		log.Error().Err(err).Msg("Failed to exchange code for token")
		respondError(w, http.StatusUnauthorized, "auth_failed", "Failed to authenticate with GOG")
		return
	}

	// Parse expiration time
	expiresAt, err := time.Parse(time.RFC3339, expiresAtStr)
	if err != nil {
		log.Error().Err(err).Str("expires_at", expiresAtStr).Msg("Failed to parse token expiration")
		respondError(w, http.StatusInternalServerError, "internal_error", "Failed to process token")
		return
	}

	// Get GOG user info to identify the user
	gogUserID, username, err := fetchGOGUserInfo(r.Context(), accessToken)
	if err != nil {
		log.Error().Err(err).Msg("Failed to fetch GOG user info")
		// Fall back to provided username or generate one
		if req.Username != "" {
			username = req.Username
		} else {
			username = "user_" + time.Now().Format("20060102150405")
		}
		gogUserID = username // Use username as fallback ID
	}

	// Get or create user
	user, err := h.userRepo.GetOrCreateByGOGUserID(r.Context(), gogUserID, username)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get or create user")
		respondError(w, http.StatusInternalServerError, "internal_error", "Failed to create user")
		return
	}

	// Store tokens for this user
	userToken := &models.UserToken{
		UserID:       user.ID,
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresAt:    expiresAt,
	}
	if err := h.tokenRepo.Upsert(r.Context(), userToken); err != nil {
		log.Error().Err(err).Msg("Failed to store user token")
		respondError(w, http.StatusInternalServerError, "internal_error", "Failed to store credentials")
		return
	}

	// Generate JWT for session
	jwtToken, err := h.generateJWT(user.ID, user.Username)
	if err != nil {
		log.Error().Err(err).Msg("Failed to generate JWT")
		respondError(w, http.StatusInternalServerError, "internal_error", "Failed to create session")
		return
	}

	respondJSON(w, http.StatusOK, ExchangeCodeResponse{
		Token: jwtToken,
		User:  user,
	})
}

// Logout clears the user's session.
func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	claims := UserFromContext(r.Context())
	if claims == nil {
		respondError(w, http.StatusUnauthorized, "unauthorized", "Not authenticated")
		return
	}

	// Optionally delete stored GOG tokens
	if err := h.tokenRepo.Delete(r.Context(), claims.UserID); err != nil && err != repository.ErrNotFound {
		log.Error().Err(err).Uint("user_id", claims.UserID).Msg("Failed to delete user tokens")
	}

	respondJSON(w, http.StatusOK, map[string]string{"message": "Logged out successfully"})
}

// GetCurrentUser returns the currently authenticated user.
func (h *Handler) GetCurrentUser(w http.ResponseWriter, r *http.Request) {
	claims := UserFromContext(r.Context())
	if claims == nil {
		respondError(w, http.StatusUnauthorized, "unauthorized", "Not authenticated")
		return
	}

	user, err := h.userRepo.GetByID(r.Context(), claims.UserID)
	if err != nil {
		if err == repository.ErrNotFound {
			respondError(w, http.StatusNotFound, "not_found", "User not found")
			return
		}
		log.Error().Err(err).Msg("Failed to get user")
		respondError(w, http.StatusInternalServerError, "internal_error", "Failed to get user")
		return
	}

	respondJSON(w, http.StatusOK, user)
}

// AuthMiddleware validates JWT tokens and adds user to context.
func (h *Handler) AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			respondError(w, http.StatusUnauthorized, "unauthorized", "Authorization header required")
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
			respondError(w, http.StatusUnauthorized, "unauthorized", "Invalid authorization header format")
			return
		}

		claims, err := h.parseJWT(parts[1])
		if err != nil {
			log.Debug().Err(err).Msg("Invalid JWT token")
			respondError(w, http.StatusUnauthorized, "unauthorized", "Invalid or expired token")
			return
		}

		// Add claims to context
		ctx := context.WithValue(r.Context(), userContextKey, claims)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// fetchGOGUserInfo fetches the user's GOG profile to get their user ID and username.
func fetchGOGUserInfo(ctx context.Context, accessToken string) (string, string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", "https://embed.gog.com/userData.json", nil)
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	var userData struct {
		UserID   string `json:"userId"`
		Username string `json:"username"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&userData); err != nil {
		return "", "", err
	}

	return userData.UserID, userData.Username, nil
}
