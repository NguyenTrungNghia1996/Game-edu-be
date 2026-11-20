package domain

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

const (
	ProgressStatusLocked    = "locked"
	ProgressStatusUnlocked  = "unlocked"
	ProgressStatusCompleted = "completed"
)

type ProfileProgress struct {
	ID                uuid.UUID  `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	ProfileID         uuid.UUID  `gorm:"type:uuid;not null;index:idx_progress_profile_status" json:"profile_id"`
	StageID           uuid.UUID  `gorm:"type:uuid;not null" json:"stage_id"`
	Status            string     `gorm:"type:varchar(20);not null;default:'locked';index:idx_progress_profile_status" json:"status"`
	Stars             int        `gorm:"not null;default:0" json:"stars"`
	BestWrongAttempts int        `gorm:"not null;default:0" json:"best_wrong_attempts"`
	CompletedAt       *time.Time `json:"completed_at,omitempty"`
	UpdatedAt         time.Time  `gorm:"not null;default:now()" json:"updated_at"`
}

func (ProfileProgress) TableName() string {
	return "profile_progress"
}

type StartStageResponse struct {
	AttemptID  string               `json:"attempt_id"`
	Stage      StageStartInfo       `json:"stage"`
	Challenges []SanitizedChallenge `json:"challenges"`
}

type SubmitAnswerRequest struct {
	AttemptID   string          `json:"attempt_id"`
	ChallengeID uuid.UUID       `json:"challenge_id"`
	Answer      json.RawMessage `json:"answer"`
}

type SubmitRewards struct {
	Coins int                `json:"coins"`
	Item  *RewardItemSummary `json:"item"`
}

type SubmitUnlocks struct {
	NextStageID *uuid.UUID `json:"next_stage_id"`
	NextWorldID *uuid.UUID `json:"next_world_id"`
}

type SubmitAnswerResponse struct {
	IsCorrect      bool           `json:"is_correct"`
	Explanation    string         `json:"explanation,omitempty"`
	Hint           string         `json:"hint,omitempty"`
	WrongAttempts  *int           `json:"wrong_attempts,omitempty"`
	StageCompleted bool           `json:"stage_completed"`
	StarsEarned    *int           `json:"stars_earned,omitempty"`
	FirstTimeClear *bool          `json:"first_time_clear,omitempty"`
	Rewards        *SubmitRewards `json:"rewards,omitempty"`
	Unlocks        *SubmitUnlocks `json:"unlocks,omitempty"`
}

type UploadURLRequest struct {
	ObjectKey   string `json:"object_key"`
	Folder      string `json:"folder"`
	FileName    string `json:"file_name"`
	ContentType string `json:"content_type"`
}

type UploadURLResponse struct {
	UploadURL string `json:"upload_url"`
	PublicURL string `json:"public_url"`
	ObjectKey string `json:"object_key"`
	ExpiresIn int    `json:"expires_in"`
}
