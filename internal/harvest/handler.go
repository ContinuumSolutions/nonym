package harvest

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/egokernel/ek1/internal/notifications"
	"github.com/gofiber/fiber/v2"
)

// opportunityThreshold is the USD debt value above which an OPPORTUNITY
// notification is created after a harvest scan.
const opportunityThreshold = 10_000.0

type Handler struct {
	scanner *Scanner
	store   *Store
	notifs  *notifications.Store

	mu      sync.Mutex // guards running
	running bool       // true while a scan goroutine is executing
}

func NewHandler(scanner *Scanner, store *Store, notifs *notifications.Store) *Handler {
	return &Handler{scanner: scanner, store: store, notifs: notifs}
}

func (h *Handler) RegisterRoutes(r fiber.Router) {
	r.Post("/harvest/scan", h.scan)
	r.Get("/harvest/results", h.results)
	r.Get("/harvest/status", h.status)
}

// @Summary      Trigger harvest scan (async)
// @Description  Starts a social-debt scan in the background and returns immediately. Poll GET /harvest/results for the completed result or GET /harvest/status for running state.
// @Tags         harvest
// @Produce      json
// @Success      202  {object}  map[string]interface{}
// @Failure      409  {object}  map[string]interface{}
// @Router       /harvest/scan [post]
func (h *Handler) scan(c *fiber.Ctx) error {
	h.mu.Lock()
	if h.running {
		h.mu.Unlock()
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{
			"status":  "already_running",
			"message": "a harvest scan is already in progress — poll GET /harvest/results for completion",
		})
	}
	h.running = true
	h.mu.Unlock()

	go func() {
		defer func() {
			h.mu.Lock()
			h.running = false
			h.mu.Unlock()
		}()

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		result, err := h.scanner.Scan(ctx)
		if err != nil {
			log.Printf("harvest: scan error: %v", err)
			return
		}
		if err := h.store.Save(result); err != nil {
			log.Printf("harvest: save result: %v", err)
		}
		h.createNotifications(result)
	}()

	return c.Status(fiber.StatusAccepted).JSON(fiber.Map{
		"status":  "started",
		"message": "harvest scan started — poll GET /harvest/results for the completed result",
	})
}

// @Summary      Get harvest scan status
// @Tags         harvest
// @Produce      json
// @Success      200  {object}  map[string]interface{}
// @Router       /harvest/status [get]
func (h *Handler) status(c *fiber.Ctx) error {
	h.mu.Lock()
	running := h.running
	h.mu.Unlock()
	return c.JSON(fiber.Map{"running": running})
}

// @Summary      Get latest harvest results
// @Tags         harvest
// @Produce      json
// @Success      200  {object}  harvest.HarvestResult
// @Failure      500  {object}  map[string]interface{}
// @Router       /harvest/results [get]
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

// createNotifications fires OPPORTUNITY and HARVEST notifications for
// significant findings from a completed harvest scan.
func (h *Handler) createNotifications(result HarvestResult) {
	for _, debt := range result.Debts {
		if debt.EstimatedValue >= opportunityThreshold {
			_, err := h.notifs.Create(notifications.Notification{
				Type: notifications.TypeOpportunity,
				Title: fmt.Sprintf(
					"High-value social debt: %s owes you $%.0f",
					debt.Contact.Name, debt.EstimatedValue,
				),
				Body: fmt.Sprintf(
					"%d unreciprocated favour(s) — $%.0f estimated value. Recommended action: %s",
					debt.NetFavors, debt.EstimatedValue, debt.Action,
				),
			})
			if err != nil {
				log.Printf("harvest: create OPPORTUNITY notification: %v", err)
			}
		}
	}

	for _, opp := range result.Opportunities {
		_, err := h.notifs.Create(notifications.Notification{
			Type:  notifications.TypeHarvest,
			Title: "Ghost-agreement opportunity detected",
			Body:  opp,
		})
		if err != nil {
			log.Printf("harvest: create HARVEST notification: %v", err)
		}
	}
}
