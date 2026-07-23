package repository

import (
	"context"
	"errors"
	"time"

	"github.com/habedi/gogg/server/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrNotFound     = errors.New("record not found")
	ErrUnauthorized = errors.New("unauthorized")
)

// UserRepository handles user persistence.
type UserRepository interface {
	Create(ctx context.Context, user *models.User) error
	GetByID(ctx context.Context, id uint) (*models.User, error)
	GetByUsername(ctx context.Context, username string) (*models.User, error)
	GetOrCreateByGOGUserID(ctx context.Context, gogUserID, username string) (*models.User, error)
}

// TokenRepository handles user token persistence.
type TokenRepository interface {
	Get(ctx context.Context, userID uint) (*models.UserToken, error)
	Upsert(ctx context.Context, token *models.UserToken) error
	Delete(ctx context.Context, userID uint) error
}

// GameRepository handles user game catalogue persistence.
type GameRepository interface {
	Put(ctx context.Context, userID uint, gameID int, title, data string) error
	GetByID(ctx context.Context, userID uint, gameID int) (*models.UserGame, error)
	List(ctx context.Context, userID uint, limit, offset int) ([]models.UserGame, int64, error)
	Search(ctx context.Context, userID uint, query string, limit, offset int) ([]models.UserGame, int64, error)
	Clear(ctx context.Context, userID uint) error
}

// DownloadJobRepository handles download job persistence.
type DownloadJobRepository interface {
	Create(ctx context.Context, job *models.DownloadJob) error
	GetByID(ctx context.Context, id uint) (*models.DownloadJob, error)
	GetByIDForUser(ctx context.Context, userID, jobID uint) (*models.DownloadJob, error)
	ListForUser(ctx context.Context, userID uint, status *models.DownloadStatus, limit, offset int) ([]models.DownloadJob, int64, error)
	UpdateStatus(ctx context.Context, id uint, status models.DownloadStatus, errorMsg string) error
	UpdateProgress(ctx context.Context, id uint, progressBytes int64) error
	GetPending(ctx context.Context) (*models.DownloadJob, error)
	Cancel(ctx context.Context, userID, jobID uint) error
}

// --- GORM Implementations ---

type gormUserRepo struct{ db *gorm.DB }
type gormTokenRepo struct{ db *gorm.DB }
type gormGameRepo struct{ db *gorm.DB }
type gormDownloadJobRepo struct{ db *gorm.DB }

func NewUserRepository(db *gorm.DB) UserRepository               { return &gormUserRepo{db: db} }
func NewTokenRepository(db *gorm.DB) TokenRepository             { return &gormTokenRepo{db: db} }
func NewGameRepository(db *gorm.DB) GameRepository               { return &gormGameRepo{db: db} }
func NewDownloadJobRepository(db *gorm.DB) DownloadJobRepository { return &gormDownloadJobRepo{db: db} }

// --- User Repository ---

func (r *gormUserRepo) Create(ctx context.Context, user *models.User) error {
	return r.db.WithContext(ctx).Create(user).Error
}

func (r *gormUserRepo) GetByID(ctx context.Context, id uint) (*models.User, error) {
	var user models.User
	err := r.db.WithContext(ctx).First(&user, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	return &user, err
}

func (r *gormUserRepo) GetByUsername(ctx context.Context, username string) (*models.User, error) {
	var user models.User
	err := r.db.WithContext(ctx).Where("username = ?", username).First(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	return &user, err
}

func (r *gormUserRepo) GetOrCreateByGOGUserID(ctx context.Context, gogUserID, username string) (*models.User, error) {
	var user models.User
	err := r.db.WithContext(ctx).Where("gog_user_id = ?", gogUserID).First(&user).Error
	if err == nil {
		return &user, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	// Create new user
	user = models.User{
		Username:  username,
		GOGUserID: gogUserID,
	}
	if err := r.db.WithContext(ctx).Create(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

// --- Token Repository ---

func (r *gormTokenRepo) Get(ctx context.Context, userID uint) (*models.UserToken, error) {
	var token models.UserToken
	err := r.db.WithContext(ctx).Where("user_id = ?", userID).First(&token).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	return &token, err
}

func (r *gormTokenRepo) Upsert(ctx context.Context, token *models.UserToken) error {
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"access_token", "refresh_token", "expires_at", "updated_at"}),
	}).Create(token).Error
}

func (r *gormTokenRepo) Delete(ctx context.Context, userID uint) error {
	return r.db.WithContext(ctx).Where("user_id = ?", userID).Delete(&models.UserToken{}).Error
}

// --- Game Repository ---

func (r *gormGameRepo) Put(ctx context.Context, userID uint, gameID int, title, data string) error {
	game := models.UserGame{
		UserID: userID,
		GameID: gameID,
		Title:  title,
		Data:   data,
	}
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_id"}, {Name: "game_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"title", "data", "updated_at"}),
	}).Create(&game).Error
}

func (r *gormGameRepo) GetByID(ctx context.Context, userID uint, gameID int) (*models.UserGame, error) {
	var game models.UserGame
	err := r.db.WithContext(ctx).Where("user_id = ? AND game_id = ?", userID, gameID).First(&game).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	return &game, err
}

func (r *gormGameRepo) List(ctx context.Context, userID uint, limit, offset int) ([]models.UserGame, int64, error) {
	var games []models.UserGame
	var total int64

	db := r.db.WithContext(ctx).Model(&models.UserGame{}).Where("user_id = ?", userID)
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if err := db.Order("title ASC").Limit(limit).Offset(offset).Find(&games).Error; err != nil {
		return nil, 0, err
	}
	return games, total, nil
}

func (r *gormGameRepo) Search(ctx context.Context, userID uint, query string, limit, offset int) ([]models.UserGame, int64, error) {
	var games []models.UserGame
	var total int64

	db := r.db.WithContext(ctx).Model(&models.UserGame{}).
		Where("user_id = ? AND title ILIKE ?", userID, "%"+query+"%")

	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if err := db.Order("title ASC").Limit(limit).Offset(offset).Find(&games).Error; err != nil {
		return nil, 0, err
	}
	return games, total, nil
}

func (r *gormGameRepo) Clear(ctx context.Context, userID uint) error {
	return r.db.WithContext(ctx).Where("user_id = ?", userID).Delete(&models.UserGame{}).Error
}

// --- Download Job Repository ---

func (r *gormDownloadJobRepo) Create(ctx context.Context, job *models.DownloadJob) error {
	return r.db.WithContext(ctx).Create(job).Error
}

func (r *gormDownloadJobRepo) GetByID(ctx context.Context, id uint) (*models.DownloadJob, error) {
	var job models.DownloadJob
	err := r.db.WithContext(ctx).First(&job, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	return &job, err
}

func (r *gormDownloadJobRepo) GetByIDForUser(ctx context.Context, userID, jobID uint) (*models.DownloadJob, error) {
	var job models.DownloadJob
	err := r.db.WithContext(ctx).Where("id = ? AND user_id = ?", jobID, userID).First(&job).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	return &job, err
}

func (r *gormDownloadJobRepo) ListForUser(ctx context.Context, userID uint, status *models.DownloadStatus, limit, offset int) ([]models.DownloadJob, int64, error) {
	var jobs []models.DownloadJob
	var total int64

	db := r.db.WithContext(ctx).Model(&models.DownloadJob{}).Where("user_id = ?", userID)
	if status != nil {
		db = db.Where("status = ?", *status)
	}

	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if err := db.Order("created_at DESC").Limit(limit).Offset(offset).Find(&jobs).Error; err != nil {
		return nil, 0, err
	}
	return jobs, total, nil
}

func (r *gormDownloadJobRepo) UpdateStatus(ctx context.Context, id uint, status models.DownloadStatus, errorMsg string) error {
	updates := map[string]interface{}{
		"status":     status,
		"updated_at": time.Now(),
	}
	if errorMsg != "" {
		updates["error_message"] = errorMsg
	}
	if status == models.DownloadStatusDownloading {
		now := time.Now()
		updates["started_at"] = &now
	}
	if status == models.DownloadStatusCompleted || status == models.DownloadStatusFailed || status == models.DownloadStatusCancelled {
		now := time.Now()
		updates["completed_at"] = &now
	}
	return r.db.WithContext(ctx).Model(&models.DownloadJob{}).Where("id = ?", id).Updates(updates).Error
}

func (r *gormDownloadJobRepo) UpdateProgress(ctx context.Context, id uint, progressBytes int64) error {
	return r.db.WithContext(ctx).Model(&models.DownloadJob{}).Where("id = ?", id).
		Updates(map[string]interface{}{"progress_bytes": progressBytes, "updated_at": time.Now()}).Error
}

func (r *gormDownloadJobRepo) GetPending(ctx context.Context) (*models.DownloadJob, error) {
	var job models.DownloadJob
	err := r.db.WithContext(ctx).Where("status = ?", models.DownloadStatusPending).
		Order("created_at ASC").First(&job).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &job, err
}

func (r *gormDownloadJobRepo) Cancel(ctx context.Context, userID, jobID uint) error {
	result := r.db.WithContext(ctx).Model(&models.DownloadJob{}).
		Where("id = ? AND user_id = ? AND status IN ?", jobID, userID,
			[]models.DownloadStatus{models.DownloadStatusPending, models.DownloadStatusDownloading}).
		Updates(map[string]interface{}{
			"status":       models.DownloadStatusCancelled,
			"completed_at": time.Now(),
			"updated_at":   time.Now(),
		})
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return result.Error
}
