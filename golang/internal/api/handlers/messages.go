package handlers

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"go.mau.fi/whatsmeow"
	waProto "go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"google.golang.org/protobuf/proto"

	"golang-whatsapp-bot/internal/whatsapp"
)

const maxRemoteMediaBytes = 20 * 1024 * 1024 // 20 MB

var mediaHTTPClient = &http.Client{
	Timeout: 15 * time.Second,
}

func isPrivateIP(ip net.IP) bool {
	if ip == nil {
		return true
	}
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsMulticast() || ip.IsUnspecified() {
		return true
	}
	if ip4 := ip.To4(); ip4 != nil {
		// CGNAT 100.64.0.0/10
		if ip4[0] == 100 && ip4[1] >= 64 && ip4[1] <= 127 {
			return true
		}
	}
	return false
}

func validateExternalMediaURL(rawURL string) (string, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return "", errors.New("only http and https media URLs are allowed")
	}
	host := parsed.Hostname()
	if host == "" || host == "localhost" || strings.HasSuffix(host, ".local") || strings.HasSuffix(host, ".internal") {
		return "", errors.New("fetching media from local or internal hostnames is not allowed")
	}
	if ip := net.ParseIP(host); ip != nil {
		if isPrivateIP(ip) {
			return "", errors.New("fetching media from private or loopback IP addresses is not allowed")
		}
		return parsed.String(), nil
	}
	ips, err := net.LookupIP(host)
	if err != nil || len(ips) == 0 {
		return "", errors.New("failed to resolve media URL hostname")
	}
	for _, ip := range ips {
		if isPrivateIP(ip) {
			return "", errors.New("media URL resolves to a private or disallowed IP address")
		}
	}
	return parsed.String(), nil
}

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

func getContextInfo(quotedMessageId string, remoteJID types.JID) *waProto.ContextInfo {
	if quotedMessageId == "" {
		return nil
	}
	return &waProto.ContextInfo{
		StanzaID:      proto.String(quotedMessageId),
		Participant:   proto.String(remoteJID.ToNonAD().String()),
		QuotedMessage: &waProto.Message{Conversation: proto.String("")},
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
	if !ok || !session.Client.IsLoggedIn() {
		return c.Status(404).JSON(fiber.Map{"success": false, "error": fiber.Map{"message": "Session not active or not logged in"}})
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
			ContextInfo: getContextInfo(req.QuotedMessageId, jid),
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
	if !ok || !session.Client.IsLoggedIn() {
		return c.Status(404).JSON(fiber.Map{"success": false, "error": fiber.Map{"message": "Session not active or not logged in"}})
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
		safeURL, urlErr := validateExternalMediaURL(req.URL)
		if urlErr != nil {
			return c.Status(400).JSON(fiber.Map{"success": false, "error": fiber.Map{"message": urlErr.Error()}})
		}
		resp, err := mediaHTTPClient.Get(safeURL)
		if err != nil || resp.StatusCode != 200 {
			if resp != nil {
				_ = resp.Body.Close()
			}
			return c.Status(400).JSON(fiber.Map{"success": false, "error": fiber.Map{"message": "Failed to fetch media from URL"}})
		}
		defer resp.Body.Close()

		if resp.ContentLength > maxRemoteMediaBytes {
			return c.Status(413).JSON(fiber.Map{"success": false, "error": fiber.Map{"message": "Remote media exceeds 20 MB limit"}})
		}

		limitedReader := io.LimitReader(resp.Body, maxRemoteMediaBytes+1)
		mediaData, err = io.ReadAll(limitedReader)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"success": false, "error": fiber.Map{"message": "Failed to read media body"}})
		}
		if len(mediaData) > maxRemoteMediaBytes {
			return c.Status(413).JSON(fiber.Map{"success": false, "error": fiber.Map{"message": "Remote media exceeds 20 MB limit"}})
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
	case "image", "sticker":
		waMediaType = whatsmeow.MediaImage
	case "video":
		waMediaType = whatsmeow.MediaVideo
	case "audio":
		waMediaType = whatsmeow.MediaAudio
	case "document":
		waMediaType = whatsmeow.MediaDocument
	default:
		waMediaType = whatsmeow.MediaDocument
	}

	uploaded, err := session.Client.Upload(context.Background(), mediaData, waMediaType)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "error": fiber.Map{"message": fmt.Sprintf("Failed to upload media: %v", err)}})
	}

	msg := &waProto.Message{}
	ctxInfo := getContextInfo(req.QuotedMessageId, jid)

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
	if !ok || !session.Client.IsLoggedIn() {
		return c.Status(404).JSON(fiber.Map{"success": false, "error": fiber.Map{"message": "Session not active or not logged in"}})
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
	if !ok || !session.Client.IsLoggedIn() {
		return c.Status(404).JSON(fiber.Map{"success": false, "error": fiber.Map{"message": "Session not active or not logged in"}})
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
	if !ok || !session.Client.IsLoggedIn() {
		return c.Status(404).JSON(fiber.Map{"success": false, "error": fiber.Map{"message": "Session not active or not logged in"}})
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
	messageID := c.Params("messageId")

	session, ok := whatsapp.Manager.GetSession(sessionID)
	if !ok || !session.Client.IsLoggedIn() {
		return c.Status(404).JSON(fiber.Map{"success": false, "error": fiber.Map{"message": "Session not active or not logged in"}})
	}

	cached, found := whatsapp.Manager.GetCachedMedia(sessionID, messageID)
	if !found || cached.Message == nil {
		return c.Status(404).JSON(fiber.Map{
			"success": false,
			"error": fiber.Map{
				"code":    "NOT_FOUND",
				"message": "Message media not found in cache. Media messages are only cached for 24 hours.",
			},
		})
	}

	data, err := session.Client.DownloadAny(context.Background(), cached.Message)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"success": false,
			"error":   fiber.Map{"message": fmt.Sprintf("Failed to download media: %v", err)},
		})
	}

	mimetype := cached.Mimetype
	if mimetype == "" {
		mimetype = "application/octet-stream"
	}

	c.Set("Content-Type", mimetype)
	return c.Send(data)
}
