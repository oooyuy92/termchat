// internal/storage/conversation.go
package storage

import (
	"database/sql"
	"errors"
	"os"
	"path/filepath"

	"github.com/termchat/termchat/internal/chat"
	_ "modernc.org/sqlite"
)

// ErrNotFound is returned by Load when no conversation with the given name exists.
var ErrNotFound = errors.New("conversation not found")

// ConvInfo holds summary information for a conversation.
type ConvInfo struct {
	Name    string
	Date    string // "YYYY-MM-DD" (date portion of updated_at)
	Summary string // first user message content (empty if none)
}

// ConvSearchItem holds a conversation's name, date, and all message content
// concatenated for full-text fuzzy search.
type ConvSearchItem struct {
	Name     string
	Date     string
	FullText string // name + " " + all message content joined by " "
}

type Store struct {
	db *sql.DB
}

// New opens (or creates) the SQLite database at dir/termchat.db.
func New(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}
	dbPath := filepath.Join(dir, "termchat.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}
	// Use a single connection so PRAGMA settings apply to all operations.
	db.SetMaxOpenConns(1)
	// Enable foreign key enforcement (SQLite disables it by default).
	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		db.Close()
		return nil, err
	}
	if err := migrate(db); err != nil {
		db.Close()
		return nil, err
	}
	return &Store{db: db}, nil
}

func migrate(db *sql.DB) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	_, err = tx.Exec(`
		CREATE TABLE IF NOT EXISTS conversations (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			name       TEXT NOT NULL UNIQUE,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return err
	}
	_, err = tx.Exec(`
		CREATE TABLE IF NOT EXISTS messages (
			id              INTEGER PRIMARY KEY AUTOINCREMENT,
			conversation_id INTEGER NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
			role            TEXT NOT NULL,
			content         TEXT NOT NULL,
			seq             INTEGER NOT NULL
		)
	`)
	if err != nil {
		return err
	}
	return tx.Commit()
}

// Save writes all messages for the named conversation, replacing any prior messages.
func (s *Store) Save(name string, messages []chat.Message) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Upsert conversation row.
	_, err = tx.Exec(
		`INSERT INTO conversations(name, updated_at) VALUES(?, CURRENT_TIMESTAMP)
		 ON CONFLICT(name) DO UPDATE SET updated_at = CURRENT_TIMESTAMP`,
		name,
	)
	if err != nil {
		return err
	}

	var convID int64
	if err := tx.QueryRow(`SELECT id FROM conversations WHERE name = ?`, name).Scan(&convID); err != nil {
		return err
	}

	// Delete old messages for this conversation.
	if _, err := tx.Exec(`DELETE FROM messages WHERE conversation_id = ?`, convID); err != nil {
		return err
	}

	// Insert new messages.
	for i, msg := range messages {
		if _, err := tx.Exec(
			`INSERT INTO messages(conversation_id, role, content, seq) VALUES(?, ?, ?, ?)`,
			convID, msg.Role, msg.Content, i,
		); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// Load returns all messages for the named conversation ordered by seq.
// Returns ErrNotFound if no conversation with that name exists.
// Returns an empty slice (not an error) if the conversation exists but has no messages.
func (s *Store) Load(name string) ([]chat.Message, error) {
	// Check conversation exists.
	var convID int64
	err := s.db.QueryRow(`SELECT id FROM conversations WHERE name = ?`, name).Scan(&convID)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	rows, err := s.db.Query(
		`SELECT role, content FROM messages WHERE conversation_id = ? ORDER BY seq`,
		convID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	messages := []chat.Message{}
	for rows.Next() {
		var msg chat.Message
		if err := rows.Scan(&msg.Role, &msg.Content); err != nil {
			return nil, err
		}
		messages = append(messages, msg)
	}
	return messages, rows.Err()
}

// List returns all conversation names ordered by most recently updated.
func (s *Store) List() ([]string, error) {
	rows, err := s.db.Query(`SELECT name FROM conversations ORDER BY updated_at DESC, id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	names := []string{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		names = append(names, name)
	}
	return names, rows.Err()
}

// ListWithDate returns all conversations with their date string (YYYY-MM-DD)
// and first user message summary, ordered by most recently updated.
func (s *Store) ListWithDate() ([]ConvInfo, error) {
	rows, err := s.db.Query(
		`SELECT c.name, date(c.updated_at),
			COALESCE((SELECT m.content FROM messages m
				WHERE m.conversation_id = c.id AND m.role = 'user'
				ORDER BY m.seq ASC LIMIT 1), '')
		FROM conversations c ORDER BY c.updated_at DESC, c.id DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var convs []ConvInfo
	for rows.Next() {
		var c ConvInfo
		if err := rows.Scan(&c.Name, &c.Date, &c.Summary); err != nil {
			return nil, err
		}
		convs = append(convs, c)
	}
	return convs, rows.Err()
}

// LoadAllForSearch returns all conversations with their full message content
// concatenated into FullText, ordered by most recently updated.
func (s *Store) LoadAllForSearch() ([]ConvSearchItem, error) {
	rows, err := s.db.Query(
		`SELECT c.name, date(c.updated_at),
			COALESCE(GROUP_CONCAT(m.content, ' '), '')
		 FROM conversations c
		 LEFT JOIN messages m ON m.conversation_id = c.id
		 GROUP BY c.id
		 ORDER BY c.updated_at DESC, c.id DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []ConvSearchItem
	for rows.Next() {
		var item ConvSearchItem
		var msgContent string
		if err := rows.Scan(&item.Name, &item.Date, &msgContent); err != nil {
			return nil, err
		}
		item.FullText = item.Name + " " + msgContent
		items = append(items, item)
	}
	return items, rows.Err()
}

// Close releases the database connection.
func (s *Store) Close() error {
	return s.db.Close()
}
