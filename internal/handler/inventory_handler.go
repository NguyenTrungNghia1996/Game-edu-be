package handler

import (
	"edu-game-be/internal/domain"
	"edu-game-be/internal/middleware"
	"edu-game-be/internal/service"

	"github.com/gofiber/fiber/v2"
)

type InventoryHandler struct {
	inventoryService *service.InventoryService
}

func NewInventoryHandler(inventoryService *service.InventoryService) *InventoryHandler {
	return &InventoryHandler{inventoryService: inventoryService}
}

func (h *InventoryHandler) List(c *fiber.Ctx) error {
	profileID := middleware.GetProfileID(c)
	items, err := h.inventoryService.ListInventory(c.UserContext(), profileID)
	if err != nil {
		return handleError(c, err)
	}
	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"items": items,
	})
}

func (h *InventoryHandler) Equip(c *fiber.Ctx) error {
	itemID, err := parseUUIDParam(c, "itemId")
	if err != nil {
		return handleError(c, err)
	}

	var req domain.EquipItemRequest
	if len(c.Body()) > 0 {
		if err := c.BodyParser(&req); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
		}
	}

	profileID := middleware.GetProfileID(c)
	updated, err := h.inventoryService.EquipItem(c.UserContext(), profileID, itemID, req)
	if err != nil {
		return handleError(c, err)
	}
	return c.Status(fiber.StatusOK).JSON(updated)
}
