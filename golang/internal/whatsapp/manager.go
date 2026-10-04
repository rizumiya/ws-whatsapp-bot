package whatsapp

import (
	"context"
	"fmt"
	"log"
	"sync"


	"go.mau.fi/whatsmeow"

	"go.mau.fi/whatsmeow/types/events"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/store"
	waLog "go.mau.fi/whatsmeow/util/log"

	"golang-whatsapp-bot/internal/database"
)

var (
	Manager = &SessionManager{
		sessions: make(map[string]*ClientSession),
	}
)

type ClientSession struct {
	SessionID string
	Client    *whatsmeow.Client
	QRChan    <-chan whatsmeow.QRChannelItem
	LatestQR  string
	cancel    context.CancelFunc
}

type SessionManager struct {
	sync.RWMutex
	sessions map[string]*ClientSession
}

func (sm *SessionManager) AddSession(sessionID string, client *whatsmeow.Client, qrChan <-chan whatsmeow.QRChannelItem, cancel context.CancelFunc) {
	sm.Lock()
	defer sm.Unlock()
	sm.sessions[sessionID] = &ClientSession{
		SessionID: sessionID,
		Client:    client,
		QRChan:    qrChan,
		cancel:    cancel,
	}
}

func (sm *SessionManager) GetSession(sessionID string) (*ClientSession, bool) {
	sm.RLock()
	defer sm.RUnlock()
	session, exists := sm.sessions[sessionID]
	return session, exists
}

func (sm *SessionManager) RemoveSession(sessionID string) {
	sm.Lock()
	defer sm.Unlock()
	if session, exists := sm.sessions[sessionID]; exists {
		if session.cancel != nil {
			session.cancel()
		}
		delete(sm.sessions, sessionID)
	}
}

func InitSession(sessionID string) (*ClientSession, error) {
	_, _, _, jidStr, err := database.GetSessionData(sessionID)

	var deviceStore *store.Device
	if err == nil && jidStr != "" {
		jid, _ := types.ParseJID(jidStr)
		deviceStore, _ = database.WhatsmeowStore.GetDevice(context.Background(), jid)
	}
	if deviceStore == nil {
		deviceStore = database.WhatsmeowStore.NewDevice()
	}

	clientLog := waLog.Stdout("Client", "WARN", true)
	client := whatsmeow.NewClient(deviceStore, clientLog)

	var qrChan <-chan whatsmeow.QRChannelItem
	if client.Store.ID == nil {
		qr, err := client.GetQRChannel(context.Background())
		if err != nil {
			return nil, fmt.Errorf("failed to get QR channel: %v", err)
		}
		qrChan = qr
	}

	_, cancel := context.WithCancel(context.Background())

	eventHandlerID := client.AddEventHandler(func(evt interface{}) {
		handleEvent(sessionID, client, evt)
	})
	_ = eventHandlerID // keep it if needed later

	err = client.Connect()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to connect client: %v", err)
	}

	Manager.AddSession(sessionID, client, qrChan, cancel)

	// Start QR listener routine if needed
	if qrChan != nil {
		go func() {
			for evt := range qrChan {
				if evt.Event == "code" {
					// Update status to QR_PENDING
					database.UpdateSessionStatus(sessionID, "QR_PENDING")
					if session, ok := Manager.GetSession(sessionID); ok {
						session.LatestQR = evt.Code
					}
				} else if evt.Event == "timeout" {
					database.UpdateSessionStatus(sessionID, "DISCONNECTED")
				}
			}
		}()
	}

	if client.Store.ID != nil {
		database.UpdateSessionStatus(sessionID, "CONNECTED")
	} else {
		database.UpdateSessionStatus(sessionID, "STARTING")
	}

	return Manager.sessions[sessionID], nil
}

func RestoreSessions() {
	sessions, err := database.GetAllSessions()
	if err != nil {
		log.Printf("Failed to restore sessions: %v", err)
		return
	}

	for _, s := range sessions {
		id := s["id"].(string)

		// If status is QR_PENDING or STARTING but the app crashed, we might need to recreate
		status := s["status"].(string)
		if status == "LOGGED_OUT" {
			continue
		}

		// Only restore if device exists in DB
		devices, err := database.WhatsmeowStore.GetAllDevices(context.Background())
		if err != nil {
			log.Printf("Error checking devices: %v", err)
			continue
		}


		for _, dev := range devices {
			if dev.ID != nil {
				// How to map session_id to device? Whatsmeow doesn't natively map arbitrary session IDs.
				// For a full implementation, we might need to store device.JID string in our api_sessions table.
				// Since we just started, we'll initialize it and Whatsmeow handles reconnection if device data exists in SQLite/PG.
				// Simplification: We initialize using the new device method, but set a specific ID if known.
				// Since whatsmeow handles it by JID, and we don't have JID mapped directly yet...
				// We'll initialize it, and if it's paired, it'll connect.
				_ = dev // To be refined.
			}
		}

		// Simplified Restore
		_, err = InitSession(id)
		if err != nil {
			log.Printf("Failed to restore session %s: %v", id, err)
		} else {
			log.Printf("Session %s restored", id)
		}
	}
}

func handleEvent(sessionID string, client *whatsmeow.Client, evt interface{}) {
	switch v := evt.(type) {
	case *events.Connected, *events.PushNameSetting:
		if len(client.Store.PushName) == 0 {
			return
		}
		database.UpdateSessionStatus(sessionID, "CONNECTED")
		if client.Store.ID != nil {
			database.UpdateSessionJID(sessionID, client.Store.ID.String())
		}

	case *events.LoggedOut:
		database.UpdateSessionStatus(sessionID, "LOGGED_OUT")
		Manager.RemoveSession(sessionID)

	case *events.Disconnected:
		database.UpdateSessionStatus(sessionID, "DISCONNECTED")

	case *events.Message:
		// Forward to webhook logic here
		_ = v

	case *events.CallOffer:
		// Handle incoming call
		_ = v
	}
}
