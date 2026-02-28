package brain

import (
	"github.com/egokernel/ek1/internal/activities"
	"github.com/gofiber/fiber/v2"
)

// StatusResponse is the full brain status returned by GET /brain/status.
type StatusResponse struct {
	KernelSnapshot
	ReputationScore int64  `json:"reputation_score"`
	ReputationTier  string `json:"reputation_tier"`
}

type Handler struct {
	svc    *Service
	events *activities.Store
}

func NewHandler(svc *Service, events *activities.Store) *Handler {
	return &Handler{svc: svc, events: events}
}

func (h *Handler) RegisterRoutes(r fiber.Router) {
	r.Get("/brain/status", h.status)
	r.Post("/brain/sync-acknowledge", h.syncAcknowledge)
	r.Get("/brain/events", h.events_)
}

func (h *Handler) status(c *fiber.Ctx) error {
	snap := h.svc.kernel.Snapshot()
	score := h.svc.ledger.Score(h.svc.uid)
	tier := h.svc.ledger.Tier(h.svc.uid)

	return c.JSON(StatusResponse{
		KernelSnapshot:  snap,
		ReputationScore: score,
		ReputationTier:  string(tier),
	})
}

func (h *Handler) syncAcknowledge(c *fiber.Ctx) error {
	h.svc.kernel.AcknowledgeManualSync()
	snap := h.svc.kernel.Snapshot()
	return c.JSON(snap)
}

// events_ is an alias for GET /activities/events — all events originate from the brain.
func (h *Handler) events_(c *fiber.Ctx) error {
	list, err := h.events.List()
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	if list == nil {
		list = []activities.Event{}
	}
	return c.JSON(list)
}
