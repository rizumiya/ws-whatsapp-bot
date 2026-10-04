package handlers

import (
	"context"
	"fmt"

	"go.mau.fi/whatsmeow"
	"github.com/gofiber/fiber/v2"
	"go.mau.fi/whatsmeow/types"

	"golang-whatsapp-bot/internal/whatsapp"
)

type CreateGroupReq struct {
	Subject      string   `json:"subject"`
	Participants []string `json:"participants"`
}

func CreateGroup(c *fiber.Ctx) error {
	sessionID := c.Params("sessionId")
	session, ok := whatsapp.Manager.GetSession(sessionID)
	if !ok {
		return c.Status(404).JSON(fiber.Map{"success": false, "error": fiber.Map{"message": "Session not active"}})
	}

	var req CreateGroupReq
	if err := c.BodyParser(&req); err != nil || req.Subject == "" || len(req.Participants) == 0 {
		return c.Status(400).JSON(fiber.Map{"success": false, "error": fiber.Map{"message": "Invalid request"}})
	}

	var participants []types.JID
	for _, p := range req.Participants {
		jid, ok := parseJID(p)
		if ok {
			participants = append(participants, jid)
		}
	}

	resp, err := session.Client.CreateGroup(context.Background(), whatsmeow.ReqCreateGroup{Name: req.Subject, Participants: participants})
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "error": fiber.Map{"message": fmt.Sprintf("Failed to create group: %v", err)}})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data": fiber.Map{
			"jid": resp.JID.String(),
		},
	})
}

func GetGroupInfo(c *fiber.Ctx) error {
	sessionID := c.Params("sessionId")
	session, ok := whatsapp.Manager.GetSession(sessionID)
	if !ok {
		return c.Status(404).JSON(fiber.Map{"success": false, "error": fiber.Map{"message": "Session not active"}})
	}

	jid, ok := parseJID(c.Params("jid"))
	if !ok {
		return c.Status(400).JSON(fiber.Map{"success": false, "error": fiber.Map{"message": "Invalid JID"}})
	}

	info, err := session.Client.GetGroupInfo(context.Background(), jid)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "error": fiber.Map{"message": fmt.Sprintf("Failed to get group info: %v", err)}})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    info,
	})
}

type GroupParticipantsReq struct {
	Participants []string `json:"participants"`
}

func AddParticipants(c *fiber.Ctx) error {
	sessionID := c.Params("sessionId")
	session, ok := whatsapp.Manager.GetSession(sessionID)
	if !ok {
		return c.Status(404).JSON(fiber.Map{"success": false, "error": fiber.Map{"message": "Session not active"}})
	}

	jid, ok := parseJID(c.Params("jid"))
	if !ok {
		return c.Status(400).JSON(fiber.Map{"success": false, "error": fiber.Map{"message": "Invalid JID"}})
	}

	var req GroupParticipantsReq
	if err := c.BodyParser(&req); err != nil || len(req.Participants) == 0 {
		return c.Status(400).JSON(fiber.Map{"success": false, "error": fiber.Map{"message": "Invalid request"}})
	}

	var participants []types.JID
	for _, p := range req.Participants {
		pjid, ok := parseJID(p)
		if ok {
			participants = append(participants, pjid)
		}
	}

	_, err := session.Client.UpdateGroupParticipants(context.Background(), jid, participants, whatsmeow.ParticipantChangeAdd)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "error": fiber.Map{"message": fmt.Sprintf("Failed to add participants: %v", err)}})
	}

	return c.JSON(fiber.Map{"success": true})
}

func RemoveParticipants(c *fiber.Ctx) error {
	sessionID := c.Params("sessionId")
	session, ok := whatsapp.Manager.GetSession(sessionID)
	if !ok {
		return c.Status(404).JSON(fiber.Map{"success": false, "error": fiber.Map{"message": "Session not active"}})
	}

	jid, ok := parseJID(c.Params("jid"))
	if !ok {
		return c.Status(400).JSON(fiber.Map{"success": false, "error": fiber.Map{"message": "Invalid JID"}})
	}

	var req GroupParticipantsReq
	if err := c.BodyParser(&req); err != nil || len(req.Participants) == 0 {
		return c.Status(400).JSON(fiber.Map{"success": false, "error": fiber.Map{"message": "Invalid request"}})
	}

	var participants []types.JID
	for _, p := range req.Participants {
		pjid, ok := parseJID(p)
		if ok {
			participants = append(participants, pjid)
		}
	}

	_, err := session.Client.UpdateGroupParticipants(context.Background(), jid, participants, whatsmeow.ParticipantChangeRemove)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "error": fiber.Map{"message": fmt.Sprintf("Failed to remove participants: %v", err)}})
	}

	return c.JSON(fiber.Map{"success": true})
}

func PromoteParticipants(c *fiber.Ctx) error {
	sessionID := c.Params("sessionId")
	session, ok := whatsapp.Manager.GetSession(sessionID)
	if !ok {
		return c.Status(404).JSON(fiber.Map{"success": false, "error": fiber.Map{"message": "Session not active"}})
	}

	jid, ok := parseJID(c.Params("jid"))
	if !ok {
		return c.Status(400).JSON(fiber.Map{"success": false, "error": fiber.Map{"message": "Invalid JID"}})
	}

	var req GroupParticipantsReq
	if err := c.BodyParser(&req); err != nil || len(req.Participants) == 0 {
		return c.Status(400).JSON(fiber.Map{"success": false, "error": fiber.Map{"message": "Invalid request"}})
	}

	var participants []types.JID
	for _, p := range req.Participants {
		pjid, ok := parseJID(p)
		if ok {
			participants = append(participants, pjid)
		}
	}

	_, err := session.Client.UpdateGroupParticipants(context.Background(), jid, participants, whatsmeow.ParticipantChangePromote)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "error": fiber.Map{"message": fmt.Sprintf("Failed to promote participants: %v", err)}})
	}

	return c.JSON(fiber.Map{"success": true})
}

func DemoteParticipants(c *fiber.Ctx) error {
	sessionID := c.Params("sessionId")
	session, ok := whatsapp.Manager.GetSession(sessionID)
	if !ok {
		return c.Status(404).JSON(fiber.Map{"success": false, "error": fiber.Map{"message": "Session not active"}})
	}

	jid, ok := parseJID(c.Params("jid"))
	if !ok {
		return c.Status(400).JSON(fiber.Map{"success": false, "error": fiber.Map{"message": "Invalid JID"}})
	}

	var req GroupParticipantsReq
	if err := c.BodyParser(&req); err != nil || len(req.Participants) == 0 {
		return c.Status(400).JSON(fiber.Map{"success": false, "error": fiber.Map{"message": "Invalid request"}})
	}

	var participants []types.JID
	for _, p := range req.Participants {
		pjid, ok := parseJID(p)
		if ok {
			participants = append(participants, pjid)
		}
	}

	_, err := session.Client.UpdateGroupParticipants(context.Background(), jid, participants, whatsmeow.ParticipantChangeDemote)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "error": fiber.Map{"message": fmt.Sprintf("Failed to demote participants: %v", err)}})
	}

	return c.JSON(fiber.Map{"success": true})
}

type GroupSubjectReq struct {
	Subject string `json:"subject"`
}

func UpdateGroupSubject(c *fiber.Ctx) error {
	sessionID := c.Params("sessionId")
	session, ok := whatsapp.Manager.GetSession(sessionID)
	if !ok {
		return c.Status(404).JSON(fiber.Map{"success": false, "error": fiber.Map{"message": "Session not active"}})
	}

	jid, ok := parseJID(c.Params("jid"))
	if !ok {
		return c.Status(400).JSON(fiber.Map{"success": false, "error": fiber.Map{"message": "Invalid JID"}})
	}

	var req GroupSubjectReq
	if err := c.BodyParser(&req); err != nil || req.Subject == "" {
		return c.Status(400).JSON(fiber.Map{"success": false, "error": fiber.Map{"message": "Invalid request"}})
	}

	err := session.Client.SetGroupName(context.Background(), jid, req.Subject)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "error": fiber.Map{"message": fmt.Sprintf("Failed to update subject: %v", err)}})
	}

	return c.JSON(fiber.Map{"success": true})
}

type GroupDescriptionReq struct {
	Description string `json:"description"`
}

func UpdateGroupDescription(c *fiber.Ctx) error {
	sessionID := c.Params("sessionId")
	session, ok := whatsapp.Manager.GetSession(sessionID)
	if !ok {
		return c.Status(404).JSON(fiber.Map{"success": false, "error": fiber.Map{"message": "Session not active"}})
	}

	jid, ok := parseJID(c.Params("jid"))
	if !ok {
		return c.Status(400).JSON(fiber.Map{"success": false, "error": fiber.Map{"message": "Invalid JID"}})
	}

	var req GroupDescriptionReq
	if err := c.BodyParser(&req); err != nil || req.Description == "" {
		return c.Status(400).JSON(fiber.Map{"success": false, "error": fiber.Map{"message": "Invalid request"}})
	}

	// whatsmeow needs the current description ID to update it.
	info, err := session.Client.GetGroupInfo(context.Background(), jid)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "error": fiber.Map{"message": "Failed to get group info to update description"}})
	}

	err = session.Client.SetGroupTopic(context.Background(), jid, info.TopicID, info.TopicID, req.Description)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "error": fiber.Map{"message": fmt.Sprintf("Failed to update description: %v", err)}})
	}

	return c.JSON(fiber.Map{"success": true})
}

func GetGroupInviteLink(c *fiber.Ctx) error {
	sessionID := c.Params("sessionId")
	session, ok := whatsapp.Manager.GetSession(sessionID)
	if !ok {
		return c.Status(404).JSON(fiber.Map{"success": false, "error": fiber.Map{"message": "Session not active"}})
	}

	jid, ok := parseJID(c.Params("jid"))
	if !ok {
		return c.Status(400).JSON(fiber.Map{"success": false, "error": fiber.Map{"message": "Invalid JID"}})
	}

	code, err := session.Client.GetGroupInviteLink(context.Background(), jid, false)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "error": fiber.Map{"message": fmt.Sprintf("Failed to get invite link: %v", err)}})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data": fiber.Map{
			"code": code,
			"url":  "https://chat.whatsapp.com/" + code,
		},
	})
}
