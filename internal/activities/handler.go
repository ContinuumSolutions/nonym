package activities

import (
	"errors"
	"strconv"

	"github.com/gofiber/fiber/v2"
)

type Handler struct {
	store *Store
}

func NewHandler(store *Store) *Handler {
	return &Handler{store: store}
}

func (h *Handler) RegisterRoutes(r fiber.Router) {
	r.Get("/activities/events", h.list)
	r.Get("/activities/events/:id", h.get)
	r.Put("/activities/events/:id/read", h.toggleRead)
}

// @Summary      List events
// @Tags         activities
// @Produce      json
// @Success      200  {array}   activities.Event
// @Failure      500  {object}  map[string]interface{}
// @Router       /activities/events [get]
func (h *Handler) list(c *fiber.Ctx) error {
	events, err := h.store.List()
	if err != nil {
		return err
	}
	if events == nil {
		events = []Event{}
	}
	return c.JSON(events)
}

// @Summary      Get event by ID
// @Tags         activities
// @Produce      json
// @Param        id   path      int  true  "Event ID"
// @Success      200  {object}  activities.Event
// @Failure      400  {object}  map[string]interface{}
// @Failure      404  {object}  map[string]interface{}
// @Failure      500  {object}  map[string]interface{}
// @Router       /activities/events/{id} [get]
func (h *Handler) get(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid id"})
	}
	event, err := h.store.Get(id)
	if errors.Is(err, ErrNotFound) {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": err.Error()})
	}
	if err != nil {
		return err
	}
	return c.JSON(event)
}

// @Summary      Toggle read status
// @Tags         activities
// @Produce      json
// @Param        id   path      int  true  "Event ID"
// @Success      200  {object}  activities.Event
// @Failure      400  {object}  map[string]interface{}
// @Failure      404  {object}  map[string]interface{}
// @Failure      500  {object}  map[string]interface{}
// @Router       /activities/events/{id}/read [put]
func (h *Handler) toggleRead(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid id"})
	}
	event, err := h.store.ToggleRead(id)
	if errors.Is(err, ErrNotFound) {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": err.Error()})
	}
	if err != nil {
		return err
	}
	return c.JSON(event)
}
