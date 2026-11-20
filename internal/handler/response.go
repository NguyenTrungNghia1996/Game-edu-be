package handler

import (
	"errors"
	"strings"

	"edu-game-be/internal/service"
	"edu-game-be/internal/storage"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

func handleError(c *fiber.Ctx, err error) error {
	switch {
	case errors.Is(err, service.ErrValidation):
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	case errors.Is(err, service.ErrAttemptExpired):
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	case errors.Is(err, service.ErrUnauthorized):
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": err.Error()})
	case errors.Is(err, service.ErrForbidden):
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": err.Error()})
	case errors.Is(err, service.ErrNotFound):
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": err.Error()})
	case errors.Is(err, service.ErrConflict):
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": err.Error()})
	case errors.Is(err, storage.ErrR2NotConfigured):
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": err.Error()})
	default:
		msg := err.Error()
		if strings.Contains(msg, "SQLSTATE 23505") || strings.Contains(msg, "duplicate key") {
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": "resource already exists"})
		}
		if strings.Contains(msg, "SQLSTATE 23503") || strings.Contains(msg, "foreign key constraint") {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "referenced resource does not exist"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "internal server error"})
	}
}

func parseUUIDParam(c *fiber.Ctx, paramName string) (uuid.UUID, error) {
	raw := strings.TrimSpace(c.Params(paramName))
	id, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, service.WrapErr(service.ErrValidation, "invalid "+paramName)
	}
	return id, nil
}
