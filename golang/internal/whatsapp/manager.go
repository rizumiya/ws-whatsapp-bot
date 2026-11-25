package whatsapp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"sync"
	"time"

	"go.mau.fi/whatsmeow"
	waProto "go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/store"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"

	"golang-whatsapp-bot/internal/database"
)

type CachedMediaMessage struct {
	Message   *waProto.Message
	Mimetype  string
	ExpiresAt time.Time
}

var (
	Manager = &SessionManager{
		sessions:   make(map[string]*ClientSession),
		mediaCache: make(map[string]CachedMediaMessage),
	}
	webhookHTTPClient = &http.Client{
		Timeout: 15 * time.Second,
	}
)

func init() {
	// Periodic cleanup of expired cached media messages every 30 minutes
	go func() {
		ticker := time.NewTicker(30 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			Manager.PruneExpiredMedia()
		}
	}()
}

type ClientSession struct {
	mu        sync.RWMutex
	SessionID string
	Client    *whatsmeow.Client
	QRChan    <-chan whatsmeow.QRChannelItem
	latestQR  string
	cancel    context.CancelFunc
}

func (cs *ClientSession) SetLatestQR(qr string) {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	cs.latestQR = qr
}

func (cs *ClientSession) GetLatestQR() string {
	cs.mu.RLock()
	defer cs.mu.RUnlock()
	return cs.latestQR
}

type SessionManager struct {
	sync.RWMutex
	sessions   map[string]*ClientSession
	mediaMu    sync.RWMutex
	mediaCache map[string]CachedMediaMessage
}

func (sm *SessionManager) CacheMedia(sessionID, messageID string, msg *waProto.Message, mimetype string) {
	sm.mediaMu.Lock()
	defer sm.mediaMu.Unlock()
	key := sessionID + ":" + messageID
	sm.mediaCache[key] = CachedMediaMessage{
		Message:   msg,
		Mimetype:  mimetype,
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}
}

func (sm *SessionManager) GetCachedMedia(sessionID, messageID string) (CachedMediaMessage, bool) {
	sm.mediaMu.RLock()
	defer sm.mediaMu.RUnlock()
	key := sessionID + ":" + messageID
	item, ok := sm.mediaCache[key]
	if !ok || time.Now().After(item.ExpiresAt) {
		return CachedMediaMessage{}, false
	}
	return item, true
}

func (sm *SessionManager) PruneExpiredMedia() {
	sm.mediaMu.Lock()
	defer sm.mediaMu.Unlock()
	now := time.Now()
	for k, v := range sm.mediaCache {
		if now.After(v.ExpiresAt) {
			delete(sm.mediaCache, k)
		}
	}
}

func (sm *SessionManager) AddSession(sessionID string, client *whatsmeow.Client, qrChan <-chan whatsmeow.QRChannelItem, cancel context.CancelFunc) *ClientSession {
	sm.Lock()
	defer sm.Unlock()
	cs := &ClientSession{
		SessionID: sessionID,
		Client:    client,
		QRChan:    qrChan,
		cancel:    cancel,
	}
	sm.sessions[sessionID] = cs
	return cs
}

func (sm *SessionManager) GetSession(sessionID string) (*ClientSession, bool) {
	sm.RLock()
	defer sm.RUnlock()
	session, exists := sm.sessions[sessionID]
	return session, exists
}

func (sm *SessionManager) RuntimeState(sessionID string) string {
	sm.RLock()
	session, exists := sm.sessions[sessionID]
	sm.RUnlock()
	if !exists || session == nil || session.Client == nil {
		return "stopped"
	}
	if session.Client.IsConnected() {
		return "socket"
	}
	return "pending"
}

func (sm *SessionManager) RemoveSession(sessionID string) {
	sm.Lock()
	session, exists := sm.sessions[sessionID]
	if exists {
		delete(sm.sessions, sessionID)
	}
	sm.Unlock()

	if exists && session != nil {
		if session.cancel != nil {
			session.cancel()
		}
		if session.Client != nil {
			session.Client.Disconnect()
		}
	}
}

func (sm *SessionManager) DisconnectAll() {
	sm.Lock()
	defer sm.Unlock()
	for _, s := range sm.sessions {
		if s.cancel != nil {
			s.cancel()
		}
		if s.Client != nil {
			s.Client.Disconnect()
		}
	}
}

func InitSession(sessionID string) (*ClientSession, error) {
	// Clean up any previous active client for this sessionID first
	Manager.RemoveSession(sessionID)

	_, _, _, jidStr, err := database.GetSessionData(sessionID)

	var deviceStore *store.Device
	if err == nil && jidStr != "" {
		if jid, parseErr := types.ParseJID(jidStr); parseErr == nil {
			deviceStore, _ = database.WhatsmeowStore.GetDevice(context.Background(), jid)
		}
	}
	if deviceStore == nil {
		deviceStore = database.WhatsmeowStore.NewDevice()
	}

	clientLog := waLog.Stdout("Client", "WARN", true)
	client := whatsmeow.NewClient(deviceStore, clientLog)

	ctx, cancel := context.WithCancel(context.Background())

	var qrChan <-chan whatsmeow.QRChannelItem
	if client.Store.ID == nil {
		qr, err := client.GetQRChannel(ctx)
		if err != nil {
			cancel()
			return nil, fmt.Errorf("failed to get QR channel: %v", err)
		}
		qrChan = qr
	}

	client.AddEventHandler(func(evt interface{}) {
		handleEvent(sessionID, client, evt)
	})

	err = client.Connect()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to connect client: %v", err)
	}

	cs := Manager.AddSession(sessionID, client, qrChan, cancel)

	// Start QR listener routine if needed
	if qrChan != nil {
		go func() {
			for evt := range qrChan {
				switch evt.Event {
				case "code":
					_ = database.UpdateSessionStatus(sessionID, "QR_PENDING")
					cs.SetLatestQR(evt.Code)
				case "success":
					cs.SetLatestQR("")
					if client.Store.ID != nil {
						_ = database.UpdateSessionJID(sessionID, client.Store.ID.String(), client.Store.ID.User)
					}
					_ = database.UpdateSessionStatus(sessionID, "CONNECTED")
				case "timeout", "err-client-outdated":
					cs.SetLatestQR("")
					_ = database.UpdateSessionStatus(sessionID, "DISCONNECTED")
				}
			}
		}()
	}

	if client.Store.ID != nil {
		_ = database.UpdateSessionJID(sessionID, client.Store.ID.String(), client.Store.ID.User)
		_ = database.UpdateSessionStatus(sessionID, "CONNECTED")
	} else {
		_ = database.UpdateSessionStatus(sessionID, "STARTING")
	}

	return cs, nil
}

func RestoreSessions() {
	sessions, err := database.GetAllSessions()
	if err != nil {
		log.Printf("Failed to restore sessions: %v", err)
		return
	}

	for _, s := range sessions {
		id, _ := s["id"].(string)
		status, _ := s["status"].(string)
		jidStr, _ := s["jid"].(string)

		if id == "" || status == "LOGGED_OUT" {
			continue
		}

		// Only restore sessions that were paired (have a stored JID)
		if jidStr == "" {
			_ = database.UpdateSessionStatus(id, "DISCONNECTED")
			continue
		}

		_, err = InitSession(id)
		if err != nil {
			log.Printf("Failed to restore session %s: %v", id, err)
		} else {
			log.Printf("Session %s restored", id)
		}
		time.Sleep(500 * time.Millisecond)
	}
}

func dispatchWebhook(sessionID, eventType string, payload interface{}) {
	_, webhookURL, _, _, err := database.GetSessionData(sessionID)
	if err != nil || webhookURL == "" {
		return
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return
	}

	go sendWebhookWithRetry(sessionID, webhookURL, eventType, bodyBytes, 0)
}

func sendWebhookWithRetry(sessionID, webhookURL, eventType string, bodyBytes []byte, retryCount int) {
	req, err := http.NewRequest("POST", webhookURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Session-Id", sessionID)
	req.Header.Set("X-Event-Type", eventType)

	resp, err := webhookHTTPClient.Do(req)
	if err == nil {
		_ = resp.Body.Close()
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return
		}
	}

	if retryCount < 3 && eventType != "presence.update" {
		delay := time.Duration(math.Pow(3, float64(retryCount))) * 5 * time.Second
		time.AfterFunc(delay, func() {
			sendWebhookWithRetry(sessionID, webhookURL, eventType, bodyBytes, retryCount+1)
		})
	}
}

func handleEvent(sessionID string, client *whatsmeow.Client, evt interface{}) {
	switch v := evt.(type) {
	case *events.PairSuccess:
		if session, ok := Manager.GetSession(sessionID); ok {
			session.SetLatestQR("")
		}
		_ = database.UpdateSessionJID(sessionID, v.ID.String(), v.ID.User)
		_ = database.UpdateSessionStatus(sessionID, "CONNECTED")

	case *events.Connected, *events.PushNameSetting:
		if session, ok := Manager.GetSession(sessionID); ok {
			session.SetLatestQR("")
		}
		if client.Store.ID != nil {
			_ = database.UpdateSessionJID(sessionID, client.Store.ID.String(), client.Store.ID.User)
		}
		_ = database.UpdateSessionStatus(sessionID, "CONNECTED")

	case *events.LoggedOut:
		_ = database.UpdateSessionStatus(sessionID, "LOGGED_OUT")
		Manager.RemoveSession(sessionID)

	case *events.Disconnected:
		_ = database.UpdateSessionStatus(sessionID, "DISCONNECTED")

	case *events.Message:
		if v.Message != nil {
			var mimetype string
			switch {
			case v.Message.ImageMessage != nil:
				mimetype = v.Message.ImageMessage.GetMimetype()
			case v.Message.VideoMessage != nil:
				mimetype = v.Message.VideoMessage.GetMimetype()
			case v.Message.AudioMessage != nil:
				mimetype = v.Message.AudioMessage.GetMimetype()
			case v.Message.DocumentMessage != nil:
				mimetype = v.Message.DocumentMessage.GetMimetype()
			case v.Message.StickerMessage != nil:
				mimetype = v.Message.StickerMessage.GetMimetype()
			}
			if mimetype != "" && v.Info.ID != "" {
				Manager.CacheMedia(sessionID, v.Info.ID, v.Message, mimetype)
			}
		}
		dispatchWebhook(sessionID, "messages.upsert", v)

	case *events.Receipt:
		dispatchWebhook(sessionID, "message-receipt.update", v)

	case *events.ChatPresence:
		dispatchWebhook(sessionID, "presence.update", v)

	case *events.CallOffer:
		dispatchWebhook(sessionID, "call.offer", v)
	}
}
