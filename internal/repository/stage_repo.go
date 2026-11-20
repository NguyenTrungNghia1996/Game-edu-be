package repository

import (
	"context"
	"errors"

	"edu-game-be/internal/domain"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type StageRepository struct {
	db *gorm.DB
}

func NewStageRepository(db *gorm.DB) *StageRepository {
	return &StageRepository{db: db}
}

func (r *StageRepository) ListByWorldID(ctx context.Context, worldID uuid.UUID) ([]domain.Stage, error) {
	var stages []domain.Stage
	err := r.db.WithContext(ctx).
		Preload("RewardItem").
		Where("world_id = ?", worldID).
		Order("order_index ASC").
		Find(&stages).Error
	return stages, err
}

func (r *StageRepository) ListAll(ctx context.Context, worldID *uuid.UUID) ([]domain.Stage, error) {
	var stages []domain.Stage
	q := r.db.WithContext(ctx).Preload("RewardItem")
	if worldID != nil {
		q = q.Where("world_id = ?", *worldID)
	}
	err := q.Order("world_id ASC, order_index ASC").Find(&stages).Error
	return stages, err
}

func (r *StageRepository) FindByID(ctx context.Context, id uuid.UUID) (*domain.Stage, error) {
	var stage domain.Stage
	err := r.db.WithContext(ctx).Preload("RewardItem").First(&stage, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &stage, nil
}

func (r *StageRepository) FindFirstInWorld(ctx context.Context, tx *gorm.DB, worldID uuid.UUID) (*domain.Stage, error) {
	q := r.db
	if tx != nil {
		q = tx
	}
	var stage domain.Stage
	err := q.WithContext(ctx).
		Where("world_id = ?", worldID).
		Order("order_index ASC").
		First(&stage).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &stage, nil
}

func (r *StageRepository) FindNextInWorld(ctx context.Context, tx *gorm.DB, worldID uuid.UUID, currentOrderIndex int) (*domain.Stage, error) {
	q := r.db
	if tx != nil {
		q = tx
	}
	var stage domain.Stage
	err := q.WithContext(ctx).
		Where("world_id = ? AND order_index > ?", worldID, currentOrderIndex).
		Order("order_index ASC").
		First(&stage).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &stage, nil
}

func (r *StageRepository) CountByWorldIDs(ctx context.Context, worldIDs []uuid.UUID) (map[uuid.UUID]int, error) {
	result := make(map[uuid.UUID]int, len(worldIDs))
	if len(worldIDs) == 0 {
		return result, nil
	}
	type row struct {
		WorldID uuid.UUID
		Count   int
	}
	var rows []row
	err := r.db.WithContext(ctx).
		Model(&domain.Stage{}).
		Select("world_id, COUNT(*) AS count").
		Where("world_id IN ?", worldIDs).
		Group("world_id").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, r := range rows {
		result[r.WorldID] = r.Count
	}
	return result, nil
}

func (r *StageRepository) CreateStage(ctx context.Context, stage *domain.Stage) error {
	return r.db.WithContext(ctx).Create(stage).Error
}

func (r *StageRepository) UpdateStage(ctx context.Context, stage *domain.Stage) error {
	return r.db.WithContext(ctx).Save(stage).Error
}

func (r *StageRepository) DeleteStage(ctx context.Context, id uuid.UUID) (int64, error) {
	res := r.db.WithContext(ctx).Delete(&domain.Stage{}, "id = ?", id)
	return res.RowsAffected, res.Error
}

// Challenges

func (r *StageRepository) ListChallengesByStageID(ctx context.Context, stageID uuid.UUID) ([]domain.Challenge, error) {
	var challenges []domain.Challenge
	err := r.db.WithContext(ctx).
		Where("stage_id = ?", stageID).
		Order("order_index ASC, created_at ASC").
		Find(&challenges).Error
	return challenges, err
}

func (r *StageRepository) ListChallenges(ctx context.Context, stageID *uuid.UUID) ([]domain.Challenge, error) {
	var challenges []domain.Challenge
	q := r.db.WithContext(ctx)
	if stageID != nil {
		q = q.Where("stage_id = ?", *stageID)
	}
	err := q.Order("stage_id ASC, order_index ASC, created_at ASC").Find(&challenges).Error
	return challenges, err
}

func (r *StageRepository) FindChallengeByID(ctx context.Context, id uuid.UUID) (*domain.Challenge, error) {
	var challenge domain.Challenge
	err := r.db.WithContext(ctx).First(&challenge, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &challenge, nil
}

func (r *StageRepository) CreateChallenge(ctx context.Context, challenge *domain.Challenge) error {
	return r.db.WithContext(ctx).Create(challenge).Error
}

func (r *StageRepository) UpdateChallenge(ctx context.Context, challenge *domain.Challenge) error {
	return r.db.WithContext(ctx).Save(challenge).Error
}

func (r *StageRepository) DeleteChallenge(ctx context.Context, id uuid.UUID) (int64, error) {
	res := r.db.WithContext(ctx).Delete(&domain.Challenge{}, "id = ?", id)
	return res.RowsAffected, res.Error
}
