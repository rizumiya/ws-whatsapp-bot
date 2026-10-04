package handlers

import (
	"context"
	"time"
	"fmt"

	"github.com/gofiber/fiber/v2"
	"go.mau.fi/whatsmeow/types"

	"golang-whatsapp-bot/internal/whatsapp"
)

type ReadMsgReq struct {
	To        string `json:"to"`
	MessageId string `json:"messageId"`
}

func MarkRead(c *fiber.Ctx) error {
	sessionID := c.Params("sessionId")
	session, ok := whatsapp.Manager.GetSession(sessionID)
	if !ok {
		return c.Status(404).JSON(fiber.Map{"success": false, "error": fiber.Map{"message": "Session not active"}})
	}

	var req ReadMsgReq
	if err := c.BodyParser(&req); err != nil || req.To == "" || req.MessageId == "" {
		return c.Status(400).JSON(fiber.Map{"success": false, "error": fiber.Map{"message": "Invalid request"}})
	}

	jid, ok := parseJID(req.To)
	if !ok {
		return c.Status(400).JSON(fiber.Map{"success": false, "error": fiber.Map{"message": "Invalid JID"}})
	}

	// For read receipts, Whatsmeow uses MarkRead with message IDs
	err := session.Client.MarkRead(context.Background(), []types.MessageID{req.MessageId}, time.Now(), jid, session.Client.Store.ID.ToNonAD())
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "error": fiber.Map{"message": fmt.Sprintf("Failed to mark as read: %v", err)}})
	}

	return c.JSON(fiber.Map{"success": true})
}

type PresenceReq struct {
	To       string `json:"to"`
	Presence string `json:"presence"` // composing, recording, paused
}

func UpdatePresence(c *fiber.Ctx) error {
	sessionID := c.Params("sessionId")
	session, ok := whatsapp.Manager.GetSession(sessionID)
	if !ok {
		return c.Status(404).JSON(fiber.Map{"success": false, "error": fiber.Map{"message": "Session not active"}})
	}

	var req PresenceReq
	if err := c.BodyParser(&req); err != nil || req.To == "" || req.Presence == "" {
		return c.Status(400).JSON(fiber.Map{"success": false, "error": fiber.Map{"message": "Invalid request"}})
	}

	jid, ok := parseJID(req.To)
	if !ok {
		return c.Status(400).JSON(fiber.Map{"success": false, "error": fiber.Map{"message": "Invalid JID"}})
	}

	var state types.ChatPresence
	switch req.Presence {
	case "composing":
		state = types.ChatPresenceComposing
	case "recording":
		state = types.ChatPresenceComposing // We'll differentiate by mediaType below
	case "paused":
		state = types.ChatPresencePaused
	default:
		return c.Status(400).JSON(fiber.Map{"success": false, "error": fiber.Map{"message": "Invalid presence state"}})
	}


	var mediaType types.ChatPresenceMedia
	if req.Presence == "recording" {
		mediaType = types.ChatPresenceMediaAudio
	} else {
		mediaType = types.ChatPresenceMediaText
	}
	err := session.Client.SendChatPresence(context.Background(), jid, state, mediaType)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "error": fiber.Map{"message": fmt.Sprintf("Failed to update presence: %v", err)}})
	}

	return c.JSON(fiber.Map{"success": true})
}
