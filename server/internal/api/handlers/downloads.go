package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/habedi/gogg/client"
	"github.com/habedi/gogg/server/internal/models"
	"github.com/habedi/gogg/server/internal/repository"
	"github.com/rs/zerolog/log"
)

// CreateDownloadRequest is the request body for creating a download.
type CreateDownloadRequest struct {
	GameID        int    `json:"game_id"`
	Platform      string `json:"platform"`
	Language      string `json:"language"`
	IncludeExtras bool   `json:"include_extras"`
	IncludeDLC    bool   `json:"include_dlc"`
	Threads       int    `json:"threads"`
}

// ListDownloads returns paginated list of user's downloads.
func (h *Handler) ListDownloads(w http.ResponseWriter, r *http.Request) {
	claims := UserFromContext(r.Context())
	if claims == nil {
		respondError(w, http.StatusUnauthorized, "unauthorized", "Not authenticated")
		return
	}

	limit := parseInt(r.URL.Query().Get("limit"), 50)
	offset := parseInt(r.URL.Query().Get("offset"), 0)

	// Optional status filter
	var statusFilter *models.DownloadStatus
	if statusStr := r.URL.Query().Get("status"); statusStr != "" {
		status := models.DownloadStatus(statusStr)
		statusFilter = &status
	}

	if limit > 100 {
		limit = 100
	}

	jobs, total, err := h.downloadRepo.ListForUser(r.Context(), claims.UserID, statusFilter, limit, offset)
	if err != nil {
		log.Error().Err(err).Msg("Failed to list downloads")
		respondError(w, http.StatusInternalServerError, "internal_error", "Failed to list downloads")
		return
	}

	respondJSON(w, http.StatusOK, PaginatedResponse{
		Data:   jobs,
		Total:  total,
		Limit:  limit,
		Offset: offset,
	})
}

// CreateDownload queues a new download job.
func (h *Handler) CreateDownload(w http.ResponseWriter, r *http.Request) {
	claims := UserFromContext(r.Context())
	if claims == nil {
		respondError(w, http.StatusUnauthorized, "unauthorized", "Not authenticated")
		return
	}

	var req CreateDownloadRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
		return
	}

	// Validate request
	if req.GameID == 0 {
		respondError(w, http.StatusBadRequest, "invalid_request", "Game ID is required")
		return
	}

	// Set defaults
	if req.Platform == "" {
		req.Platform = "windows"
	}
	if req.Language == "" {
		req.Language = "en"
	}
	if req.Threads <= 0 || req.Threads > 20 {
		req.Threads = 5
	}

	// Validate platform
	validPlatforms := map[string]bool{"windows": true, "mac": true, "linux": true, "all": true}
	if !validPlatforms[req.Platform] {
		respondError(w, http.StatusBadRequest, "invalid_request", "Invalid platform. Must be: windows, mac, linux, or all")
		return
	}

	// Validate language
	validLanguages := map[string]bool{
		"en": true, "fr": true, "de": true, "es": true, "it": true,
		"ru": true, "pl": true, "pt-BR": true, "zh-Hans": true, "ja": true, "ko": true,
	}
	if !validLanguages[req.Language] {
		respondError(w, http.StatusBadRequest, "invalid_request", "Invalid language code")
		return
	}

	// Get game from user's catalogue to get the title
	game, err := h.gameRepo.GetByID(r.Context(), claims.UserID, req.GameID)
	if err != nil {
		if err == repository.ErrNotFound {
			respondError(w, http.StatusNotFound, "not_found", "Game not found in your catalogue. Please refresh your catalogue first.")
			return
		}
		log.Error().Err(err).Msg("Failed to get game")
		respondError(w, http.StatusInternalServerError, "internal_error", "Failed to get game")
		return
	}

	// Calculate estimated total size
	var gameData client.Game
	var totalBytes int64
	if err := json.Unmarshal([]byte(game.Data), &gameData); err == nil {
		// Map language code to full name for size estimation
		langMap := map[string]string{
			"en": "English", "fr": "Français", "de": "Deutsch", "es": "Español",
			"it": "Italiano", "ru": "Русский", "pl": "Polski", "pt-BR": "Português do Brasil",
			"zh-Hans": "简体中文", "ja": "日本語", "ko": "한국어",
		}
		langName := langMap[req.Language]
		if langName == "" {
			langName = "English"
		}

		size, err := gameData.EstimateStorageSize(langName, req.Platform, req.IncludeExtras, req.IncludeDLC)
		if err == nil {
			totalBytes = size
		}
	}

	// Create download job
	job := &models.DownloadJob{
		UserID:        claims.UserID,
		GameID:        req.GameID,
		GameTitle:     game.Title,
		Status:        models.DownloadStatusPending,
		Platform:      req.Platform,
		Language:      req.Language,
		IncludeExtras: req.IncludeExtras,
		IncludeDLC:    req.IncludeDLC,
		Threads:       req.Threads,
		TotalBytes:    totalBytes,
	}

	if err := h.downloadRepo.Create(r.Context(), job); err != nil {
		log.Error().Err(err).Msg("Failed to create download job")
		respondError(w, http.StatusInternalServerError, "internal_error", "Failed to create download job")
		return
	}

	log.Info().
		Uint("user_id", claims.UserID).
		Int("game_id", req.GameID).
		Str("game_title", game.Title).
		Uint("job_id", job.ID).
		Msg("Download job created")

	respondJSON(w, http.StatusCreated, job)
}

// GetDownload returns details of a specific download job.
func (h *Handler) GetDownload(w http.ResponseWriter, r *http.Request) {
	claims := UserFromContext(r.Context())
	if claims == nil {
		respondError(w, http.StatusUnauthorized, "unauthorized", "Not authenticated")
		return
	}

	downloadIDStr := chi.URLParam(r, "downloadID")
	downloadID, err := parseUint(downloadIDStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid_request", "Invalid download ID")
		return
	}

	job, err := h.downloadRepo.GetByIDForUser(r.Context(), claims.UserID, downloadID)
	if err != nil {
		if err == repository.ErrNotFound {
			respondError(w, http.StatusNotFound, "not_found", "Download not found")
			return
		}
		log.Error().Err(err).Msg("Failed to get download")
		respondError(w, http.StatusInternalServerError, "internal_error", "Failed to get download")
		return
	}

	respondJSON(w, http.StatusOK, job)
}

// CancelDownload cancels a pending or active download.
func (h *Handler) CancelDownload(w http.ResponseWriter, r *http.Request) {
	claims := UserFromContext(r.Context())
	if claims == nil {
		respondError(w, http.StatusUnauthorized, "unauthorized", "Not authenticated")
		return
	}

	downloadIDStr := chi.URLParam(r, "downloadID")
	downloadID, err := parseUint(downloadIDStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid_request", "Invalid download ID")
		return
	}

	if err := h.downloadRepo.Cancel(r.Context(), claims.UserID, downloadID); err != nil {
		if err == repository.ErrNotFound {
			respondError(w, http.StatusNotFound, "not_found", "Download not found or already completed")
			return
		}
		log.Error().Err(err).Msg("Failed to cancel download")
		respondError(w, http.StatusInternalServerError, "internal_error", "Failed to cancel download")
		return
	}

	log.Info().
		Uint("user_id", claims.UserID).
		Uint("job_id", downloadID).
		Msg("Download job cancelled")

	respondJSON(w, http.StatusOK, map[string]string{"message": "Download cancelled"})
}
