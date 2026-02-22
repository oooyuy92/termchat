// internal/storage/conversation.go
package storage

import (
	"database/sql"
	"os"
	"path/filepath"

	"github.com/termchat/termchat/internal/chat"
	_ "modernc.org/sqlite"
)

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
	if err := migrate(db); err != nil {
		db.Close()
		return nil, err
	}
	return &Store{db: db}, nil
}

func migrate(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS conversations (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			name       TEXT NOT NULL UNIQUE,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
		CREATE TABLE IF NOT EXISTS messages (
			id              INTEGER PRIMARY KEY AUTOINCREMENT,
			conversation_id INTEGER NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
			role            TEXT NOT NULL,
			content         TEXT NOT NULL,
			seq             INTEGER NOT NULL
		);
	`)
	return err
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

// Load returns all messages for the named conversation, ordered by seq.
func (s *Store) Load(name string) ([]chat.Message, error) {
	rows, err := s.db.Query(
		`SELECT m.role, m.content
		 FROM messages m
		 JOIN conversations c ON c.id = m.conversation_id
		 WHERE c.name = ?
		 ORDER BY m.seq`,
		name,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []chat.Message
	for rows.Next() {
		var msg chat.Message
		if err := rows.Scan(&msg.Role, &msg.Content); err != nil {
			return nil, err
		}
		messages = append(messages, msg)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if messages == nil {
		return nil, sql.ErrNoRows
	}
	return messages, nil
}

// List returns all conversation names ordered by most recently updated.
func (s *Store) List() ([]string, error) {
	rows, err := s.db.Query(`SELECT name FROM conversations ORDER BY updated_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		names = append(names, name)
	}
	return names, rows.Err()
}

// Close releases the database connection.
func (s *Store) Close() error {
	return s.db.Close()
}
