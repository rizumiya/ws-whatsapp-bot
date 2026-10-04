package handlers

import (
	"context"
	"fmt"
	"math/rand"

	"github.com/gofiber/fiber/v2"
	waBinary "go.mau.fi/whatsmeow/binary"


	"golang-whatsapp-bot/internal/whatsapp"
)

type RejectCallReq struct {
	To     string `json:"to"`
	CallID string `json:"callId"`
}

func RejectCall(c *fiber.Ctx) error {
	sessionID := c.Params("sessionId")
	session, ok := whatsapp.Manager.GetSession(sessionID)
	if !ok {
		return c.Status(404).JSON(fiber.Map{"success": false, "error": fiber.Map{"message": "Session not active"}})
	}

	var req RejectCallReq
	if err := c.BodyParser(&req); err != nil || req.To == "" || req.CallID == "" {
		return c.Status(400).JSON(fiber.Map{"success": false, "error": fiber.Map{"message": "Invalid request"}})
	}

	jid, ok := parseJID(req.To)
	if !ok {
		return c.Status(400).JSON(fiber.Map{"success": false, "error": fiber.Map{"message": "Invalid JID"}})
	}

	err := session.Client.RejectCall(context.Background(), jid, req.CallID)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "error": fiber.Map{"message": fmt.Sprintf("Failed to reject call: %v", err)}})
	}

	return c.JSON(fiber.Map{"success": true})
}

// Additional Call Features using DangerousInternals to send custom XML Node

type AcceptCallReq struct {
	To     string `json:"to"`
	CallID string `json:"callId"`
}

func AcceptCall(c *fiber.Ctx) error {
	sessionID := c.Params("sessionId")
	session, ok := whatsapp.Manager.GetSession(sessionID)
	if !ok {
		return c.Status(404).JSON(fiber.Map{"success": false, "error": fiber.Map{"message": "Session not active"}})
	}

	var req AcceptCallReq
	if err := c.BodyParser(&req); err != nil || req.To == "" || req.CallID == "" {
		return c.Status(400).JSON(fiber.Map{"success": false, "error": fiber.Map{"message": "Invalid request"}})
	}

	callCreatorJID, ok := parseJID(req.To)
	if !ok {
		return c.Status(400).JSON(fiber.Map{"success": false, "error": fiber.Map{"message": "Invalid JID"}})
	}

	ownID := session.Client.Store.ID.ToNonAD()

	// Mimicking Accept Call packet
	acceptNode := waBinary.Node{
		Tag: "accept",
		Attrs: waBinary.Attrs{
			"call-id":      req.CallID,
			"call-creator": callCreatorJID,
		},
	}

	callNode := waBinary.Node{
		Tag: "call",
		Attrs: waBinary.Attrs{
			"id":   session.Client.GenerateMessageID(),
			"from": ownID,
			"to":   callCreatorJID,
		},
		Content: []waBinary.Node{acceptNode},
	}

	err := session.Client.DangerousInternals().SendNode(context.Background(), callNode)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "error": fiber.Map{"message": fmt.Sprintf("Failed to send accept call node: %v", err)}})
	}

	return c.JSON(fiber.Map{"success": true})
}


type OfferCallReq struct {
	To        string `json:"to"`
	MediaType string `json:"mediaType"` // "audio" or "video"
}

func OfferCall(c *fiber.Ctx) error {
	sessionID := c.Params("sessionId")
	session, ok := whatsapp.Manager.GetSession(sessionID)
	if !ok {
		return c.Status(404).JSON(fiber.Map{"success": false, "error": fiber.Map{"message": "Session not active"}})
	}

	var req OfferCallReq
	if err := c.BodyParser(&req); err != nil || req.To == "" {
		return c.Status(400).JSON(fiber.Map{"success": false, "error": fiber.Map{"message": "Invalid request"}})
	}

	if req.MediaType == "" {
		req.MediaType = "audio"
	}

	jid, ok := parseJID(req.To)
	if !ok {
		return c.Status(400).JSON(fiber.Map{"success": false, "error": fiber.Map{"message": "Invalid JID"}})
	}

	ownID := session.Client.Store.ID.ToNonAD()

	// Generate 16 bytes hex for Call ID (32 chars)
	callID := generateCallID()

	// Basic mimic of call offer, not full WebRTC negotiation but triggers the event on target
	offerNode := waBinary.Node{
		Tag: "offer",
		Attrs: waBinary.Attrs{
			"call-id":      callID,
			"call-creator": ownID,
			"device-class": "0", // 0 for desktop
		},
		Content: []waBinary.Node{
			{
				Tag: "audio", // or video
				Attrs: waBinary.Attrs{
					"enc": "opus",
					"rate": "16000",
				},
			},
		},
	}

	if req.MediaType == "video" {
		offerNode.Content.([]waBinary.Node)[0].Tag = "video"
		offerNode.Content.([]waBinary.Node)[0].Attrs["enc"] = "vp8"
		delete(offerNode.Content.([]waBinary.Node)[0].Attrs, "rate")
	}

	callNode := waBinary.Node{
		Tag: "call",
		Attrs: waBinary.Attrs{
			"id":   session.Client.GenerateMessageID(),
			"from": ownID,
			"to":   jid,
		},
		Content: []waBinary.Node{offerNode},
	}

	err := session.Client.DangerousInternals().SendNode(context.Background(), callNode)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "error": fiber.Map{"message": fmt.Sprintf("Failed to send call offer node: %v", err)}})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data": fiber.Map{
			"callId": callID,
		},
	})
}

func generateCallID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%x", b)
}
