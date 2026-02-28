package harvest

import (
	"github.com/gofiber/fiber/v2"
)

type Handler struct {
	scanner *Scanner
	store   *Store
}

func NewHandler(scanner *Scanner, store *Store) *Handler {
	return &Handler{scanner: scanner, store: store}
}

func (h *Handler) RegisterRoutes(r fiber.Router) {
	r.Post("/harvest/scan", h.scan)
	r.Get("/harvest/results", h.results)
}

// scan triggers a full social graph scan synchronously and returns the result.
// In production this will be moved to the scheduler (step 9) for background runs.
func (h *Handler) scan(c *fiber.Ctx) error {
	result, err := h.scanner.Scan(c.Context())
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	// Persist — non-fatal if storage fails.
	if err := h.store.Save(result); err != nil {
		// Log but don't block the response.
		_ = err
	}
	return c.JSON(result)
}

// results returns the most recent stored harvest result.
func (h *Handler) results(c *fiber.Ctx) error {
	result, err := h.store.Latest()
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	if result == nil {
		return c.JSON(fiber.Map{
			"result":  nil,
			"message": "no harvest scan has been run yet — POST /harvest/scan to start one",
		})
	}
	return c.JSON(result)
}
