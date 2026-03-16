package auth

import (
	"strings"

	"github.com/gofiber/fiber/v2"
)

// HandleRegister handles POST /api/auth/register
func HandleRegister(c *fiber.Ctx) error {
	var req SignupRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// Basic validation
	if req.Email == "" || req.Password == "" {
		return c.Status(400).JSON(fiber.Map{
			"error": "Email and password are required",
		})
	}

	if req.FirstName == "" || req.LastName == "" {
		return c.Status(400).JSON(fiber.Map{
			"error": "First name and last name are required",
		})
	}

	// Register user with atomic transaction
	user, organization, err := RegisterUser(&req)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	// Send welcome email (non-blocking)
	go func() {
		if err := SendWelcomeEmail(user, organization); err != nil {
			// Log error but don't fail the registration since it's already committed
			// In a real implementation, you'd handle email send failures gracefully
		}
	}()

	return c.Status(201).JSON(fiber.Map{
		"message":      "User registered successfully",
		"user":         user.ToProfile(),
		"organization": organization,
	})
}

// HandleLogin handles POST /api/auth/login
func HandleLogin(c *fiber.Ctx) error {
	var req LoginRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// Basic validation
	if req.Email == "" || req.Password == "" {
		return c.Status(400).JSON(fiber.Map{
			"error": "Email and password are required",
		})
	}

	// Get client info
	clientIP := c.IP()
	userAgent := c.Get("User-Agent")

	// Login user
	response, err := LoginUser(&req, clientIP, userAgent)
	if err != nil {
		return c.Status(401).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"message":      "Login successful",
		"token":        response.Token,
		"expires_at":   response.ExpiresAt,
		"user":         response.User,
		"organization": response.Organization,
	})
}

// HandleLogout handles POST /api/auth/logout
func HandleLogout(c *fiber.Ctx) error {
	// Extract token from Authorization header
	authHeader := c.Get("Authorization")
	if authHeader == "" {
		return c.Status(400).JSON(fiber.Map{
			"error": "Authorization header required",
		})
	}

	token := strings.TrimPrefix(authHeader, "Bearer ")
	if token == authHeader {
		return c.Status(400).JSON(fiber.Map{
			"error": "Invalid authorization header format. Use 'Bearer <token>'",
		})
	}

	// Logout user
	err := LogoutUser(token)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error": "Failed to logout",
		})
	}

	return c.JSON(fiber.Map{
		"message": "Logged out successfully",
	})
}

// HandleGetMe handles GET /api/auth/me
func HandleGetMe(c *fiber.Ctx) error {
	// Extract user from context (set by middleware)
	user, ok := c.Locals("user").(*User)
	if !ok {
		return c.Status(401).JSON(fiber.Map{
			"error": "Authentication required",
		})
	}

	// Get fresh user profile
	profile, err := GetUserProfile(user.ID)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error": "Failed to fetch user profile",
		})
	}

	return c.JSON(fiber.Map{
		"user": profile,
	})
}

// AuthMiddleware validates JWT tokens for protected routes
func AuthMiddleware(c *fiber.Ctx) error {
	// Extract token from Authorization header
	authHeader := c.Get("Authorization")
	if authHeader == "" {
		return c.Status(401).JSON(fiber.Map{
			"error": "Authorization header required",
		})
	}

	token := strings.TrimPrefix(authHeader, "Bearer ")
	if token == authHeader {
		return c.Status(401).JSON(fiber.Map{
			"error": "Invalid authorization header format. Use 'Bearer <token>'",
		})
	}

	// Validate token
	user, err := ValidateToken(token)
	if err != nil {
		return c.Status(401).JSON(fiber.Map{
			"error": "Invalid or expired token",
		})
	}

	// Store user and organization context for use in handlers
	c.Locals("user", user)
	c.Locals("organization_id", user.OrganizationID)

	return c.Next()
}

// AdminMiddleware ensures the user has admin role
func AdminMiddleware(c *fiber.Ctx) error {
	user, ok := c.Locals("user").(*User)
	if !ok {
		return c.Status(401).JSON(fiber.Map{
			"error": "Authentication required",
		})
	}

	if !user.IsAdmin() {
		return c.Status(403).JSON(fiber.Map{
			"error": "Admin access required",
		})
	}

	return c.Next()
}

// OwnerMiddleware ensures the user has owner role
func OwnerMiddleware(c *fiber.Ctx) error {
	user, ok := c.Locals("user").(*User)
	if !ok {
		return c.Status(401).JSON(fiber.Map{
			"error": "Authentication required",
		})
	}

	if user.Role != RoleOwner {
		return c.Status(403).JSON(fiber.Map{
			"error": "Owner access required",
		})
	}

	return c.Next()
}