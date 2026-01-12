package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/habedi/gogg/server/internal/models"
	"github.com/rs/zerolog/log"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		// TODO: In production, validate origin against allowed origins
		return true
	},
}

// WSMessage represents a WebSocket message.
type WSMessage struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

// ProgressHub manages WebSocket connections for progress updates.
type ProgressHub struct {
	mu          sync.RWMutex
	connections map[uint]map[*websocket.Conn]bool // userID -> connections
}

var progressHub = &ProgressHub{
	connections: make(map[uint]map[*websocket.Conn]bool),
}

// AddConnection registers a WebSocket connection for a user.
func (hub *ProgressHub) AddConnection(userID uint, conn *websocket.Conn) {
	hub.mu.Lock()
	defer hub.mu.Unlock()

	if hub.connections[userID] == nil {
		hub.connections[userID] = make(map[*websocket.Conn]bool)
	}
	hub.connections[userID][conn] = true
}

// RemoveConnection unregisters a WebSocket connection.
func (hub *ProgressHub) RemoveConnection(userID uint, conn *websocket.Conn) {
	hub.mu.Lock()
	defer hub.mu.Unlock()

	if hub.connections[userID] != nil {
		delete(hub.connections[userID], conn)
		if len(hub.connections[userID]) == 0 {
			delete(hub.connections, userID)
		}
	}
}

// BroadcastToUser sends a message to all connections for a user.
func (hub *ProgressHub) BroadcastToUser(userID uint, msg WSMessage) {
	hub.mu.RLock()
	conns := hub.connections[userID]
	hub.mu.RUnlock()

	if conns == nil {
		return
	}

	data, err := json.Marshal(msg)
	if err != nil {
		log.Error().Err(err).Msg("Failed to marshal WebSocket message")
		return
	}

	for conn := range conns {
		if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
			log.Debug().Err(err).Msg("Failed to write to WebSocket")
			// Connection will be cleaned up by the read loop
		}
	}
}

// GetProgressHub returns the global progress hub for broadcasting.
func GetProgressHub() *ProgressHub {
	return progressHub
}

// DownloadProgressWS handles WebSocket connections for download progress.
func (h *Handler) DownloadProgressWS(w http.ResponseWriter, r *http.Request) {
	// Authenticate via query parameter (WebSocket doesn't support headers easily)
	token := r.URL.Query().Get("token")
	if token == "" {
		respondError(w, http.StatusUnauthorized, "unauthorized", "Token required")
		return
	}

	claims, err := h.parseJWT(token)
	if err != nil {
		log.Debug().Err(err).Msg("Invalid WebSocket JWT token")
		respondError(w, http.StatusUnauthorized, "unauthorized", "Invalid or expired token")
		return
	}

	// Upgrade to WebSocket
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Error().Err(err).Msg("Failed to upgrade WebSocket connection")
		return
	}
	defer conn.Close()

	// Register connection
	progressHub.AddConnection(claims.UserID, conn)
	defer progressHub.RemoveConnection(claims.UserID, conn)

	log.Debug().Uint("user_id", claims.UserID).Msg("WebSocket connection established")

	// Send initial state - list of active downloads
	h.sendActiveDownloads(claims.UserID, conn)

	// Set up ping/pong for connection health
	conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	// Start ping ticker
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	// Handle incoming messages and keep connection alive
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					log.Debug().Err(err).Msg("WebSocket read error")
				}
				return
			}
		}
	}()

	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (h *Handler) sendActiveDownloads(userID uint, conn *websocket.Conn) {
	// Get active downloads for this user
	activeStatus := models.DownloadStatusDownloading
	jobs, _, err := h.downloadRepo.ListForUser(context.Background(), userID, &activeStatus, 100, 0)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get active downloads for WebSocket")
		return
	}

	msg := WSMessage{
		Type: "initial_state",
		Data: jobs,
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return
	}

	conn.WriteMessage(websocket.TextMessage, data)
}

// BroadcastProgress sends a progress update to all connections for a user.
func BroadcastProgress(userID uint, jobID uint, progressBytes int64, totalBytes int64, status models.DownloadStatus) {
	progressHub.BroadcastToUser(userID, WSMessage{
		Type: "progress",
		Data: map[string]interface{}{
			"job_id":         jobID,
			"progress_bytes": progressBytes,
			"total_bytes":    totalBytes,
			"status":         status,
		},
	})
}

// BroadcastJobUpdate sends a job status update to all connections for a user.
func BroadcastJobUpdate(userID uint, job *models.DownloadJob) {
	progressHub.BroadcastToUser(userID, WSMessage{
		Type: "job_update",
		Data: job,
	})
}
