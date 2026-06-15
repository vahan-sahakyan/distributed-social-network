package handler

import (
	"github.com/gofiber/fiber/v2"
	"github.com/vahan-sahakyan/distributed-social-network/likes-service/internal/model"
	"github.com/vahan-sahakyan/distributed-social-network/likes-service/internal/service"
)

type Handler struct {
	svc *service.Service
}

func New(svc *service.Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) CreateLike(c *fiber.Ctx) error {
	var req model.CreateLikeRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	like, err := h.svc.CreateLike(c.Context(), &req)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to create like"})
	}

	return c.Status(fiber.StatusCreated).JSON(like)
}
