package repository

import (
	"context"
	"errors"

	"edu-game-be/internal/domain"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type AuthRepository struct {
	db *gorm.DB
}

func NewAuthRepository(db *gorm.DB) *AuthRepository {
	return &AuthRepository{db: db}
}

func (r *AuthRepository) CreateUser(ctx context.Context, user *domain.User) error {
	return r.db.WithContext(ctx).Create(user).Error
}

func (r *AuthRepository) FindUserByEmail(ctx context.Context, email string) (*domain.User, error) {
	var user domain.User
	err := r.db.WithContext(ctx).Where("LOWER(email) = LOWER(?)", email).First(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *AuthRepository) FindUserByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	var user domain.User
	err := r.db.WithContext(ctx).First(&user, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *AuthRepository) ListProfilesByUserID(ctx context.Context, userID uuid.UUID) ([]domain.Profile, error) {
	var profiles []domain.Profile
	err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("created_at ASC").
		Find(&profiles).Error
	return profiles, err
}

func (r *AuthRepository) FindProfileByIDAndUserID(ctx context.Context, profileID, userID uuid.UUID) (*domain.Profile, error) {
	var profile domain.Profile
	err := r.db.WithContext(ctx).
		Where("id = ? AND user_id = ?", profileID, userID).
		First(&profile).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &profile, nil
}

func (r *AuthRepository) CreateProfileTx(ctx context.Context, tx *gorm.DB, profile *domain.Profile) error {
	return tx.WithContext(ctx).Create(profile).Error
}

func (r *AuthRepository) DB() *gorm.DB {
	return r.db
}
