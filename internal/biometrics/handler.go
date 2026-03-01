package biometrics

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
	r.Get("/biometrics/checkin", h.get)
	r.Put("/biometrics/checkin", h.update)
	r.Get("/biometrics/checkin/history", h.history)
}

// @Summary      Get current check-in
// @Tags         biometrics
// @Produce      json
// @Success      200  {object}  biometrics.CheckIn
// @Failure      404  {object}  map[string]interface{}
// @Router       /biometrics/checkin [get]
func (h *Handler) get(c *fiber.Ctx) error {
	checkin, err := h.store.Get()
	if errors.Is(err, ErrNotFound) {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": err.Error()})
	}
	if err != nil {
		return err
	}
	return c.JSON(checkin)
}

// @Summary      Update check-in
// @Tags         biometrics
// @Accept       json
// @Produce      json
// @Param        body  body      biometrics.CheckIn  true  "Check-in data"
// @Success      200   {object}  biometrics.CheckIn
// @Failure      400   {object}  map[string]interface{}
// @Failure      500   {object}  map[string]interface{}
// @Router       /biometrics/checkin [put]
func (h *Handler) update(c *fiber.Ctx) error {
	var body CheckIn
	if err := c.BodyParser(&body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	updated, err := h.store.Upsert(&body)
	if err != nil {
		return err
	}
	return c.JSON(updated)
}

// @Summary      Check-in history
// @Description  Returns past check-ins newest first. Default limit is 7 (one week); maximum is 90.
// @Tags         biometrics
// @Produce      json
// @Param        limit  query     int  false  "Number of entries to return (1–90, default 7)"
// @Success      200    {array}   biometrics.CheckIn
// @Failure      500    {object}  map[string]interface{}
// @Router       /biometrics/checkin/history [get]
func (h *Handler) history(c *fiber.Ctx) error {
	limit := 7
	if q := c.Query("limit"); q != "" {
		if n, err := strconv.Atoi(q); err == nil {
			limit = n
		}
	}
	entries, err := h.store.History(limit)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	if entries == nil {
		entries = []CheckIn{}
	}
	return c.JSON(entries)
}
