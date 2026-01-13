package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/habedi/gogg/client"
	"github.com/habedi/gogg/server/internal/repository"
	"github.com/rs/zerolog/log"
)

// fixImageURL converts GOG image URLs to our local proxy URLs.
// GOG returns URLs like "//images-1.gog-statics.com/{hash}" which we convert
// to "/api/images/{hash}" to serve through our caching proxy.
func fixImageURL(url *string) *string {
	if url == nil {
		return nil
	}

	// Extract the hash from the URL
	// Format: "//images-X.gog-statics.com/{hash}" -> "/api/images/{hash}"
	s := *url
	if idx := strings.LastIndex(s, "/"); idx != -1 {
		hash := s[idx+1:]
		if len(hash) == 64 { // GOG hashes are 64 hex chars
			proxyURL := "/api/images/" + hash
			return &proxyURL
		}
	}

	// Fallback: if we can't parse it, return with https prefix
	if strings.HasPrefix(s, "//") {
		fixed := "https:" + s + ".jpg"
		return &fixed
	}
	return url
}

// GameResponse wraps a game with parsed metadata.
type GameResponse struct {
	ID        uint        `json:"id"`
	GameID    int         `json:"game_id"`
	Title     string      `json:"title"`
	Data      client.Game `json:"data"`
	CreatedAt time.Time   `json:"created_at"`
	UpdatedAt time.Time   `json:"updated_at"`
}

// ListGames returns paginated list of user's games.
func (h *Handler) ListGames(w http.ResponseWriter, r *http.Request) {
	claims := UserFromContext(r.Context())
	if claims == nil {
		respondError(w, http.StatusUnauthorized, "unauthorized", "Not authenticated")
		return
	}

	limit := parseInt(r.URL.Query().Get("limit"), 50)
	offset := parseInt(r.URL.Query().Get("offset"), 0)

	if limit > 100 {
		limit = 100
	}

	games, total, err := h.gameRepo.List(r.Context(), claims.UserID, limit, offset)
	if err != nil {
		log.Error().Err(err).Msg("Failed to list games")
		respondError(w, http.StatusInternalServerError, "internal_error", "Failed to list games")
		return
	}

	// Parse game data for response
	var response []map[string]interface{}
	for _, g := range games {
		item := map[string]interface{}{
			"id":         g.ID,
			"game_id":    g.GameID,
			"title":      g.Title,
			"created_at": g.CreatedAt,
			"updated_at": g.UpdatedAt,
		}

		// Parse the JSON data to extract useful fields for listing
		var gameData client.Game
		if err := json.Unmarshal([]byte(g.Data), &gameData); err == nil {
			// Fix protocol-relative URLs from GOG (they start with //)
			item["background_image"] = fixImageURL(gameData.BackgroundImage)
			item["has_dlc"] = len(gameData.DLCs) > 0
			item["has_extras"] = len(gameData.Extras) > 0

			// Extract available platforms
			platforms := make(map[string]bool)
			for _, dl := range gameData.Downloads {
				if len(dl.Platforms.Windows) > 0 {
					platforms["windows"] = true
				}
				if len(dl.Platforms.Mac) > 0 {
					platforms["mac"] = true
				}
				if len(dl.Platforms.Linux) > 0 {
					platforms["linux"] = true
				}
			}
			item["platforms"] = platforms
		}

		response = append(response, item)
	}

	respondJSON(w, http.StatusOK, PaginatedResponse{
		Data:   response,
		Total:  total,
		Limit:  limit,
		Offset: offset,
	})
}

// SearchGames searches user's games by title.
func (h *Handler) SearchGames(w http.ResponseWriter, r *http.Request) {
	claims := UserFromContext(r.Context())
	if claims == nil {
		respondError(w, http.StatusUnauthorized, "unauthorized", "Not authenticated")
		return
	}

	query := r.URL.Query().Get("q")
	if query == "" {
		respondError(w, http.StatusBadRequest, "invalid_request", "Search query 'q' is required")
		return
	}

	limit := parseInt(r.URL.Query().Get("limit"), 50)
	offset := parseInt(r.URL.Query().Get("offset"), 0)

	if limit > 100 {
		limit = 100
	}

	games, total, err := h.gameRepo.Search(r.Context(), claims.UserID, query, limit, offset)
	if err != nil {
		log.Error().Err(err).Msg("Failed to search games")
		respondError(w, http.StatusInternalServerError, "internal_error", "Failed to search games")
		return
	}

	// Parse game data for response (same as ListGames)
	var response []map[string]interface{}
	for _, g := range games {
		item := map[string]interface{}{
			"id":         g.ID,
			"game_id":    g.GameID,
			"title":      g.Title,
			"created_at": g.CreatedAt,
			"updated_at": g.UpdatedAt,
		}

		var gameData client.Game
		if err := json.Unmarshal([]byte(g.Data), &gameData); err == nil {
			// Fix protocol-relative URLs from GOG
			item["background_image"] = fixImageURL(gameData.BackgroundImage)
			item["has_dlc"] = len(gameData.DLCs) > 0
			item["has_extras"] = len(gameData.Extras) > 0

			platforms := make(map[string]bool)
			for _, dl := range gameData.Downloads {
				if len(dl.Platforms.Windows) > 0 {
					platforms["windows"] = true
				}
				if len(dl.Platforms.Mac) > 0 {
					platforms["mac"] = true
				}
				if len(dl.Platforms.Linux) > 0 {
					platforms["linux"] = true
				}
			}
			item["platforms"] = platforms
		}

		response = append(response, item)
	}

	respondJSON(w, http.StatusOK, PaginatedResponse{
		Data:   response,
		Total:  total,
		Limit:  limit,
		Offset: offset,
	})
}

// GetGame returns detailed information about a specific game.
func (h *Handler) GetGame(w http.ResponseWriter, r *http.Request) {
	claims := UserFromContext(r.Context())
	if claims == nil {
		respondError(w, http.StatusUnauthorized, "unauthorized", "Not authenticated")
		return
	}

	gameIDStr := chi.URLParam(r, "gameID")
	gameID, err := strconv.Atoi(gameIDStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid_request", "Invalid game ID")
		return
	}

	game, err := h.gameRepo.GetByID(r.Context(), claims.UserID, gameID)
	if err != nil {
		if err == repository.ErrNotFound {
			respondError(w, http.StatusNotFound, "not_found", "Game not found")
			return
		}
		log.Error().Err(err).Msg("Failed to get game")
		respondError(w, http.StatusInternalServerError, "internal_error", "Failed to get game")
		return
	}

	// Parse full game data
	var gameData client.Game
	if err := json.Unmarshal([]byte(game.Data), &gameData); err != nil {
		log.Error().Err(err).Int("game_id", gameID).Msg("Failed to parse game data")
		respondError(w, http.StatusInternalServerError, "internal_error", "Failed to parse game data")
		return
	}

	// Fix protocol-relative URLs from GOG
	gameData.BackgroundImage = fixImageURL(gameData.BackgroundImage)
	for i := range gameData.DLCs {
		gameData.DLCs[i].BackgroundImage = fixImageURL(gameData.DLCs[i].BackgroundImage)
	}

	response := GameResponse{
		ID:        game.ID,
		GameID:    game.GameID,
		Title:     game.Title,
		Data:      gameData,
		CreatedAt: game.CreatedAt,
		UpdatedAt: game.UpdatedAt,
	}

	respondJSON(w, http.StatusOK, response)
}

// RefreshCatalogueResponse is the response for catalogue refresh.
type RefreshCatalogueResponse struct {
	Message    string `json:"message"`
	GamesCount int    `json:"games_count"`
}

// RefreshCatalogue syncs the user's game catalogue from GOG.
func (h *Handler) RefreshCatalogue(w http.ResponseWriter, r *http.Request) {
	claims := UserFromContext(r.Context())
	if claims == nil {
		respondError(w, http.StatusUnauthorized, "unauthorized", "Not authenticated")
		return
	}

	// Get user's GOG tokens
	userToken, err := h.tokenRepo.Get(r.Context(), claims.UserID)
	if err != nil {
		if err == repository.ErrNotFound {
			respondError(w, http.StatusUnauthorized, "unauthorized", "GOG credentials not found. Please log in again.")
			return
		}
		log.Error().Err(err).Msg("Failed to get user token")
		respondError(w, http.StatusInternalServerError, "internal_error", "Failed to get credentials")
		return
	}

	// Check if token needs refresh
	accessToken := userToken.AccessToken
	if time.Now().Add(5 * time.Minute).After(userToken.ExpiresAt) {
		// Token is expired or about to expire, refresh it
		gogClient := &client.GogClient{TokenURL: "https://auth.gog.com/token"}
		newAccessToken, newRefreshToken, expiresIn, err := gogClient.PerformTokenRefresh(userToken.RefreshToken)
		if err != nil {
			log.Error().Err(err).Msg("Failed to refresh GOG token")
			respondError(w, http.StatusUnauthorized, "auth_expired", "GOG session expired. Please log in again.")
			return
		}

		// Update stored tokens
		userToken.AccessToken = newAccessToken
		userToken.RefreshToken = newRefreshToken
		userToken.ExpiresAt = time.Now().Add(time.Duration(expiresIn) * time.Second)
		if err := h.tokenRepo.Upsert(r.Context(), userToken); err != nil {
			log.Error().Err(err).Msg("Failed to update token")
		}
		accessToken = newAccessToken
	}

	// Fetch all owned game IDs from GOG
	gameIDs, err := client.FetchAllOwnedGameIDs(r.Context(), accessToken, "https://embed.gog.com/user/data/games")
	if err != nil {
		log.Error().Err(err).Msg("Failed to fetch owned games")
		respondError(w, http.StatusInternalServerError, "internal_error", "Failed to fetch game list from GOG")
		return
	}

	// Note: We intentionally do NOT clear existing games.
	// This preserves game metadata even if GOG removes them later (archival).
	// The Put() method does upsert, so existing games get updated.

	// Fetch details for each game (with progress tracking)
	var successCount int32
	var failCount int32

	// Use a simple sequential fetch for now (can be parallelized later)
	for _, gameID := range gameIDs {
		url := "https://embed.gog.com/account/gameDetails/" + strconv.Itoa(gameID) + ".json"
		gameData, rawJSON, err := client.FetchGameData(r.Context(), accessToken, url)
		if err != nil {
			log.Warn().Err(err).Int("game_id", gameID).Msg("Failed to fetch game details")
			atomic.AddInt32(&failCount, 1)
			continue
		}

		// Store game in user's catalogue
		if err := h.gameRepo.Put(r.Context(), claims.UserID, gameID, gameData.Title, rawJSON); err != nil {
			log.Warn().Err(err).Int("game_id", gameID).Msg("Failed to store game")
			atomic.AddInt32(&failCount, 1)
			continue
		}
		atomic.AddInt32(&successCount, 1)
	}

	log.Info().
		Uint("user_id", claims.UserID).
		Int32("success", successCount).
		Int32("failed", failCount).
		Msg("Catalogue refresh completed")

	respondJSON(w, http.StatusOK, RefreshCatalogueResponse{
		Message:    "Catalogue refreshed successfully",
		GamesCount: int(successCount),
	})
}
