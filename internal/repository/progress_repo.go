package repository

import (
	"context"
	"errors"
	"time"

	"edu-game-be/internal/domain"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type WorldProgressStats struct {
	WorldID         uuid.UUID
	CompletedStages int
	UnlockedOrDone  int
	EarnedStars     int
}

type ProgressRepository struct {
	db *gorm.DB
}

func NewProgressRepository(db *gorm.DB) *ProgressRepository {
	return &ProgressRepository{db: db}
}

func (r *ProgressRepository) DB() *gorm.DB {
	return r.db
}

func (r *ProgressRepository) FindByProfileAndStage(ctx context.Context, profileID, stageID uuid.UUID) (*domain.ProfileProgress, error) {
	var progress domain.ProfileProgress
	err := r.db.WithContext(ctx).
		Where("profile_id = ? AND stage_id = ?", profileID, stageID).
		First(&progress).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &progress, nil
}

// FindByProfileAndStageForUpdate acquires a row-level FOR UPDATE lock inside a transaction.
func (r *ProgressRepository) FindByProfileAndStageForUpdate(ctx context.Context, tx *gorm.DB, profileID, stageID uuid.UUID) (*domain.ProfileProgress, error) {
	var progress domain.ProfileProgress
	err := tx.WithContext(ctx).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("profile_id = ? AND stage_id = ?", profileID, stageID).
		First(&progress).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &progress, nil
}

func (r *ProgressRepository) ListByProfileAndStageIDs(ctx context.Context, profileID uuid.UUID, stageIDs []uuid.UUID) (map[uuid.UUID]domain.ProfileProgress, error) {
	result := make(map[uuid.UUID]domain.ProfileProgress, len(stageIDs))
	if len(stageIDs) == 0 {
		return result, nil
	}
	var list []domain.ProfileProgress
	err := r.db.WithContext(ctx).
		Where("profile_id = ? AND stage_id IN ?", profileID, stageIDs).
		Find(&list).Error
	if err != nil {
		return nil, err
	}
	for _, p := range list {
		result[p.StageID] = p
	}
	return result, nil
}

func (r *ProgressRepository) GetWorldStatsForProfile(ctx context.Context, profileID uuid.UUID, worldIDs []uuid.UUID) (map[uuid.UUID]WorldProgressStats, error) {
	result := make(map[uuid.UUID]WorldProgressStats, len(worldIDs))
	if len(worldIDs) == 0 {
		return result, nil
	}

	var rows []WorldProgressStats
	err := r.db.WithContext(ctx).
		Table("profile_progress pp").
		Select(`
			s.world_id AS world_id,
			SUM(CASE WHEN pp.status = 'completed' THEN 1 ELSE 0 END) AS completed_stages,
			SUM(CASE WHEN pp.status IN ('unlocked', 'completed') THEN 1 ELSE 0 END) AS unlocked_or_done,
			COALESCE(SUM(pp.stars), 0) AS earned_stars
		`).
		Joins("JOIN stages s ON s.id = pp.stage_id").
		Where("pp.profile_id = ? AND s.world_id IN ?", profileID, worldIDs).
		Group("s.world_id").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		result[row.WorldID] = row
	}
	return result, nil
}

// UnlockStageTx inserts an unlocked profile_progress row if it doesn't exist yet (keeps existing status if already unlocked/completed).
func (r *ProgressRepository) UnlockStageTx(ctx context.Context, tx *gorm.DB, profileID, stageID uuid.UUID) error {
	now := time.Now().UTC()
	progress := domain.ProfileProgress{
		ProfileID: profileID,
		StageID:   stageID,
		Status:    domain.ProgressStatusUnlocked,
		Stars:     0,
		UpdatedAt: now,
	}
	return tx.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "profile_id"}, {Name: "stage_id"}},
			DoNothing: true,
		}).
		Create(&progress).Error
}

func (r *ProgressRepository) SaveProgressTx(ctx context.Context, tx *gorm.DB, progress *domain.ProfileProgress) error {
	progress.UpdatedAt = time.Now().UTC()
	return tx.WithContext(ctx).Save(progress).Error
}

func (r *ProgressRepository) AddProfileRewardsTx(ctx context.Context, tx *gorm.DB, profileID uuid.UUID, coinsDelta, starsDelta int) error {
	return tx.WithContext(ctx).
		Model(&domain.Profile{}).
		Where("id = ?", profileID).
		Updates(map[string]any{
			"coins":       gorm.Expr("coins + ?", coinsDelta),
			"total_stars": gorm.Expr("total_stars + ?", starsDelta),
			"updated_at":  time.Now().UTC(),
		}).Error
}

// GrantItemTx inserts the item into profile_inventory or increments quantity on conflict.
func (r *ProgressRepository) GrantItemTx(ctx context.Context, tx *gorm.DB, profileID, itemID uuid.UUID) error {
	entry := domain.ProfileInventory{
		ProfileID:  profileID,
		ItemID:     itemID,
		Quantity:   1,
		IsEquipped: false,
		AcquiredAt: time.Now().UTC(),
	}
	return tx.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "profile_id"}, {Name: "item_id"}},
			DoUpdates: clause.Assignments(map[string]any{
				"quantity": gorm.Expr("profile_inventory.quantity + 1"),
			}),
		}).
		Create(&entry).Error
}

// Inventory & Item methods

func (r *ProgressRepository) ListInventoryByProfile(ctx context.Context, profileID uuid.UUID) ([]domain.ProfileInventory, error) {
	var items []domain.ProfileInventory
	err := r.db.WithContext(ctx).
		Preload("Item").
		Where("profile_id = ?", profileID).
		Order("acquired_at DESC").
		Find(&items).Error
	return items, err
}

func (r *ProgressRepository) FindInventoryEntry(ctx context.Context, tx *gorm.DB, profileID, itemID uuid.UUID) (*domain.ProfileInventory, error) {
	q := r.db
	if tx != nil {
		q = tx
	}
	var entry domain.ProfileInventory
	err := q.WithContext(ctx).
		Preload("Item").
		Where("profile_id = ? AND item_id = ?", profileID, itemID).
		First(&entry).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &entry, nil
}

func (r *ProgressRepository) UnequipCategoryTx(ctx context.Context, tx *gorm.DB, profileID uuid.UUID, category string) error {
	return tx.WithContext(ctx).Exec(`
		UPDATE profile_inventory pi
		SET is_equipped = FALSE
		FROM items i
		WHERE pi.item_id = i.id
		  AND pi.profile_id = ?
		  AND i.category = ?
		  AND pi.is_equipped = TRUE
	`, profileID, category).Error
}

func (r *ProgressRepository) SetInventoryEquippedTx(ctx context.Context, tx *gorm.DB, entryID uuid.UUID, equipped bool) error {
	return tx.WithContext(ctx).
		Model(&domain.ProfileInventory{}).
		Where("id = ?", entryID).
		Update("is_equipped", equipped).Error
}

// Items CRUD (Admin)

func (r *ProgressRepository) ListItems(ctx context.Context) ([]domain.Item, error) {
	var items []domain.Item
	err := r.db.WithContext(ctx).Order("created_at DESC").Find(&items).Error
	return items, err
}

func (r *ProgressRepository) FindItemByID(ctx context.Context, id uuid.UUID) (*domain.Item, error) {
	var item domain.Item
	err := r.db.WithContext(ctx).First(&item, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *ProgressRepository) CreateItem(ctx context.Context, item *domain.Item) error {
	return r.db.WithContext(ctx).Create(item).Error
}

func (r *ProgressRepository) UpdateItem(ctx context.Context, item *domain.Item) error {
	return r.db.WithContext(ctx).Save(item).Error
}

func (r *ProgressRepository) DeleteItem(ctx context.Context, id uuid.UUID) (int64, error) {
	res := r.db.WithContext(ctx).Delete(&domain.Item{}, "id = ?", id)
	return res.RowsAffected, res.Error
}
