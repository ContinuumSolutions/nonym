package integrations

import (
	"errors"
	"strconv"

	"github.com/gofiber/fiber/v2"
)

type Handler struct {
	store *Store
}

func NewHandler(store *Store) *Handler {
	return &Handler{store: store}
}

func (h *Handler) RegisterRoutes(r fiber.Router) {
	r.Get("/integrations/services", h.list)
	r.Post("/integrations/services/custom", h.createCustom) // must be before /:id
	r.Get("/integrations/services/:id", h.get)
	r.Post("/integrations/services/:id/connect", h.startConnect)
	r.Put("/integrations/services/:id/connect", h.completeConnect)
	r.Delete("/integrations/services/:id/connect", h.uninstall)
}

// @Summary      List all services
// @Tags         integrations
// @Produce      json
// @Success      200  {array}   integrations.Service
// @Failure      500  {object}  map[string]interface{}
// @Router       /integrations/services [get]
func (h *Handler) list(c *fiber.Ctx) error {
	services, err := h.store.List()
	if err != nil {
		return err
	}
	if services == nil {
		services = []Service{}
	}
	return c.JSON(services)
}

// @Summary      Get service by ID
// @Tags         integrations
// @Produce      json
// @Param        id   path      int  true  "Service ID"
// @Success      200  {object}  integrations.Service
// @Failure      400  {object}  map[string]interface{}
// @Failure      404  {object}  map[string]interface{}
// @Failure      500  {object}  map[string]interface{}
// @Router       /integrations/services/{id} [get]
func (h *Handler) get(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid id"})
	}
	svc, err := h.store.Get(id)
	if errors.Is(err, ErrNotFound) {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": err.Error()})
	}
	if err != nil {
		return err
	}
	return c.JSON(svc)
}

// @Summary      Create custom service
// @Tags         integrations
// @Accept       json
// @Produce      json
// @Param        body  body      integrations.Service  true  "Custom service (name, category, api_endpoint, api_key required)"
// @Success      201   {object}  integrations.Service
// @Failure      400   {object}  map[string]interface{}
// @Failure      500   {object}  map[string]interface{}
// @Router       /integrations/services/custom [post]
func (h *Handler) createCustom(c *fiber.Ctx) error {
	var body Service
	if err := c.BodyParser(&body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	if body.Name == "" || body.Category == "" || body.APIEndpoint == "" || body.APIKey == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "name, category, api_endpoint, and api_key are required",
		})
	}
	svc, err := h.store.CreateCustom(&body)
	if err != nil {
		return err
	}
	return c.Status(fiber.StatusCreated).JSON(svc)
}

// @Summary      Start connecting a service
// @Tags         integrations
// @Produce      json
// @Param        id   path      int  true  "Service ID"
// @Success      200  {object}  integrations.Service
// @Failure      400  {object}  map[string]interface{}
// @Failure      404  {object}  map[string]interface{}
// @Failure      500  {object}  map[string]interface{}
// @Router       /integrations/services/{id}/connect [post]
func (h *Handler) startConnect(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid id"})
	}
	svc, err := h.store.StartConnect(id)
	if errors.Is(err, ErrNotFound) {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": err.Error()})
	}
	if err != nil {
		return err
	}
	return c.JSON(svc)
}

// @Summary      Complete connecting a service
// @Tags         integrations
// @Accept       json
// @Produce      json
// @Param        id    path      int                         true  "Service ID"
// @Param        body  body      integrations.ConnectInput   true  "Credentials (api_key or oauth_access_token)"
// @Success      200   {object}  integrations.Service
// @Failure      400   {object}  map[string]interface{}
// @Failure      404   {object}  map[string]interface{}
// @Failure      500   {object}  map[string]interface{}
// @Router       /integrations/services/{id}/connect [put]
func (h *Handler) completeConnect(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid id"})
	}
	var input ConnectInput
	if err := c.BodyParser(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	if input.APIKey == "" && input.OAuthAccessToken == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "api_key or oauth_access_token is required",
		})
	}
	svc, err := h.store.CompleteConnect(id, input)
	if errors.Is(err, ErrNotFound) {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": err.Error()})
	}
	if err != nil {
		return err
	}
	return c.JSON(svc)
}

// @Summary      Uninstall a service
// @Tags         integrations
// @Produce      json
// @Param        id   path      int  true  "Service ID"
// @Success      200  {object}  integrations.Service
// @Failure      400  {object}  map[string]interface{}
// @Failure      404  {object}  map[string]interface{}
// @Failure      500  {object}  map[string]interface{}
// @Router       /integrations/services/{id}/connect [delete]
func (h *Handler) uninstall(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid id"})
	}
	svc, err := h.store.Uninstall(id)
	if errors.Is(err, ErrNotFound) {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": err.Error()})
	}
	if err != nil {
		return err
	}
	return c.JSON(svc)
}
