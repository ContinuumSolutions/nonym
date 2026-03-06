package auth

import (
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
)

// JWTHandler implements the JWT-based PIN authentication system
type JWTHandler struct {
	pinStore     *PINStore
	jwtService   *JWTService
	denylist     *TokenDenylist
	rateLimiter  *RateLimiter
}

// NewJWTHandler creates a new JWT-based auth handler
func NewJWTHandler(pinStore *PINStore, jwtService *JWTService, denylist *TokenDenylist) *JWTHandler {
	return &JWTHandler{
		pinStore:    pinStore,
		jwtService:  jwtService,
		denylist:    denylist,
		rateLimiter: NewRateLimiter(),
	}
}

// RegisterJWTRoutes registers the JWT auth endpoints (public - no auth required)
func (h *JWTHandler) RegisterJWTRoutes(r fiber.Router) {
	auth := r.Group("/auth")

	// Public endpoints (no authentication required)
	auth.Get("/pin/status", h.getPinStatus)
	auth.Post("/pin/setup", h.setupPin)
	auth.Post("/login", h.login)

	// Authenticated endpoints (require valid JWT)
	auth.Post("/logout", h.logout)
	auth.Post("/pin/change", h.changePin)
	auth.Delete("/pin", h.removePin)
}

// Request/Response types

type SetupPinRequest struct {
	PIN string `json:"pin"`
}

type LoginRequest struct {
	PIN string `json:"pin"`
}

type ChangePinRequest struct {
	CurrentPIN string `json:"current_pin"`
	NewPIN     string `json:"new_pin"`
}

type RemovePinRequest struct {
	CurrentPIN string `json:"current_pin"`
}

type AuthTokenResponse struct {
	Token     string `json:"token"`
	ExpiresAt string `json:"expires_at"`
}

type PinStatusResponse struct {
	Configured bool `json:"configured"`
}

// @Summary      Check PIN configuration status
// @Description  Returns whether a PIN has been configured for this device
// @Tags         auth
// @Produce      json
// @Success      200  {object}  PinStatusResponse
// @Router       /auth/pin/status [get]
func (h *JWTHandler) getPinStatus(c *fiber.Ctx) error {
	configured, err := h.pinStore.IsConfigured()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to check pin status",
		})
	}

	return c.JSON(PinStatusResponse{
		Configured: configured,
	})
}

// @Summary      Setup initial PIN
// @Description  Sets up the PIN for first-time use (only if no PIN is configured)
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        body  body      SetupPinRequest  true  "PIN setup request"
// @Success      200   {object}  AuthTokenResponse
// @Failure      400   {object}  map[string]string
// @Failure      409   {object}  map[string]string
// @Router       /auth/pin/setup [post]
func (h *JWTHandler) setupPin(c *fiber.Ctx) error {
	var req SetupPinRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid request body",
		})
	}

	// Set up the PIN
	err := h.pinStore.SetupPIN(req.PIN)
	if err != nil {
		switch err {
		case ErrPINAlreadySet:
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{
				"error": "pin already configured",
			})
		case ErrPINInvalid:
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "pin must be exactly 4 digits",
			})
		default:
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "failed to setup pin",
			})
		}
	}

	// Generate initial JWT token
	tokenResp, err := h.jwtService.GenerateToken()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to generate token",
		})
	}

	return c.JSON(AuthTokenResponse{
		Token:     tokenResp.Token,
		ExpiresAt: tokenResp.ExpiresAt.Format(time.RFC3339),
	})
}

// @Summary      Login with PIN
// @Description  Validates PIN and returns JWT token
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        body  body      LoginRequest  true  "Login request"
// @Success      200   {object}  AuthTokenResponse
// @Failure      401   {object}  map[string]string
// @Failure      429   {object}  map[string]string
// @Router       /auth/login [post]
func (h *JWTHandler) login(c *fiber.Ctx) error {
	clientIP := c.IP()

	// Check rate limiting
	isLocked, retryAfter := h.rateLimiter.IsLocked(clientIP)
	if isLocked {
		c.Set("Retry-After", strconv.Itoa(int(retryAfter.Seconds())))
		return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
			"error": "too many failed attempts, try again later",
		})
	}

	var req LoginRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid request body",
		})
	}

	// Verify PIN
	err := h.pinStore.VerifyPIN(req.PIN)
	if err != nil {
		// Record failed attempt for rate limiting
		h.rateLimiter.RecordFailedAttempt(clientIP)

		// Use consistent timing to prevent timing attacks
		time.Sleep(100 * time.Millisecond)

		switch err {
		case ErrPINInvalid, ErrPINNotSet:
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "unauthorized",
			})
		default:
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "login failed",
			})
		}
	}

	// Clear failed attempts on successful login
	h.rateLimiter.ClearAttempts(clientIP)

	// Generate JWT token
	tokenResp, err := h.jwtService.GenerateToken()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to generate token",
		})
	}

	return c.JSON(AuthTokenResponse{
		Token:     tokenResp.Token,
		ExpiresAt: tokenResp.ExpiresAt.Format(time.RFC3339),
	})
}

// @Summary      Logout
// @Description  Invalidates the current JWT token
// @Tags         auth
// @Produce      json
// @Success      200   {object}  map[string]string
// @Security     BearerAuth
// @Router       /auth/logout [post]
func (h *JWTHandler) logout(c *fiber.Ctx) error {
	// Extract token from header
	authHeader := c.Get("Authorization")
	tokenString := ExtractTokenFromHeader(authHeader)
	if tokenString == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "unauthorized",
		})
	}

	// Validate token to get claims
	claims, err := h.jwtService.ValidateToken(tokenString)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "unauthorized",
		})
	}

	// Add token to denylist
	expiresAt := time.Unix(claims.ExpiresAt, 0)
	if claims.TokenID != "" {
		h.denylist.Add(claims.TokenID, expiresAt)
	}

	return c.JSON(fiber.Map{})
}

// @Summary      Change PIN
// @Description  Changes the PIN after verifying current PIN, issues new JWT
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        body  body      ChangePinRequest  true  "PIN change request"
// @Success      200   {object}  AuthTokenResponse
// @Failure      401   {object}  map[string]string
// @Security     BearerAuth
// @Router       /auth/pin/change [post]
func (h *JWTHandler) changePin(c *fiber.Ctx) error {
	var req ChangePinRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid request body",
		})
	}

	// Change the PIN
	err := h.pinStore.ChangePIN(req.CurrentPIN, req.NewPIN)
	if err != nil {
		switch err {
		case ErrPINInvalid:
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "unauthorized",
			})
		default:
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "failed to change pin",
			})
		}
	}

	// Generate new JWT token
	tokenResp, err := h.jwtService.GenerateToken()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to generate token",
		})
	}

	return c.JSON(AuthTokenResponse{
		Token:     tokenResp.Token,
		ExpiresAt: tokenResp.ExpiresAt.Format(time.RFC3339),
	})
}

// @Summary      Remove PIN
// @Description  Removes PIN protection after verification, ends session
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        body  body      RemovePinRequest  true  "PIN removal request"
// @Success      200   {object}  map[string]string
// @Failure      401   {object}  map[string]string
// @Security     BearerAuth
// @Router       /auth/pin [delete]
func (h *JWTHandler) removePin(c *fiber.Ctx) error {
	var req RemovePinRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid request body",
		})
	}

	// Remove the PIN
	err := h.pinStore.RemovePIN(req.CurrentPIN)
	if err != nil {
		switch err {
		case ErrPINInvalid:
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "unauthorized",
			})
		default:
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "failed to remove pin",
			})
		}
	}

	return c.JSON(fiber.Map{})
}