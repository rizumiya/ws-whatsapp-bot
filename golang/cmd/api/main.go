package main

import (
	"log"

	"github.com/gofiber/fiber/v2"
	"golang-whatsapp-bot/config"
	"golang-whatsapp-bot/internal/database"
	"golang-whatsapp-bot/internal/whatsapp"
	"golang-whatsapp-bot/internal/api/routes"
)

func main() {
	cfg := config.LoadConfig()
	database.InitDB(cfg)

	// Restore sessions from DB
	whatsapp.RestoreSessions()

	app := fiber.New(fiber.Config{
		BodyLimit: 25 * 1024 * 1024, // 25MB
	})

	routes.SetupRoutes(app)

	log.Printf("Server listening on port %s", cfg.Port)
	if err := app.Listen(":" + cfg.Port); err != nil {
		log.Fatalf("Error starting server: %v", err)
	}
}
