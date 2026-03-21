package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v2"
	// "github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/joho/godotenv"
	"github.com/ContinuumSolutions/nonym/pkg/audit"
	"github.com/ContinuumSolutions/nonym/pkg/auth"
	"github.com/ContinuumSolutions/nonym/pkg/interceptor"
	"github.com/ContinuumSolutions/nonym/pkg/ner"
	"github.com/ContinuumSolutions/nonym/pkg/router"
)

// Config holds the application configuration
type Config struct {
	Port           string
	DatabasePath   string
	LogLevel       string
	MaxConcurrency int
	Providers      map[string]ProviderConfig
}

type ProviderConfig = router.ProviderConfig

func main() {
	// Load environment variables
	if err := godotenv.Load(); err != nil {
		log.Printf("Warning: .env file not found: %v", err)
	}

	// Initialize configuration
	config := &Config{
		Port:           getEnv("PORT", "8080"),
		DatabasePath:   getEnv("DATABASE_PATH", "./data/gateway.db"),
		LogLevel:       getEnv("LOG_LEVEL", "info"),
		MaxConcurrency: 100,
		Providers: map[string]ProviderConfig{
			"openai": {
				BaseURL: "https://api.openai.com",
				Enabled: true,
			},
			"anthropic": {
				BaseURL: "https://api.anthropic.com",
				Enabled: true,
			},
			"google": {
				BaseURL: "https://generativelanguage.googleapis.com",
				Enabled: true,
			},
			"local": {
				BaseURL: getEnv("LOCAL_LLM_URL", "http://localhost:11434"),
				Enabled: true,
			},
		},
	}

	// Initialize core services
	if err := initializeServices(config); err != nil {
		log.Fatalf("Failed to initialize services: %v", err)
	}

	// Setup graceful shutdown
	_, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start servers
	errChan := make(chan error, 1)

	// Start main gateway server
	go startGatewayServer(config, errChan)

	// Wait for shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-errChan:
		log.Fatalf("Server error: %v", err)
	case sig := <-sigChan:
		log.Printf("Received signal %v, shutting down gracefully...", sig)
		cancel()

		// Give servers time to shut down gracefully
		time.Sleep(5 * time.Second)
	}
}

func initializeServices(config *Config) error {
	// Initialize NER engine
	if err := ner.Initialize(); err != nil {
		return fmt.Errorf("failed to initialize NER engine: %w", err)
	}

	// Initialize audit database
	if err := audit.Initialize(config.DatabasePath); err != nil {
		return fmt.Errorf("failed to initialize audit system: %w", err)
	}

	// Initialize events tables
	if err := audit.InitializeEventsTables(); err != nil {
		return fmt.Errorf("failed to initialize events tables: %w", err)
	}

	// Initialize compliance tables
	if err := audit.InitializeComplianceTables(); err != nil {
		return fmt.Errorf("failed to initialize compliance tables: %w", err)
	}

	// Initialize router with provider configs

	// Initialize authentication system
	if err := auth.Initialize(audit.GetDatabase()); err != nil {
		return fmt.Errorf("failed to initialize auth system: %w", err)
	}
	if err := router.Initialize(config.Providers); err != nil {
		return fmt.Errorf("failed to initialize router: %w", err)
	}

	return nil
}

func startGatewayServer(config *Config, errChan chan<- error) {
	app := fiber.New(fiber.Config{
		Prefork:               false,
		DisableStartupMessage: false,
		AppName:               "Nonym",
		ServerHeader:          "SPG/1.0",
	})

	// Middleware
	app.Use(logger.New(logger.Config{
		Format: "${time} ${status} - ${method} ${path} (${latency})\n",
	}))
	app.Use(recover.New())
	// app.Use(cors.New(cors.Config{
	// 	AllowOrigins: "*",
	// 	AllowMethods: "GET,POST,PUT,DELETE,PATCH,OPTIONS",
	// 	AllowHeaders: "Origin,Content-Type,Accept,Authorization,X-API-Key",
	// }))

	// Health check
	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status":    "healthy",
			"timestamp": time.Now().Unix(),
			"version":   "1.0.0",
		})
	})

	// Direct test route to debug routing
	app.Get("/api/v1/debug", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"message": "Direct route works!"})
	})

	// Authentication API routes - must be on gateway server since nginx proxies here
	app.Post("/api/v1/auth/login", auth.HandleLogin)
	app.Post("/api/v1/auth/register", auth.HandleRegister)
	app.Post("/api/v1/auth/logout", auth.HandleLogout)

	// API Keys management endpoints - protected routes
	authMiddleware := func(c *fiber.Ctx) error {
		fmt.Printf("*** AUTH MIDDLEWARE CALLED for %s %s ***\n", c.Method(), c.Path())
		authHeader := c.Get("Authorization")
		if authHeader == "" {
			fmt.Printf("*** AUTH MIDDLEWARE: No auth header ***\n")
			return c.Status(401).JSON(fiber.Map{"error": "Authorization header required"})
		}
		if !strings.HasPrefix(authHeader, "Bearer ") {
			fmt.Printf("*** AUTH MIDDLEWARE: Invalid auth header format ***\n")
			return c.Status(401).JSON(fiber.Map{"error": "Invalid authorization header format"})
		}
		token := authHeader[len("Bearer "):]
		user, err := auth.ValidateToken(token)
		if err != nil {
			fmt.Printf("*** AUTH MIDDLEWARE: Invalid token: %v ***\n", err)
			return c.Status(401).JSON(fiber.Map{"error": "Invalid token"})
		}
		fmt.Printf("*** AUTH MIDDLEWARE: User validated, calling next ***\n")
		c.Locals("user", user)
		c.Locals("organization_id", user.OrganizationID)
		return c.Next()
	}

	// User profile endpoint
	app.Get("/api/v1/auth/me", authMiddleware, auth.HandleGetMe)

	app.Get("/api/v1/api-keys", authMiddleware, auth.HandleGetAPIKeys)
	app.Post("/api/v1/api-keys", authMiddleware, auth.HandleCreateAPIKey)
	app.Get("/api/v1/api-keys/:id/full", authMiddleware, auth.HandleGetFullAPIKey)
	app.Patch("/api/v1/api-keys/:id/revoke", authMiddleware, auth.HandleRevokeAPIKey)
	app.Delete("/api/v1/api-keys/:id", authMiddleware, auth.HandleDeleteAPIKey)

	// Critical missing endpoints - inline implementations
	// Statistics endpoint
	app.Get("/api/v1/statistics", authMiddleware, audit.HandleGetStatisticsV1)

	// Organization management endpoints
	app.Get("/api/v1/organization", authMiddleware, auth.HandleGetOrganizationInfo)
	app.Put("/api/v1/organization", authMiddleware, auth.HandleUpdateOrganizationInfo)

	// Team management endpoints
	app.Get("/api/v1/team/members", authMiddleware, auth.HandleGetTeamMembers)
	app.Post("/api/v1/team/members", authMiddleware, auth.HandleInviteTeamMember)
	app.Delete("/api/v1/team/members/:id", authMiddleware, auth.HandleRemoveTeamMember)

	// Security endpoints
	app.Put("/api/v1/security/2fa", authMiddleware, auth.HandleUpdateTwoFactor)

	// TOTP 2FA endpoints
	app.Get("/api/v1/auth/2fa/status", authMiddleware, auth.HandleTOTPStatus)
	app.Post("/api/v1/auth/2fa/setup/begin", authMiddleware, auth.HandleTOTPSetupBegin)
	app.Post("/api/v1/auth/2fa/setup/verify", authMiddleware, auth.HandleTOTPSetupVerify)
	app.Delete("/api/v1/auth/2fa", authMiddleware, auth.HandleTOTPDisable)
	app.Post("/api/v1/auth/2fa/backup-codes/regenerate", authMiddleware, auth.HandleTOTPRegenerateBackupCodes)
	app.Post("/api/v1/auth/2fa/challenge", auth.HandleTOTPChallenge) // No auth — uses mfa_token
	app.Delete("/api/v1/security/sessions/:id", authMiddleware, auth.HandleTerminateSession)
	app.Put("/api/v1/security/settings", authMiddleware, auth.HandleUpdateSecuritySettings)

	// Compliance settings
	app.Get("/api/v1/settings/compliance", authMiddleware, audit.HandleGetComplianceSettings)
	app.Put("/api/v1/settings/compliance", authMiddleware, audit.HandleUpdateComplianceSettings)

	// Protection and analytics endpoints
	app.Get("/api/v1/protection-events", authMiddleware, audit.HandleGetProtectionEvents)
	app.Get("/api/v1/protection-stats", authMiddleware, audit.HandleGetProtectionStats)

	// Transactions endpoint
	app.Get("/api/v1/transactions", authMiddleware, audit.HandleGetTransactionsV1)

	// Dashboard API endpoints - support both JWT and API key authentication
	dashboardAuthMiddleware := func(c *fiber.Ctx) error {
		// Try JWT token first
		authHeader := c.Get("Authorization")
		if strings.HasPrefix(authHeader, "Bearer ") {
			token := authHeader[len("Bearer "):]
			user, err := auth.ValidateToken(token)
			if err == nil {
				// JWT validation successful
				c.Locals("user", user)
				c.Locals("organization_id", user.OrganizationID)
				c.Locals("auth_method", "jwt")
				return c.Next()
			}
		}

		// Try API key authentication as fallback
		apiKey := c.Get("X-API-Key")
		if apiKey == "" && strings.HasPrefix(authHeader, "Bearer ") {
			// Also check if Bearer token is actually an API key
			apiKey = strings.TrimPrefix(authHeader, "Bearer ")
		}

		if apiKey != "" {
			// Check test keys for development
			validTestKeys := map[string]int{
				"test-api-key": 1,
				"demo-key":     1,
				"dev-key":      1,
			}

			if orgID, isValid := validTestKeys[apiKey]; isValid {
				c.Locals("organization_id", orgID)
				c.Locals("auth_method", "api_key")
				return c.Next()
			}
		}

		return c.Status(401).JSON(fiber.Map{
			"error": "Authentication required. Please provide a valid JWT token or API key.",
		})
	}

	app.Get("/api/v1/dashboard/layout", dashboardAuthMiddleware, audit.HandleGetDashboardLayout)
	app.Get("/api/v1/dashboard/widgets/:widget_id", dashboardAuthMiddleware, audit.HandleGetWidgetData)

	// Main proxy endpoints (for AI providers) - NOW REQUIRING API KEY AUTHENTICATION
	// Apply API key middleware to all proxy routes for security
	app.All("/v1/chat/*", auth.APIKeyMiddleware, interceptor.HandleProxy)
	app.All("/v1/completions", auth.APIKeyMiddleware, interceptor.HandleProxy)
	app.All("/v1/embeddings", auth.APIKeyMiddleware, interceptor.HandleProxy)
	app.All("/v1/models", auth.APIKeyMiddleware, interceptor.HandleProxy)
	app.All("/v1/images/*", auth.APIKeyMiddleware, interceptor.HandleProxy)
	app.All("/v1/audio/*", auth.APIKeyMiddleware, interceptor.HandleProxy)
	app.All("/v1/files", auth.APIKeyMiddleware, interceptor.HandleProxy)
	app.All("/v1/files/*", auth.APIKeyMiddleware, interceptor.HandleProxy)
	app.All("/v1/fine-tuning/*", auth.APIKeyMiddleware, interceptor.HandleProxy)
	app.All("/v1/assistants", auth.APIKeyMiddleware, interceptor.HandleProxy)
	app.All("/v1/assistants/*", auth.APIKeyMiddleware, interceptor.HandleProxy)
	app.All("/v1/threads", auth.APIKeyMiddleware, interceptor.HandleProxy)
	app.All("/v1/threads/*", auth.APIKeyMiddleware, interceptor.HandleProxy)
	app.All("/v1/vector_stores", auth.APIKeyMiddleware, interceptor.HandleProxy)
	app.All("/v1/vector_stores/*", auth.APIKeyMiddleware, interceptor.HandleProxy)
	// Exclude auth routes - only proxy specific API patterns
	app.All("/api/chat/*", auth.APIKeyMiddleware, interceptor.HandleProxy)
	app.All("/api/completions/*", auth.APIKeyMiddleware, interceptor.HandleProxy)
	app.All("/api/models/*", auth.APIKeyMiddleware, interceptor.HandleProxy)
	app.All("/api/embeddings/*", auth.APIKeyMiddleware, interceptor.HandleProxy)

	// API Documentation routes (now served directly by nginx)
	// app.Get("/api/docs", func(c *fiber.Ctx) error {
	// 	return c.SendFile("./api-docs/index.html")
	// })
	// app.Get("/swagger.yaml", func(c *fiber.Ctx) error {
	// 	return c.SendFile("./api-docs/swagger.yaml")
	// })
	// app.Get("/docs", func(c *fiber.Ctx) error {
	// 	return c.Redirect("/api/docs")
	// })

	// Privacy gateway specific routes
	app.Get("/gateway/status", interceptor.HandleStatus)
	app.Get("/gateway/stats", interceptor.HandleStats)
	app.Get("/gateway/auth-test", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"message": "Gateway route works!"})
	})

	log.Printf("Privacy Gateway starting on port %s", config.Port)
	if err := app.Listen(":" + config.Port); err != nil {
		errChan <- fmt.Errorf("gateway server failed: %w", err)
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
