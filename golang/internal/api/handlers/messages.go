package handlers

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/gofiber/fiber/v2"
	"go.mau.fi/whatsmeow"
	waProto "go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"google.golang.org/protobuf/proto"

	"golang-whatsapp-bot/internal/whatsapp"
)

func parseJID(arg string) (types.JID, bool) {
	if arg == "" {
		return types.EmptyJID, false
	}
	if !strings.Contains(arg, "@") {
		arg += "@s.whatsapp.net"
	}
	recipient, err := types.ParseJID(arg)
	if err != nil {
		return types.EmptyJID, false
	}
	if recipient.User == "" {
		return types.EmptyJID, false
	}
	return recipient, true
}

func getContextInfo(quotedMessageId string) *waProto.ContextInfo {
	if quotedMessageId == "" {
		return nil
	}
	return &waProto.ContextInfo{
		StanzaID: proto.String(quotedMessageId),
	}
}

type TextMsgReq struct {
	To              string `json:"to"`
	Text            string `json:"text"`
	QuotedMessageId string `json:"quotedMessageId"`
}

func SendText(c *fiber.Ctx) error {
	sessionID := c.Params("sessionId")
	session, ok := whatsapp.Manager.GetSession(sessionID)
	if !ok {
		return c.Status(404).JSON(fiber.Map{"success": false, "error": fiber.Map{"message": "Session not active"}})
	}

	var req TextMsgReq
	if err := c.BodyParser(&req); err != nil || req.To == "" || req.Text == "" {
		return c.Status(400).JSON(fiber.Map{"success": false, "error": fiber.Map{"message": "Invalid request"}})
	}

	jid, ok := parseJID(req.To)
	if !ok {
		return c.Status(400).JSON(fiber.Map{"success": false, "error": fiber.Map{"message": "Invalid JID"}})
	}

	msg := &waProto.Message{
		ExtendedTextMessage: &waProto.ExtendedTextMessage{
			Text:        proto.String(req.Text),
			ContextInfo: getContextInfo(req.QuotedMessageId),
		},
	}

	resp, err := session.Client.SendMessage(context.Background(), jid, msg)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "error": fiber.Map{"message": fmt.Sprintf("Failed to send text: %v", err)}})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data": fiber.Map{
			"messageId": resp.ID,
		},
	})
}

type MediaMsgReq struct {
	To              string `json:"to"`
	Type            string `json:"type"` // image, video, audio, document, sticker
	URL             string `json:"url"`
	Base64          string `json:"base64"`
	Caption         string `json:"caption"`
	FileName        string `json:"fileName"`
	Mimetype        string `json:"mimetype"`
	Ptt             bool   `json:"ptt"`
	QuotedMessageId string `json:"quotedMessageId"`
}

func SendMedia(c *fiber.Ctx) error {
	sessionID := c.Params("sessionId")
	session, ok := whatsapp.Manager.GetSession(sessionID)
	if !ok {
		return c.Status(404).JSON(fiber.Map{"success": false, "error": fiber.Map{"message": "Session not active"}})
	}

	var req MediaMsgReq
	if err := c.BodyParser(&req); err != nil || req.To == "" || req.Type == "" {
		return c.Status(400).JSON(fiber.Map{"success": false, "error": fiber.Map{"message": "Invalid request"}})
	}

	jid, ok := parseJID(req.To)
	if !ok {
		return c.Status(400).JSON(fiber.Map{"success": false, "error": fiber.Map{"message": "Invalid JID"}})
	}

	var mediaData []byte
	var err error
	if req.Base64 != "" {
		b64 := req.Base64
		// strip data:image/png;base64, prefix if present
		if strings.Contains(b64, "base64,") {
			parts := strings.SplitN(b64, "base64,", 2)
			b64 = parts[1]
		}
		mediaData, err = base64.StdEncoding.DecodeString(b64)
		if err != nil {
			return c.Status(400).JSON(fiber.Map{"success": false, "error": fiber.Map{"message": "Invalid base64 string"}})
		}
	} else if req.URL != "" {
		resp, err := http.Get(req.URL)
		if err != nil || resp.StatusCode != 200 {
			return c.Status(400).JSON(fiber.Map{"success": false, "error": fiber.Map{"message": "Failed to fetch media from URL"}})
		}
		defer resp.Body.Close()
		mediaData, err = io.ReadAll(resp.Body)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"success": false, "error": fiber.Map{"message": "Failed to read media body"}})
		}
		if req.Mimetype == "" {
			req.Mimetype = resp.Header.Get("Content-Type")
		}
	} else {
		return c.Status(400).JSON(fiber.Map{"success": false, "error": fiber.Map{"message": "Must provide url or base64"}})
	}

	if req.Mimetype == "" {
		req.Mimetype = http.DetectContentType(mediaData)
	}

	var waMediaType whatsmeow.MediaType
	switch req.Type {
	case "image": waMediaType = whatsmeow.MediaImage
	case "video": waMediaType = whatsmeow.MediaVideo
	case "audio": waMediaType = whatsmeow.MediaAudio
	case "document": waMediaType = whatsmeow.MediaDocument
	default: waMediaType = whatsmeow.MediaDocument // fallback
	}

	uploaded, err := session.Client.Upload(context.Background(), mediaData, waMediaType)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "error": fiber.Map{"message": fmt.Sprintf("Failed to upload media: %v", err)}})
	}

	msg := &waProto.Message{}
	ctxInfo := getContextInfo(req.QuotedMessageId)

	switch req.Type {
	case "image":
		msg.ImageMessage = &waProto.ImageMessage{
			Caption:       proto.String(req.Caption),
			Mimetype:      proto.String(req.Mimetype),
			URL:           &uploaded.URL,
			DirectPath:    &uploaded.DirectPath,
			MediaKey:      uploaded.MediaKey,
			FileEncSHA256: uploaded.FileEncSHA256,
			FileSHA256:    uploaded.FileSHA256,
			FileLength:    proto.Uint64(uint64(len(mediaData))),
			ContextInfo:   ctxInfo,
		}
	case "video":
		msg.VideoMessage = &waProto.VideoMessage{
			Caption:       proto.String(req.Caption),
			Mimetype:      proto.String(req.Mimetype),
			URL:           &uploaded.URL,
			DirectPath:    &uploaded.DirectPath,
			MediaKey:      uploaded.MediaKey,
			FileEncSHA256: uploaded.FileEncSHA256,
			FileSHA256:    uploaded.FileSHA256,
			FileLength:    proto.Uint64(uint64(len(mediaData))),
			ContextInfo:   ctxInfo,
		}
	case "audio":
		msg.AudioMessage = &waProto.AudioMessage{
			PTT:           proto.Bool(req.Ptt),
			Mimetype:      proto.String(req.Mimetype),
			URL:           &uploaded.URL,
			DirectPath:    &uploaded.DirectPath,
			MediaKey:      uploaded.MediaKey,
			FileEncSHA256: uploaded.FileEncSHA256,
			FileSHA256:    uploaded.FileSHA256,
			FileLength:    proto.Uint64(uint64(len(mediaData))),
			ContextInfo:   ctxInfo,
		}
	case "document":
		msg.DocumentMessage = &waProto.DocumentMessage{
			Title:         proto.String(req.FileName),
			FileName:      proto.String(req.FileName),
			Mimetype:      proto.String(req.Mimetype),
			URL:           &uploaded.URL,
			DirectPath:    &uploaded.DirectPath,
			MediaKey:      uploaded.MediaKey,
			FileEncSHA256: uploaded.FileEncSHA256,
			FileSHA256:    uploaded.FileSHA256,
			FileLength:    proto.Uint64(uint64(len(mediaData))),
			ContextInfo:   ctxInfo,
		}
	case "sticker":
		msg.StickerMessage = &waProto.StickerMessage{
			Mimetype:      proto.String(req.Mimetype),
			URL:           &uploaded.URL,
			DirectPath:    &uploaded.DirectPath,
			MediaKey:      uploaded.MediaKey,
			FileEncSHA256: uploaded.FileEncSHA256,
			FileSHA256:    uploaded.FileSHA256,
			FileLength:    proto.Uint64(uint64(len(mediaData))),
			ContextInfo:   ctxInfo,
		}
	}

	resp, err := session.Client.SendMessage(context.Background(), jid, msg)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "error": fiber.Map{"message": fmt.Sprintf("Failed to send media: %v", err)}})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data": fiber.Map{
			"messageId": resp.ID,
		},
	})
}

type LocationMsgReq struct {
	To        string  `json:"to"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Name      string  `json:"name"`
	Address   string  `json:"address"`
}

func SendLocation(c *fiber.Ctx) error {
	sessionID := c.Params("sessionId")
	session, ok := whatsapp.Manager.GetSession(sessionID)
	if !ok {
		return c.Status(404).JSON(fiber.Map{"success": false, "error": fiber.Map{"message": "Session not active"}})
	}

	var req LocationMsgReq
	if err := c.BodyParser(&req); err != nil || req.To == "" || req.Latitude == 0 || req.Longitude == 0 {
		return c.Status(400).JSON(fiber.Map{"success": false, "error": fiber.Map{"message": "Invalid request"}})
	}

	jid, ok := parseJID(req.To)
	if !ok {
		return c.Status(400).JSON(fiber.Map{"success": false, "error": fiber.Map{"message": "Invalid JID"}})
	}

	msg := &waProto.Message{
		LocationMessage: &waProto.LocationMessage{
			DegreesLatitude:  proto.Float64(req.Latitude),
			DegreesLongitude: proto.Float64(req.Longitude),
			Name:             proto.String(req.Name),
			Address:          proto.String(req.Address),
		},
	}

	resp, err := session.Client.SendMessage(context.Background(), jid, msg)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "error": fiber.Map{"message": fmt.Sprintf("Failed to send location: %v", err)}})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data": fiber.Map{
			"messageId": resp.ID,
		},
	})
}

type ReactMsgReq struct {
	To        string `json:"to"`
	MessageId string `json:"messageId"`
	Emoji     string `json:"emoji"`
}

func SendReaction(c *fiber.Ctx) error {
	sessionID := c.Params("sessionId")
	session, ok := whatsapp.Manager.GetSession(sessionID)
	if !ok {
		return c.Status(404).JSON(fiber.Map{"success": false, "error": fiber.Map{"message": "Session not active"}})
	}

	var req ReactMsgReq
	if err := c.BodyParser(&req); err != nil || req.To == "" || req.MessageId == "" {
		return c.Status(400).JSON(fiber.Map{"success": false, "error": fiber.Map{"message": "Invalid request"}})
	}

	jid, ok := parseJID(req.To)
	if !ok {
		return c.Status(400).JSON(fiber.Map{"success": false, "error": fiber.Map{"message": "Invalid JID"}})
	}

	_, err := session.Client.SendMessage(context.Background(), jid, session.Client.BuildReaction(jid, types.EmptyJID, req.MessageId, req.Emoji))
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "error": fiber.Map{"message": fmt.Sprintf("Failed to react: %v", err)}})
	}

	return c.JSON(fiber.Map{
		"success": true,
	})
}

type RevokeMsgReq struct {
	To        string `json:"to"`
	MessageId string `json:"messageId"`
}

func RevokeMessage(c *fiber.Ctx) error {
	sessionID := c.Params("sessionId")
	session, ok := whatsapp.Manager.GetSession(sessionID)
	if !ok {
		return c.Status(404).JSON(fiber.Map{"success": false, "error": fiber.Map{"message": "Session not active"}})
	}

	var req RevokeMsgReq
	if err := c.BodyParser(&req); err != nil || req.To == "" || req.MessageId == "" {
		return c.Status(400).JSON(fiber.Map{"success": false, "error": fiber.Map{"message": "Invalid request"}})
	}

	jid, ok := parseJID(req.To)
	if !ok {
		return c.Status(400).JSON(fiber.Map{"success": false, "error": fiber.Map{"message": "Invalid JID"}})
	}

	_, err := session.Client.SendMessage(context.Background(), jid, session.Client.BuildRevoke(jid, types.EmptyJID, req.MessageId))
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "error": fiber.Map{"message": fmt.Sprintf("Failed to revoke: %v", err)}})
	}

	return c.JSON(fiber.Map{
		"success": true,
	})
}

func DownloadMedia(c *fiber.Ctx) error {
	sessionID := c.Params("sessionId")
	_, ok := whatsapp.Manager.GetSession(sessionID)
	if !ok {
		return c.Status(404).JSON(fiber.Map{"success": false, "error": fiber.Map{"message": "Session not active"}})
	}

	// This is a complex functionality because whatsmeow doesn't store media locally like Baileys.
	// A proper implementation would require storing Message data upon receipt, grabbing the MediaKey/URL,
	// and using whatsmeow.Download() then returning the bytes.
	// Due to architectural constraints, this is stubbed for future implementation.
	return c.Status(501).JSON(fiber.Map{"success": false, "error": fiber.Map{"message": "Media download requires message storage which is not implemented in this boilerplate"}})
}
