package service

import (
	"context"

	"edu-game-be/internal/domain"
	"edu-game-be/internal/repository"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type InventoryService struct {
	progressRepo *repository.ProgressRepository
}

func NewInventoryService(progressRepo *repository.ProgressRepository) *InventoryService {
	return &InventoryService{progressRepo: progressRepo}
}

func (s *InventoryService) ListInventory(ctx context.Context, profileID uuid.UUID) ([]domain.ProfileInventory, error) {
	return s.progressRepo.ListInventoryByProfile(ctx, profileID)
}

func (s *InventoryService) EquipItem(ctx context.Context, profileID, itemID uuid.UUID, req domain.EquipItemRequest) (*domain.ProfileInventory, error) {
	var updated *domain.ProfileInventory

	err := s.progressRepo.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		entry, err := s.progressRepo.FindInventoryEntry(ctx, tx, profileID, itemID)
		if err != nil {
			return err
		}
		if entry == nil {
			return WrapErr(ErrNotFound, "item not found in profile inventory")
		}

		targetEquip := !entry.IsEquipped
		if req.IsEquipped != nil {
			targetEquip = *req.IsEquipped
		}

		if targetEquip {
			// Ensure only one item per category is equipped at a time.
			if err := s.progressRepo.UnequipCategoryTx(ctx, tx, profileID, entry.Item.Category); err != nil {
				return err
			}
		}

		if err := s.progressRepo.SetInventoryEquippedTx(ctx, tx, entry.ID, targetEquip); err != nil {
			return err
		}

		entry.IsEquipped = targetEquip
		updated = entry
		return nil
	})
	if err != nil {
		return nil, err
	}

	return updated, nil
}
