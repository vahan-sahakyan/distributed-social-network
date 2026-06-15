package handler

import (
	"github.com/gofiber/fiber/v2"
	"github.com/vahan-sahakyan/distributed-social-network/cache-rebuilder-service/internal/service"
)

type Handler struct {
	svc *service.Service
}

func New(svc *service.Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) TriggerRebuild(c *fiber.Ctx) error {
	if err := h.svc.RebuildCache(c.Context()); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"status": "rebuild complete"})
}
