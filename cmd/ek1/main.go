// @title          EK-1 Ego-Kernel API
// @version        1.0
// @description    Personal AI agent — calendar, email, finance & negotiations.
// @host           localhost:3000
// @BasePath       /
// @schemes        http
package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/getsentry/sentry-go"
	sentryfiber "github.com/getsentry/sentry-go/fiber"
	"github.com/joho/godotenv"
	"golang.org/x/crypto/acme/autocert"

	_ "github.com/egokernel/ek1/docs"
	"github.com/egokernel/ek1/internal/ai"
	"github.com/egokernel/ek1/internal/auth"
	"github.com/egokernel/ek1/internal/biometrics"
	"github.com/egokernel/ek1/internal/datasync"
	"github.com/egokernel/ek1/internal/integrations"
	"github.com/egokernel/ek1/internal/notifications"
	"github.com/egokernel/ek1/internal/profile"
	"github.com/egokernel/ek1/internal/scheduler"
	"github.com/egokernel/ek1/internal/signals"
	"github.com/gofiber/fiber/v2"

	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	fiberswagger "github.com/swaggo/fiber-swagger"
	_ "modernc.org/sqlite"
)

func initDB() (*sql.DB, error) {
	db, err := sql.Open("sqlite", "./ek1.db")
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		return nil, err
	}
	_, err = db.Exec(`PRAGMA journal_mode=WAL; PRAGMA foreign_keys=ON;`)
	return db, err
}

// newAIClient builds an ai.Client from environment variables.
//
// Tuning knobs (all optional):
//
//	OLLAMA_HOST         Ollama base URL          (default http://localhost:11434)
//	OLLAMA_MODEL        Model name               (default llama3.2)
//	OLLAMA_NUM_CTX      Context window in tokens (default: Ollama model default)
//	                    Smaller = faster on CPU; try 2048 for quick replies.
//	OLLAMA_NUM_PREDICT  Max tokens to generate   (default 400)
//	                    Reduce to 200 for faster but shorter replies.
func newAIClient() *ai.Client {
	c := ai.NewClient(os.Getenv("OLLAMA_HOST"), os.Getenv("OLLAMA_MODEL"))
	if v, _ := strconv.Atoi(os.Getenv("OLLAMA_NUM_CTX")); v > 0 {
		c.WithNumCtx(v)
	}
	if v, _ := strconv.Atoi(os.Getenv("OLLAMA_NUM_PREDICT")); v > 0 {
		c.WithNumPredict(v)
	}
	return c
}

func syncInterval() time.Duration {
	mins, _ := strconv.Atoi(os.Getenv("SYNC_INTERVAL_MINUTES"))
	if mins <= 0 {
		mins = 15
	}
	return time.Duration(mins) * time.Minute
}

// handleCLICommands processes admin CLI operations
func handleCLICommands(resetPIN, showPINStatus bool) {
	db, err := initDB()
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	pinStore := auth.NewPINStore(db)
	if err := pinStore.Migrate(); err != nil {
		log.Fatalf("pin store migration failed: %v", err)
	}

	if showPINStatus {
		configured, err := pinStore.IsConfigured()
		if err != nil {
			log.Fatalf("failed to check PIN status: %v", err)
		}
		if configured {
			fmt.Println("✅ PIN is configured")
		} else {
			fmt.Println("❌ PIN is not configured")
		}
	}

	if resetPIN {
		fmt.Print("⚠️  Are you sure you want to reset the PIN? (y/N): ")
		var response string
		fmt.Scanln(&response)
		if response == "y" || response == "Y" || response == "yes" {
			if err := pinStore.ResetPIN(); err != nil {
				log.Fatalf("failed to reset PIN: %v", err)
			}
			fmt.Println("✅ PIN has been reset successfully")
			fmt.Println("💡 You can now set up a new PIN via the frontend or API")
		} else {
			fmt.Println("❌ PIN reset cancelled")
		}
	}
}

func main() {
	// ── CLI Commands ─────────────────────────────────────────────────────────
	var resetPIN = flag.Bool("reset-pin", false, "Reset the PIN authentication")
	var showPINStatus = flag.Bool("pin-status", false, "Show PIN configuration status")
	flag.Parse()

	if err := godotenv.Load(".env"); err != nil {
		log.Println("no .env file found, falling back to environment")
	}

	// Handle CLI commands before starting server
	if *resetPIN || *showPINStatus {
		handleCLICommands(*resetPIN, *showPINStatus)
		return
	}

	if dsn := os.Getenv("SENTRY_DSN"); dsn != "" {
		if err := sentry.Init(sentry.ClientOptions{
			Dsn:              dsn,
			TracesSampleRate: 1.0,
		}); err != nil {
			log.Printf("sentry init failed: %v", err)
		} else {
			log.Println("sentry enabled")
			defer sentry.Flush(2 * time.Second)
		}
	}

	db, err := initDB()
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	// ── Stores & migrations ──────────────────────────────────────────────────
	profileStore := profile.NewStore(db)
	if err := profileStore.Migrate(); err != nil {
		log.Fatalf("profile migration failed: %v", err)
	}

	checkInStore := biometrics.NewStore(db)
	if err := checkInStore.Migrate(); err != nil {
		log.Fatalf("biometrics migration failed: %v", err)
	}

	rawKey := os.Getenv("EK1_SECRET_KEY")
	if rawKey == "" {
		log.Fatal("EK1_SECRET_KEY is required — generate one with: openssl rand -hex 32")
	}
	encKey, err := integrations.ParseKey(rawKey)
	if err != nil {
		log.Fatalf("invalid EK1_SECRET_KEY: %v", err)
	}

	servicesStore := integrations.NewStore(db, encKey)
	if err := servicesStore.Migrate(); err != nil {
		log.Fatalf("integrations migration failed: %v", err)
	}
	if err := servicesStore.Seed(); err != nil {
		log.Fatalf("integrations seed failed: %v", err)
	}

	authStore := auth.NewStore(db)
	if err := authStore.Migrate(); err != nil {
		log.Fatalf("auth migration failed: %v", err)
	}

	notifsStore := notifications.NewStore(db)
	if err := notifsStore.Migrate(); err != nil {
		log.Fatalf("notifications migration failed: %v", err)
	}

	signalsStore := signals.NewStore(db)
	if err := signalsStore.Migrate(); err != nil {
		log.Fatalf("signals store migration failed: %v", err)
	}

	// ── JWT Authentication ──────────────────────────────────────────────────
	pinStore := auth.NewPINStore(db)
	if err := pinStore.Migrate(); err != nil {
		log.Fatalf("pin store migration failed: %v", err)
	}

	jwtService, err := auth.NewJWTService("")
	if err != nil {
		log.Fatalf("JWT service initialization failed: %v", err)
	}

	tokenDenylist := auth.NewTokenDenylist()
	defer tokenDenylist.Stop() // Clean shutdown

	// ── Simple signal processing - no complex brain/execution logic ─────────

	// ── Pipeline ─────────────────────────────────────────────────────────────
	aiClient := newAIClient()

	// Inject user identity into every LLM prompt. Reads fresh from DB on each call
	// so any update via PUT /profile/identity is reflected in the next pipeline run.
	aiClient.WithIdentityProvider(func() string {
		id, err := profileStore.GetIdentity()
		if err != nil || id.IsEmpty() {
			return ""
		}
		return id.IdentityContext()
	})

	allAdapters, waAdapter := datasync.NewDefaultAdapters(os.Getenv("WHATSAPP_VERIFY_TOKEN"))
	syncEngine := datasync.NewEngine(servicesStore, allAdapters)
	signalsProcessor := signals.NewProcessor(signalsStore, aiClient, checkInStore)

	// ── Scheduler ────────────────────────────────────────────────────────────
	sched := scheduler.NewScheduler(syncEngine, signalsProcessor, notifsStore, syncInterval())
	sched.Start()

	// ── HTTP app ─────────────────────────────────────────────────────────────
	app := fiber.New(fiber.Config{AppName: "EK-1"})
	app.Use(logger.New())
	app.Use(recover.New())
	if os.Getenv("SENTRY_DSN") != "" {
		app.Use(sentryfiber.New(sentryfiber.Options{Repanic: true}))
	}

	allowedOrigins := os.Getenv("CORS_ORIGINS")
	if allowedOrigins == "" {
		allowedOrigins = "http://genesis.egokernel.com:8080"
	}
	app.Use(cors.New(cors.Config{
		AllowOrigins:     allowedOrigins,
		AllowHeaders:     "Origin, Content-Type, Accept, Authorization",
		AllowMethods:     "GET, POST, PUT, DELETE, OPTIONS",
		AllowCredentials: true,
	}))

	// @Summary      Health check
	// @Tags         system
	// @Produce      json
	// @Success      200  {object}  map[string]interface{}
	// @Router       /health [get]
	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok"})
	})

	app.Get("/swagger/*", fiberswagger.WrapHandler)

	// Simplified narratives callback - no longer depends on activities
	narrativesFn := profile.NarrativesFunc(func(limit int) []string {
		return []string{} // Simplified: return empty narratives for now
	})

	waAdapter.RegisterRoutes(app)

	// ── JWT Authentication & Authorization ────────────────────────────────
	jwtHandler := auth.NewJWTHandler(pinStore, jwtService, tokenDenylist)
	jwtMiddleware := auth.NewJWTMiddleware(jwtService, tokenDenylist)

	// Register JWT auth endpoints (public - no auth required)
	jwtHandler.RegisterJWTRoutes(app)

	// Register OAuth callback route (public - external services need access)
	domain := os.Getenv("DOMAIN")
	apiBaseURL := os.Getenv("API_BASE_URL")
	if apiBaseURL == "" {
		if domain != "" {
			apiBaseURL = "https://" + domain
		} else {
			apiBaseURL = "http://localhost:3000"
		}
	}
	frontendOrigin := os.Getenv("FRONTEND_ORIGIN")
	if frontendOrigin == "" {
		frontendOrigin = "http://localhost:8080"
	}
	integrationsHandler := integrations.NewHandler(servicesStore, apiBaseURL, frontendOrigin)
	integrationsHandler.RegisterPublicRoutes(app)

	// Apply JWT middleware to protect all routes
	app.Use(jwtMiddleware.RequireAuth())

	// ── Protected Routes (require authentication) ──────────────────────────
	profile.NewHandler(profileStore, aiClient, narrativesFn).RegisterRoutes(app)
	auth.NewHandler(authStore, profileStore).RegisterRoutes(app)
	biometrics.NewHandler(checkInStore).RegisterRoutes(app)
	integrationsHandler.RegisterRoutes(app)
	notifications.NewHandler(notifsStore).RegisterRoutes(app)
	scheduler.NewHandler(sched).RegisterRoutes(app)
	signals.NewHandler(signalsStore).RegisterRoutes(app)
	// TODO: Chat handler temporarily disabled during cleanup - can be re-enabled later
	// chat.NewHandler(...).RegisterRoutes(app)

	if domain != "" {
		cacheDir := os.Getenv("CERT_CACHE_DIR")
		if cacheDir == "" {
			cacheDir = "./certs"
		}

		m := &autocert.Manager{
			Prompt:     autocert.AcceptTOS,
			HostPolicy: autocert.HostWhitelist(domain),
			Cache:      autocert.DirCache(cacheDir),
		}

		// Port 80: serve ACME HTTP-01 challenges and redirect everything else to HTTPS.
		go func() {
			redirect := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				http.Redirect(w, r, "https://"+r.Host+r.URL.RequestURI(), http.StatusMovedPermanently)
			})
			srv := &http.Server{Addr: ":80", Handler: m.HTTPHandler(redirect)}
			log.Println("HTTP listening on :80 (ACME challenges + HTTPS redirect)")
			if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.Printf("HTTP server error: %v", err)
			}
		}()

		log.Printf("HTTPS listening on :443 for %s", domain)
		log.Fatal(app.Listener(m.Listener()))
	} else {
		port := os.Getenv("PORT")
		if port == "" {
			port = "3000"
		}
		log.Fatal(app.Listen(":" + port))
	}
}
