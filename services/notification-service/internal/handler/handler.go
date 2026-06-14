package handler

import (
	"github.com/gofiber/fiber/v2"
	"github.com/vahan/distributed-social-network/notification-service/internal/service"
)

type Handler struct {
	svc *service.Service
}

func New(svc *service.Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) GetNotifications(c *fiber.Ctx) error {
	userID := c.Params("user_id")

	notifications, err := h.svc.GetByUserID(c.Context(), userID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to get notifications"})
	}

	return c.JSON(fiber.Map{"notifications": notifications})
}
