package service

import (
	"context"
	"encoding/json"
	"fmt"
	"path"
	"strings"
	"time"

	"edu-game-be/internal/domain"
	"edu-game-be/internal/repository"
	"edu-game-be/internal/storage"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"gorm.io/datatypes"
)

type AdminService struct {
	worldRepo    *repository.WorldRepository
	stageRepo    *repository.StageRepository
	progressRepo *repository.ProgressRepository
	storage      storage.ObjectStorage
	redis        *redis.Client
}

func NewAdminService(
	worldRepo *repository.WorldRepository,
	stageRepo *repository.StageRepository,
	progressRepo *repository.ProgressRepository,
	objectStorage storage.ObjectStorage,
	redisClient *redis.Client,
) *AdminService {
	return &AdminService{
		worldRepo:    worldRepo,
		stageRepo:    stageRepo,
		progressRepo: progressRepo,
		storage:      objectStorage,
		redis:        redisClient,
	}
}

func (s *AdminService) GenerateUploadURL(ctx context.Context, req domain.UploadURLRequest) (*domain.UploadURLResponse, error) {
	contentType := strings.TrimSpace(req.ContentType)
	if contentType == "" {
		return nil, WrapErr(ErrValidation, "content_type is required")
	}

	objectKey := strings.TrimSpace(strings.TrimLeft(req.ObjectKey, "/"))
	if objectKey == "" {
		fileName := strings.TrimSpace(req.FileName)
		if fileName == "" {
			return nil, WrapErr(ErrValidation, "either object_key or file_name is required")
		}
		folder := strings.Trim(strings.TrimSpace(req.Folder), "/")
		if folder == "" {
			folder = "uploads"
		}
		ext := path.Ext(fileName)
		base := strings.TrimSuffix(path.Base(fileName), ext)
		safeBase := sanitizeKeySegment(base)
		if safeBase == "" {
			safeBase = "asset"
		}
		objectKey = fmt.Sprintf("%s/%d_%s_%s%s", folder, time.Now().UTC().Unix(), uuid.NewString()[:8], safeBase, ext)
	}
	if strings.Contains(objectKey, "..") {
		return nil, WrapErr(ErrValidation, "object_key must not contain '..'")
	}

	uploadURL, publicURL, err := s.storage.GenerateUploadURL(ctx, objectKey, contentType)
	if err != nil {
		return nil, err
	}

	return &domain.UploadURLResponse{
		UploadURL: uploadURL,
		PublicURL: publicURL,
		ObjectKey: objectKey,
		ExpiresIn: 300,
	}, nil
}

// --- Worlds CRUD ---

func (s *AdminService) ListWorlds(ctx context.Context) ([]domain.World, error) {
	return s.worldRepo.ListAll(ctx)
}

func (s *AdminService) GetWorld(ctx context.Context, id uuid.UUID) (*domain.World, error) {
	world, err := s.worldRepo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if world == nil {
		return nil, WrapErr(ErrNotFound, "world not found")
	}
	return world, nil
}

func (s *AdminService) CreateWorld(ctx context.Context, req domain.CreateWorldRequest) (*domain.World, error) {
	if strings.TrimSpace(req.Code) == "" || strings.TrimSpace(req.Name) == "" || strings.TrimSpace(req.MapBgURL) == "" {
		return nil, WrapErr(ErrValidation, "code, name, and map_bg_url are required")
	}
	if req.OrderIndex <= 0 {
		return nil, WrapErr(ErrValidation, "order_index must be > 0")
	}
	isActive := true
	if req.IsActive != nil {
		isActive = *req.IsActive
	}

	world := &domain.World{
		Code:        strings.TrimSpace(req.Code),
		Name:        strings.TrimSpace(req.Name),
		Description: strings.TrimSpace(req.Description),
		OrderIndex:  req.OrderIndex,
		MapBgURL:    strings.TrimSpace(req.MapBgURL),
		BgmURL:      strings.TrimSpace(req.BgmURL),
		IsActive:    isActive,
	}
	if err := s.worldRepo.Create(ctx, world); err != nil {
		return nil, err
	}
	s.invalidateWorldCaches(ctx, world.ID)
	return world, nil
}

func (s *AdminService) UpdateWorld(ctx context.Context, id uuid.UUID, req domain.UpdateWorldRequest) (*domain.World, error) {
	world, err := s.worldRepo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if world == nil {
		return nil, WrapErr(ErrNotFound, "world not found")
	}

	if req.Code != nil {
		world.Code = strings.TrimSpace(*req.Code)
	}
	if req.Name != nil {
		world.Name = strings.TrimSpace(*req.Name)
	}
	if req.Description != nil {
		world.Description = strings.TrimSpace(*req.Description)
	}
	if req.OrderIndex != nil {
		if *req.OrderIndex <= 0 {
			return nil, WrapErr(ErrValidation, "order_index must be > 0")
		}
		world.OrderIndex = *req.OrderIndex
	}
	if req.MapBgURL != nil {
		world.MapBgURL = strings.TrimSpace(*req.MapBgURL)
	}
	if req.BgmURL != nil {
		world.BgmURL = strings.TrimSpace(*req.BgmURL)
	}
	if req.IsActive != nil {
		world.IsActive = *req.IsActive
	}

	if err := s.worldRepo.Update(ctx, world); err != nil {
		return nil, err
	}
	s.invalidateWorldCaches(ctx, world.ID)
	return world, nil
}

func (s *AdminService) DeleteWorld(ctx context.Context, id uuid.UUID) error {
	rows, err := s.worldRepo.Delete(ctx, id)
	if err != nil {
		return err
	}
	if rows == 0 {
		return WrapErr(ErrNotFound, "world not found")
	}
	s.invalidateWorldCaches(ctx, id)
	return nil
}

// --- Stages CRUD ---

func (s *AdminService) ListStages(ctx context.Context, worldID *uuid.UUID) ([]domain.Stage, error) {
	return s.stageRepo.ListAll(ctx, worldID)
}

func (s *AdminService) GetStage(ctx context.Context, id uuid.UUID) (*domain.Stage, error) {
	stage, err := s.stageRepo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if stage == nil {
		return nil, WrapErr(ErrNotFound, "stage not found")
	}
	return stage, nil
}

func (s *AdminService) CreateStage(ctx context.Context, req domain.CreateStageRequest) (*domain.Stage, error) {
	if req.WorldID == uuid.Nil {
		return nil, WrapErr(ErrValidation, "world_id is required")
	}
	if req.OrderIndex <= 0 {
		return nil, WrapErr(ErrValidation, "order_index must be > 0")
	}
	if strings.TrimSpace(req.Name) == "" || strings.TrimSpace(req.ObstacleType) == "" {
		return nil, WrapErr(ErrValidation, "name and obstacle_type are required")
	}

	coins := 50
	if req.RewardCoins != nil {
		if *req.RewardCoins < 0 {
			return nil, WrapErr(ErrValidation, "reward_coins must be >= 0")
		}
		coins = *req.RewardCoins
	}

	stage := &domain.Stage{
		WorldID:      req.WorldID,
		OrderIndex:   req.OrderIndex,
		Name:         strings.TrimSpace(req.Name),
		ObstacleType: strings.TrimSpace(req.ObstacleType),
		PosX:         req.PosX,
		PosY:         req.PosY,
		RewardItemID: req.RewardItemID,
		RewardCoins:  coins,
	}
	if err := s.stageRepo.CreateStage(ctx, stage); err != nil {
		return nil, err
	}
	s.invalidateWorldCaches(ctx, stage.WorldID)
	return s.stageRepo.FindByID(ctx, stage.ID)
}

func (s *AdminService) UpdateStage(ctx context.Context, id uuid.UUID, req domain.UpdateStageRequest) (*domain.Stage, error) {
	stage, err := s.stageRepo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if stage == nil {
		return nil, WrapErr(ErrNotFound, "stage not found")
	}
	oldWorldID := stage.WorldID

	if req.WorldID != nil && *req.WorldID != uuid.Nil {
		stage.WorldID = *req.WorldID
	}
	if req.OrderIndex != nil {
		if *req.OrderIndex <= 0 {
			return nil, WrapErr(ErrValidation, "order_index must be > 0")
		}
		stage.OrderIndex = *req.OrderIndex
	}
	if req.Name != nil {
		stage.Name = strings.TrimSpace(*req.Name)
	}
	if req.ObstacleType != nil {
		stage.ObstacleType = strings.TrimSpace(*req.ObstacleType)
	}
	if req.PosX != nil {
		stage.PosX = *req.PosX
	}
	if req.PosY != nil {
		stage.PosY = *req.PosY
	}
	if req.ClearReward {
		stage.RewardItemID = nil
		stage.RewardItem = nil
	} else if req.RewardItemID != nil {
		stage.RewardItemID = req.RewardItemID
		stage.RewardItem = nil
	}
	if req.RewardCoins != nil {
		if *req.RewardCoins < 0 {
			return nil, WrapErr(ErrValidation, "reward_coins must be >= 0")
		}
		stage.RewardCoins = *req.RewardCoins
	}

	if err := s.stageRepo.UpdateStage(ctx, stage); err != nil {
		return nil, err
	}
	s.invalidateWorldCaches(ctx, oldWorldID, stage.WorldID)
	return s.stageRepo.FindByID(ctx, stage.ID)
}

func (s *AdminService) DeleteStage(ctx context.Context, id uuid.UUID) error {
	stage, err := s.stageRepo.FindByID(ctx, id)
	if err != nil {
		return err
	}
	if stage == nil {
		return WrapErr(ErrNotFound, "stage not found")
	}
	if _, err := s.stageRepo.DeleteStage(ctx, id); err != nil {
		return err
	}
	s.invalidateWorldCaches(ctx, stage.WorldID)
	return nil
}

// --- Challenges CRUD ---

func (s *AdminService) ListChallenges(ctx context.Context, stageID *uuid.UUID) ([]domain.Challenge, error) {
	return s.stageRepo.ListChallenges(ctx, stageID)
}

func (s *AdminService) GetChallenge(ctx context.Context, id uuid.UUID) (*domain.Challenge, error) {
	challenge, err := s.stageRepo.FindChallengeByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if challenge == nil {
		return nil, WrapErr(ErrNotFound, "challenge not found")
	}
	return challenge, nil
}

func (s *AdminService) CreateChallenge(ctx context.Context, req domain.CreateChallengeRequest) (*domain.Challenge, error) {
	if req.StageID == uuid.Nil {
		return nil, WrapErr(ErrValidation, "stage_id is required")
	}
	if strings.TrimSpace(req.PromptText) == "" {
		return nil, WrapErr(ErrValidation, "prompt_text is required")
	}
	if err := validateChallengePayload(req.Type, req.Payload); err != nil {
		return nil, WrapErr(ErrValidation, err.Error())
	}

	orderIdx := 1
	if req.OrderIndex != nil && *req.OrderIndex > 0 {
		orderIdx = *req.OrderIndex
	}

	challenge := &domain.Challenge{
		StageID:        req.StageID,
		Type:           req.Type,
		PromptText:     strings.TrimSpace(req.PromptText),
		PromptAudioURL: strings.TrimSpace(req.PromptAudioURL),
		Payload:        datatypes.JSON(req.Payload),
		OrderIndex:     orderIdx,
	}
	if err := s.stageRepo.CreateChallenge(ctx, challenge); err != nil {
		return nil, err
	}
	return challenge, nil
}

func (s *AdminService) UpdateChallenge(ctx context.Context, id uuid.UUID, req domain.UpdateChallengeRequest) (*domain.Challenge, error) {
	challenge, err := s.stageRepo.FindChallengeByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if challenge == nil {
		return nil, WrapErr(ErrNotFound, "challenge not found")
	}

	if req.StageID != nil && *req.StageID != uuid.Nil {
		challenge.StageID = *req.StageID
	}
	if req.Type != nil {
		challenge.Type = strings.TrimSpace(*req.Type)
	}
	if req.PromptText != nil {
		challenge.PromptText = strings.TrimSpace(*req.PromptText)
	}
	if req.PromptAudioURL != nil {
		challenge.PromptAudioURL = strings.TrimSpace(*req.PromptAudioURL)
	}
	if len(req.Payload) > 0 {
		challenge.Payload = datatypes.JSON(req.Payload)
	}
	if req.OrderIndex != nil && *req.OrderIndex > 0 {
		challenge.OrderIndex = *req.OrderIndex
	}

	if err := validateChallengePayload(challenge.Type, json.RawMessage(challenge.Payload)); err != nil {
		return nil, WrapErr(ErrValidation, err.Error())
	}

	if err := s.stageRepo.UpdateChallenge(ctx, challenge); err != nil {
		return nil, err
	}
	return challenge, nil
}

func (s *AdminService) DeleteChallenge(ctx context.Context, id uuid.UUID) error {
	rows, err := s.stageRepo.DeleteChallenge(ctx, id)
	if err != nil {
		return err
	}
	if rows == 0 {
		return WrapErr(ErrNotFound, "challenge not found")
	}
	return nil
}

// --- Items CRUD ---

func (s *AdminService) ListItems(ctx context.Context) ([]domain.Item, error) {
	return s.progressRepo.ListItems(ctx)
}

func (s *AdminService) GetItem(ctx context.Context, id uuid.UUID) (*domain.Item, error) {
	item, err := s.progressRepo.FindItemByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, WrapErr(ErrNotFound, "item not found")
	}
	return item, nil
}

func (s *AdminService) CreateItem(ctx context.Context, req domain.CreateItemRequest) (*domain.Item, error) {
	if strings.TrimSpace(req.Code) == "" || strings.TrimSpace(req.Name) == "" || strings.TrimSpace(req.IconURL) == "" {
		return nil, WrapErr(ErrValidation, "code, name, and icon_url are required")
	}
	if !isValidItemCategory(req.Category) {
		return nil, WrapErr(ErrValidation, "category must be one of: hat, outfit, badge, tool")
	}

	item := &domain.Item{
		Code:        strings.TrimSpace(req.Code),
		Name:        strings.TrimSpace(req.Name),
		Description: strings.TrimSpace(req.Description),
		Category:    req.Category,
		IconURL:     strings.TrimSpace(req.IconURL),
		SpriteURL:   strings.TrimSpace(req.SpriteURL),
	}
	if err := s.progressRepo.CreateItem(ctx, item); err != nil {
		return nil, err
	}
	return item, nil
}

func (s *AdminService) UpdateItem(ctx context.Context, id uuid.UUID, req domain.UpdateItemRequest) (*domain.Item, error) {
	item, err := s.progressRepo.FindItemByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, WrapErr(ErrNotFound, "item not found")
	}

	if req.Code != nil {
		item.Code = strings.TrimSpace(*req.Code)
	}
	if req.Name != nil {
		item.Name = strings.TrimSpace(*req.Name)
	}
	if req.Description != nil {
		item.Description = strings.TrimSpace(*req.Description)
	}
	if req.Category != nil {
		if !isValidItemCategory(*req.Category) {
			return nil, WrapErr(ErrValidation, "category must be one of: hat, outfit, badge, tool")
		}
		item.Category = *req.Category
	}
	if req.IconURL != nil {
		item.IconURL = strings.TrimSpace(*req.IconURL)
	}
	if req.SpriteURL != nil {
		item.SpriteURL = strings.TrimSpace(*req.SpriteURL)
	}

	if err := s.progressRepo.UpdateItem(ctx, item); err != nil {
		return nil, err
	}
	s.invalidateAllMapCaches(ctx)
	return item, nil
}

func (s *AdminService) DeleteItem(ctx context.Context, id uuid.UUID) error {
	rows, err := s.progressRepo.DeleteItem(ctx, id)
	if err != nil {
		return err
	}
	if rows == 0 {
		return WrapErr(ErrNotFound, "item not found")
	}
	s.invalidateAllMapCaches(ctx)
	return nil
}

func validateChallengePayload(challengeType string, raw json.RawMessage) error {
	if len(raw) == 0 {
		return fmt.Errorf("payload is required")
	}
	switch challengeType {
	case domain.ChallengeTypeMultipleChoice, domain.ChallengeTypeAudioMatch:
		var p domain.MultipleChoicePayload
		if err := json.Unmarshal(raw, &p); err != nil {
			return fmt.Errorf("invalid multiple_choice payload: %w", err)
		}
		if len(p.Options) < 2 {
			return fmt.Errorf("payload.options must have at least 2 options")
		}
		if strings.TrimSpace(p.CorrectOptionID) == "" {
			return fmt.Errorf("payload.correct_option_id is required")
		}
		found := false
		for _, opt := range p.Options {
			if opt.ID == p.CorrectOptionID {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("payload.correct_option_id must match one of payload.options[].id")
		}
		return nil

	case domain.ChallengeTypeDragDrop:
		var p domain.DragDropPayload
		if err := json.Unmarshal(raw, &p); err != nil {
			return fmt.Errorf("invalid drag_drop payload: %w", err)
		}
		if len(p.Items) == 0 || len(p.Targets) == 0 || len(p.CorrectMap) == 0 {
			return fmt.Errorf("drag_drop payload requires items, targets, and correct_map")
		}
		return nil

	default:
		return fmt.Errorf("type must be one of: multiple_choice, drag_drop, audio_match")
	}
}

func isValidItemCategory(cat string) bool {
	switch cat {
	case domain.ItemCategoryHat, domain.ItemCategoryOutfit, domain.ItemCategoryBadge, domain.ItemCategoryTool:
		return true
	default:
		return false
	}
}

func sanitizeKeySegment(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			b.WriteRune(r)
		} else if r == ' ' {
			b.WriteRune('-')
		}
	}
	return b.String()
}

func (s *AdminService) invalidateWorldCaches(ctx context.Context, worldIDs ...uuid.UUID) {
	if s.redis == nil {
		return
	}
	keys := []string{cacheKeyWorlds}
	for _, id := range worldIDs {
		if id != uuid.Nil {
			keys = append(keys, cacheKeyMapPref+id.String())
		}
	}
	_ = s.redis.Del(ctx, keys...).Err()
}

func (s *AdminService) invalidateAllMapCaches(ctx context.Context) {
	if s.redis == nil {
		return
	}
	_ = s.redis.Del(ctx, cacheKeyWorlds).Err()
	var cursor uint64
	for {
		keys, nextCursor, err := s.redis.Scan(ctx, cursor, cacheKeyMapPref+"*", 100).Result()
		if err != nil {
			break
		}
		if len(keys) > 0 {
			_ = s.redis.Del(ctx, keys...).Err()
		}
		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}
}
