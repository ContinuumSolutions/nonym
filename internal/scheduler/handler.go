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

// @Summary      Get scheduler status
// @Tags         scheduler
// @Produce      json
// @Success      200  {object}  scheduler.Status
// @Router       /scheduler/status [get]
func (h *Handler) status(c *fiber.Ctx) error {
	return c.JSON(h.scheduler.GetStatus())
}

// @Summary      Trigger immediate pipeline cycle
// @Tags         scheduler
// @Produce      json
// @Success      200  {object}  scheduler.RunNowResponse
// @Failure      500  {object}  map[string]interface{}
// @Router       /scheduler/run-now [post]
func (h *Handler) runNow(c *fiber.Ctx) error {
	result, err := h.scheduler.RunNow(c.Context())
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	return c.JSON(result)
}
