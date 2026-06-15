package handler

import (
	"github.com/gofiber/fiber/v2"
	"github.com/vahan-sahakyan/distributed-social-network/users-service/internal/model"
	"github.com/vahan-sahakyan/distributed-social-network/users-service/internal/service"
)

type Handler struct {
	svc *service.Service
}

func New(svc *service.Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) CreateUser(c *fiber.Ctx) error {
	var req model.CreateUserRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	user, err := h.svc.CreateUser(c.Context(), &req)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to create user"})
	}

	return c.Status(fiber.StatusCreated).JSON(user)
}

func (h *Handler) GetUser(c *fiber.Ctx) error {
	id := c.Params("id")

	user, err := h.svc.GetUser(c.Context(), id)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "user not found"})
	}

	return c.JSON(user)
}

func (h *Handler) FollowUser(c *fiber.Ctx) error {
	var req model.FollowRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	targetID := c.Params("id")
	if err := h.svc.Follow(c.Context(), req.FollowerID, targetID); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to follow"})
	}

	return c.Status(fiber.StatusNoContent).Send(nil)
}

func (h *Handler) GetFollowers(c *fiber.Ctx) error {
	id := c.Params("id")

	followers, err := h.svc.GetFollowers(c.Context(), id)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to get followers"})
	}

	return c.JSON(fiber.Map{"followers": followers})
}

func (h *Handler) GetFollowing(c *fiber.Ctx) error {
	id := c.Params("id")

	following, err := h.svc.GetFollowing(c.Context(), id)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to get following"})
	}

	return c.JSON(fiber.Map{"following": following})
}
