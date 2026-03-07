package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/joho/godotenv"
	"github.com/sovereignprivacy/gateway/pkg/audit"
	"github.com/sovereignprivacy/gateway/pkg/auth"
	"github.com/sovereignprivacy/gateway/pkg/interceptor"
	"github.com/sovereignprivacy/gateway/pkg/ner"
	"github.com/sovereignprivacy/gateway/pkg/router"
)

// Config holds the application configuration
type Config struct {
	Port           string
	DashboardPort  string
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
		DashboardPort:  getEnv("DASHBOARD_PORT", "8081"),
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
	errChan := make(chan error, 2)

	// Start main gateway server
	go startGatewayServer(config, errChan)

	// Start dashboard server
	go startDashboardServer(config, errChan)

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

	// Initialize events tables
	if err := audit.InitializeEventsTables(); err != nil {
		return fmt.Errorf("failed to initialize events tables: %w", err)
	}
		return fmt.Errorf("failed to initialize audit system: %w", err)
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
		AppName:               "Sovereign Privacy Gateway",
		ServerHeader:          "SPG/1.0",
	})

	// Middleware
	app.Use(logger.New(logger.Config{
		Format: "${time} ${status} - ${method} ${path} (${latency})\n",
	}))
	app.Use(recover.New())
	app.Use(cors.New(cors.Config{
		AllowOrigins: "*",
		AllowMethods: "GET,POST,PUT,DELETE,PATCH,OPTIONS",
		AllowHeaders: "Origin,Content-Type,Accept,Authorization,X-API-Key",
	}))

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

	// Simple test without /api/v1 pattern

	// Authentication routes - temporary inline handlers for testing
	// Test direct registration instead of group

	// Debug endpoint to test route registration
	app.Get("/gateway/auth/test", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"message": "Auth routes are working!"})
	})

	app.Post("/gateway/auth/login", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{"status": "ok", "endpoint": c.Path()}) })
	app.Post("/gateway/auth/register", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{"status": "ok", "endpoint": c.Path()}) })
	app.Get("/gateway/auth/me", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{"status": "ok", "endpoint": c.Path()}) })
	app.Post("/gateway/auth/logout", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{"status": "ok", "endpoint": c.Path()}) })

	// Main proxy endpoints (for AI providers) - specific patterns to avoid auth conflicts
	app.All("/v1/chat/*", interceptor.HandleProxy)
	app.All("/v1/completions", interceptor.HandleProxy)
	app.All("/v1/embeddings", interceptor.HandleProxy)
	app.All("/v1/models", interceptor.HandleProxy)
	app.All("/v1/images/*", interceptor.HandleProxy)
	app.All("/v1/audio/*", interceptor.HandleProxy)
	app.All("/v1/files", interceptor.HandleProxy)
	app.All("/v1/files/*", interceptor.HandleProxy)
	app.All("/v1/fine-tuning/*", interceptor.HandleProxy)
	app.All("/v1/assistants", interceptor.HandleProxy)
	app.All("/v1/assistants/*", interceptor.HandleProxy)
	app.All("/v1/threads", interceptor.HandleProxy)
	app.All("/v1/threads/*", interceptor.HandleProxy)
	app.All("/v1/vector_stores", interceptor.HandleProxy)
	app.All("/v1/vector_stores/*", interceptor.HandleProxy)
	// Exclude auth routes - only proxy specific API patterns
	app.All("/api/chat/*", interceptor.HandleProxy)
	app.All("/api/completions/*", interceptor.HandleProxy)
	app.All("/api/models/*", interceptor.HandleProxy)
	app.All("/api/embeddings/*", interceptor.HandleProxy)

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

func startDashboardServer(config *Config, errChan chan<- error) {
	app := fiber.New(fiber.Config{
		AppName: "SPG Dashboard",
	})

	// Middleware
	app.Use(logger.New())
	app.Use(recover.New())
	app.Use(cors.New())

	// Serve static dashboard files
	app.Static("/", "./dashboard/dist")

	// Public API routes (no auth required)
	api := app.Group("/api/v1")
	
	// Authentication routes
	api.Post("/auth/login", auth.HandleLogin)
	api.Post("/auth/register", auth.HandleRegister)
	api.Post("/auth/logout", auth.HandleLogout)

	// Protected API routes (require authentication)
	protected := api.Use(auth.AuthMiddleware)
	
	// Dashboard data
	protected.Get("/statistics", audit.HandleGetStatistics)
	protected.Get("/transactions", audit.HandleGetTransactions)
	protected.Get("/protection-events", audit.HandleGetTransactions)
	protected.Get("/protection-stats", auth.HandleProtectionStats)
	
	// Events API
	protected.Get("/events", audit.HandleGetEvents)
	protected.Get("/events/:id", audit.HandleGetEvent)
	protected.Patch("/events/:id/status", audit.HandleUpdateEventStatus)
	protected.Post("/events/webhook", audit.HandleCreateWebhook)
	protected.Get("/events/webhooks", audit.HandleGetWebhooks)
	
	// Settings
	protected.Get("/settings", audit.HandleGetSettings)
	protected.Put("/settings", audit.HandleUpdateSettings)
	
	// API Keys management
	protected.Get("/api-keys", auth.HandleGetAPIKeys)
	protected.Post("/api-keys", auth.HandleCreateAPIKey)
	protected.Patch("/api-keys/:id/revoke", auth.HandleRevokeAPIKey)
	protected.Delete("/api-keys/:id", auth.HandleDeleteAPIKey)
	
	// Provider configuration
	protected.Get("/provider-config", auth.HandleGetProviderConfig)
	protected.Put("/provider-config", auth.HandleSaveProviderConfig)
	protected.Post("/providers/:provider/test", auth.HandleTestProviderConnection)
	
	// Organization management
	protected.Get("/organization", auth.HandleGetOrganization)
	protected.Put("/organization", auth.HandleUpdateOrganization)
	
	// Team management
	protected.Get("/team/members", auth.HandleGetTeamMembers)
	protected.Post("/team/members", auth.HandleInviteTeamMember)
	protected.Delete("/team/members/:id", auth.HandleRemoveTeamMember)
	
	// Security settings
	protected.Put("/security/2fa", auth.HandleUpdateTwoFactor)
	protected.Delete("/security/sessions/:id", auth.HandleTerminateSession)
	protected.Put("/security/settings", auth.HandleUpdateSecuritySettings)
	
	// User profile
	protected.Get("/auth/me", auth.HandleGetMe)

	// WebSocket for real-time updates
	app.Get("/ws", audit.HandleWebSocket)

	log.Printf("Dashboard starting on port %s", config.DashboardPort)
	if err := app.Listen(":" + config.DashboardPort); err != nil {
		errChan <- fmt.Errorf("dashboard server failed: %w", err)
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}