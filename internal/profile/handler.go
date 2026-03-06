package profile

import (
	"context"

	"github.com/gofiber/fiber/v2"
)

// IdentityInferrer is satisfied by *ai.Client. Defined here as an interface
// to avoid an import cycle (ai already imports activities; profile is lower-level).
type IdentityInferrer interface {
	InferIdentity(ctx context.Context, narratives []string) (string, error)
}

// NarrativesFunc returns the most recent N event narratives for identity inference.
// Wired in main.go as a closure over the activities store.
type NarrativesFunc func(limit int) []string

type Handler struct {
	store        *Store
	inferrer     IdentityInferrer // nil = infer endpoint disabled
	narrativesFn NarrativesFunc   // nil = infer endpoint disabled
}

func NewHandler(store *Store, inferrer IdentityInferrer, narrativesFn NarrativesFunc) *Handler {
	return &Handler{store: store, inferrer: inferrer, narrativesFn: narrativesFn}
}

func (h *Handler) RegisterRoutes(r fiber.Router) {
	r.Get("/profile", h.get)
	r.Put("/profile/preferences", h.updatePreferences)
	r.Put("/profile/connection", h.updateConnection)
	r.Put("/profile/identity", h.updateIdentity)
	r.Post("/profile/infer", h.inferIdentity)
}

// @Summary      Get profile
// @Tags         profile
// @Produce      json
// @Success      200  {object}  profile.Profile
// @Failure      500  {object}  map[string]interface{}
// @Router       /profile [get]
func (h *Handler) get(c *fiber.Ctx) error {
	profile, err := h.store.Get()
	if err != nil {
		return err
	}
	return c.JSON(profile)
}

// @Summary      Update decision preferences
// @Tags         profile
// @Accept       json
// @Produce      json
// @Param        body  body      profile.DecisionPreference  true  "Preference weights (1–10 each)"
// @Success      200   {object}  profile.Profile
// @Failure      400   {object}  map[string]interface{}
// @Failure      500   {object}  map[string]interface{}
// @Router       /profile/preferences [put]
func (h *Handler) updatePreferences(c *fiber.Ctx) error {
	var body DecisionPreference
	if err := c.BodyParser(&body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	if err := validatePreferences(body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	profile, err := h.store.UpdatePreferences(body)
	if err != nil {
		return err
	}
	return c.JSON(profile)
}

// @Summary      Update connection settings
// @Tags         profile
// @Accept       json
// @Produce      json
// @Param        body  body      profile.ConnectionSetting  true  "Connection settings"
// @Success      200   {object}  profile.Profile
// @Failure      400   {object}  map[string]interface{}
// @Failure      500   {object}  map[string]interface{}
// @Router       /profile/connection [put]
func (h *Handler) updateConnection(c *fiber.Ctx) error {
	var body ConnectionSetting
	if err := c.BodyParser(&body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	if body.KernelName == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "kernel_name is required"})
	}
	profile, err := h.store.UpdateConnection(body)
	if err != nil {
		return err
	}
	return c.JSON(profile)
}

// @Summary      Update user identity
// @Description  Sets profession, industry, skills, goals and other identity fields used to
//               personalise LLM signal scoring. All fields are optional; send only what you want
//               to update — empty string fields are ignored (existing values preserved).
// @Tags         profile
// @Accept       json
// @Produce      json
// @Param        body  body      profile.UserIdentity  true  "Identity fields"
// @Success      200   {object}  profile.Profile
// @Failure      400   {object}  map[string]interface{}
// @Failure      500   {object}  map[string]interface{}
// @Router       /profile/identity [put]
func (h *Handler) updateIdentity(c *fiber.Ctx) error {
	var body UserIdentity
	if err := c.BodyParser(&body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	profile, err := h.store.UpdateIdentity(body)
	if err != nil {
		return err
	}
	return c.JSON(profile)
}

// @Summary      Auto-infer user identity from signal history
// @Description  Runs the local LLM over the most recent event narratives to generate a
//               plain-text professional summary. The result is stored in inferred_summary
//               and injected into future LLM prompts alongside any user-declared fields.
//               Requires at least 5 events in history to produce a meaningful result.
// @Tags         profile
// @Produce      json
// @Success      200  {object}  profile.Profile
// @Failure      422  {object}  map[string]interface{}
// @Failure      500  {object}  map[string]interface{}
// @Router       /profile/infer [post]
func (h *Handler) inferIdentity(c *fiber.Ctx) error {
	if h.inferrer == nil || h.narrativesFn == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(
			fiber.Map{"error": "identity inference not available"},
		)
	}

	narratives := h.narrativesFn(40)
	if len(narratives) < 5 {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(
			fiber.Map{"error": "not enough signal history — run at least one sync cycle first"},
		)
	}

	summary, err := h.inferrer.InferIdentity(c.Context(), narratives)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	profile, err := h.store.SaveInferredSummary(summary)
	if err != nil {
		return err
	}
	return c.JSON(profile)
}

// validatePreferences checks each weight is in the range 1–10 and base_hourly_rate is positive.
func validatePreferences(p DecisionPreference) error {
	fields := map[string]int{
		"time_sovereignty":    p.TimeSovereignty,
		"financial_growth":    p.FinacialGrowth,
		"health_recovery":     p.HealthRecovery,
		"reputation_building": p.ReputationBuilding,
		"privacy_protection":  p.PrivacyProtection,
		"autonomy":            p.Autonomy,
	}
	for name, val := range fields {
		if val < 1 || val > 10 {
			return fiber.NewError(fiber.StatusBadRequest,
				name+" must be between 1 and 10",
			)
		}
	}
	if p.BaseHourlyRate < 0 {
		return fiber.NewError(fiber.StatusBadRequest, "base_hourly_rate must be a positive number")
	}
	return nil
}
