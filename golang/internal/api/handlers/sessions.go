package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"regexp"

	"github.com/gofiber/fiber/v2"
	"context"
	"go.mau.fi/whatsmeow"
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
			"error": fiber.Map{"message": "Failed to retrieve sessions"},
		})
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
		return c.Status(400).JSON(fiber.Map{"success": false, "error": fiber.Map{"message": "Invalid sessionId format"}})
	}

	// Check if exists
	_, _, _, _, err := database.GetSessionData(req.SessionID)
	if err == nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "error": fiber.Map{"message": "Session already exists"}})
	}

	apiKey := GenerateAPIKey()
	database.SaveSessionData(req.SessionID, apiKey, req.WebhookURL, "STARTING")

	_, err = whatsapp.InitSession(req.SessionID)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "error": fiber.Map{"message": "Failed to initialize session"}})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data": fiber.Map{
			"sessionId": req.SessionID,
			"apiKey":    apiKey,
			"status":    "STARTING",
		},
	})
}

func GetSessionStatus(c *fiber.Ctx) error {
	sessionID := c.Params("sessionId")
	_, webhookUrl, status, _, err := database.GetSessionData(sessionID)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"success": false, "error": fiber.Map{"message": "Session not found"}})
	}

	// Try to get phone number from client
	phoneNumber := ""
	if session, ok := whatsapp.Manager.GetSession(sessionID); ok && session.Client.Store.ID != nil {
		phoneNumber = session.Client.Store.ID.User
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data": fiber.Map{
			"id":           sessionID,
			"phone_number": phoneNumber,
			"status":       status,
			"webhook_url":  webhookUrl,
		},
	})
}

func GetQR(c *fiber.Ctx) error {
	sessionID := c.Params("sessionId")
	session, ok := whatsapp.Manager.GetSession(sessionID)
	if !ok {
		return c.Status(404).JSON(fiber.Map{"success": false, "error": fiber.Map{"message": "Session not active"}})
	}

	// Try to grab the latest QR if QR channel is active
	// Note: In whatsmeow, QR codes are pushed via channel.
	// We need a mechanism to store the latest QR or fetch it.
	// For simplicity, we can fetch from the channel non-blocking if possible.

	if session.LatestQR != "" {
		return c.JSON(fiber.Map{
			"success": true,
			"data": fiber.Map{
				"qr": session.LatestQR,
			},
		})
	}

	if session.Client.IsConnected() && session.Client.IsLoggedIn() {
		return c.Status(400).JSON(fiber.Map{"success": false, "error": fiber.Map{"message": "Session already connected"}})
	}

	return c.Status(404).JSON(fiber.Map{"success": false, "error": fiber.Map{"message": "QR not available, wait or restart"}})
}

type PairingCodeReq struct {
	PhoneNumber string `json:"phoneNumber"`
}

func GetPairingCode(c *fiber.Ctx) error {
	sessionID := c.Params("sessionId")
	session, ok := whatsapp.Manager.GetSession(sessionID)
	if !ok {
		return c.Status(404).JSON(fiber.Map{"success": false, "error": fiber.Map{"message": "Session not active"}})
	}

	var req PairingCodeReq
	if err := c.BodyParser(&req); err != nil || req.PhoneNumber == "" {
		return c.Status(400).JSON(fiber.Map{"success": false, "error": fiber.Map{"message": "Invalid request"}})
	}

	code, err := session.Client.PairPhone(context.Background(), req.PhoneNumber, true, whatsmeow.PairClientChrome, "Chrome (Linux)")
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "error": fiber.Map{"message": fmt.Sprintf("Failed to get pairing code: %v", err)}})
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
	session, ok := whatsapp.Manager.GetSession(sessionID)
	if ok {
		session.Client.Disconnect()
		err := session.Client.Connect()
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"success": false, "error": fiber.Map{"message": "Failed to reconnect"}})
		}
	} else {
		// Not in memory, try to re-init
		_, err := whatsapp.InitSession(sessionID)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"success": false, "error": fiber.Map{"message": "Failed to init session"}})
		}
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
		session.Client.Logout(context.Background())
		whatsapp.Manager.RemoveSession(sessionID)
	}

	database.DeleteSessionData(sessionID)

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Session logged out and deleted",
	})
}
