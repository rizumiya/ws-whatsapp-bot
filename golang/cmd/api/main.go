package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"golang-whatsapp-bot/config"
	"golang-whatsapp-bot/internal/api/routes"
	"golang-whatsapp-bot/internal/database"
	"golang-whatsapp-bot/internal/whatsapp"
)

func main() {
	cfg := config.LoadConfig()
	database.InitDB(cfg)

	// Restore sessions from DB
	whatsapp.RestoreSessions()

	app := fiber.New(fiber.Config{
		BodyLimit: 25 * 1024 * 1024, // 25MB
	})

	app.Use(recover.New())
	app.Use(cors.New())

	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"success": true, "message": "Server is healthy"})
	})

	if _, err := os.Stat("../public"); err == nil {
		app.Static("/", "../public")
	} else if _, err := os.Stat("./public"); err == nil {
		app.Static("/", "./public")
	}

	routes.SetupRoutes(app)

	// Graceful shutdown handling
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-quit
		log.Println("Shutting down server gracefully...")
		whatsapp.Manager.DisconnectAll()
		if database.DB != nil {
			_ = database.DB.Close()
		}
		_ = app.Shutdown()
	}()

	log.Printf("Server listening on port %s", cfg.Port)
	if err := app.Listen(":" + cfg.Port); err != nil {
		log.Fatalf("Error starting server: %v", err)
	}
}
