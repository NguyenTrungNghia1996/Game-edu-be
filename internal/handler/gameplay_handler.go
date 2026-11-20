package handler

import (
	"edu-game-be/internal/domain"
	"edu-game-be/internal/middleware"
	"edu-game-be/internal/service"

	"github.com/gofiber/fiber/v2"
)

type GameplayHandler struct {
	gameplayService *service.GameplayService
}

func NewGameplayHandler(gameplayService *service.GameplayService) *GameplayHandler {
	return &GameplayHandler{gameplayService: gameplayService}
}

func (h *GameplayHandler) ListWorlds(c *fiber.Ctx) error {
	profileID := middleware.GetProfileID(c)
	worlds, err := h.gameplayService.ListWorlds(c.UserContext(), profileID)
	if err != nil {
		return handleError(c, err)
	}
	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"worlds": worlds,
	})
}

func (h *GameplayHandler) GetWorldMap(c *fiber.Ctx) error {
	worldID, err := parseUUIDParam(c, "worldId")
	if err != nil {
		return handleError(c, err)
	}
	profileID := middleware.GetProfileID(c)

	worldMap, err := h.gameplayService.GetWorldMap(c.UserContext(), profileID, worldID)
	if err != nil {
		return handleError(c, err)
	}
	return c.Status(fiber.StatusOK).JSON(worldMap)
}

func (h *GameplayHandler) StartStage(c *fiber.Ctx) error {
	stageID, err := parseUUIDParam(c, "stageId")
	if err != nil {
		return handleError(c, err)
	}
	profileID := middleware.GetProfileID(c)

	resp, err := h.gameplayService.StartStage(c.UserContext(), profileID, stageID)
	if err != nil {
		return handleError(c, err)
	}
	return c.Status(fiber.StatusOK).JSON(resp)
}

func (h *GameplayHandler) SubmitAnswer(c *fiber.Ctx) error {
	stageID, err := parseUUIDParam(c, "stageId")
	if err != nil {
		return handleError(c, err)
	}
	var req domain.SubmitAnswerRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	profileID := middleware.GetProfileID(c)

	resp, err := h.gameplayService.SubmitAnswer(c.UserContext(), profileID, stageID, req)
	if err != nil {
		return handleError(c, err)
	}
	return c.Status(fiber.StatusOK).JSON(resp)
}
