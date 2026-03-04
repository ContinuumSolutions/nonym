// @title          EK-1 Ego-Kernel API
// @version        1.0
// @description    Personal AI agent — calendar, email, finance & negotiations.
// @host           localhost:3000
// @BasePath       /
// @schemes        http
package main

import (
	"database/sql"
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
	"github.com/egokernel/ek1/internal/activities"
	"github.com/egokernel/ek1/internal/ai"
	"github.com/egokernel/ek1/internal/auth"
	"github.com/egokernel/ek1/internal/biometrics"
	"github.com/egokernel/ek1/internal/brain"
	"github.com/egokernel/ek1/internal/chat"
	"github.com/egokernel/ek1/internal/datasync"
	"github.com/egokernel/ek1/internal/execution"
	"github.com/egokernel/ek1/internal/harvest"
	"github.com/egokernel/ek1/internal/integrations"
	"github.com/egokernel/ek1/internal/ledger"
	"github.com/egokernel/ek1/internal/notifications"
	"github.com/egokernel/ek1/internal/profile"
	"github.com/egokernel/ek1/internal/scheduler"
	"github.com/gofiber/fiber/v2"

	// "github.com/gofiber/fiber/v2/middleware/cors"
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

func syncInterval() time.Duration {
	mins, _ := strconv.Atoi(os.Getenv("SYNC_INTERVAL_MINUTES"))
	if mins <= 0 {
		mins = 15
	}
	return time.Duration(mins) * time.Minute
}

func main() {
	if err := godotenv.Load(".env"); err != nil {
		log.Println("no .env file found, falling back to environment")
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

	sqliteLedger := ledger.NewSQLiteLedger(db)
	if err := sqliteLedger.Migrate(); err != nil {
		log.Fatalf("ledger migration failed: %v", err)
	}
	sqliteLedger.Initialize("ek1-kernel")

	checkInStore := biometrics.NewStore(db)
	if err := checkInStore.Migrate(); err != nil {
		log.Fatalf("biometrics migration failed: %v", err)
	}

	eventsStore := activities.NewStore(db)
	if err := eventsStore.Migrate(); err != nil {
		log.Fatalf("activities migration failed: %v", err)
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

	harvestStore := harvest.NewStore(db)
	if err := harvestStore.Migrate(); err != nil {
		log.Fatalf("harvest migration failed: %v", err)
	}

	chatHistoryStore := chat.NewHistoryStore(db)
	if err := chatHistoryStore.Migrate(); err != nil {
		log.Fatalf("chat history migration failed: %v", err)
	}

	notifsStore := notifications.NewStore(db)
	if err := notifsStore.Migrate(); err != nil {
		log.Fatalf("notifications migration failed: %v", err)
	}

	execQueueStore := execution.NewStore(db)
	if err := execQueueStore.Migrate(); err != nil {
		log.Fatalf("execution queue migration failed: %v", err)
	}

	// ── Brain ────────────────────────────────────────────────────────────────
	prof, err := profileStore.Get()
	if err != nil {
		log.Fatalf("failed to load profile: %v", err)
	}
	brainSvc := brain.NewService("ek1-kernel", prof.Preferences, sqliteLedger)

	// ── Execution engine (Stage 2) ────────────────────────────────────────────
	execEngine := execution.NewEngine(
		servicesStore,
		execution.DefaultExecutors(),
		execQueueStore,
		notifsStore,
		execution.MicroWalletThreshold(),
	)

	// ── Pipeline ─────────────────────────────────────────────────────────────
	aiClient := ai.NewClient(os.Getenv("OLLAMA_HOST"), os.Getenv("OLLAMA_MODEL"))
	syncEngine := datasync.NewEngine(servicesStore, datasync.DefaultAdapters())
	pipeline := brain.NewPipeline(brainSvc, aiClient, eventsStore, checkInStore, execEngine)

	// ── Scheduler ────────────────────────────────────────────────────────────
	sched := scheduler.NewScheduler(syncEngine, pipeline, brainSvc, notifsStore, syncInterval())
	sched.Start()

	// ── Harvest ──────────────────────────────────────────────────────────────
	harvestScanner := harvest.NewScanner(syncEngine, aiClient, eventsStore)

	// ── HTTP app ─────────────────────────────────────────────────────────────
	app := fiber.New(fiber.Config{AppName: "EK-1"})
	app.Use(logger.New())
	app.Use(recover.New())
	if os.Getenv("SENTRY_DSN") != "" {
		app.Use(sentryfiber.New(sentryfiber.Options{Repanic: true}))
	}

	// allowedOrigins := os.Getenv("CORS_ORIGINS")
	// if allowedOrigins == "" {
	// 	allowedOrigins = "http://genesis.egokernel.com:8080"
	// }
	// app.Use(cors.New(cors.Config{
	// 	AllowOrigins: allowedOrigins,
	// 	AllowHeaders: "Origin, Content-Type, Accept, Authorization",
	// 	AllowMethods: "GET, POST, PUT, DELETE, OPTIONS",
	// }))

	// @Summary      Health check
	// @Tags         system
	// @Produce      json
	// @Success      200  {object}  map[string]interface{}
	// @Router       /health [get]
	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok"})
	})

	app.Get("/swagger/*", fiberswagger.WrapHandler)

	profile.NewHandler(profileStore).RegisterRoutes(app)
	auth.NewHandler(authStore, profileStore).RegisterRoutes(app)
	brain.NewHandler(brainSvc, eventsStore).RegisterRoutes(app)
	ledger.NewHandler(sqliteLedger, "ek1-kernel").RegisterRoutes(app)
	biometrics.NewHandler(checkInStore).RegisterRoutes(app)
	activities.NewHandler(eventsStore).RegisterRoutes(app)
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
	integrations.NewHandler(servicesStore, apiBaseURL, frontendOrigin).RegisterRoutes(app)
	harvest.NewHandler(harvestScanner, harvestStore, notifsStore).RegisterRoutes(app)
	notifications.NewHandler(notifsStore).RegisterRoutes(app)
	scheduler.NewHandler(sched).RegisterRoutes(app)
	execution.NewHandler(execEngine, execQueueStore, eventsStore).RegisterRoutes(app)
	chat.NewHandler(aiClient, brainSvc, profileStore, checkInStore, eventsStore, sqliteLedger, notifsStore, harvestStore, sched, chatHistoryStore, "ek1-kernel").RegisterRoutes(app)

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
		log.Fatal(app.Listen(":3000"))
	}
}
