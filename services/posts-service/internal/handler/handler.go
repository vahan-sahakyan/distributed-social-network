package handler

import (
	"github.com/gofiber/fiber/v2"
	"github.com/vahan/distributed-social-network/posts-service/internal/model"
	"github.com/vahan/distributed-social-network/posts-service/internal/service"
)

type Handler struct {
	svc *service.Service
}

func New(svc *service.Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) CreatePost(c *fiber.Ctx) error {
	var req model.CreatePostRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	post, err := h.svc.CreatePost(c.Context(), &req)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to create post"})
	}

	return c.Status(fiber.StatusCreated).JSON(post)
}

func (h *Handler) GetPost(c *fiber.Ctx) error {
	id := c.Params("id")

	post, err := h.svc.GetPost(c.Context(), id)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "post not found"})
	}

	return c.JSON(post)
}
