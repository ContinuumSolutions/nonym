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

// @Summary      Get brain status
// @Tags         brain
// @Produce      json
// @Success      200  {object}  brain.StatusResponse
// @Failure      500  {object}  map[string]interface{}
// @Router       /brain/status [get]
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

// @Summary      Acknowledge manual sync (clears H2HI)
// @Tags         brain
// @Produce      json
// @Success      200  {object}  brain.KernelSnapshot
// @Failure      500  {object}  map[string]interface{}
// @Router       /brain/sync-acknowledge [post]
func (h *Handler) syncAcknowledge(c *fiber.Ctx) error {
	h.svc.kernel.AcknowledgeManualSync()
	snap := h.svc.kernel.Snapshot()
	return c.JSON(snap)
}

// @Summary      List brain events (alias for /activities/events)
// @Tags         brain
// @Produce      json
// @Success      200  {array}   activities.Event
// @Failure      500  {object}  map[string]interface{}
// @Router       /brain/events [get]
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
