package ledger

import (
	"strconv"

	"github.com/gofiber/fiber/v2"
)

type Handler struct {
	ledger *SQLiteLedger
	uid    string
}

func NewHandler(l *SQLiteLedger, uid string) *Handler {
	return &Handler{ledger: l, uid: uid}
}

func (h *Handler) RegisterRoutes(r fiber.Router) {
	r.Get("/ledger/score", h.score)
	r.Get("/ledger/history", h.history)
}

// @Summary      Get reputation score
// @Tags         ledger
// @Produce      json
// @Success      200  {object}  map[string]interface{}
// @Failure      500  {object}  map[string]interface{}
// @Router       /ledger/score [get]
func (h *Handler) score(c *fiber.Ctx) error {
	s := h.ledger.Score(h.uid)
	tier := h.ledger.Tier(h.uid)
	return c.JSON(fiber.Map{
		"uid":        h.uid,
		"score":      s,
		"tier":       tier,
		"trust_tax":  tier.TrustTax(),
		"is_exiled":  h.ledger.IsExiled(h.uid),
	})
}

// @Summary      Get reputation history
// @Tags         ledger
// @Produce      json
// @Param        limit   query     int  false  "Results per page (1–100, default 20)"
// @Param        offset  query     int  false  "Pagination offset (default 0)"
// @Success      200     {array}   ledger.HistoryEntry
// @Failure      500     {object}  map[string]interface{}
// @Router       /ledger/history [get]
func (h *Handler) history(c *fiber.Ctx) error {
	limit, _ := strconv.Atoi(c.Query("limit", "20"))
	offset, _ := strconv.Atoi(c.Query("offset", "0"))

	if limit < 1 || limit > 100 {
		limit = 20
	}

	entries, err := h.ledger.History(h.uid, limit, offset)
	if err != nil {
		return err
	}
	if entries == nil {
		entries = []HistoryEntry{}
	}
	return c.JSON(entries)
}
