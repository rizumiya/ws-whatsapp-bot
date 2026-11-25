package database

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	"golang-whatsapp-bot/config"
	_ "github.com/lib/pq"
	"go.mau.fi/whatsmeow/store/sqlstore"
	waLog "go.mau.fi/whatsmeow/util/log"
)

var (
	DB             *sql.DB
	WhatsmeowStore *sqlstore.Container
)

func InitDB(cfg *config.Config) {
	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		cfg.DBHost, cfg.DBPort, cfg.DBUser, cfg.DBPassword, cfg.DBName)

	var err error
	DB, err = sql.Open("postgres", dsn)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	if err = DB.Ping(); err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}

	// Create table for our internal sessions
	createTableQuery := `
	CREATE TABLE IF NOT EXISTS api_sessions (
		session_id VARCHAR(64) PRIMARY KEY,
		api_key VARCHAR(128) NOT NULL,
		webhook_url TEXT,
		status VARCHAR(32) DEFAULT 'IDLE',
		jid VARCHAR(128) DEFAULT '',
		phone_number VARCHAR(30) DEFAULT '',
		created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
	);
	ALTER TABLE api_sessions ADD COLUMN IF NOT EXISTS phone_number VARCHAR(30) DEFAULT '';
	ALTER TABLE api_sessions ADD COLUMN IF NOT EXISTS updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP;
	`
	_, err = DB.Exec(createTableQuery)
	if err != nil {
		log.Fatalf("Failed to create api_sessions table: %v", err)
	}

	dbLog := waLog.Stdout("Database", "WARN", true)
	WhatsmeowStore = sqlstore.NewWithDB(DB, "postgres", dbLog)
	err = WhatsmeowStore.Upgrade(context.Background())
	if err != nil {
		log.Fatalf("Failed to upgrade whatsmeow database schema: %v", err)
	}

	log.Println("Database initialized successfully")
}

func GetSessionData(sessionID string) (apiKey string, webhookURL string, status string, jid string, err error) {
	var wh sql.NullString
	query := `SELECT api_key, webhook_url, status, jid FROM api_sessions WHERE session_id = $1`
	err = DB.QueryRow(query, sessionID).Scan(&apiKey, &wh, &status, &jid)
	if wh.Valid {
		webhookURL = wh.String
	}
	return
}

func SaveSessionData(sessionID, apiKey, webhookURL, status string) error {
	query := `
		INSERT INTO api_sessions (session_id, api_key, webhook_url, status, updated_at)
		VALUES ($1, $2, $3, $4, CURRENT_TIMESTAMP)
		ON CONFLICT (session_id) DO UPDATE SET
			api_key = EXCLUDED.api_key,
			webhook_url = EXCLUDED.webhook_url,
			status = EXCLUDED.status,
			updated_at = CURRENT_TIMESTAMP;
	`
	_, err := DB.Exec(query, sessionID, apiKey, webhookURL, status)
	return err
}

func UpdateSessionStatus(sessionID, status string) error {
	query := `UPDATE api_sessions SET status = $1, updated_at = CURRENT_TIMESTAMP WHERE session_id = $2`
	_, err := DB.Exec(query, status, sessionID)
	return err
}

func UpdateSessionJID(sessionID, jid, phoneNumber string) error {
	query := `UPDATE api_sessions SET jid = $1, phone_number = $2, updated_at = CURRENT_TIMESTAMP WHERE session_id = $3`
	_, err := DB.Exec(query, jid, phoneNumber, sessionID)
	return err
}

func DeleteSessionData(sessionID string) error {
	query := `DELETE FROM api_sessions WHERE session_id = $1`
	_, err := DB.Exec(query, sessionID)
	return err
}

func GetAllSessions() ([]map[string]interface{}, error) {
	query := `SELECT session_id, webhook_url, status, jid, COALESCE(phone_number, ''), created_at, COALESCE(updated_at, created_at) FROM api_sessions`
	rows, err := DB.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	sessions := make([]map[string]interface{}, 0)
	for rows.Next() {
		var id, status, jid, phoneNumber string
		var webhookURL sql.NullString
		var createdAt, updatedAt time.Time
		if err := rows.Scan(&id, &webhookURL, &status, &jid, &phoneNumber, &createdAt, &updatedAt); err != nil {
			return nil, err
		}

		session := map[string]interface{}{
			"id":           id,
			"status":       status,
			"jid":          jid,
			"phone_number": phoneNumber,
			"created_at":   createdAt.Format(time.RFC3339),
			"updated_at":   updatedAt.Format(time.RFC3339),
		}
		if webhookURL.Valid && webhookURL.String != "" {
			session["webhook_url"] = webhookURL.String
		} else {
			session["webhook_url"] = nil
		}
		sessions = append(sessions, session)
	}
	return sessions, nil
}
