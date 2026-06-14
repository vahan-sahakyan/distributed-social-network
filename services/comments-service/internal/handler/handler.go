package handler

import (
	"github.com/gofiber/fiber/v2"
	"github.com/vahan/distributed-social-network/comments-service/internal/model"
	"github.com/vahan/distributed-social-network/comments-service/internal/service"
)

type Handler struct {
	svc *service.Service
}

func New(svc *service.Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) CreateComment(c *fiber.Ctx) error {
	var req model.CreateCommentRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	comment, err := h.svc.CreateComment(c.Context(), &req)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to create comment"})
	}

	return c.Status(fiber.StatusCreated).JSON(comment)
}

func (h *Handler) GetComments(c *fiber.Ctx) error {
	entityID := c.Params("entity_id")

	comments, err := h.svc.GetCommentsByEntity(c.Context(), entityID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to get comments"})
	}

	return c.JSON(comments)
}
