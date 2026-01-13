package jobs

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"sync"
	"time"

	"github.com/habedi/gogg/client"
	"github.com/habedi/gogg/server/internal/api/handlers"
	"github.com/habedi/gogg/server/internal/models"
	"github.com/habedi/gogg/server/internal/repository"
	"github.com/rs/zerolog/log"
)

// Worker processes download jobs from the queue.
type Worker struct {
	downloadPath string
	userRepo     repository.UserRepository
	tokenRepo    repository.TokenRepository
	gameRepo     repository.GameRepository
	downloadRepo repository.DownloadJobRepository

	ctx        context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup
	maxWorkers int

	// Track active downloads for cancellation
	activeJobs   map[uint]context.CancelFunc
	activeJobsMu sync.Mutex
}

// WorkerConfig holds configuration for the download worker.
type WorkerConfig struct {
	DownloadPath string
	MaxWorkers   int
	UserRepo     repository.UserRepository
	TokenRepo    repository.TokenRepository
	GameRepo     repository.GameRepository
	DownloadRepo repository.DownloadJobRepository
}

// NewWorker creates a new download worker.
func NewWorker(cfg WorkerConfig) *Worker {
	if cfg.MaxWorkers <= 0 {
		cfg.MaxWorkers = 3
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &Worker{
		downloadPath: cfg.DownloadPath,
		userRepo:     cfg.UserRepo,
		tokenRepo:    cfg.TokenRepo,
		gameRepo:     cfg.GameRepo,
		downloadRepo: cfg.DownloadRepo,
		ctx:          ctx,
		cancel:       cancel,
		maxWorkers:   cfg.MaxWorkers,
		activeJobs:   make(map[uint]context.CancelFunc),
	}
}

// Start begins processing download jobs.
func (w *Worker) Start() {
	log.Info().Int("max_workers", w.maxWorkers).Msg("Starting download worker")

	w.wg.Add(1)
	go w.processLoop()
}

// Stop gracefully shuts down the worker.
func (w *Worker) Stop() {
	log.Info().Msg("Stopping download worker")
	w.cancel()
	w.wg.Wait()
}

// CancelJob cancels an active download job.
func (w *Worker) CancelJob(jobID uint) {
	w.activeJobsMu.Lock()
	cancel, exists := w.activeJobs[jobID]
	w.activeJobsMu.Unlock()

	if exists {
		cancel()
	}
}

func (w *Worker) processLoop() {
	defer w.wg.Done()

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	// Semaphore for limiting concurrent downloads
	sem := make(chan struct{}, w.maxWorkers)

	for {
		select {
		case <-w.ctx.Done():
			// Wait for all active downloads to finish
			for i := 0; i < w.maxWorkers; i++ {
				sem <- struct{}{}
			}
			return
		case <-ticker.C:
			// Try to get a pending job
			job, err := w.downloadRepo.GetPending(w.ctx)
			if err != nil {
				log.Error().Err(err).Msg("Failed to get pending job")
				continue
			}
			if job == nil {
				continue
			}

			// Try to acquire semaphore (non-blocking)
			select {
			case sem <- struct{}{}:
				w.wg.Add(1)
				go func(job *models.DownloadJob) {
					defer w.wg.Done()
					defer func() { <-sem }()
					w.processJob(job)
				}(job)
			default:
				// All workers busy, will retry next tick
			}
		}
	}
}

func (w *Worker) processJob(job *models.DownloadJob) {
	log.Info().
		Uint("job_id", job.ID).
		Uint("user_id", job.UserID).
		Int("game_id", job.GameID).
		Str("game_title", job.GameTitle).
		Msg("Starting download job")

	// Create cancellable context for this job
	jobCtx, jobCancel := context.WithCancel(w.ctx)
	defer jobCancel()

	// Register for cancellation
	w.activeJobsMu.Lock()
	w.activeJobs[job.ID] = jobCancel
	w.activeJobsMu.Unlock()
	defer func() {
		w.activeJobsMu.Lock()
		delete(w.activeJobs, job.ID)
		w.activeJobsMu.Unlock()
	}()

	// Update status to downloading
	if err := w.downloadRepo.UpdateStatus(w.ctx, job.ID, models.DownloadStatusDownloading, ""); err != nil {
		log.Error().Err(err).Uint("job_id", job.ID).Msg("Failed to update job status")
		return
	}

	// Broadcast status change
	job.Status = models.DownloadStatusDownloading
	handlers.BroadcastJobUpdate(job.UserID, job)

	// Get user's GOG token
	userToken, err := w.tokenRepo.Get(w.ctx, job.UserID)
	if err != nil {
		w.failJob(job, "Failed to get GOG credentials")
		return
	}

	// Refresh token if needed
	accessToken := userToken.AccessToken
	if time.Now().Add(5 * time.Minute).After(userToken.ExpiresAt) {
		gogClient := &client.GogClient{TokenURL: "https://auth.gog.com/token"}
		newAccessToken, newRefreshToken, expiresIn, err := gogClient.PerformTokenRefresh(userToken.RefreshToken)
		if err != nil {
			w.failJob(job, "GOG session expired. Please log in again.")
			return
		}

		userToken.AccessToken = newAccessToken
		userToken.RefreshToken = newRefreshToken
		userToken.ExpiresAt = time.Now().Add(time.Duration(expiresIn) * time.Second)
		w.tokenRepo.Upsert(w.ctx, userToken)
		accessToken = newAccessToken
	}

	// Get game data
	game, err := w.gameRepo.GetByID(w.ctx, job.UserID, job.GameID)
	if err != nil {
		w.failJob(job, "Game not found in catalogue")
		return
	}

	var gameData client.Game
	if err := json.Unmarshal([]byte(game.Data), &gameData); err != nil {
		w.failJob(job, "Failed to parse game data")
		return
	}

	// Map language code to full name
	langMap := map[string]string{
		"en": "English", "fr": "Français", "de": "Deutsch", "es": "Español",
		"it": "Italiano", "ru": "Русский", "pl": "Polski", "pt-BR": "Português do Brasil",
		"zh-Hans": "简体中文", "ja": "日本語", "ko": "한국어",
	}
	langName := langMap[job.Language]
	if langName == "" {
		langName = "English"
	}

	// Create progress writer that broadcasts to WebSocket
	progressWriter := &wsProgressWriter{
		jobID:        job.ID,
		userID:       job.UserID,
		downloadRepo: w.downloadRepo,
		fileProgress: make(map[string]int64),
	}

	// Execute download
	err = client.DownloadGameFiles(
		jobCtx,
		accessToken,
		gameData,
		w.downloadPath,
		langName,
		job.Platform,
		job.IncludeExtras,
		job.IncludeDLC,
		true,  // resume
		false, // flatten
		false, // skipPatches
		false, // rommLayout
		job.Threads,
		progressWriter,
	)

	if err != nil {
		if jobCtx.Err() == context.Canceled {
			// Job was cancelled
			log.Info().Uint("job_id", job.ID).Msg("Download job cancelled")
			w.downloadRepo.UpdateStatus(w.ctx, job.ID, models.DownloadStatusCancelled, "")
			job.Status = models.DownloadStatusCancelled
			handlers.BroadcastJobUpdate(job.UserID, job)
		} else {
			w.failJob(job, err.Error())
		}
		return
	}

	// Success
	if err := w.downloadRepo.UpdateStatus(w.ctx, job.ID, models.DownloadStatusCompleted, ""); err != nil {
		log.Error().Err(err).Uint("job_id", job.ID).Msg("Failed to update job status")
	}

	log.Info().
		Uint("job_id", job.ID).
		Str("game_title", job.GameTitle).
		Msg("Download job completed")

	job.Status = models.DownloadStatusCompleted
	handlers.BroadcastJobUpdate(job.UserID, job)
}

func (w *Worker) failJob(job *models.DownloadJob, errorMsg string) {
	log.Error().
		Uint("job_id", job.ID).
		Str("error", errorMsg).
		Msg("Download job failed")

	if err := w.downloadRepo.UpdateStatus(w.ctx, job.ID, models.DownloadStatusFailed, errorMsg); err != nil {
		log.Error().Err(err).Uint("job_id", job.ID).Msg("Failed to update job status")
	}

	job.Status = models.DownloadStatusFailed
	job.ErrorMessage = errorMsg
	handlers.BroadcastJobUpdate(job.UserID, job)
}

// wsProgressWriter implements io.Writer to capture progress and broadcast via WebSocket.
type wsProgressWriter struct {
	jobID        uint
	userID       uint
	downloadRepo repository.DownloadJobRepository
	totalBytes   int64
	buffer       bytes.Buffer
	lastUpdate   time.Time

	// Track per-file progress to compute aggregate
	fileProgress   map[string]int64
	fileProgressMu sync.Mutex
}

func (w *wsProgressWriter) Write(p []byte) (n int, err error) {
	n = len(p)
	w.buffer.Write(p)

	// Parse complete JSON lines from buffer
	for {
		line, err := w.buffer.ReadBytes('\n')
		if err == io.EOF {
			// Incomplete line, put it back
			w.buffer.Write(line)
			break
		}
		if err != nil {
			break
		}

		// Parse the progress update
		var update client.ProgressUpdate
		if err := json.Unmarshal(line, &update); err != nil {
			continue
		}

		if update.Type == "start" {
			w.totalBytes = update.OverallTotalBytes
		} else if update.Type == "file_progress" {
			// Track per-file progress and compute aggregate
			w.fileProgressMu.Lock()
			w.fileProgress[update.FileName] = update.CurrentBytes
			var aggregateBytes int64
			for _, bytes := range w.fileProgress {
				aggregateBytes += bytes
			}
			w.fileProgressMu.Unlock()

			// Throttle database updates
			if time.Since(w.lastUpdate) > time.Second {
				w.downloadRepo.UpdateProgress(context.Background(), w.jobID, aggregateBytes)
				w.lastUpdate = time.Now()
			}

			// Broadcast aggregate progress to WebSocket
			handlers.BroadcastProgress(w.userID, w.jobID, aggregateBytes, w.totalBytes, models.DownloadStatusDownloading)
		}
	}

	return n, nil
}
