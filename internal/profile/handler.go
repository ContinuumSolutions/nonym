package profile

import (
	"github.com/gofiber/fiber/v2"
)

type Handler struct {
	store *Store
}

func NewHandler(store *Store) *Handler {
	return &Handler{store: store}
}

func (h *Handler) RegisterRoutes(r fiber.Router) {
	r.Get("/profile", h.get)
	r.Put("/profile/preferences", h.updatePreferences)
	r.Put("/profile/connection", h.updateConnection)
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
