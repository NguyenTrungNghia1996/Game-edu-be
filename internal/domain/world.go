package domain

import (
	"time"

	"github.com/google/uuid"
)

type World struct {
	ID          uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	Code        string    `gorm:"type:varchar(64);uniqueIndex;not null" json:"code"`
	Name        string    `gorm:"type:varchar(150);not null" json:"name"`
	Description string    `gorm:"type:text" json:"description"`
	OrderIndex  int       `gorm:"uniqueIndex;not null" json:"order_index"`
	MapBgURL    string    `gorm:"type:text;not null" json:"map_bg_url"`
	BgmURL      string    `gorm:"type:text" json:"bgm_url"`
	IsActive    bool      `gorm:"not null;default:true" json:"is_active"`
	CreatedAt   time.Time `gorm:"not null;default:now()" json:"created_at"`
}

func (World) TableName() string {
	return "worlds"
}

type WorldSummaryResponse struct {
	ID              uuid.UUID `json:"id"`
	Code            string    `json:"code"`
	Name            string    `json:"name"`
	Description     string    `json:"description"`
	OrderIndex      int       `json:"order_index"`
	MapBgURL        string    `json:"map_bg_url"`
	BgmURL          string    `json:"bgm_url,omitempty"`
	TotalStages     int       `json:"total_stages"`
	CompletedStages int       `json:"completed_stages"`
	EarnedStars     int       `json:"earned_stars"`
	MaxStars        int       `json:"max_stars"`
	IsUnlocked      bool      `json:"is_unlocked"`
}

type WorldMapResponse struct {
	World  World                  `json:"world"`
	Stages []StageMapNodeResponse `json:"stages"`
}

type CreateWorldRequest struct {
	Code        string `json:"code"`
	Name        string `json:"name"`
	Description string `json:"description"`
	OrderIndex  int    `json:"order_index"`
	MapBgURL    string `json:"map_bg_url"`
	BgmURL      string `json:"bgm_url"`
	IsActive    *bool  `json:"is_active"`
}

type UpdateWorldRequest struct {
	Code        *string `json:"code"`
	Name        *string `json:"name"`
	Description *string `json:"description"`
	OrderIndex  *int    `json:"order_index"`
	MapBgURL    *string `json:"map_bg_url"`
	BgmURL      *string `json:"bgm_url"`
	IsActive    *bool   `json:"is_active"`
}
