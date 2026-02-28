package scheduler

import (
	"github.com/gofiber/fiber/v2"
)

type Handler struct {
	scheduler *Scheduler
}

func NewHandler(s *Scheduler) *Handler { return &Handler{scheduler: s} }

func (h *Handler) RegisterRoutes(r fiber.Router) {
	r.Get("/scheduler/status", h.status)
	r.Post("/scheduler/run-now", h.runNow)
}

func (h *Handler) status(c *fiber.Ctx) error {
	return c.JSON(h.scheduler.GetStatus())
}

// runNow triggers an immediate cycle and returns the pipeline result.
// Useful for testing without waiting for the next tick.
func (h *Handler) runNow(c *fiber.Ctx) error {
	result, err := h.scheduler.RunNow(c.Context())
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	return c.JSON(result)
}
