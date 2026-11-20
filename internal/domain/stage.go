package domain

import (
	"time"

	"github.com/google/uuid"
)

type Stage struct {
	ID           uuid.UUID  `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	WorldID      uuid.UUID  `gorm:"type:uuid;not null;index:idx_stages_world_id" json:"world_id"`
	OrderIndex   int        `gorm:"not null" json:"order_index"`
	Name         string     `gorm:"type:varchar(150);not null" json:"name"`
	ObstacleType string     `gorm:"type:varchar(50);not null" json:"obstacle_type"`
	PosX         int        `gorm:"not null;default:0" json:"pos_x"`
	PosY         int        `gorm:"not null;default:0" json:"pos_y"`
	RewardItemID *uuid.UUID `gorm:"type:uuid" json:"reward_item_id"`
	RewardItem   *Item      `gorm:"foreignKey:RewardItemID" json:"reward_item,omitempty"`
	RewardCoins  int        `gorm:"not null;default:50" json:"reward_coins"`
	CreatedAt    time.Time  `gorm:"not null;default:now()" json:"created_at"`
}

func (Stage) TableName() string {
	return "stages"
}

type StageMapNodeResponse struct {
	ID                uuid.UUID  `json:"id"`
	OrderIndex        int        `json:"order_index"`
	Name              string     `json:"name"`
	ObstacleType      string     `json:"obstacle_type"`
	PosX              int        `json:"pos_x"`
	PosY              int        `json:"pos_y"`
	RewardCoins       int        `json:"reward_coins"`
	RewardItem        *Item      `json:"reward_item,omitempty"`
	Status            string     `json:"status"`
	Stars             int        `json:"stars"`
	BestWrongAttempts int        `json:"best_wrong_attempts"`
	CompletedAt       *time.Time `json:"completed_at,omitempty"`
}

type StageStartInfo struct {
	ID           uuid.UUID `json:"id"`
	Name         string    `json:"name"`
	ObstacleType string    `json:"obstacle_type"`
}

type CreateStageRequest struct {
	WorldID      uuid.UUID  `json:"world_id"`
	OrderIndex   int        `json:"order_index"`
	Name         string     `json:"name"`
	ObstacleType string     `json:"obstacle_type"`
	PosX         int        `json:"pos_x"`
	PosY         int        `json:"pos_y"`
	RewardItemID *uuid.UUID `json:"reward_item_id"`
	RewardCoins  *int       `json:"reward_coins"`
}

type UpdateStageRequest struct {
	WorldID      *uuid.UUID `json:"world_id"`
	OrderIndex   *int       `json:"order_index"`
	Name         *string    `json:"name"`
	ObstacleType *string    `json:"obstacle_type"`
	PosX         *int       `json:"pos_x"`
	PosY         *int       `json:"pos_y"`
	RewardItemID *uuid.UUID `json:"reward_item_id"`
	ClearReward  bool       `json:"clear_reward_item"`
	RewardCoins  *int       `json:"reward_coins"`
}
