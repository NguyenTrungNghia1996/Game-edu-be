package service

import (
	"context"
	"fmt"
	"net/mail"
	"strings"
	"time"

	"edu-game-be/internal/config"
	"edu-game-be/internal/domain"
	"edu-game-be/internal/repository"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type TokenClaims struct {
	UserID    string `json:"sub"`
	Role      string `json:"role"`
	TokenType string `json:"token_type"` // "access" | "refresh"
	jwt.RegisteredClaims
}

type AuthService struct {
	authRepo     *repository.AuthRepository
	worldRepo    *repository.WorldRepository
	stageRepo    *repository.StageRepository
	progressRepo *repository.ProgressRepository
	cfg          *config.Config
}

func NewAuthService(
	authRepo *repository.AuthRepository,
	worldRepo *repository.WorldRepository,
	stageRepo *repository.StageRepository,
	progressRepo *repository.ProgressRepository,
	cfg *config.Config,
) *AuthService {
	return &AuthService{
		authRepo:     authRepo,
		worldRepo:    worldRepo,
		stageRepo:    stageRepo,
		progressRepo: progressRepo,
		cfg:          cfg,
	}
}

func (s *AuthService) Register(ctx context.Context, req domain.RegisterRequest) (*domain.AuthResponse, error) {
	email := strings.TrimSpace(strings.ToLower(req.Email))
	if _, err := mail.ParseAddress(email); err != nil {
		return nil, WrapErr(ErrValidation, "invalid email address")
	}
	if len(req.Password) < 6 {
		return nil, WrapErr(ErrValidation, "password must be at least 6 characters")
	}

	existing, err := s.authRepo.FindUserByEmail(ctx, email)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, WrapErr(ErrConflict, "email already registered")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	user := &domain.User{
		Email:        email,
		PasswordHash: string(hash),
		Role:         domain.RoleParent,
	}
	if err := s.authRepo.CreateUser(ctx, user); err != nil {
		return nil, err
	}

	return s.buildAuthResponse(*user)
}

func (s *AuthService) Login(ctx context.Context, req domain.LoginRequest) (*domain.AuthResponse, error) {
	email := strings.TrimSpace(strings.ToLower(req.Email))
	if email == "" || req.Password == "" {
		return nil, WrapErr(ErrValidation, "email and password are required")
	}

	user, err := s.authRepo.FindUserByEmail(ctx, email)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, WrapErr(ErrUnauthorized, "invalid email or password")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		return nil, WrapErr(ErrUnauthorized, "invalid email or password")
	}

	return s.buildAuthResponse(*user)
}

func (s *AuthService) RefreshToken(ctx context.Context, refreshToken string) (*domain.AuthResponse, error) {
	if strings.TrimSpace(refreshToken) == "" {
		return nil, WrapErr(ErrValidation, "refresh_token is required")
	}

	claims, err := ParseToken(refreshToken, s.cfg.JWTSecret, "refresh")
	if err != nil {
		return nil, WrapErr(ErrUnauthorized, "invalid or expired refresh token")
	}

	userID, err := uuid.Parse(claims.UserID)
	if err != nil {
		return nil, WrapErr(ErrUnauthorized, "invalid user id in token")
	}

	user, err := s.authRepo.FindUserByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, WrapErr(ErrUnauthorized, "user not found")
	}

	return s.buildAuthResponse(*user)
}

func (s *AuthService) ListProfiles(ctx context.Context, userID uuid.UUID) ([]domain.Profile, error) {
	return s.authRepo.ListProfilesByUserID(ctx, userID)
}

func (s *AuthService) CreateProfile(ctx context.Context, userID uuid.UUID, req domain.CreateProfileRequest) (*domain.Profile, error) {
	nickname := strings.TrimSpace(req.Nickname)
	if nickname == "" {
		return nil, WrapErr(ErrValidation, "nickname is required")
	}
	if len([]rune(nickname)) > 100 {
		return nil, WrapErr(ErrValidation, "nickname must be 100 characters or fewer")
	}

	profile := &domain.Profile{
		UserID:    userID,
		Nickname:  nickname,
		AvatarURL: strings.TrimSpace(req.AvatarURL),
	}

	err := s.authRepo.DB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := s.authRepo.CreateProfileTx(ctx, tx, profile); err != nil {
			return err
		}

		// Automatically unlock World 1 - Stage 1 if available.
		firstWorld, err := s.worldRepo.FindFirstActive(ctx, tx)
		if err != nil {
			return err
		}
		if firstWorld == nil {
			return nil
		}

		firstStage, err := s.stageRepo.FindFirstInWorld(ctx, tx, firstWorld.ID)
		if err != nil {
			return err
		}
		if firstStage == nil {
			return nil
		}

		return s.progressRepo.UnlockStageTx(ctx, tx, profile.ID, firstStage.ID)
	})
	if err != nil {
		return nil, err
	}

	return profile, nil
}

func (s *AuthService) buildAuthResponse(user domain.User) (*domain.AuthResponse, error) {
	accessToken, err := s.generateToken(user, "access", time.Duration(s.cfg.JWTExpireHours)*time.Hour)
	if err != nil {
		return nil, err
	}
	refreshToken, err := s.generateToken(user, "refresh", time.Duration(s.cfg.JWTRefreshExpireHours)*time.Hour)
	if err != nil {
		return nil, err
	}
	return &domain.AuthResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		User:         user,
	}, nil
}

func (s *AuthService) generateToken(user domain.User, tokenType string, ttl time.Duration) (string, error) {
	now := time.Now().UTC()
	claims := TokenClaims{
		UserID:    user.ID.String(),
		Role:      user.Role,
		TokenType: tokenType,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   user.ID.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(s.cfg.JWTSecret))
	if err != nil {
		return "", fmt.Errorf("sign jwt: %w", err)
	}
	return signed, nil
}

func ParseToken(tokenString, secret, expectedType string) (*TokenClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &TokenClaims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(secret), nil
	})
	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*TokenClaims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token claims")
	}
	if expectedType != "" && claims.TokenType != expectedType {
		return nil, fmt.Errorf("unexpected token type: %s", claims.TokenType)
	}
	return claims, nil
}
