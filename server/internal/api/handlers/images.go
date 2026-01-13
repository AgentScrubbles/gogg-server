package handlers

import (
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog/log"
)

var (
	// validHashRegex matches GOG image hashes (64 hex characters)
	validHashRegex = regexp.MustCompile(`^[a-f0-9]{64}$`)

	// GOG CDN hosts to try
	gogCDNHosts = []string{
		"images-1.gog-statics.com",
		"images-2.gog-statics.com",
		"images-3.gog-statics.com",
		"images-4.gog-statics.com",
	}
)

// GetImage serves a cached GOG image or fetches and caches it.
func (h *Handler) GetImage(w http.ResponseWriter, r *http.Request) {
	hash := chi.URLParam(r, "hash")

	// Validate hash format to prevent path traversal
	if !validHashRegex.MatchString(hash) {
		http.Error(w, "Invalid image hash", http.StatusBadRequest)
		return
	}

	// Build cache path: /downloads/images/{first2chars}/{hash}.jpg
	cacheDir := filepath.Join(h.config.DownloadPath, "images", hash[:2])
	cachePath := filepath.Join(cacheDir, hash+".jpg")

	// Check if cached
	if _, err := os.Stat(cachePath); err == nil {
		// Serve from cache
		w.Header().Set("Content-Type", "image/jpeg")
		w.Header().Set("Cache-Control", "public, max-age=31536000") // 1 year
		http.ServeFile(w, r, cachePath)
		return
	}

	// Not cached - fetch from GOG CDN
	imageData, err := fetchFromGOG(hash)
	if err != nil {
		log.Warn().Err(err).Str("hash", hash).Msg("Failed to fetch image from GOG")
		http.Error(w, "Image not found", http.StatusNotFound)
		return
	}

	// Ensure cache directory exists
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		log.Error().Err(err).Str("path", cacheDir).Msg("Failed to create image cache directory")
		// Still serve the image even if we can't cache it
		w.Header().Set("Content-Type", "image/jpeg")
		w.Write(imageData)
		return
	}

	// Write to cache
	if err := os.WriteFile(cachePath, imageData, 0644); err != nil {
		log.Warn().Err(err).Str("path", cachePath).Msg("Failed to cache image")
	} else {
		log.Debug().Str("hash", hash).Msg("Cached GOG image")
	}

	// Serve the image
	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Cache-Control", "public, max-age=31536000")
	w.Write(imageData)
}

// fetchFromGOG tries to fetch an image from GOG's CDN.
func fetchFromGOG(hash string) ([]byte, error) {
	client := &http.Client{Timeout: 30 * time.Second}

	// Try each CDN host
	for _, host := range gogCDNHosts {
		url := fmt.Sprintf("https://%s/%s.jpg", host, hash)

		resp, err := client.Get(url)
		if err != nil {
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			data, err := io.ReadAll(resp.Body)
			if err != nil {
				continue
			}
			return data, nil
		}
	}

	return nil, fmt.Errorf("image not found on any GOG CDN")
}

// ImageURLForHash returns the local proxy URL for a GOG image hash.
func ImageURLForHash(hash string) string {
	// Extract just the hash from full URL if needed
	// e.g., "//images-1.gog-statics.com/abc123" -> "abc123"
	if idx := strings.LastIndex(hash, "/"); idx != -1 {
		hash = hash[idx+1:]
	}
	return "/api/images/" + hash
}

// ComputeImageHash computes a SHA256 hash for an image URL (for cache key).
func ComputeImageHash(data []byte) string {
	h := sha256.Sum256(data)
	return fmt.Sprintf("%x", h)
}
