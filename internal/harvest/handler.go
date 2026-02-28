package harvest

import (
	"fmt"
	"log"

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
}

func NewHandler(scanner *Scanner, store *Store, notifs *notifications.Store) *Handler {
	return &Handler{scanner: scanner, store: store, notifs: notifs}
}

func (h *Handler) RegisterRoutes(r fiber.Router) {
	r.Post("/harvest/scan", h.scan)
	r.Get("/harvest/results", h.results)
}

// scan triggers a full social graph scan synchronously, persists the result,
// creates notifications for high-value findings, and returns the result.
func (h *Handler) scan(c *fiber.Ctx) error {
	result, err := h.scanner.Scan(c.Context())
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	if err := h.store.Save(result); err != nil {
		log.Printf("harvest: save result: %v", err)
	}

	h.createNotifications(result)

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
