package execution

import (
	"strconv"

	"github.com/egokernel/ek1/internal/activities"
	"github.com/gofiber/fiber/v2"
)

// Handler serves the approval queue routes under /brain/queue.
type Handler struct {
	engine *Engine
	queue  *Store
	events *activities.Store
}

func NewHandler(engine *Engine, queue *Store, events *activities.Store) *Handler {
	return &Handler{engine: engine, queue: queue, events: events}
}

func (h *Handler) RegisterRoutes(app *fiber.App) {
	app.Get("/brain/queue", h.listPending)
	app.Post("/brain/queue/:id/approve", h.approve)
	app.Post("/brain/queue/:id/reject", h.reject)
}

// GET /brain/queue — list all pending queue entries
func (h *Handler) listPending(c *fiber.Ctx) error {
	entries, err := h.queue.ListPending()
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	if entries == nil {
		entries = []QueueEntry{}
	}
	return c.JSON(entries)
}

// POST /brain/queue/:id/approve — execute the action and mark event as Automated
func (h *Handler) approve(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid id")
	}

	entry, err := h.queue.Get(id)
	if err != nil {
		return fiber.NewError(fiber.StatusNotFound, err.Error())
	}

	if execErr := h.engine.ApproveAndExecute(c.Context(), id); execErr != nil {
		return fiber.NewError(fiber.StatusInternalServerError, execErr.Error())
	}

	// Update the linked event to Decision=Automated.
	if entry.EventID > 0 {
		_, _ = h.events.UpdateDecision(entry.EventID, activities.Automated)
	}

	updated, _ := h.queue.Get(id)
	return c.JSON(updated)
}

// POST /brain/queue/:id/reject — mark the entry rejected and event as Declined
func (h *Handler) reject(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid id")
	}

	entry, err := h.queue.Get(id)
	if err != nil {
		return fiber.NewError(fiber.StatusNotFound, err.Error())
	}

	if setErr := h.queue.SetStatus(id, "rejected"); setErr != nil {
		return fiber.NewError(fiber.StatusInternalServerError, setErr.Error())
	}

	// Update the linked event to Decision=Declined.
	if entry.EventID > 0 {
		_, _ = h.events.UpdateDecision(entry.EventID, activities.Declined)
	}

	updated, _ := h.queue.Get(id)
	return c.JSON(updated)
}
