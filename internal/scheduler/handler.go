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

// @Summary      Trigger immediate pipeline cycle (async)
// @Description  Starts a sync cycle in the background and returns immediately. Poll GET /scheduler/status for completion (running=false + last_result populated).
// @Tags         scheduler
// @Produce      json
// @Success      202  {object}  map[string]interface{}
// @Failure      409  {object}  map[string]interface{}
// @Router       /scheduler/run-now [post]
func (h *Handler) runNow(c *fiber.Ctx) error {
	if !h.scheduler.RunNowAsync() {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{
			"status":  "already_running",
			"message": "a pipeline cycle is already in progress — poll GET /scheduler/status for completion",
		})
	}
	return c.Status(fiber.StatusAccepted).JSON(fiber.Map{
		"status":  "started",
		"message": "pipeline cycle started — poll GET /scheduler/status for results",
	})
}
