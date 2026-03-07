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
		return fmt.Errorf("failed to initialize audit system: %w", err)
	}

	// Initialize router with provider configs
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

	// Main proxy endpoints
	app.All("/v1/*", interceptor.HandleProxy)
	app.All("/api/*", interceptor.HandleProxy)

	// Privacy gateway specific routes
	app.Get("/gateway/status", interceptor.HandleStatus)
	app.Get("/gateway/stats", interceptor.HandleStats)

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
	app.Static("/", "./dashboard/public")

	// Dashboard API routes
	api := app.Group("/api")
	api.Get("/transactions", audit.HandleGetTransactions)
	api.Get("/statistics", audit.HandleGetStatistics)
	api.Get("/settings", audit.HandleGetSettings)
	api.Put("/settings", audit.HandleUpdateSettings)

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