package signals

import (
	"strconv"

	"github.com/gofiber/fiber/v2"
)

// Handler provides the simplified signals API.
type Handler struct {
	store *Store
}

func NewHandler(store *Store) *Handler {
	return &Handler{store: store}
}

// RegisterRoutes mounts the signals endpoints.
func (h *Handler) RegisterRoutes(r fiber.Router) {
	r.Get("/signals", h.list)
	r.Get("/signals/summary", h.summary)
	r.Get("/signals/relevant", h.relevant)
	r.Get("/signals/replies", h.replies)
	r.Get("/signals/:id", h.get)
	r.Put("/signals/:id/status", h.updateStatus)
	r.Get("/replies/pending", h.pendingReplies)
	r.Put("/replies/:id", h.updateReply)
}

// @Summary      List all signals with optional filtering
// @Tags         signals
// @Produce      json
// @Param        category     query     string  false  "Filter by category (relevant|newsletter|automated|notification)"
// @Param        priority     query     string  false  "Filter by priority (high|medium|low)"
// @Param        status       query     string  false  "Filter by status (pending|done|ignored|snoozed)"
// @Param        service      query     string  false  "Filter by service slug"
// @Param        needs_reply  query     boolean false  "Filter by reply requirement"
// @Param        limit        query     int     false  "Max results (default 50, max 100)"
// @Success      200          {array}   Signal
// @Router       /signals [get]
func (h *Handler) list(c *fiber.Ctx) error {
	filter := FilterCriteria{
		Category:    c.Query("category"),
		Priority:    c.Query("priority"),
		ServiceSlug: c.Query("service"),
	}

	if status := c.Query("status"); status != "" {
		switch status {
		case "done":
			s := StatusDone
			filter.Status = &s
		case "ignored":
			s := StatusIgnored
			filter.Status = &s
		case "snoozed":
			s := StatusSnoozed
			filter.Status = &s
		default:
			s := StatusPending
			filter.Status = &s
		}
	}

	if needsReply := c.Query("needs_reply"); needsReply != "" {
		if needsReply == "true" {
			b := true
			filter.NeedsReply = &b
		} else if needsReply == "false" {
			b := false
			filter.NeedsReply = &b
		}
	}

	limit := c.QueryInt("limit", 50)
	if limit > 100 {
		limit = 100
	}

	signals, err := h.store.List(filter, limit)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(signals)
}

// @Summary      Get signals summary for dashboard
// @Tags         signals
// @Produce      json
// @Success      200  {object}  SignalSummary
// @Router       /signals/summary [get]
func (h *Handler) summary(c *fiber.Ctx) error {
	summary, err := h.store.GetSummary()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(summary)
}

// @Summary      Get relevant signals that need attention
// @Tags         signals
// @Produce      json
// @Success      200  {array}  Signal
// @Router       /signals/relevant [get]
func (h *Handler) relevant(c *fiber.Ctx) error {
	relevantFilter := FilterCriteria{Category: "relevant"}
	pending := StatusPending
	relevantFilter.Status = &pending

	signals, err := h.store.List(relevantFilter, 50)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(signals)
}

// @Summary      Get signals that need replies
// @Tags         signals
// @Produce      json
// @Success      200  {array}  Signal
// @Router       /signals/replies [get]
func (h *Handler) replies(c *fiber.Ctx) error {
	needsReply := true
	replyFilter := FilterCriteria{NeedsReply: &needsReply}
	pending := StatusPending
	replyFilter.Status = &pending

	signals, err := h.store.List(replyFilter, 50)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(signals)
}

// @Summary      Get a specific signal
// @Tags         signals
// @Produce      json
// @Param        id   path      int  true  "Signal ID"
// @Success      200  {object}  Signal
// @Router       /signals/{id} [get]
func (h *Handler) get(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid signal ID"})
	}

	signal, err := h.store.Get(id)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "signal not found"})
	}

	return c.JSON(signal)
}

// @Summary      Update signal status
// @Tags         signals
// @Accept       json
// @Produce      json
// @Param        id    path      int           true  "Signal ID"
// @Param        body  body      UpdateStatusRequest  true  "Status update"
// @Success      200   {object}  map[string]string
// @Router       /signals/{id}/status [put]
func (h *Handler) updateStatus(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid signal ID"})
	}

	var req UpdateStatusRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	var status Status
	switch req.Status {
	case "done":
		status = StatusDone
	case "ignored":
		status = StatusIgnored
	case "snoozed":
		status = StatusSnoozed
	default:
		status = StatusPending
	}

	err = h.store.UpdateStatus(id, status, req.Notes)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{"status": "updated"})
}

// @Summary      Get pending draft replies
// @Tags         replies
// @Produce      json
// @Success      200  {array}  DraftReply
// @Router       /replies/pending [get]
func (h *Handler) pendingReplies(c *fiber.Ctx) error {
	drafts, err := h.store.GetPendingDrafts()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(drafts)
}

// @Summary      Update a draft reply
// @Tags         replies
// @Accept       json
// @Produce      json
// @Param        id    path      int                 true  "Draft reply ID"
// @Param        body  body      UpdateReplyRequest  true  "Reply update"
// @Success      200   {object}  map[string]string
// @Router       /replies/{id} [put]
func (h *Handler) updateReply(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid reply ID"})
	}

	var req UpdateReplyRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	var status ReplyStatus
	switch req.Action {
	case "approve":
		status = ReplyApproved
	case "reject":
		status = ReplyRejected
	case "edit":
		status = ReplyEdited
	default:
		status = ReplyDrafted
	}

	err = h.store.UpdateDraftReply(id, req.EditedText, status)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{"status": "updated"})
}

// Request/Response types for API

type UpdateStatusRequest struct {
	Status string `json:"status"` // pending|done|ignored|snoozed
	Notes  string `json:"notes"`  // optional user notes
}

type UpdateReplyRequest struct {
	Action     string `json:"action"`      // approve|reject|edit
	EditedText string `json:"edited_text"` // modified reply text (for edit action)
}
