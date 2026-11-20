package domain

import (
	"time"

	"github.com/google/uuid"
)

const (
	ItemCategoryHat    = "hat"
	ItemCategoryOutfit = "outfit"
	ItemCategoryBadge  = "badge"
	ItemCategoryTool   = "tool"
)

type Item struct {
	ID          uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	Code        string    `gorm:"type:varchar(64);uniqueIndex;not null" json:"code"`
	Name        string    `gorm:"type:varchar(150);not null" json:"name"`
	Description string    `gorm:"type:text" json:"description"`
	Category    string    `gorm:"type:varchar(32);not null" json:"category"`
	IconURL     string    `gorm:"type:text;not null" json:"icon_url"`
	SpriteURL   string    `gorm:"type:text" json:"sprite_url,omitempty"`
	CreatedAt   time.Time `gorm:"not null;default:now()" json:"created_at"`
}

func (Item) TableName() string {
	return "items"
}

type RewardItemSummary struct {
	ID      uuid.UUID `json:"id"`
	Name    string    `json:"name"`
	IconURL string    `json:"icon_url"`
}

type ProfileInventory struct {
	ID         uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	ProfileID  uuid.UUID `gorm:"type:uuid;not null" json:"profile_id"`
	ItemID     uuid.UUID `gorm:"type:uuid;not null" json:"item_id"`
	Item       Item      `gorm:"foreignKey:ItemID" json:"item"`
	Quantity   int       `gorm:"not null;default:1" json:"quantity"`
	IsEquipped bool      `gorm:"not null;default:false" json:"is_equipped"`
	AcquiredAt time.Time `gorm:"not null;default:now()" json:"acquired_at"`
}

func (ProfileInventory) TableName() string {
	return "profile_inventory"
}

type EquipItemRequest struct {
	IsEquipped *bool `json:"is_equipped"`
}

type CreateItemRequest struct {
	Code        string `json:"code"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Category    string `json:"category"`
	IconURL     string `json:"icon_url"`
	SpriteURL   string `json:"sprite_url"`
}

type UpdateItemRequest struct {
	Code        *string `json:"code"`
	Name        *string `json:"name"`
	Description *string `json:"description"`
	Category    *string `json:"category"`
	IconURL     *string `json:"icon_url"`
	SpriteURL   *string `json:"sprite_url"`
}
