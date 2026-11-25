package handlers

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"regexp"

	"github.com/gofiber/fiber/v2"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types"
	"golang-whatsapp-bot/internal/api/middlewares"
	"golang-whatsapp-bot/internal/database"
	"golang-whatsapp-bot/internal/whatsapp"
)

func GenerateAPIKey() string {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return ""
	}
	return hex.EncodeToString(bytes)
}

func GetSessions(c *fiber.Ctx) error {
	sessions, err := database.GetAllSessions()
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"success": false,
			"error":   fiber.Map{"message": "Failed to retrieve sessions"},
		})
	}
	for i := range sessions {
		if id, ok := sessions[i]["id"].(string); ok {
			sessions[i]["runtime"] = whatsapp.Manager.RuntimeState(id)
			if session, exists := whatsapp.Manager.GetSession(id); exists && session.Client.Store.ID != nil {
				sessions[i]["phone_number"] = session.Client.Store.ID.User
			}
		}
	}
	return c.JSON(fiber.Map{
		"success": true,
		"data":    sessions,
	})
}

type CreateSessionReq struct {
	SessionID  string `json:"sessionId"`
	WebhookURL string `json:"webhookUrl"`
}

func CreateSession(c *fiber.Ctx) error {
	var req CreateSessionReq
	if err := c.BodyParser(&req); err != nil || req.SessionID == "" {
		return c.Status(400).JSON(fiber.Map{"success": false, "error": fiber.Map{"message": "Invalid request"}})
	}

	match, _ := regexp.MatchString("^[a-zA-Z0-9_-]+$", req.SessionID)
	if !match || len(req.SessionID) < 3 || len(req.SessionID) > 64 {
		return c.Status(400).JSON(fiber.Map{"success": false, "error": fiber.Map{"code": "VALIDATION_ERROR", "message": "Invalid sessionId format"}})
	}

	// Check if exists
	_, _, _, _, err := database.GetSessionData(req.SessionID)
	if err == nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "error": fiber.Map{"code": "ALREADY_EXISTS", "message": "Session already exists"}})
	}

	plainAPIKey := GenerateAPIKey()
	hashedAPIKey := middlewares.HashAPIKey(plainAPIKey)
	if err := database.SaveSessionData(req.SessionID, hashedAPIKey, req.WebhookURL, "STARTING"); err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "error": fiber.Map{"message": "Failed to save session"}})
	}

	_, err = whatsapp.InitSession(req.SessionID)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "error": fiber.Map{"message": "Failed to initialize session"}})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data": fiber.Map{
			"sessionId": req.SessionID,
			"apiKey":    plainAPIKey,
			"status":    "STARTING",
		},
	})
}

func GetSessionStatus(c *fiber.Ctx) error {
	sessionID := c.Params("sessionId")
	_, webhookUrl, status, jidStr, err := database.GetSessionData(sessionID)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"success": false, "error": fiber.Map{"code": "NOT_FOUND", "message": "Session not found"}})
	}

	phoneNumber := ""
	if session, ok := whatsapp.Manager.GetSession(sessionID); ok && session.Client.Store.ID != nil {
		phoneNumber = session.Client.Store.ID.User
	} else if jidStr != "" {
		if parsed, parseErr := types.ParseJID(jidStr); parseErr == nil {
			phoneNumber = parsed.User
		}
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data": fiber.Map{
			"id":           sessionID,
			"phone_number": phoneNumber,
			"status":       status,
			"webhook_url":  webhookUrl,
			"runtime":      whatsapp.Manager.RuntimeState(sessionID),
		},
	})
}

func GetQR(c *fiber.Ctx) error {
	sessionID := c.Params("sessionId")
	session, ok := whatsapp.Manager.GetSession(sessionID)
	if !ok {
		return c.Status(404).JSON(fiber.Map{"success": false, "error": fiber.Map{"code": "NOT_FOUND", "message": "Session not active"}})
	}

	qr := session.GetLatestQR()
	if qr != "" {
		return c.JSON(fiber.Map{
			"success": true,
			"data": fiber.Map{
				"qr": qr,
			},
		})
	}

	if session.Client.IsConnected() && session.Client.IsLoggedIn() {
		return c.Status(400).JSON(fiber.Map{"success": false, "error": fiber.Map{"message": "Session already connected"}})
	}

	return c.Status(404).JSON(fiber.Map{"success": false, "error": fiber.Map{"code": "NOT_FOUND", "message": "QR not available, wait or restart"}})
}

type PairingCodeReq struct {
	PhoneNumber string `json:"phoneNumber"`
}

func GetPairingCode(c *fiber.Ctx) error {
	sessionID := c.Params("sessionId")
	session, ok := whatsapp.Manager.GetSession(sessionID)
	if !ok {
		return c.Status(400).JSON(fiber.Map{"success": false, "error": fiber.Map{"code": "NOT_ACTIVE", "message": "Session not active"}})
	}

	var req PairingCodeReq
	if err := c.BodyParser(&req); err != nil || req.PhoneNumber == "" {
		return c.Status(400).JSON(fiber.Map{"success": false, "error": fiber.Map{"message": "Invalid request"}})
	}

	code, err := session.Client.PairPhone(context.Background(), req.PhoneNumber, true, whatsmeow.PairClientChrome, "Chrome (Linux)")
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "error": fiber.Map{"code": "PAIRING_ERROR", "message": fmt.Sprintf("Failed to get pairing code: %v", err)}})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data": fiber.Map{
			"code": code,
		},
	})
}

func RestartSession(c *fiber.Ctx) error {
	sessionID := c.Params("sessionId")
	_, err := whatsapp.InitSession(sessionID)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "error": fiber.Map{"message": "Failed to restart session"}})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Session restart initiated",
	})
}

func LogoutSession(c *fiber.Ctx) error {
	sessionID := c.Params("sessionId")

	session, ok := whatsapp.Manager.GetSession(sessionID)
	if ok {
		if session.Client.IsConnected() && session.Client.IsLoggedIn() {
			if err := session.Client.Logout(context.Background()); err != nil && session.Client.Store != nil {
				_ = session.Client.Store.Delete(context.Background())
			}
		} else if session.Client.Store != nil && session.Client.Store.ID != nil {
			_ = session.Client.Store.Delete(context.Background())
		}
		whatsapp.Manager.RemoveSession(sessionID)
	} else {
		// Clean up stored device from WhatsmeowStore if session wasn't in memory
		_, _, _, jidStr, err := database.GetSessionData(sessionID)
		if err == nil && jidStr != "" {
			if jid, parseErr := types.ParseJID(jidStr); parseErr == nil {
				if dev, devErr := database.WhatsmeowStore.GetDevice(context.Background(), jid); devErr == nil && dev != nil {
					_ = dev.Delete(context.Background())
				}
			}
		}
	}

	_ = database.DeleteSessionData(sessionID)

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Session logged out and deleted",
	})
}
