package domain

import (
	"time"

	"github.com/google/uuid"
)

type Profile struct {
	ID         uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	UserID     uuid.UUID `gorm:"type:uuid;not null;index:idx_profiles_user_id" json:"user_id"`
	Nickname   string    `gorm:"type:varchar(100);not null" json:"nickname"`
	AvatarURL  string    `gorm:"type:text" json:"avatar_url"`
	Coins      int       `gorm:"not null;default:0" json:"coins"`
	TotalStars int       `gorm:"not null;default:0" json:"total_stars"`
	CreatedAt  time.Time `gorm:"not null;default:now()" json:"created_at"`
	UpdatedAt  time.Time `gorm:"not null;default:now()" json:"updated_at"`
}

func (Profile) TableName() string {
	return "profiles"
}

type CreateProfileRequest struct {
	Nickname  string `json:"nickname"`
	AvatarURL string `json:"avatar_url"`
}
