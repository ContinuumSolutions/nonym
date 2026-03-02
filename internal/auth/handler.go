package auth

import (
	"time"

	"github.com/egokernel/ek1/internal/profile"
	"github.com/gofiber/fiber/v2"
)

// Handler exposes screen-lock and PIN-management endpoints.
type Handler struct {
	sessions *Store
	profile  *profile.Store
}

func NewHandler(sessions *Store, profile *profile.Store) *Handler {
	return &Handler{sessions: sessions, profile: profile}
}

func (h *Handler) RegisterRoutes(r fiber.Router) {
	r.Post("/auth/session/lock", h.lock)
	r.Post("/auth/session/unlock", h.unlock)
	r.Put("/profile/pin", h.setPin)
	r.Delete("/profile/pin", h.removePin)
}

// lock godoc
// @Summary      Lock the screen session
// @Tags         auth
// @Produce      json
// @Success      200  {object}  map[string]interface{}
// @Router       /auth/session/lock [post]
func (h *Handler) lock(c *fiber.Ctx) error {
	resumeToken, lockedAt, err := h.sessions.Lock()
	if err != nil {
		return err
	}
	return c.JSON(fiber.Map{
		"locked_at":    lockedAt.Format(time.RFC3339),
		"resume_token": resumeToken,
	})
}

type unlockReq struct {
	PINHash string `json:"pin_hash"`
}

// unlock godoc
// @Summary      Unlock the screen session
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        body  body      unlockReq  true  "SHA-256 hex of the PIN"
// @Success      200   {object}  map[string]interface{}
// @Failure      400   {object}  map[string]interface{}
// @Failure      401   {object}  map[string]interface{}
// @Failure      429   {object}  map[string]interface{}
// @Router       /auth/session/unlock [post]
func (h *Handler) unlock(c *fiber.Ctx) error {
	var req unlockReq
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	if req.PINHash == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "pin_hash is required"})
	}

	storedHash, err := h.profile.GetPINHash()
	if err != nil {
		return err
	}
	if storedHash == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "no_pin_set"})
	}

	token, unlockedAt, err := h.sessions.Unlock(req.PINHash, storedHash)
	if err != nil {
		switch err {
		case ErrInvalidPIN:
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid_pin"})
		case ErrCooldown:
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{"error": "cooldown"})
		case ErrSessionInvalidated:
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "session_invalidated"})
		default:
			return err
		}
	}

	return c.JSON(fiber.Map{
		"token":       token,
		"unlocked_at": unlockedAt.Format(time.RFC3339),
	})
}

type pinReq struct {
	PINHash string `json:"pin_hash"`
}

// setPin godoc
// @Summary      Store the screen lock PIN hash
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        body  body      pinReq  true  "SHA-256 hex of the PIN (64 chars)"
// @Success      200   {object}  map[string]interface{}
// @Failure      400   {object}  map[string]interface{}
// @Router       /profile/pin [put]
func (h *Handler) setPin(c *fiber.Ctx) error {
	var req pinReq
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	if len(req.PINHash) != 64 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "pin_hash must be a 64-char SHA-256 hex string",
		})
	}
	updatedAt, err := h.profile.SetPIN(req.PINHash)
	if err != nil {
		return err
	}
	return c.JSON(fiber.Map{"updated_at": updatedAt.Format(time.RFC3339)})
}

// removePin godoc
// @Summary      Remove the screen lock PIN
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        body  body      pinReq  true  "Current PIN hash for verification"
// @Success      200   {object}  map[string]interface{}
// @Failure      400   {object}  map[string]interface{}
// @Failure      401   {object}  map[string]interface{}
// @Router       /profile/pin [delete]
func (h *Handler) removePin(c *fiber.Ctx) error {
	var req pinReq
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	if req.PINHash == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "pin_hash is required"})
	}

	storedHash, err := h.profile.GetPINHash()
	if err != nil {
		return err
	}
	if storedHash != req.PINHash {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid_pin"})
	}

	removedAt, err := h.profile.RemovePIN()
	if err != nil {
		return err
	}
	return c.JSON(fiber.Map{"removed_at": removedAt.Format(time.RFC3339)})
}
