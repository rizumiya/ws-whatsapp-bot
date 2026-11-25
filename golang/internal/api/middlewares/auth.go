package middlewares

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"golang-whatsapp-bot/config"
	"golang-whatsapp-bot/internal/database"
	"strings"

	"github.com/gofiber/fiber/v2"
)

var cfg = config.LoadConfig()

func HashAPIKey(apiKey string) string {
	sum := sha256.Sum256([]byte(apiKey))
	return hex.EncodeToString(sum[:])
}

func safeCompare(a, b string) bool {
	if len(a) == 0 || len(a) != len(b) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

func extractAPIKey(c *fiber.Ctx) string {
	apiKey := c.Get("x-api-key")
	if apiKey == "" {
		auth := c.Get("Authorization")
		if strings.HasPrefix(auth, "Bearer ") {
			apiKey = strings.TrimPrefix(auth, "Bearer ")
		} else {
			apiKey = auth
		}
	}
	return strings.TrimSpace(apiKey)
}

func MasterAuth() fiber.Handler {
	return func(c *fiber.Ctx) error {
		apiKey := extractAPIKey(c)

		if !safeCompare(apiKey, cfg.MasterAPIKey) {
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
		apiKey := extractAPIKey(c)

		sessionID := c.Params("sessionId")
		if sessionID == "" {
			pathParts := strings.Split(c.Path(), "/")
			for i, p := range pathParts {
				if p == "sessions" && i+1 < len(pathParts) {
					sessionID = pathParts[i+1]
					break
				}
			}
		}

		storedKey, _, _, _, err := database.GetSessionData(sessionID)
		if err != nil {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"success": false,
				"error": fiber.Map{
					"code":    "NOT_FOUND",
					"message": "Session not found",
				},
			})
		}

		// Master key can access any existing session
		if safeCompare(apiKey, cfg.MasterAPIKey) {
			return c.Next()
		}

		hashedInput := HashAPIKey(apiKey)
		if !safeCompare(hashedInput, storedKey) && !safeCompare(apiKey, storedKey) {
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
