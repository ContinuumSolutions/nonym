package auth
import (
	"strings"
	"github.com/gofiber/fiber/v2"
)
// JWTMiddleware validates JWT tokens for protected routes
type JWTMiddleware struct {
	jwtService *JWTService
	denylist   *TokenDenylist
}
// NewJWTMiddleware creates a new JWT middleware
func NewJWTMiddleware(jwtService *JWTService, denylist *TokenDenylist) *JWTMiddleware {
	return &JWTMiddleware{
		jwtService: jwtService,
		denylist:   denylist,
	}
}
// RequireAuth returns middleware that validates JWT tokens
func (m *JWTMiddleware) RequireAuth() fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Skip auth for public endpoints
		if m.isPublicEndpoint(c.Path()) {
			return c.Next()
		}
		// Extract token from Authorization header
		authHeader := c.Get("Authorization")
		tokenString := ExtractTokenFromHeader(authHeader)
		if tokenString == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "unauthorized",
			})
		}
		// Validate the token
		claims, err := m.jwtService.ValidateToken(tokenString)
		if err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "unauthorized",
			})
		}
		// Check if token is in the denylist (logged out)
		if claims.TokenID != "" && m.denylist.IsBlacklisted(claims.TokenID) {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "unauthorized",
			})
		}
		// Store user info in context for use by handlers
		c.Locals("user_id", claims.Subject)
		c.Locals("token_id", claims.TokenID)
		return c.Next()
	}
}
// isPublicEndpoint checks if the given path is a public endpoint that doesn't require auth
func (m *JWTMiddleware) isPublicEndpoint(path string) bool {
	publicPaths := []string{
		"/health",
		"/auth/pin/status",
		"/auth/pin/setup",
		"/auth/login",
		"/docs",                 // Swagger docs
		"/swagger",              // Swagger UI
	}
	for _, publicPath := range publicPaths {
		if path == publicPath || strings.HasPrefix(path, publicPath+"/") {
			return true
		}
	}
	// Allow Swagger/docs paths
	if strings.HasPrefix(path, "/docs") ||
	   strings.HasPrefix(path, "/swagger") ||
	   strings.Contains(path, "swagger") {
		return true
	}
	return false
}
// GetUserID extracts the user ID from the JWT claims stored in the context
func GetUserID(c *fiber.Ctx) string {
	userID := c.Locals("user_id")
	if userID == nil {
		return ""
	}
	return userID.(string)
}
// GetTokenID extracts the token ID from the JWT claims stored in the context
func GetTokenID(c *fiber.Ctx) string {
	tokenID := c.Locals("token_id")
	if tokenID == nil {
		return ""
	}
	return tokenID.(string)
}