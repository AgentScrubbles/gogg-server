package models

import (
	"time"

	"gorm.io/gorm"
)

// User represents a user account in the system.
type User struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	Username  string         `gorm:"uniqueIndex;not null;size:255" json:"username"`
	GOGUserID string         `gorm:"index;size:255" json:"gog_user_id,omitempty"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// UserToken stores GOG OAuth tokens for a user.
type UserToken struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	UserID       uint      `gorm:"uniqueIndex;not null" json:"user_id"`
	AccessToken  string    `gorm:"not null" json:"-"`
	RefreshToken string    `gorm:"not null" json:"-"`
	ExpiresAt    time.Time `gorm:"not null" json:"expires_at"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`

	User User `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE" json:"-"`
}

// UserGame stores a game in a user's catalogue.
type UserGame struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `gorm:"index;not null" json:"user_id"`
	GameID    int       `gorm:"index;not null" json:"game_id"`
	Title     string    `gorm:"index;size:500" json:"title"`
	Data      string    `gorm:"type:text" json:"data"` // JSON blob of full game metadata
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	User User `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE" json:"-"`
}

// TableName sets a unique constraint on user_id + game_id.
func (UserGame) TableName() string {
	return "user_games"
}

// DownloadStatus represents the state of a download job.
type DownloadStatus string

const (
	DownloadStatusPending     DownloadStatus = "pending"
	DownloadStatusDownloading DownloadStatus = "downloading"
	DownloadStatusCompleted   DownloadStatus = "completed"
	DownloadStatusFailed      DownloadStatus = "failed"
	DownloadStatusCancelled   DownloadStatus = "cancelled"
)

// DownloadJob represents a queued or active download.
type DownloadJob struct {
	ID            uint           `gorm:"primaryKey" json:"id"`
	UserID        uint           `gorm:"index;not null" json:"user_id"`
	GameID        int            `gorm:"not null" json:"game_id"`
	GameTitle     string         `gorm:"size:500" json:"game_title"`
	Status        DownloadStatus `gorm:"index;size:50;default:pending" json:"status"`
	Platform      string         `gorm:"size:50" json:"platform"`
	Language      string         `gorm:"size:50" json:"language"`
	IncludeExtras bool           `gorm:"default:true" json:"include_extras"`
	IncludeDLC    bool           `gorm:"default:true" json:"include_dlc"`
	Threads       int            `gorm:"default:5" json:"threads"`
	ProgressBytes int64          `gorm:"default:0" json:"progress_bytes"`
	TotalBytes    int64          `json:"total_bytes"`
	ErrorMessage  string         `gorm:"type:text" json:"error_message,omitempty"`
	StartedAt     *time.Time     `json:"started_at,omitempty"`
	CompletedAt   *time.Time     `json:"completed_at,omitempty"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`

	User User `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE" json:"-"`
}

// AutoMigrate runs GORM auto-migration for all server models.
func AutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&User{},
		&UserToken{},
		&UserGame{},
		&DownloadJob{},
	)
}
