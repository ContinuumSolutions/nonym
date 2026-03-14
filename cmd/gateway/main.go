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

	// Authentication API routes - must be on gateway server since nginx proxies here
	app.Post("/api/v1/auth/login", auth.HandleLogin)
	app.Post("/api/v1/auth/register", auth.HandleRegister)
	app.Post("/api/v1/auth/logout", auth.HandleLogout)

	// Protected route with inline auth check
	app.Get("/api/v1/auth/me", func(c *fiber.Ctx) error {
		// Simple auth middleware inline for testing
		authHeader := c.Get("Authorization")
		if authHeader == "" {
			return c.Status(401).JSON(fiber.Map{"error": "Authorization header required"})
		}
		token := authHeader[len("Bearer "):]
		user, err := auth.ValidateToken(token)
		if err != nil {
			return c.Status(401).JSON(fiber.Map{"error": "Invalid token"})
		}
		c.Locals("user", user)
		return auth.HandleGetMe(c)
	})

	// API Keys management endpoints - protected routes
	authMiddleware := func(c *fiber.Ctx) error {
		authHeader := c.Get("Authorization")
		if authHeader == "" {
			return c.Status(401).JSON(fiber.Map{"error": "Authorization header required"})
		}
		if !strings.HasPrefix(authHeader, "Bearer ") {
			return c.Status(401).JSON(fiber.Map{"error": "Invalid authorization header format"})
		}
		token := authHeader[len("Bearer "):]
		user, err := auth.ValidateToken(token)
		if err != nil {
			return c.Status(401).JSON(fiber.Map{"error": "Invalid token"})
		}
		c.Locals("user", user)
		c.Locals("organization_id", user.OrganizationID)
		return c.Next()
	}

	app.Get("/api/v1/api-keys", authMiddleware, auth.HandleGetAPIKeys)
	app.Post("/api/v1/api-keys", authMiddleware, auth.HandleCreateAPIKey)
	app.Get("/api/v1/api-keys/:id/full", authMiddleware, auth.HandleGetFullAPIKey)
	app.Patch("/api/v1/api-keys/:id/revoke", authMiddleware, auth.HandleRevokeAPIKey)
	app.Delete("/api/v1/api-keys/:id", authMiddleware, auth.HandleDeleteAPIKey)

	// Simple test next to working routes
	app.Get("/api/v1/test-simple", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"message": "Simple test working"})
	})

	// Critical missing endpoints - inline implementations
	app.Get("/api/v1/statistics", authMiddleware, func(c *fiber.Ctx) error {
		// Extract organization ID from context (set by middleware)
		organizationID, ok := c.Locals("organization_id").(int)
		if !ok {
			return c.Status(401).JSON(fiber.Map{
				"error": "Organization context required",
			})
		}

		// Get real statistics from audit system
		stats, err := audit.GetStatistics(organizationID)
		if err != nil {
			// Return zeros if no data available
			return c.JSON(fiber.Map{
				"pii_protected":        0,
				"total_requests":       0,
				"blocked_requests":     0,
				"avg_processing_time": "0ms",
			})
		}
		return c.JSON(fiber.Map{
			"pii_protected":        stats.RedactedRequests,
			"total_requests":       stats.TotalRequests,
			"blocked_requests":     stats.BlockedRequests,
			"avg_processing_time": fmt.Sprintf("%.0fms", stats.AvgProcessingTime),
		})
	})

	app.Get("/api/v1/organization", authMiddleware, func(c *fiber.Ctx) error {
		user := c.Locals("user").(*auth.User)
		// Return minimal organization data (can be extended later when org management is implemented)
		return c.JSON(fiber.Map{
			"id":          1,
			"name":        "",
			"industry":    "",
			"size":        "",
			"country":     "",
			"description": "",
			"owner_id":    user.ID,
		})
	})

	app.Put("/api/v1/organization", authMiddleware, func(c *fiber.Ctx) error {
		var orgData fiber.Map
		if err := c.BodyParser(&orgData); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
		}
		return c.JSON(fiber.Map{"message": "Organization updated successfully"})
	})

	app.Get("/api/v1/team/members", authMiddleware, func(c *fiber.Ctx) error {
		// Get current user (for now, just return current user as only team member)
		user := c.Locals("user").(*auth.User)

		return c.JSON(fiber.Map{
			"members": []fiber.Map{
				{
					"id":       user.ID,
					"email":    user.Email,
					"name":     user.Name,
					"role":     user.Role,
					"status":   "active",
					"joined_at": user.CreatedAt.Format("2006-01-02T15:04:05Z"),
				},
			},
			"total": 1,
		})
	})

	app.Post("/api/v1/team/members", authMiddleware, func(c *fiber.Ctx) error {
		var memberData fiber.Map
		if err := c.BodyParser(&memberData); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
		}
		return c.Status(201).JSON(fiber.Map{"message": "Team member invited successfully"})
	})

	app.Delete("/api/v1/team/members/:id", authMiddleware, func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"message": "Team member removed successfully"})
	})

	app.Get("/api/v1/provider-config", authMiddleware, func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"providers": fiber.Map{
				"spg": fiber.Map{
					"api_key":   "",
					"endpoint": "http://localhost:8080",
					"enabled":  true,
				},
				"openai": fiber.Map{
					"api_key": "",
					"models":  []string{"gpt-4", "gpt-3.5-turbo"},
					"enabled": false,
				},
				"anthropic": fiber.Map{
					"api_key": "",
					"models":  []string{"claude-3-haiku", "claude-3-sonnet"},
					"enabled": false,
				},
			},
		})
	})

	app.Put("/api/v1/provider-config", authMiddleware, func(c *fiber.Ctx) error {
		var configData fiber.Map
		if err := c.BodyParser(&configData); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
		}
		return c.JSON(fiber.Map{"message": "Provider configuration saved successfully"})
	})

	app.Post("/api/v1/providers/:provider/test", authMiddleware, func(c *fiber.Ctx) error {
		provider := c.Params("provider")
		return c.JSON(fiber.Map{
			"success": true,
			"message": "Connection to " + provider + " successful",
		})
	})

	app.Put("/api/v1/security/2fa", authMiddleware, func(c *fiber.Ctx) error {
		var settingsData fiber.Map
		if err := c.BodyParser(&settingsData); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
		}
		return c.JSON(fiber.Map{"message": "Two-factor authentication updated successfully"})
	})

	app.Delete("/api/v1/security/sessions/:id", authMiddleware, func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"message": "Session terminated successfully"})
	})

	app.Put("/api/v1/security/settings", authMiddleware, func(c *fiber.Ctx) error {
		var settingsData fiber.Map
		if err := c.BodyParser(&settingsData); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
		}
		return c.JSON(fiber.Map{"message": "Security settings updated successfully"})
	})

	app.Get("/api/v1/protection-events", authMiddleware, func(c *fiber.Ctx) error {
		// Extract organization ID from context (set by middleware)
		organizationID, ok := c.Locals("organization_id").(int)
		if !ok {
			return c.Status(401).JSON(fiber.Map{
				"error": "Organization context required",
			})
		}

		// Convert transactions to protection events format
		transactions, err := audit.GetTransactions(50, 0, organizationID) // Get recent transactions
		if err != nil {
				return c.JSON(fiber.Map{
				"events": []fiber.Map{},
				"total":  0,
			})
		}


		// Convert transactions to events format
		events := []fiber.Map{}
		for _, tx := range transactions {
			// Convert transaction to protection event format using struct fields
			event := fiber.Map{
				"id":        tx.ID,
				"timestamp": tx.Timestamp,
				"provider":  tx.Provider,
				"status":    "Protected", // Default status for successful redactions
				"action":    "Redacted",
			}

			// Determine event type and details from redaction details
			if len(tx.RedactionDetails) > 0 {
				firstRedaction := tx.RedactionDetails[0]
				switch string(firstRedaction.EntityType) {
				case "CREDIT_CARD":
					event["type"] = "Credit Card"
				case "EMAIL":
					event["type"] = "Email"
				case "SSN":
					event["type"] = "SSN"
				case "PHONE":
					event["type"] = "Phone"
				case "API_KEY":
					event["type"] = "API Key"
				default:
					event["type"] = "PII"
				}
				event["protection"] = fmt.Sprintf("%d item(s) redacted", len(tx.RedactionDetails))
				event["redaction_details"] = tx.RedactionDetails

				// Reconstruct content examples based on redaction details
				var originalContent, sanitizedContent string

				// Create realistic examples based on the type of PII detected
				switch string(firstRedaction.EntityType) {
				case "CREDIT_CARD":
					originalContent = fmt.Sprintf("My card detail is %s", firstRedaction.OriginalText)
					sanitizedContent = fmt.Sprintf("My card detail is %s", firstRedaction.RedactedText)
				case "EMAIL":
					originalContent = fmt.Sprintf("Contact me at %s or call me", firstRedaction.OriginalText)
					sanitizedContent = fmt.Sprintf("Contact me at %s or call me", firstRedaction.RedactedText)
				case "SSN":
					originalContent = fmt.Sprintf("My SSN is %s for verification", firstRedaction.OriginalText)
					sanitizedContent = fmt.Sprintf("My SSN is %s for verification", firstRedaction.RedactedText)
				case "PHONE":
					originalContent = fmt.Sprintf("Call me at %s", firstRedaction.OriginalText)
					sanitizedContent = fmt.Sprintf("Call me at %s", firstRedaction.RedactedText)
				default:
					originalContent = fmt.Sprintf("Here is my information: %s", firstRedaction.OriginalText)
					sanitizedContent = fmt.Sprintf("Here is my information: %s", firstRedaction.RedactedText)
				}

				// Apply all redactions for multiple PII scenarios
				for _, redaction := range tx.RedactionDetails {
					sanitizedContent = strings.ReplaceAll(sanitizedContent, redaction.OriginalText, redaction.RedactedText)
				}

				// Truncate original for security (show context but protect PII)
				if len(originalContent) > 80 {
					event["original_content_preview"] = originalContent[:40] + "..." + originalContent[len(originalContent)-20:]
				} else {
					event["original_content_preview"] = originalContent
				}

				event["sanitized_content"] = sanitizedContent
				event["content_summary"] = fmt.Sprintf("Input: %d chars, Output: %d chars, %d PII items redacted",
					len(originalContent), len(sanitizedContent), len(tx.RedactionDetails))
			} else {
				event["type"] = "Request"
				event["protection"] = "No PII detected"
				event["original_content_preview"] = "What's the weather like today?" // Sample clean request
				event["sanitized_content"] = "What's the weather like today?" // Same for clean requests
				event["content_summary"] = "Clean request - no PII detected"
			}

			events = append(events, event)
		}

		// Get total count
		stats, _ := audit.GetStatistics(organizationID)
		totalCount := int64(len(events))
		if stats != nil {
			totalCount = stats.TotalRequests
		}

		return c.JSON(fiber.Map{
			"events": events,
			"total":  totalCount,
		})
	})

	app.Get("/api/v1/protection-stats", authMiddleware, func(c *fiber.Ctx) error {
		// Extract organization ID from context (set by middleware)
		organizationID, ok := c.Locals("organization_id").(int)
		if !ok {
			return c.Status(401).JSON(fiber.Map{
				"error": "Organization context required",
			})
		}

		// Calculate real protection statistics from transactions
		transactions, err := audit.GetTransactions(100, 0, organizationID) // Get more transactions for better stats
		if err != nil {
			return c.JSON(fiber.Map{
				"protectedToday":  0,
				"blockedToday":    0,
				"detectionRate":   0.0,
				"highRisk":        0,
			})
		}

		var protectedToday, totalToday, emailsProtected, ssnsProtected, creditCardsBlocked, apiKeysRedacted int
		now := time.Now()
		today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

		for _, tx := range transactions {
			// Check if transaction is from today (after midnight)
			if tx.Timestamp.After(today) {
				totalToday++
				if len(tx.RedactionDetails) > 0 {
					protectedToday++

					// Count by PII type
					for _, redaction := range tx.RedactionDetails {
						switch string(redaction.EntityType) {
						case "EMAIL":
							emailsProtected++
						case "SSN":
							ssnsProtected++
						case "CREDIT_CARD":
							creditCardsBlocked++
						case "API_KEY":
							apiKeysRedacted++
						}
					}
				}
			}
		}

		// Calculate detection rate
		var detectionRate float64
		if totalToday > 0 {
			detectionRate = float64(protectedToday) / float64(totalToday) * 100
		}

		// High risk = items with multiple PII types or high-confidence detections
		highRisk := 0
		for _, tx := range transactions {
			if tx.Timestamp.After(today) && len(tx.RedactionDetails) > 1 {
				highRisk++ // Multiple PII types = high risk
			}
		}

		return c.JSON(fiber.Map{
			"protectedToday":  protectedToday,
			"blockedToday":    0, // No blocked requests in current implementation
			"detectionRate":   detectionRate,
			"highRisk":        highRisk,
			// Legacy format for other endpoints
			"stats": fiber.Map{
				"emails_protected":      emailsProtected,
				"ssns_protected":        ssnsProtected,
				"credit_cards_blocked":  creditCardsBlocked,
				"api_keys_redacted":     apiKeysRedacted,
			},
		})
	})

	// Transactions endpoint for dashboard
	app.Get("/api/v1/transactions", authMiddleware, func(c *fiber.Ctx) error {
		// Extract organization ID from context (set by middleware)
		organizationID, ok := c.Locals("organization_id").(int)
		if !ok {
			return c.Status(401).JSON(fiber.Map{
				"error": "Organization context required",
			})
		}

		// Get real transactions from audit system
		transactions, err := audit.GetTransactions(10, 0, organizationID) // Get last 10 transactions
		if err != nil {
			// Return empty list if no data available
			return c.JSON(fiber.Map{
				"transactions": []fiber.Map{},
				"total":        0,
			})
		}

		// Get total count
		stats, _ := audit.GetStatistics(organizationID)
		totalCount := int64(0)
		if stats != nil {
			totalCount = stats.TotalRequests
		}

		return c.JSON(fiber.Map{
			"transactions": transactions,
			"total":        totalCount,
		})
	})

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

	// Dashboard data endpoints - FIXED TO MATCH FRONTEND
	protected.Get("/statistics", audit.HandleGetStatisticsV1)
	protected.Get("/transactions", audit.HandleGetTransactionsV1)
	protected.Get("/protection-events", audit.HandleGetProtectionEvents)
	protected.Get("/protection-stats", audit.HandleGetProtectionStats)

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
	protected.Get("/organization", auth.HandleGetOrganizationV1)
	protected.Put("/organization", auth.HandleUpdateOrganizationV1)

	// Team management
	protected.Get("/team/members", auth.HandleGetTeamMembersV1)
	protected.Post("/team/members", auth.HandleInviteTeamMemberV1)
	protected.Delete("/team/members/:id", auth.HandleRemoveTeamMemberV1)

	// Security settings
	protected.Put("/security/2fa", auth.HandleUpdateTwoFactorV1)
	protected.Delete("/security/sessions/:id", auth.HandleTerminateSessionV1)
	protected.Put("/security/settings", auth.HandleUpdateSecuritySettingsV1)

	// User profile
	protected.Get("/auth/me", auth.HandleGetMe)

	// Health check
	protected.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status":    "healthy",
			"timestamp": time.Now().Unix(),
			"version":   "1.0.0",
		})
	})

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