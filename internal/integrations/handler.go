package integrations

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/getsentry/sentry-go"
	sentryfiber "github.com/getsentry/sentry-go/fiber"
	"github.com/gofiber/fiber/v2"
)

// Handler serves all /integrations/* and /oauth/callback routes.
type Handler struct {
	store          *Store
	apiBaseURL     string // e.g. "http://localhost:3000" — used to build the OAuth redirect_uri
	frontendOrigin string // e.g. "http://localhost:8080" — where the browser popup was opened from
}

// NewHandler wires the store and environment-level configuration.
// apiBaseURL is the EK-1 server's public base URL (used as the OAuth redirect_uri base).
// frontendOrigin is the frontend's origin (used for the post-callback redirect).
func NewHandler(store *Store, apiBaseURL, frontendOrigin string) *Handler {
	return &Handler{store: store, apiBaseURL: apiBaseURL, frontendOrigin: frontendOrigin}
}

// RegisterRoutes mounts all integration endpoints on the given router.
func (h *Handler) RegisterRoutes(r fiber.Router) {
	r.Get("/integrations/services", h.list)
	r.Post("/integrations/services/custom", h.createCustom) // must be before /:id
	r.Get("/integrations/services/:id", h.get)
	r.Post("/integrations/services/:id/connect", h.startConnect)
	r.Put("/integrations/services/:id/connect", h.completeConnect)
	r.Delete("/integrations/services/:id/connect", h.uninstall)

	// OAuth BYOA flow (steps 9a–9c)
	r.Post("/integrations/services/:id/oauth-app", h.saveOAuthApp)      // 9a
	r.Post("/integrations/services/:id/oauth/initiate", h.initiateOAuth) // 9b
	r.Get("/oauth/callback", h.oauthCallback)                             // 9c
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

// @Summary      Disconnect / uninstall a service
// @Description  For OAuth services, attempts token revocation before clearing credentials.
//
//	client_id and client_secret are preserved so the user can re-authorise without
//	re-entering app credentials.
//
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

	// Step 9e: for OAuth services, attempt best-effort token revocation before clearing.
	svc, err := h.store.Get(id)
	if errors.Is(err, ErrNotFound) {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": err.Error()})
	}
	if err != nil {
		return err
	}
	if svc.AuthMethod == OAuth2Auth && svc.OAuthConnected {
		if def := lookupCatalog(svc.Slug); def != nil && def.RevokeURL != "" {
			_, _, accessToken, _, _, _, credErr := h.store.GetOAuthCreds(id)
			if credErr == nil && accessToken != "" {
				if revokeErr := revokeToken(c.Context(), def, accessToken); revokeErr != nil {
					log.Printf("integrations: revoke token for %s: %v (continuing disconnect)", svc.Slug, revokeErr)
				}
			}
		}
	}

	result, err := h.store.Uninstall(id)
	if errors.Is(err, ErrNotFound) {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": err.Error()})
	}
	if err != nil {
		return err
	}
	return c.JSON(result)
}

// oauthAppRequest is the body for POST /integrations/services/:id/oauth-app.
type oauthAppRequest struct {
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
}

// @Summary      Save OAuth app credentials (BYOA — step 9a)
// @Description  Encrypts and stores the user's OAuth client_id and client_secret at rest.
//
//	The values are write-only and never returned in any future API response.
//	Saving new credentials resets oauth_connected to false.
//
// @Tags         integrations
// @Accept       json
// @Produce      json
// @Param        id    path      int                              true  "Service ID"
// @Param        body  body      integrations.oauthAppRequest     true  "client_id and client_secret"
// @Success      200   {object}  map[string]interface{}
// @Failure      400   {object}  map[string]interface{}           "invalid_credentials"
// @Failure      404   {object}  map[string]interface{}           "service_not_found"
// @Failure      500   {object}  map[string]interface{}
// @Router       /integrations/services/{id}/oauth-app [post]
func (h *Handler) saveOAuthApp(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid id"})
	}

	var body oauthAppRequest
	if err := c.BodyParser(&body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid_credentials"})
	}
	if body.ClientID == "" || len(body.ClientSecret) < 8 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid_credentials"})
	}

	if _, err := h.store.SaveOAuthApp(id, body.ClientID, body.ClientSecret); err != nil {
		if errors.Is(err, ErrNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "service_not_found"})
		}
		return err
	}
	return c.JSON(fiber.Map{})
}

// initiateOAuthResponse is the body returned by POST /integrations/services/:id/oauth/initiate.
type initiateOAuthResponse struct {
	AuthURL string `json:"auth_url"`
	State   string `json:"state"`
}

// @Summary      Initiate OAuth2 authorization flow (step 9b)
// @Description  Generates a PKCE code_verifier/challenge and CSRF state, then returns the
//
//	full authorization URL for the frontend to open in a popup.
//	Requires OAuth app credentials to have been saved first (step 9a).
//
// @Tags         integrations
// @Produce      json
// @Param        id   path      int  true  "Service ID"
// @Success      200  {object}  integrations.initiateOAuthResponse
// @Failure      400  {object}  map[string]interface{}
// @Failure      404  {object}  map[string]interface{}
// @Failure      409  {object}  map[string]interface{}  "app_not_configured"
// @Failure      500  {object}  map[string]interface{}
// @Router       /integrations/services/{id}/oauth/initiate [post]
func (h *Handler) initiateOAuth(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid id"})
	}

	svc, err := h.store.Get(id)
	if errors.Is(err, ErrNotFound) {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "service_not_found"})
	}
	if err != nil {
		return err
	}

	if !svc.AppConfigured {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": "app_not_configured"})
	}

	def := lookupCatalog(svc.Slug)
	if def == nil || def.AuthURL == "" {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": "app_not_configured"})
	}

	clientID, _, _, _, _, _, err := h.store.GetOAuthCreds(id)
	if err != nil || clientID == "" {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": "app_not_configured"})
	}

	state, err := generateState()
	if err != nil {
		return err
	}

	// NoPKCE services (e.g. Notion) don't support the PKCE extension.
	var verifier, challenge string
	if !def.NoPKCE {
		verifier, challenge, err = generatePKCE()
		if err != nil {
			return err
		}
	}

	stateExpiry := time.Now().Add(10 * time.Minute).Unix()
	if err := h.store.SetOAuthState(id, state, verifier, stateExpiry); err != nil {
		return err
	}

	redirectURI := h.apiBaseURL + "/oauth/callback"
	authURL := buildAuthURL(def, clientID, redirectURI, state, challenge)

	return c.JSON(initiateOAuthResponse{AuthURL: authURL, State: state})
}

// captureOAuthError logs an OAuth error locally and sends it to Sentry if configured.
func captureOAuthError(c *fiber.Ctx, err error, slug, reason string) {
	log.Printf("integrations: oauth %s for service %q: %v", reason, slug, err)
	hub := sentryfiber.GetHubFromContext(c)
	if hub == nil {
		hub = sentry.CurrentHub()
	}
	hub.WithScope(func(scope *sentry.Scope) {
		scope.SetTag("service", slug)
		scope.SetTag("oauth_error", reason)
		if err != nil {
			hub.CaptureException(err)
		} else {
			hub.CaptureMessage(fmt.Sprintf("oauth %s for service %q", reason, slug))
		}
	})
}

// @Summary      OAuth2 callback handler (step 9c)
// @Description  Receives the authorization code from the third-party redirect, exchanges it
//
//	for tokens, stores them encrypted, and returns a self-closing HTML page that
//	posts a message to the opener popup. No Authorization header required.
//
// @Tags         integrations
// @Produce      html
// @Param        code            query  string  false  "Authorization code from provider"
// @Param        state           query  string  false  "CSRF state token"
// @Param        accounts-server query  string  false  "Regional token endpoint base (Zoho)"
// @Success      200             "Self-closing HTML page"
// @Router       /oauth/callback [get]
func (h *Handler) oauthCallback(c *fiber.Ctx) error {
	c.Set("Content-Type", "text/html; charset=utf-8")

	// Handle provider-level errors (e.g. user denied access).
	if providerErr := c.Query("error"); providerErr != "" {
		desc := c.Query("error_description")
		captureOAuthError(c, fmt.Errorf("provider error: %s: %s", providerErr, desc), "", "provider_denied")
		return c.SendString(h.callbackHTML("", providerErr, ""))
	}

	code := c.Query("code")
	state := c.Query("state")
	if code == "" || state == "" {
		captureOAuthError(c, fmt.Errorf("callback missing code or state params"), "", "missing_code_or_state")
		return c.SendString(h.callbackHTML("", "missing_code_or_state", ""))
	}

	serviceID, slug, codeVerifier, err := h.store.GetByState(state)
	if errors.Is(err, ErrNotFound) {
		captureOAuthError(c, fmt.Errorf("state %q not found or expired", state), "", "invalid_or_expired_state")
		return c.SendString(h.callbackHTML("", "invalid_or_expired_state", ""))
	}
	if err != nil {
		captureOAuthError(c, fmt.Errorf("GetByState: %w", err), "", "server_error")
		return c.SendString(h.callbackHTML("", "server_error", ""))
	}

	def := lookupCatalog(slug)
	if def == nil {
		captureOAuthError(c, fmt.Errorf("no catalog entry for slug %q", slug), slug, "unknown_service")
		return c.SendString(h.callbackHTML(slug, "unknown_service", ""))
	}

	clientID, clientSecret, _, _, _, _, err := h.store.GetOAuthCreds(serviceID)
	if err != nil || clientID == "" {
		captureOAuthError(c, fmt.Errorf("GetOAuthCreds service %d: %w", serviceID, err), slug, "app_not_configured")
		return c.SendString(h.callbackHTML(slug, "app_not_configured", ""))
	}

	// Some providers (Zoho) return an accounts-server param that specifies the
	// regional token endpoint. Use it to build the effective TokenURL so EU/India
	// users hit the correct datacenter. Store it for future token refreshes too.
	// Also derive and store the regional API base (accounts.zoho.X → mail.zoho.X)
	// so the datasync adapter hits the correct mail endpoint.
	effectiveDef := *def
	if accountsServer := c.Query("accounts-server"); accountsServer != "" && def.TokenURL != "" {
		parsed, parseErr := url.Parse(def.TokenURL)
		if parseErr == nil {
			effectiveDef.TokenURL = accountsServer + parsed.Path
			// Best-effort persist — don't block the flow on storage error.
			if storeErr := h.store.SetOAuthTokenURLOverride(serviceID, effectiveDef.TokenURL); storeErr != nil {
				log.Printf("integrations: store token url override for %s: %v", slug, storeErr)
			}
		}
		// Derive regional mail API base: https://accounts.zoho.eu → https://mail.zoho.eu
		if mailBase := strings.Replace(accountsServer, "//accounts.", "//mail.", 1); mailBase != accountsServer {
			if storeErr := h.store.SetAPIEndpoint(serviceID, mailBase); storeErr != nil {
				log.Printf("integrations: store api endpoint for %s: %v", slug, storeErr)
			}
		}
	}

	redirectURI := h.apiBaseURL + "/oauth/callback"
	access, refresh, expiry, err := exchangeCode(c.Context(), &effectiveDef, clientID, clientSecret, code, redirectURI, codeVerifier)
	if err != nil {
		captureOAuthError(c, fmt.Errorf("token exchange for service %d (%s): %w", serviceID, slug, err), slug, "token_exchange_failed")
		return c.SendString(h.callbackHTML(slug, "token_exchange_failed", ""))
	}

	// expiry is zero for non-expiring tokens (e.g. Notion). Store 0 so the
	// refresh loop skips them (it only refreshes when OAuthTokenExpiry != 0).
	var expiryUnix int64
	if !expiry.IsZero() {
		expiryUnix = expiry.Unix()
	}
	if _, err := h.store.CompleteOAuth(serviceID, access, refresh, expiryUnix); err != nil {
		captureOAuthError(c, fmt.Errorf("CompleteOAuth service %d (%s): %w", serviceID, slug, err), slug, "storage_error")
		return c.SendString(h.callbackHTML(slug, "storage_error", ""))
	}

	return c.SendString(h.callbackHTML(slug, "", h.frontendOrigin+"/connectors?oauth=success&service="+url.QueryEscape(slug)))
}

// friendlyOAuthError maps internal error codes to user-facing messages.
func friendlyOAuthError(reason string) string {
	switch reason {
	case "token_exchange_failed":
		return "Could not complete authorization — the code may have expired. Please try again."
	case "invalid_or_expired_state":
		return "Authorization session expired. Please start the connection again."
	case "app_not_configured":
		return "App credentials are missing. Add your Client ID and Secret first."
	case "storage_error":
		return "Authorization succeeded but could not be saved. Please try again."
	case "missing_code_or_state":
		return "Invalid callback — required parameters are missing."
	case "unknown_service":
		return "Unknown service. Please contact support."
	case "server_error":
		return "An unexpected error occurred. Please try again."
	default:
		return "Connection failed. Please try again."
	}
}

// callbackHTML returns a self-closing HTML page for the OAuth popup.
// If errReason is empty, it's a success page; otherwise an error page.
// The page sends a postMessage to window.opener and calls window.close().
// If not running inside a popup, it falls back to a redirect URL.
func (h *Handler) callbackHTML(service, errReason, fallbackURL string) string {
	var msg map[string]string
	if errReason == "" {
		msg = map[string]string{"oauth": "success", "service": service}
		if fallbackURL == "" {
			fallbackURL = h.frontendOrigin + "/connectors?oauth=success&service=" + url.QueryEscape(service)
		}
	} else {
		msg = map[string]string{"oauth": "error", "service": service, "reason": errReason}
		fallbackURL = h.frontendOrigin + "/connectors?oauth=error&service=" + url.QueryEscape(service) + "&reason=" + url.QueryEscape(errReason)
	}

	msgJSON, _ := json.Marshal(msg)
	fallbackJSON, _ := json.Marshal(fallbackURL)

	status := "Connected successfully"
	if errReason != "" {
		status = friendlyOAuthError(errReason)
	}

	return fmt.Sprintf(`<!DOCTYPE html>
<html><head><meta charset="utf-8"><title>EK-1</title></head>
<body>
<p>%s — you can close this window.</p>
<script>
(function () {
  var msg = %s;
  if (window.opener) {
    try { window.opener.postMessage(msg, '*'); } catch (_) {}
    window.close();
  } else {
    window.location.replace(%s);
  }
})();
</script>
</body></html>`, status, string(msgJSON), string(fallbackJSON))
}
