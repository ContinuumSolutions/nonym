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
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"

	_ "github.com/egokernel/ek1/docs"
	"github.com/egokernel/ek1/internal/activities"
	"github.com/egokernel/ek1/internal/ai"
	"github.com/egokernel/ek1/internal/biometrics"
	"github.com/egokernel/ek1/internal/brain"
	"github.com/egokernel/ek1/internal/chat"
	"github.com/egokernel/ek1/internal/datasync"
	"github.com/egokernel/ek1/internal/harvest"
	"github.com/egokernel/ek1/internal/integrations"
	"github.com/egokernel/ek1/internal/ledger"
	"github.com/egokernel/ek1/internal/notifications"
	"github.com/egokernel/ek1/internal/profile"
	"github.com/egokernel/ek1/internal/scheduler"
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

	// ── Brain ────────────────────────────────────────────────────────────────
	prof, err := profileStore.Get()
	if err != nil {
		log.Fatalf("failed to load profile: %v", err)
	}
	brainSvc := brain.NewService("ek1-kernel", prof.Preferences, sqliteLedger)

	// ── Pipeline ─────────────────────────────────────────────────────────────
	aiClient := ai.NewClient(os.Getenv("OLLAMA_HOST"), os.Getenv("OLLAMA_MODEL"))
	syncEngine := datasync.NewEngine(servicesStore, datasync.DefaultAdapters())
	pipeline := brain.NewPipeline(brainSvc, aiClient, eventsStore, checkInStore)

	// ── Scheduler ────────────────────────────────────────────────────────────
	sched := scheduler.NewScheduler(syncEngine, pipeline, brainSvc, notifsStore, syncInterval())
	sched.Start()

	// ── Harvest ──────────────────────────────────────────────────────────────
	harvestScanner := harvest.NewScanner(syncEngine, aiClient, eventsStore)

	// ── HTTP app ─────────────────────────────────────────────────────────────
	app := fiber.New(fiber.Config{AppName: "EK-1"})
	app.Use(logger.New())
	app.Use(recover.New())

	allowedOrigins := os.Getenv("CORS_ORIGINS")
	if allowedOrigins == "" {
		allowedOrigins = "http://genesis.egokernel.com:8080"
	}
	app.Use(cors.New(cors.Config{
		AllowOrigins: allowedOrigins,
		AllowHeaders: "Origin, Content-Type, Accept, Authorization",
		AllowMethods: "GET, POST, PUT, DELETE, OPTIONS",
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

	profile.NewHandler(profileStore).RegisterRoutes(app)
	brain.NewHandler(brainSvc, eventsStore).RegisterRoutes(app)
	ledger.NewHandler(sqliteLedger, "ek1-kernel").RegisterRoutes(app)
	biometrics.NewHandler(checkInStore).RegisterRoutes(app)
	activities.NewHandler(eventsStore).RegisterRoutes(app)
	integrations.NewHandler(servicesStore).RegisterRoutes(app)
	harvest.NewHandler(harvestScanner, harvestStore, notifsStore).RegisterRoutes(app)
	notifications.NewHandler(notifsStore).RegisterRoutes(app)
	scheduler.NewHandler(sched).RegisterRoutes(app)
	chat.NewHandler(aiClient, brainSvc, profileStore, checkInStore, eventsStore, sqliteLedger, notifsStore, harvestStore, sched, chatHistoryStore, "ek1-kernel").RegisterRoutes(app)

	log.Fatal(app.Listen(":3000"))
}
