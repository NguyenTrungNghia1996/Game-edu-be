package domain

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

const (
	ChallengeTypeMultipleChoice = "multiple_choice"
	ChallengeTypeDragDrop       = "drag_drop"
	ChallengeTypeAudioMatch     = "audio_match"
)

type Challenge struct {
	ID             uuid.UUID      `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	StageID        uuid.UUID      `gorm:"type:uuid;not null;index:idx_challenges_stage_id" json:"stage_id"`
	Type           string         `gorm:"type:varchar(32);not null" json:"type"`
	PromptText     string         `gorm:"type:text;not null" json:"prompt_text"`
	PromptAudioURL string         `gorm:"type:text" json:"prompt_audio_url"`
	Payload        datatypes.JSON `gorm:"type:jsonb;not null" json:"payload"`
	OrderIndex     int            `gorm:"not null;default:1" json:"order_index"`
	CreatedAt      time.Time      `gorm:"not null;default:now()" json:"created_at"`
}

func (Challenge) TableName() string {
	return "challenges"
}

// MultipleChoicePayload lưu trong DB cho dạng trắc nghiệm (multiple_choice / audio_match)
type MultipleChoicePayload struct {
	Options         []ChoiceOption `json:"options"`
	CorrectOptionID string         `json:"correct_option_id"` // TUYỆT ĐỐI KHÔNG TRẢ VỀ KHI START STAGE
	Hint            string         `json:"hint,omitempty"`
	Explanation     string         `json:"explanation,omitempty"`
}

type ChoiceOption struct {
	ID       string `json:"id"`
	Text     string `json:"text"`
	AudioURL string `json:"audio_url,omitempty"`
	ImageURL string `json:"image_url,omitempty"`
}

// DragDropPayload lưu trong DB cho dạng kéo thả (drag_drop)
type DragDropPayload struct {
	Items       []DragDropItem    `json:"items"`
	Targets     []DragDropTarget  `json:"targets"`
	CorrectMap  map[string]string `json:"correct_map"` // target_id -> item_id; TUYỆT ĐỐI KHÔNG TRẢ VỀ KHI START STAGE
	Hint        string            `json:"hint,omitempty"`
	Explanation string            `json:"explanation,omitempty"`
}

type DragDropItem struct {
	ID       string `json:"id"`
	Text     string `json:"text"`
	AudioURL string `json:"audio_url,omitempty"`
	ImageURL string `json:"image_url,omitempty"`
}

type DragDropTarget struct {
	ID       string `json:"id"`
	Label    string `json:"label"`
	ImageURL string `json:"image_url,omitempty"`
}

// SanitizedChallenge là dạng câu hỏi trả về cho client khi gọi POST /stages/:stageId/start.
// Tuyệt đối không chứa correct_option_id hay correct_map.
type SanitizedChallenge struct {
	ID             uuid.UUID        `json:"id"`
	Type           string           `json:"type"`
	PromptText     string           `json:"prompt_text"`
	PromptAudioURL string           `json:"prompt_audio_url,omitempty"`
	Options        []ChoiceOption   `json:"options,omitempty"`
	Items          []DragDropItem   `json:"items,omitempty"`
	Targets        []DragDropTarget `json:"targets,omitempty"`
}

type CreateChallengeRequest struct {
	StageID        uuid.UUID       `json:"stage_id"`
	Type           string          `json:"type"`
	PromptText     string          `json:"prompt_text"`
	PromptAudioURL string          `json:"prompt_audio_url"`
	Payload        json.RawMessage `json:"payload"`
	OrderIndex     *int            `json:"order_index"`
}

type UpdateChallengeRequest struct {
	StageID        *uuid.UUID      `json:"stage_id"`
	Type           *string         `json:"type"`
	PromptText     *string         `json:"prompt_text"`
	PromptAudioURL *string         `json:"prompt_audio_url"`
	Payload        json.RawMessage `json:"payload"`
	OrderIndex     *int            `json:"order_index"`
}
