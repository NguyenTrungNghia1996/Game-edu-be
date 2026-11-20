package handler

import (
	"context"
	"time"

	"edu-game-be/internal/database"

	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

type HealthHandler struct {
	db    *gorm.DB
	redis *redis.Client
}

func NewHealthHandler(db *gorm.DB, redisClient *redis.Client) *HealthHandler {
	return &HealthHandler{db: db, redis: redisClient}
}

func (h *HealthHandler) Health(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(c.UserContext(), 2*time.Second)
	defer cancel()

	pgStatus := "up"
	redisStatus := "up"
	healthy := true

	if err := database.PingPostgres(ctx, h.db); err != nil {
		pgStatus = "down"
		healthy = false
	}
	if err := h.redis.Ping(ctx).Err(); err != nil {
		redisStatus = "down"
		healthy = false
	}

	statusCode := fiber.StatusOK
	overall := "ok"
	if !healthy {
		statusCode = fiber.StatusServiceUnavailable
		overall = "degraded"
	}

	return c.Status(statusCode).JSON(fiber.Map{
		"status": overall,
		"services": fiber.Map{
			"postgres": pgStatus,
			"redis":    redisStatus,
		},
	})
}
