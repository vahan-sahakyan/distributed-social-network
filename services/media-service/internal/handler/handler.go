package handler

import (
	"github.com/gofiber/fiber/v2"
	"github.com/vahan/distributed-social-network/media-service/internal/service"
)

type Handler struct {
	svc *service.Service
}

func New(svc *service.Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) Upload(c *fiber.Ctx) error {
	file, err := c.FormFile("file")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "file required"})
	}

	f, err := file.Open()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to read file"})
	}
	defer f.Close()

	image, err := h.svc.Upload(c.Context(), f, file.Size, file.Header.Get("Content-Type"))
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to upload"})
	}

	return c.Status(fiber.StatusCreated).JSON(image)
}

func (h *Handler) Get(c *fiber.Ctx) error {
	id := c.Params("id")

	url, err := h.svc.GetURL(c.Context(), id)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "image not found"})
	}

	return c.JSON(fiber.Map{"id": id, "url": url})
}
