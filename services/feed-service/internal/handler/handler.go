package handler

import (
	"github.com/gofiber/fiber/v2"
	"github.com/vahan/distributed-social-network/feed-service/internal/service"
)

type Handler struct {
	svc *service.Service
}

func New(svc *service.Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) GetHomeFeed(c *fiber.Ctx) error {
	// TODO: extract user_id from auth context
	userID := c.Query("user_id")
	if userID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "user_id required"})
	}

	feed, err := h.svc.GetHomeFeed(c.Context(), userID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to get feed"})
	}

	return c.JSON(fiber.Map{"posts": feed})
}

func (h *Handler) GetUserFeed(c *fiber.Ctx) error {
	userID := c.Params("user_id")

	feed, err := h.svc.GetUserFeed(c.Context(), userID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to get feed"})
	}

	return c.JSON(fiber.Map{"posts": feed})
}
