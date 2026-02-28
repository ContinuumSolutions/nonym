package main

import (
	"database/sql"
	"log"
	"os"

	"github.com/joho/godotenv"

	"github.com/egokernel/ek1/internal/activities"
	"github.com/egokernel/ek1/internal/biometrics"
	"github.com/egokernel/ek1/internal/integrations"
	"github.com/egokernel/ek1/internal/profile"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
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
	if err != nil {
		return nil, err
	}

	return db, nil
}

func main() {
	if err := godotenv.Load(".env-temp"); err != nil {
		log.Println("no .env-temp file found, falling back to environment")
	}

	db, err := initDB()
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	profileStore := profile.NewStore(db)
	if err := profileStore.Migrate(); err != nil {
		log.Fatalf("profile migration failed: %v", err)
	}

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

	app := fiber.New(fiber.Config{
		AppName: "EK-1",
	})

	app.Use(logger.New())
	app.Use(recover.New())

	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok"})
	})

	profile.NewHandler(profileStore).RegisterRoutes(app)
	biometrics.NewHandler(checkInStore).RegisterRoutes(app)
	activities.NewHandler(eventsStore).RegisterRoutes(app)
	integrations.NewHandler(servicesStore).RegisterRoutes(app)

	log.Fatal(app.Listen(":3000"))
}
