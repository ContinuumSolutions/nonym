package main

import (
	"database/sql"
	"log"

	"github.com/egokernel/ek1/internal/activities"
	"github.com/egokernel/ek1/internal/biometrics"
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
	db, err := initDB()
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	checkInStore := biometrics.NewStore(db)
	if err := checkInStore.Migrate(); err != nil {
		log.Fatalf("biometrics migration failed: %v", err)
	}

	eventsStore := activities.NewStore(db)
	if err := eventsStore.Migrate(); err != nil {
		log.Fatalf("activities migration failed: %v", err)
	}

	app := fiber.New(fiber.Config{
		AppName: "EK-1",
	})

	app.Use(logger.New())
	app.Use(recover.New())

	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok"})
	})

	biometrics.NewHandler(checkInStore).RegisterRoutes(app)
	activities.NewHandler(eventsStore).RegisterRoutes(app)

	log.Fatal(app.Listen(":3000"))
}
