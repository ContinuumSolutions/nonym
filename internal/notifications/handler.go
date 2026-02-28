package notifications

import (
	"errors"
	"strconv"

	"github.com/gofiber/fiber/v2"
)

type Handler struct {
	store *Store
}

func NewHandler(store *Store) *Handler { return &Handler{store: store} }

func (h *Handler) RegisterRoutes(r fiber.Router) {
	r.Get("/notifications", h.list)
	r.Put("/notifications/read-all", h.readAll) // must come before /:id/read
	r.Put("/notifications/:id/read", h.markRead)
}

// @Summary      List unread notifications
// @Tags         notifications
// @Produce      json
// @Success      200  {array}   notifications.Notification
// @Failure      500  {object}  map[string]interface{}
// @Router       /notifications [get]
func (h *Handler) list(c *fiber.Ctx) error {
	items, err := h.store.ListUnread()
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	if items == nil {
		items = []Notification{}
	}
	return c.JSON(items)
}

// @Summary      Mark notification as read
// @Tags         notifications
// @Produce      json
// @Param        id   path      int  true  "Notification ID"
// @Success      200  {object}  map[string]interface{}
// @Failure      400  {object}  map[string]interface{}
// @Failure      404  {object}  map[string]interface{}
// @Failure      500  {object}  map[string]interface{}
// @Router       /notifications/{id}/read [put]
func (h *Handler) markRead(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid id"})
	}
	if err := h.store.MarkRead(id); err != nil {
		if errors.Is(err, ErrNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": err.Error()})
		}
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	return c.JSON(fiber.Map{"ok": true})
}

// @Summary      Mark all notifications as read
// @Tags         notifications
// @Produce      json
// @Success      200  {object}  map[string]interface{}
// @Failure      500  {object}  map[string]interface{}
// @Router       /notifications/read-all [put]
func (h *Handler) readAll(c *fiber.Ctx) error {
	if err := h.store.MarkAllRead(); err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	return c.JSON(fiber.Map{"ok": true})
}
