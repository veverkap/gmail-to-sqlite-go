package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"path/filepath"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/veverkap/gmail-to-sqlite/message"
)

// DB represents the database connection.
type DB struct {
	conn *sql.DB
}

// Init initializes the database connection and creates the necessary tables.
func Init(dataDir string, enableLogging bool) (*DB, error) {
	dbPath := filepath.Join(dataDir, "messages.db")
	conn, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %v", err)
	}

	// Create tables if they don't exist
	if err := createTables(conn); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to create tables: %v", err)
	}

	if enableLogging {
		log.Println("Database logging enabled")
	}

	return &DB{conn: conn}, nil
}

// Close closes the database connection.
func (db *DB) Close() error {
	return db.conn.Close()
}

// CreateMessage saves a message to the database.
func (db *DB) CreateMessage(msg *message.Message) error {
	lastIndexed := time.Now()

	// Convert message fields to JSON
	senderJSON, err := json.Marshal(msg.Sender)
	if err != nil {
		return fmt.Errorf("failed to marshal sender: %v", err)
	}

	recipientsJSON, err := json.Marshal(msg.Recipients)
	if err != nil {
		return fmt.Errorf("failed to marshal recipients: %v", err)
	}

	labelsJSON, err := json.Marshal(msg.Labels)
	if err != nil {
		return fmt.Errorf("failed to marshal labels: %v", err)
	}

	// Check if the message already exists
	var exists bool
	err = db.conn.QueryRow("SELECT EXISTS(SELECT 1 FROM messages WHERE message_id = ?)", msg.ID).Scan(&exists)
	if err != nil {
		return fmt.Errorf("failed to check if message exists: %v", err)
	}

	if exists {
		// Update the existing message
		_, err = db.conn.Exec(
			"UPDATE messages SET is_read = ?, last_indexed = ?, labels = ? WHERE message_id = ?",
			msg.IsRead, lastIndexed, string(labelsJSON), msg.ID,
		)
		if err != nil {
			return fmt.Errorf("failed to update message: %v", err)
		}
	} else {
		// Insert a new message
		_, err = db.conn.Exec(
			`INSERT INTO messages (
				message_id, thread_id, sender, recipients, labels,
				subject, body, size, timestamp, is_read, is_outgoing, last_indexed
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			msg.ID, msg.ThreadID, string(senderJSON), string(recipientsJSON), string(labelsJSON),
			msg.Subject, msg.Body, msg.Size, msg.Timestamp.Format(time.RFC3339),
			msg.IsRead, msg.IsOutgoing, lastIndexed.Format(time.RFC3339),
		)
		if err != nil {
			return fmt.Errorf("failed to insert message: %v", err)
		}
	}

	return nil
}

// LastIndexed returns the timestamp of the last indexed message.
func (db *DB) LastIndexed() (*time.Time, error) {
	var timestampStr string
	err := db.conn.QueryRow("SELECT timestamp FROM messages ORDER BY timestamp DESC LIMIT 1").Scan(&timestampStr)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get last indexed message: %v", err)
	}

	timestamp, err := time.Parse(time.RFC3339, timestampStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse timestamp: %v", err)
	}

	return &timestamp, nil
}

// FirstIndexed returns the timestamp of the first indexed message.
func (db *DB) FirstIndexed() (*time.Time, error) {
	var timestampStr string
	err := db.conn.QueryRow("SELECT timestamp FROM messages ORDER BY timestamp ASC LIMIT 1").Scan(&timestampStr)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get first indexed message: %v", err)
	}

	timestamp, err := time.Parse(time.RFC3339, timestampStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse timestamp: %v", err)
	}

	return &timestamp, nil
}

// createTables creates the necessary database tables.
func createTables(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS messages (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			message_id TEXT UNIQUE NOT NULL,
			thread_id TEXT NOT NULL,
			sender TEXT NOT NULL,
			recipients TEXT NOT NULL,
			labels TEXT NOT NULL,
			subject TEXT,
			body TEXT,
			size INTEGER NOT NULL,
			timestamp TEXT NOT NULL,
			is_read BOOLEAN NOT NULL,
			is_outgoing BOOLEAN NOT NULL,
			last_indexed TEXT NOT NULL
		)
	`)
	return err
}