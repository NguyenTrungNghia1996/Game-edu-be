package middleware

import (
	"strings"

	"edu-game-be/internal/repository"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

func RequireProfile(authRepo *repository.AuthRepository) fiber.Handler {
	return func(c *fiber.Ctx) error {
		rawProfileID := strings.TrimSpace(c.Get("X-Profile-ID"))
		if rawProfileID == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "X-Profile-ID header is required",
			})
		}

		profileID, err := uuid.Parse(rawProfileID)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "invalid X-Profile-ID header",
			})
		}

		userID := GetUserID(c)
		profile, err := authRepo.FindProfileByIDAndUserID(c.UserContext(), profileID, userID)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "failed to validate profile ownership",
			})
		}
		if profile == nil {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error": "profile does not belong to current user",
			})
		}

		c.Locals(LocalProfileID, profileID)
		return c.Next()
	}
}
