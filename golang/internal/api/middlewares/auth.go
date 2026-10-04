package middlewares

import (
	"golang-whatsapp-bot/config"
	"golang-whatsapp-bot/internal/database"
	"strings"

	"github.com/gofiber/fiber/v2"
)

var cfg = config.LoadConfig()

func MasterAuth() fiber.Handler {
	return func(c *fiber.Ctx) error {
		apiKey := c.Get("x-api-key")

		if apiKey != cfg.MasterAPIKey {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"success": false,
				"error": fiber.Map{
					"code":    "UNAUTHORIZED",
					"message": "Invalid Master API Key",
				},
			})
		}

		return c.Next()
	}
}

func SessionAuth() fiber.Handler {
	return func(c *fiber.Ctx) error {
		apiKey := c.Get("x-api-key")

		// Master key can access anything
		if apiKey == cfg.MasterAPIKey {
			return c.Next()
		}

		// Check session specific key
		sessionID := c.Params("sessionId")
		if sessionID == "" {
			// fallback check from URL path if params not strictly bound
			pathParts := strings.Split(c.Path(), "/")
			for i, p := range pathParts {
				if p == "sessions" && i+1 < len(pathParts) {
					sessionID = pathParts[i+1]
					break
				}
			}
		}

		storedKey, _, _, _, err := database.GetSessionData(sessionID)
		if err != nil || storedKey != apiKey {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"success": false,
				"error": fiber.Map{
					"code":    "UNAUTHORIZED",
					"message": "Invalid Session API Key",
				},
			})
		}

		return c.Next()
	}
}
