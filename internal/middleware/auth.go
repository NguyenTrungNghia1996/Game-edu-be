package middleware

import (
	"strings"

	"edu-game-be/internal/domain"
	"edu-game-be/internal/repository"
	"edu-game-be/internal/service"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

const (
	LocalUserID    = "user_id"
	LocalUserRole  = "user_role"
	LocalProfileID = "profile_id"
)

func RequireAuth(jwtSecret string, authRepo *repository.AuthRepository) fiber.Handler {
	return func(c *fiber.Ctx) error {
		authHeader := strings.TrimSpace(c.Get("Authorization"))
		if authHeader == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "missing Authorization header",
			})
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || strings.TrimSpace(parts[1]) == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "invalid Authorization header format",
			})
		}

		claims, err := service.ParseToken(strings.TrimSpace(parts[1]), jwtSecret, "access")
		if err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "invalid or expired access token",
			})
		}

		userID, err := uuid.Parse(claims.UserID)
		if err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "invalid user id in token",
			})
		}

		if authRepo != nil {
			user, err := authRepo.FindUserByID(c.UserContext(), userID)
			if err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
					"error": "failed to verify user session",
				})
			}
			if user == nil {
				return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
					"error": "user no longer exists, please login again",
				})
			}
			c.Locals(LocalUserRole, user.Role)
		} else {
			c.Locals(LocalUserRole, claims.Role)
		}

		c.Locals(LocalUserID, userID)
		return c.Next()
	}
}

func RequireAdmin() fiber.Handler {
	return func(c *fiber.Ctx) error {
		role, _ := c.Locals(LocalUserRole).(string)
		if role != domain.RoleAdmin {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error": "admin role required",
			})
		}
		return c.Next()
	}
}

func GetUserID(c *fiber.Ctx) uuid.UUID {
	id, _ := c.Locals(LocalUserID).(uuid.UUID)
	return id
}

func GetProfileID(c *fiber.Ctx) uuid.UUID {
	id, _ := c.Locals(LocalProfileID).(uuid.UUID)
	return id
}
