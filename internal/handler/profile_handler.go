package handler

import (
	"edu-game-be/internal/domain"
	"edu-game-be/internal/middleware"
	"edu-game-be/internal/service"

	"github.com/gofiber/fiber/v2"
)

type ProfileHandler struct {
	authService *service.AuthService
}

func NewProfileHandler(authService *service.AuthService) *ProfileHandler {
	return &ProfileHandler{authService: authService}
}

func (h *ProfileHandler) List(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	profiles, err := h.authService.ListProfiles(c.UserContext(), userID)
	if err != nil {
		return handleError(c, err)
	}
	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"profiles": profiles,
	})
}

func (h *ProfileHandler) Create(c *fiber.Ctx) error {
	var req domain.CreateProfileRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	userID := middleware.GetUserID(c)
	profile, err := h.authService.CreateProfile(c.UserContext(), userID, req)
	if err != nil {
		return handleError(c, err)
	}
	return c.Status(fiber.StatusCreated).JSON(profile)
}
