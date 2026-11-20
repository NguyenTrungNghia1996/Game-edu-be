package repository

import (
	"context"
	"errors"

	"edu-game-be/internal/domain"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type WorldRepository struct {
	db *gorm.DB
}

func NewWorldRepository(db *gorm.DB) *WorldRepository {
	return &WorldRepository{db: db}
}

func (r *WorldRepository) ListActive(ctx context.Context) ([]domain.World, error) {
	var worlds []domain.World
	err := r.db.WithContext(ctx).
		Where("is_active = ?", true).
		Order("order_index ASC").
		Find(&worlds).Error
	return worlds, err
}

func (r *WorldRepository) ListAll(ctx context.Context) ([]domain.World, error) {
	var worlds []domain.World
	err := r.db.WithContext(ctx).
		Order("order_index ASC").
		Find(&worlds).Error
	return worlds, err
}

func (r *WorldRepository) FindByID(ctx context.Context, id uuid.UUID) (*domain.World, error) {
	var world domain.World
	err := r.db.WithContext(ctx).First(&world, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &world, nil
}

func (r *WorldRepository) FindFirstActive(ctx context.Context, tx *gorm.DB) (*domain.World, error) {
	q := r.db
	if tx != nil {
		q = tx
	}
	var world domain.World
	err := q.WithContext(ctx).
		Where("is_active = ?", true).
		Order("order_index ASC").
		First(&world).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &world, nil
}

func (r *WorldRepository) FindNextActiveAfter(ctx context.Context, tx *gorm.DB, currentOrderIndex int) (*domain.World, error) {
	q := r.db
	if tx != nil {
		q = tx
	}
	var world domain.World
	err := q.WithContext(ctx).
		Where("is_active = ? AND order_index > ?", true, currentOrderIndex).
		Order("order_index ASC").
		First(&world).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &world, nil
}

func (r *WorldRepository) Create(ctx context.Context, world *domain.World) error {
	return r.db.WithContext(ctx).Create(world).Error
}

func (r *WorldRepository) Update(ctx context.Context, world *domain.World) error {
	return r.db.WithContext(ctx).Save(world).Error
}

func (r *WorldRepository) Delete(ctx context.Context, id uuid.UUID) (int64, error) {
	res := r.db.WithContext(ctx).Delete(&domain.World{}, "id = ?", id)
	return res.RowsAffected, res.Error
}
