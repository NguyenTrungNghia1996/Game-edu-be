package handler

import (
	"strings"

	"edu-game-be/internal/domain"
	"edu-game-be/internal/service"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

type AdminHandler struct {
	adminService *service.AdminService
}

func NewAdminHandler(adminService *service.AdminService) *AdminHandler {
	return &AdminHandler{adminService: adminService}
}

func (h *AdminHandler) GenerateUploadURL(c *fiber.Ctx) error {
	var req domain.UploadURLRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	resp, err := h.adminService.GenerateUploadURL(c.UserContext(), req)
	if err != nil {
		return handleError(c, err)
	}
	return c.Status(fiber.StatusOK).JSON(resp)
}

// --- Worlds ---

func (h *AdminHandler) ListWorlds(c *fiber.Ctx) error {
	worlds, err := h.adminService.ListWorlds(c.UserContext())
	if err != nil {
		return handleError(c, err)
	}
	return c.Status(fiber.StatusOK).JSON(fiber.Map{"worlds": worlds})
}

func (h *AdminHandler) GetWorld(c *fiber.Ctx) error {
	id, err := parseUUIDParam(c, "id")
	if err != nil {
		return handleError(c, err)
	}
	world, err := h.adminService.GetWorld(c.UserContext(), id)
	if err != nil {
		return handleError(c, err)
	}
	return c.Status(fiber.StatusOK).JSON(world)
}

func (h *AdminHandler) CreateWorld(c *fiber.Ctx) error {
	var req domain.CreateWorldRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	world, err := h.adminService.CreateWorld(c.UserContext(), req)
	if err != nil {
		return handleError(c, err)
	}
	return c.Status(fiber.StatusCreated).JSON(world)
}

func (h *AdminHandler) UpdateWorld(c *fiber.Ctx) error {
	id, err := parseUUIDParam(c, "id")
	if err != nil {
		return handleError(c, err)
	}
	var req domain.UpdateWorldRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	world, err := h.adminService.UpdateWorld(c.UserContext(), id, req)
	if err != nil {
		return handleError(c, err)
	}
	return c.Status(fiber.StatusOK).JSON(world)
}

func (h *AdminHandler) DeleteWorld(c *fiber.Ctx) error {
	id, err := parseUUIDParam(c, "id")
	if err != nil {
		return handleError(c, err)
	}
	if err := h.adminService.DeleteWorld(c.UserContext(), id); err != nil {
		return handleError(c, err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// --- Stages ---

func (h *AdminHandler) ListStages(c *fiber.Ctx) error {
	var worldID *uuid.UUID
	if raw := strings.TrimSpace(c.Query("world_id")); raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid world_id query param"})
		}
		worldID = &id
	}
	stages, err := h.adminService.ListStages(c.UserContext(), worldID)
	if err != nil {
		return handleError(c, err)
	}
	return c.Status(fiber.StatusOK).JSON(fiber.Map{"stages": stages})
}

func (h *AdminHandler) GetStage(c *fiber.Ctx) error {
	id, err := parseUUIDParam(c, "id")
	if err != nil {
		return handleError(c, err)
	}
	stage, err := h.adminService.GetStage(c.UserContext(), id)
	if err != nil {
		return handleError(c, err)
	}
	return c.Status(fiber.StatusOK).JSON(stage)
}

func (h *AdminHandler) CreateStage(c *fiber.Ctx) error {
	var req domain.CreateStageRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	stage, err := h.adminService.CreateStage(c.UserContext(), req)
	if err != nil {
		return handleError(c, err)
	}
	return c.Status(fiber.StatusCreated).JSON(stage)
}

func (h *AdminHandler) UpdateStage(c *fiber.Ctx) error {
	id, err := parseUUIDParam(c, "id")
	if err != nil {
		return handleError(c, err)
	}
	var req domain.UpdateStageRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	stage, err := h.adminService.UpdateStage(c.UserContext(), id, req)
	if err != nil {
		return handleError(c, err)
	}
	return c.Status(fiber.StatusOK).JSON(stage)
}

func (h *AdminHandler) DeleteStage(c *fiber.Ctx) error {
	id, err := parseUUIDParam(c, "id")
	if err != nil {
		return handleError(c, err)
	}
	if err := h.adminService.DeleteStage(c.UserContext(), id); err != nil {
		return handleError(c, err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// --- Challenges ---

func (h *AdminHandler) ListChallenges(c *fiber.Ctx) error {
	var stageID *uuid.UUID
	if raw := strings.TrimSpace(c.Query("stage_id")); raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid stage_id query param"})
		}
		stageID = &id
	}
	challenges, err := h.adminService.ListChallenges(c.UserContext(), stageID)
	if err != nil {
		return handleError(c, err)
	}
	return c.Status(fiber.StatusOK).JSON(fiber.Map{"challenges": challenges})
}

func (h *AdminHandler) GetChallenge(c *fiber.Ctx) error {
	id, err := parseUUIDParam(c, "id")
	if err != nil {
		return handleError(c, err)
	}
	challenge, err := h.adminService.GetChallenge(c.UserContext(), id)
	if err != nil {
		return handleError(c, err)
	}
	return c.Status(fiber.StatusOK).JSON(challenge)
}

func (h *AdminHandler) CreateChallenge(c *fiber.Ctx) error {
	var req domain.CreateChallengeRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	challenge, err := h.adminService.CreateChallenge(c.UserContext(), req)
	if err != nil {
		return handleError(c, err)
	}
	return c.Status(fiber.StatusCreated).JSON(challenge)
}

func (h *AdminHandler) UpdateChallenge(c *fiber.Ctx) error {
	id, err := parseUUIDParam(c, "id")
	if err != nil {
		return handleError(c, err)
	}
	var req domain.UpdateChallengeRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	challenge, err := h.adminService.UpdateChallenge(c.UserContext(), id, req)
	if err != nil {
		return handleError(c, err)
	}
	return c.Status(fiber.StatusOK).JSON(challenge)
}

func (h *AdminHandler) DeleteChallenge(c *fiber.Ctx) error {
	id, err := parseUUIDParam(c, "id")
	if err != nil {
		return handleError(c, err)
	}
	if err := h.adminService.DeleteChallenge(c.UserContext(), id); err != nil {
		return handleError(c, err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// --- Items ---

func (h *AdminHandler) ListItems(c *fiber.Ctx) error {
	items, err := h.adminService.ListItems(c.UserContext())
	if err != nil {
		return handleError(c, err)
	}
	return c.Status(fiber.StatusOK).JSON(fiber.Map{"items": items})
}

func (h *AdminHandler) GetItem(c *fiber.Ctx) error {
	id, err := parseUUIDParam(c, "id")
	if err != nil {
		return handleError(c, err)
	}
	item, err := h.adminService.GetItem(c.UserContext(), id)
	if err != nil {
		return handleError(c, err)
	}
	return c.Status(fiber.StatusOK).JSON(item)
}

func (h *AdminHandler) CreateItem(c *fiber.Ctx) error {
	var req domain.CreateItemRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	item, err := h.adminService.CreateItem(c.UserContext(), req)
	if err != nil {
		return handleError(c, err)
	}
	return c.Status(fiber.StatusCreated).JSON(item)
}

func (h *AdminHandler) UpdateItem(c *fiber.Ctx) error {
	id, err := parseUUIDParam(c, "id")
	if err != nil {
		return handleError(c, err)
	}
	var req domain.UpdateItemRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	item, err := h.adminService.UpdateItem(c.UserContext(), id, req)
	if err != nil {
		return handleError(c, err)
	}
	return c.Status(fiber.StatusOK).JSON(item)
}

func (h *AdminHandler) DeleteItem(c *fiber.Ctx) error {
	id, err := parseUUIDParam(c, "id")
	if err != nil {
		return handleError(c, err)
	}
	if err := h.adminService.DeleteItem(c.UserContext(), id); err != nil {
		return handleError(c, err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}
