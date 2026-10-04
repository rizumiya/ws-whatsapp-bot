package database

import (
	"context"
	"database/sql"
	"fmt"
	"log"

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
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);
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
	query := `SELECT api_key, webhook_url, status, jid FROM api_sessions WHERE session_id = $1`
	err = DB.QueryRow(query, sessionID).Scan(&apiKey, &webhookURL, &status, &jid)
	return
}

func SaveSessionData(sessionID, apiKey, webhookURL, status string) error {
	query := `
		INSERT INTO api_sessions (session_id, api_key, webhook_url, status)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (session_id) DO UPDATE SET
			api_key = EXCLUDED.api_key,
			webhook_url = EXCLUDED.webhook_url,
			status = EXCLUDED.status;
	`
	_, err := DB.Exec(query, sessionID, apiKey, webhookURL, status)
	return err
}

func UpdateSessionStatus(sessionID, status string) error {
	query := `UPDATE api_sessions SET status = $1 WHERE session_id = $2`
	_, err := DB.Exec(query, status, sessionID)
	return err
}

func UpdateSessionJID(sessionID, jid string) error {
	query := `UPDATE api_sessions SET jid = $1 WHERE session_id = $2`
	_, err := DB.Exec(query, jid, sessionID)
	return err
}

func DeleteSessionData(sessionID string) error {
	query := `DELETE FROM api_sessions WHERE session_id = $1`
	_, err := DB.Exec(query, sessionID)
	return err
}

func GetAllSessions() ([]map[string]interface{}, error) {
	query := `SELECT session_id, webhook_url, status, jid, created_at FROM api_sessions`
	rows, err := DB.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sessions []map[string]interface{}
	for rows.Next() {
		var id, status, jid string
		var webhookURL sql.NullString
		var createdAt string
		if err := rows.Scan(&id, &webhookURL, &status, &jid, &createdAt); err != nil {
			return nil, err
		}

		session := map[string]interface{}{
			"id":           id,
			"status":       status,
			"jid":          jid,
			"created_at":   createdAt,
		}
		if webhookURL.Valid {
			session["webhook_url"] = webhookURL.String
		} else {
			session["webhook_url"] = nil
		}
		sessions = append(sessions, session)
	}
	return sessions, nil
}
