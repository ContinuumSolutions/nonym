package brain

import (
	"github.com/gofiber/fiber/v2"
)

// StatusResponse is the full brain status returned by GET /brain/status.
type StatusResponse struct {
	KernelSnapshot
	ReputationScore int64  `json:"reputation_score"`
	ReputationTier  string `json:"reputation_tier"`
}

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) RegisterRoutes(r fiber.Router) {
	r.Get("/brain/status", h.status)
	r.Post("/brain/sync-acknowledge", h.syncAcknowledge)
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
