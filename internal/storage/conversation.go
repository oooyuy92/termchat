// internal/storage/conversation.go
package storage

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

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

func validateMessageSnapshot(msg chat.Message) error {
	if msg.Role != "assistant" {
		return nil
	}
	if msg.SnapshotProvider == "" || msg.SnapshotModel == "" || msg.SnapshotAPIFormat == "" {
		return fmt.Errorf("assistant messages require generation snapshot")
	}
	return nil
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
			provider_name TEXT NOT NULL DEFAULT '',
			model_name TEXT NOT NULL DEFAULT '',
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return err
	}
	if err := ensureConversationsColumns(tx); err != nil {
		return err
	}
	_, err = tx.Exec(`
		CREATE TABLE IF NOT EXISTS messages (
			id                        INTEGER PRIMARY KEY AUTOINCREMENT,
			conversation_id           INTEGER NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
			role                      TEXT NOT NULL,
			content                   TEXT NOT NULL,
			seq                       INTEGER NOT NULL,
			created_at                DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			version_group_id          INTEGER,
			version_number            INTEGER NOT NULL DEFAULT 1,
			is_active_version         INTEGER NOT NULL DEFAULT 1,
			edited_after_generation   INTEGER NOT NULL DEFAULT 0,
			stale_after_user_edit     INTEGER NOT NULL DEFAULT 0,
			is_deleted                INTEGER NOT NULL DEFAULT 0,
			deleted_batch_id          INTEGER NOT NULL DEFAULT 0,
			snapshot_provider         TEXT NOT NULL DEFAULT '',
			snapshot_model            TEXT NOT NULL DEFAULT '',
			snapshot_api_format       TEXT NOT NULL DEFAULT '',
			snapshot_role_name        TEXT NOT NULL DEFAULT '',
			snapshot_role_prompt      TEXT NOT NULL DEFAULT ''
		)
	`)
	if err != nil {
		return err
	}
	if err := ensureMessagesColumns(tx); err != nil {
		return err
	}
	return tx.Commit()
}

func ensureConversationsColumns(tx *sql.Tx) error {
	rows, err := tx.Query(`PRAGMA table_info(conversations)`)
	if err != nil {
		return err
	}
	defer rows.Close()

	existing := make(map[string]bool)
	for rows.Next() {
		var cid int
		var name string
		var colType string
		var notNull int
		var defaultValue sql.NullString
		var pk int
		if err := rows.Scan(&cid, &name, &colType, &notNull, &defaultValue, &pk); err != nil {
			return err
		}
		existing[name] = true
	}
	if err := rows.Err(); err != nil {
		return err
	}

	alterStmts := []struct {
		name string
		sql  string
	}{
		{
			name: "provider_name",
			sql:  `ALTER TABLE conversations ADD COLUMN provider_name TEXT NOT NULL DEFAULT ''`,
		},
		{
			name: "model_name",
			sql:  `ALTER TABLE conversations ADD COLUMN model_name TEXT NOT NULL DEFAULT ''`,
		},
	}

	for _, stmt := range alterStmts {
		if existing[stmt.name] {
			continue
		}
		if _, err := tx.Exec(stmt.sql); err != nil {
			return err
		}
	}

	return nil
}

func ensureMessagesColumns(tx *sql.Tx) error {
	rows, err := tx.Query(`PRAGMA table_info(messages)`)
	if err != nil {
		return err
	}
	defer rows.Close()

	existing := make(map[string]bool)
	for rows.Next() {
		var cid int
		var name string
		var colType string
		var notNull int
		var defaultValue sql.NullString
		var pk int
		if err := rows.Scan(&cid, &name, &colType, &notNull, &defaultValue, &pk); err != nil {
			return err
		}
		existing[name] = true
	}
	if err := rows.Err(); err != nil {
		return err
	}

	alterStmts := []struct {
		name string
		sql  string
	}{
		{
			name: "version_group_id",
			sql:  `ALTER TABLE messages ADD COLUMN version_group_id INTEGER`,
		},
		{
			name: "version_number",
			sql:  `ALTER TABLE messages ADD COLUMN version_number INTEGER NOT NULL DEFAULT 1`,
		},
		{
			name: "is_active_version",
			sql:  `ALTER TABLE messages ADD COLUMN is_active_version INTEGER NOT NULL DEFAULT 1`,
		},
		{
			name: "edited_after_generation",
			sql:  `ALTER TABLE messages ADD COLUMN edited_after_generation INTEGER NOT NULL DEFAULT 0`,
		},
		{
			name: "stale_after_user_edit",
			sql:  `ALTER TABLE messages ADD COLUMN stale_after_user_edit INTEGER NOT NULL DEFAULT 0`,
		},
		{
			name: "is_deleted",
			sql:  `ALTER TABLE messages ADD COLUMN is_deleted INTEGER NOT NULL DEFAULT 0`,
		},
		{
			name: "deleted_batch_id",
			sql:  `ALTER TABLE messages ADD COLUMN deleted_batch_id INTEGER NOT NULL DEFAULT 0`,
		},
		{
			name: "snapshot_provider",
			sql:  `ALTER TABLE messages ADD COLUMN snapshot_provider TEXT NOT NULL DEFAULT ''`,
		},
		{
			name: "snapshot_model",
			sql:  `ALTER TABLE messages ADD COLUMN snapshot_model TEXT NOT NULL DEFAULT ''`,
		},
		{
			name: "snapshot_api_format",
			sql:  `ALTER TABLE messages ADD COLUMN snapshot_api_format TEXT NOT NULL DEFAULT ''`,
		},
		{
			name: "snapshot_role_name",
			sql:  `ALTER TABLE messages ADD COLUMN snapshot_role_name TEXT NOT NULL DEFAULT ''`,
		},
		{
			name: "snapshot_role_prompt",
			sql:  `ALTER TABLE messages ADD COLUMN snapshot_role_prompt TEXT NOT NULL DEFAULT ''`,
		},
	}

	for _, stmt := range alterStmts {
		if existing[stmt.name] {
			continue
		}
		if _, err := tx.Exec(stmt.sql); err != nil {
			return err
		}
	}

	return nil
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
		if err := validateMessageSnapshot(msg); err != nil {
			return err
		}
		if _, err := tx.Exec(
			`INSERT INTO messages(conversation_id, role, content, seq, snapshot_provider, snapshot_model, snapshot_api_format, snapshot_role_name, snapshot_role_prompt)
			 VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			convID, msg.Role, msg.Content, i, msg.SnapshotProvider, msg.SnapshotModel, msg.SnapshotAPIFormat, msg.SnapshotRoleName, msg.SnapshotRolePrompt,
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
	return s.LoadActiveTimeline(name)
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
		 WHERE m.id IS NULL OR m.role = 'user' OR m.is_active_version = 1
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

// AppendMessage inserts a new message into the conversation and returns its ID.
func (s *Store) AppendMessage(name string, msg chat.Message) (int64, error) {
	if err := validateMessageSnapshot(msg); err != nil {
		return 0, err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	_, err = tx.Exec(
		`INSERT INTO conversations(name, updated_at) VALUES(?, CURRENT_TIMESTAMP)
		 ON CONFLICT(name) DO UPDATE SET updated_at = CURRENT_TIMESTAMP`,
		name,
	)
	if err != nil {
		return 0, err
	}

	var convID int64
	if err := tx.QueryRow(`SELECT id FROM conversations WHERE name = ?`, name).Scan(&convID); err != nil {
		return 0, err
	}

	result, err := tx.Exec(
		`INSERT INTO messages(
			conversation_id, role, content, seq, version_number, is_active_version,
			edited_after_generation, stale_after_user_edit,
			snapshot_provider, snapshot_model, snapshot_api_format, snapshot_role_name, snapshot_role_prompt
		) VALUES(?, ?, ?, ?, ?, 1, 0, 0, ?, ?, ?, ?, ?)`,
		convID, msg.Role, msg.Content, msg.Seq, msg.VersionNumber,
		msg.SnapshotProvider, msg.SnapshotModel, msg.SnapshotAPIFormat, msg.SnapshotRoleName, msg.SnapshotRolePrompt,
	)
	if err != nil {
		return 0, err
	}

	messageID, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}

	return messageID, nil
}

// InitVersionGroup sets the version_group_id of a message to its own ID.
func (s *Store) InitVersionGroup(messageID int64) error {
	_, err := s.db.Exec(
		`UPDATE messages SET version_group_id = ? WHERE id = ?`,
		messageID, messageID,
	)
	return err
}

// AppendAssistantVersion adds a new version to an existing version group.
func (s *Store) AppendAssistantVersion(name string, anchorID int64, msg chat.Message) (int64, error) {
	if err := validateMessageSnapshot(msg); err != nil {
		return 0, err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	var convID int64
	if err := tx.QueryRow(`SELECT id FROM conversations WHERE name = ?`, name).Scan(&convID); err != nil {
		return 0, err
	}

	var versionGroupID int64
	if err := tx.QueryRow(`SELECT version_group_id FROM messages WHERE id = ?`, anchorID).Scan(&versionGroupID); err != nil {
		return 0, err
	}

	result, err := tx.Exec(
		`INSERT INTO messages(
			conversation_id, role, content, seq, version_group_id, version_number,
			is_active_version, edited_after_generation, stale_after_user_edit,
			snapshot_provider, snapshot_model, snapshot_api_format, snapshot_role_name, snapshot_role_prompt
		) VALUES(?, ?, ?, ?, ?, ?, 0, 0, 0, ?, ?, ?, ?, ?)`,
		convID, msg.Role, msg.Content, msg.Seq, versionGroupID, msg.VersionNumber,
		msg.SnapshotProvider, msg.SnapshotModel, msg.SnapshotAPIFormat, msg.SnapshotRoleName, msg.SnapshotRolePrompt,
	)
	if err != nil {
		return 0, err
	}

	messageID, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}

	return messageID, nil
}

// LoadActiveTimeline returns all active messages for the conversation ordered by seq.
func (s *Store) LoadActiveTimeline(name string) ([]chat.Message, error) {
	var convID int64
	err := s.db.QueryRow(`SELECT id FROM conversations WHERE name = ?`, name).Scan(&convID)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	rows, err := s.db.Query(`
		SELECT
			m.id,
			m.role,
			m.content,
			m.seq,
			COALESCE(m.version_group_id, 0),
			m.version_number,
			CASE
				WHEN m.role != 'assistant' THEN 1
				WHEN m.version_group_id IS NULL THEN 1
				ELSE (
					SELECT COUNT(*)
					FROM messages mv
					WHERE mv.version_group_id = m.version_group_id AND mv.is_deleted = 0
				)
			END AS total_versions,
			m.is_active_version,
			m.edited_after_generation,
			m.stale_after_user_edit,
			m.is_deleted,
			m.deleted_batch_id,
			m.snapshot_provider,
			m.snapshot_model,
			m.snapshot_api_format,
			m.snapshot_role_name,
			m.snapshot_role_prompt
		FROM messages m
		WHERE m.conversation_id = ? AND m.is_active_version = 1 AND m.is_deleted = 0
		ORDER BY m.seq, m.id
	`, convID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []chat.Message
	for rows.Next() {
		var msg chat.Message
		var isActive int
		var isDeleted int
		if err := rows.Scan(
			&msg.ID,
			&msg.Role,
			&msg.Content,
			&msg.Seq,
			&msg.VersionGroupID,
			&msg.VersionNumber,
			&msg.TotalVersions,
			&isActive,
			&msg.EditedAfterGeneration,
			&msg.StaleAfterUserEdit,
			&isDeleted,
			&msg.DeletedBatchID,
			&msg.SnapshotProvider,
			&msg.SnapshotModel,
			&msg.SnapshotAPIFormat,
			&msg.SnapshotRoleName,
			&msg.SnapshotRolePrompt,
		); err != nil {
			return nil, err
		}
		msg.IsActiveVersion = isActive == 1
		msg.Deleted = isDeleted == 1
		messages = append(messages, msg)
	}
	return messages, rows.Err()
}

// ListVersions returns all versions in a version group ordered by version_number.
func (s *Store) ListVersions(name string, anchorID int64) ([]chat.Message, error) {
	var convID int64
	if err := s.db.QueryRow(`SELECT id FROM conversations WHERE name = ?`, name).Scan(&convID); err != nil {
		return nil, err
	}

	var versionGroupID int64
	if err := s.db.QueryRow(`SELECT version_group_id FROM messages WHERE id = ?`, anchorID).Scan(&versionGroupID); err != nil {
		return nil, err
	}

	rows, err := s.db.Query(`
		SELECT id, role, content, seq, version_group_id, version_number, is_active_version, edited_after_generation, stale_after_user_edit, is_deleted, deleted_batch_id, snapshot_provider, snapshot_model, snapshot_api_format, snapshot_role_name, snapshot_role_prompt
		FROM messages
		WHERE version_group_id = ? AND is_deleted = 0
		ORDER BY version_number
	`, versionGroupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []chat.Message
	for rows.Next() {
		var msg chat.Message
		var isActive int
		var isDeleted int
		if err := rows.Scan(
			&msg.ID,
			&msg.Role,
			&msg.Content,
			&msg.Seq,
			&msg.VersionGroupID,
			&msg.VersionNumber,
			&isActive,
			&msg.EditedAfterGeneration,
			&msg.StaleAfterUserEdit,
			&isDeleted,
			&msg.DeletedBatchID,
			&msg.SnapshotProvider,
			&msg.SnapshotModel,
			&msg.SnapshotAPIFormat,
			&msg.SnapshotRoleName,
			&msg.SnapshotRolePrompt,
		); err != nil {
			return nil, err
		}
		msg.IsActiveVersion = isActive == 1
		msg.Deleted = isDeleted == 1
		messages = append(messages, msg)
	}
	for i := range messages {
		messages[i].TotalVersions = len(messages)
	}
	return messages, rows.Err()
}

// LoadBrowseMessages returns all messages for browser assembly, including deleted
// and inactive assistant versions, ordered for seq-based turn grouping.
func (s *Store) LoadBrowseMessages(name string) ([]chat.Message, error) {
	var convID int64
	err := s.db.QueryRow(`SELECT id FROM conversations WHERE name = ?`, name).Scan(&convID)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	rows, err := s.db.Query(`
		SELECT
			id,
			role,
			content,
			seq,
			COALESCE(version_group_id, 0),
			version_number,
			is_active_version,
			edited_after_generation,
			stale_after_user_edit,
			is_deleted,
			deleted_batch_id,
			snapshot_provider,
			snapshot_model,
			snapshot_api_format,
			snapshot_role_name,
			snapshot_role_prompt
		FROM messages
		WHERE conversation_id = ?
		ORDER BY seq, CASE WHEN role = 'user' THEN 0 ELSE 1 END, version_number, id
	`, convID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []chat.Message
	for rows.Next() {
		var msg chat.Message
		var isActive int
		var isDeleted int
		if err := rows.Scan(
			&msg.ID,
			&msg.Role,
			&msg.Content,
			&msg.Seq,
			&msg.VersionGroupID,
			&msg.VersionNumber,
			&isActive,
			&msg.EditedAfterGeneration,
			&msg.StaleAfterUserEdit,
			&isDeleted,
			&msg.DeletedBatchID,
			&msg.SnapshotProvider,
			&msg.SnapshotModel,
			&msg.SnapshotAPIFormat,
			&msg.SnapshotRoleName,
			&msg.SnapshotRolePrompt,
		); err != nil {
			return nil, err
		}
		msg.IsActiveVersion = isActive == 1
		msg.Deleted = isDeleted == 1
		messages = append(messages, msg)
	}
	return messages, rows.Err()
}

// SetActiveVersion marks a specific version as active in its version group.
func (s *Store) SetActiveVersion(name string, versionGroupID int64, versionNumber int) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(
		`UPDATE messages SET is_active_version = 0 WHERE version_group_id = ?`,
		versionGroupID,
	); err != nil {
		return err
	}

	if _, err := tx.Exec(
		`UPDATE messages SET is_active_version = 1 WHERE version_group_id = ? AND version_number = ?`,
		versionGroupID, versionNumber,
	); err != nil {
		return err
	}

	return tx.Commit()
}

// DeleteVersion deletes a specific version from a version group and promotes
// the nearest remaining version when the deleted version was active.
func (s *Store) DeleteVersion(messageID int64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var versionGroupID int64
	var versionNumber int
	var wasActive int
	err = tx.QueryRow(
		`SELECT version_group_id, version_number, is_active_version FROM messages WHERE id = ?`,
		messageID,
	).Scan(&versionGroupID, &versionNumber, &wasActive)
	if err != nil {
		return err
	}

	var count int
	if err := tx.QueryRow(
		`SELECT COUNT(*) FROM messages WHERE version_group_id = ?`,
		versionGroupID,
	).Scan(&count); err != nil {
		return err
	}
	if count <= 1 {
		return errors.New("cannot delete last version in group")
	}

	if _, err := tx.Exec(`DELETE FROM messages WHERE id = ?`, messageID); err != nil {
		return err
	}

	if wasActive == 1 {
		var promotedID int64
		err := tx.QueryRow(
			`SELECT id
			 FROM messages
			 WHERE version_group_id = ?
			 ORDER BY ABS(version_number - ?), version_number
			 LIMIT 1`,
			versionGroupID, versionNumber,
		).Scan(&promotedID)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(`UPDATE messages SET is_active_version = 1 WHERE id = ?`, promotedID); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (s *Store) SoftDeleteMessages(ids ...int64) (int64, error) {
	if len(ids) == 0 {
		return 0, nil
	}

	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	batchID := time.Now().UnixNano()
	type promotedVersionGroup struct {
		versionGroupID int64
		versionNumber  int
	}
	var affectedGroups []promotedVersionGroup
	for _, id := range ids {
		var role string
		var versionGroupID sql.NullInt64
		var versionNumber int
		var isActive int
		if err := tx.QueryRow(
			`SELECT role, version_group_id, version_number, is_active_version FROM messages WHERE id = ?`,
			id,
		).Scan(&role, &versionGroupID, &versionNumber, &isActive); err != nil {
			return 0, err
		}
		if _, err := tx.Exec(
			`UPDATE messages
			 SET is_deleted = 1, deleted_batch_id = ?
			 WHERE id = ?`,
			batchID, id,
		); err != nil {
			return 0, err
		}
		if role == "assistant" && versionGroupID.Valid && isActive == 1 {
			affectedGroups = append(affectedGroups, promotedVersionGroup{
				versionGroupID: versionGroupID.Int64,
				versionNumber:  versionNumber,
			})
		}
	}

	for _, group := range affectedGroups {
		var promotedID int64
		err := tx.QueryRow(
			`SELECT id
			 FROM messages
			 WHERE version_group_id = ? AND is_deleted = 0
			 ORDER BY ABS(version_number - ?), version_number
			 LIMIT 1`,
			group.versionGroupID, group.versionNumber,
		).Scan(&promotedID)
		if err == sql.ErrNoRows {
			continue
		}
		if err != nil {
			return 0, err
		}

		if _, err := tx.Exec(
			`UPDATE messages
			 SET is_active_version = 0
			 WHERE version_group_id = ? AND is_deleted = 0`,
			group.versionGroupID,
		); err != nil {
			return 0, err
		}
		if _, err := tx.Exec(
			`UPDATE messages SET is_active_version = 1 WHERE id = ?`,
			promotedID,
		); err != nil {
			return 0, err
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return batchID, nil
}

func (s *Store) RestoreDeletedBatch(batchID int64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	rows, err := tx.Query(
		`SELECT id, role, version_group_id, is_active_version
		 FROM messages
		 WHERE deleted_batch_id = ?`,
		batchID,
	)
	if err != nil {
		return err
	}
	defer rows.Close()

	type restoreRow struct {
		id             int64
		role           string
		versionGroupID sql.NullInt64
		isActive       int
	}
	var restoreRows []restoreRow
	for rows.Next() {
		var row restoreRow
		if err := rows.Scan(&row.id, &row.role, &row.versionGroupID, &row.isActive); err != nil {
			return err
		}
		restoreRows = append(restoreRows, row)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	if _, err := tx.Exec(
		`UPDATE messages
		 SET is_deleted = 0, deleted_batch_id = 0
		 WHERE deleted_batch_id = ?`,
		batchID,
	); err != nil {
		return err
	}

	for _, row := range restoreRows {
		if row.role != "assistant" || !row.versionGroupID.Valid || row.isActive != 1 {
			continue
		}
		if _, err := tx.Exec(
			`UPDATE messages SET is_active_version = 0 WHERE version_group_id = ?`,
			row.versionGroupID.Int64,
		); err != nil {
			return err
		}
		if _, err := tx.Exec(
			`UPDATE messages SET is_active_version = 1 WHERE id = ?`,
			row.id,
		); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (s *Store) PurgeDeleted() error {
	_, err := s.db.Exec(`DELETE FROM messages WHERE is_deleted = 1`)
	return err
}

// TruncateAfterSeq deletes all messages with seq > the given value.
func (s *Store) TruncateAfterSeq(name string, seq int) error {
	var convID int64
	if err := s.db.QueryRow(`SELECT id FROM conversations WHERE name = ?`, name).Scan(&convID); err != nil {
		return err
	}

	_, err := s.db.Exec(
		`DELETE FROM messages WHERE conversation_id = ? AND seq > ?`,
		convID, seq,
	)
	return err
}

// UpdateMessageContent updates the content of a specific message.
func (s *Store) UpdateMessageContent(messageID int64, content string) error {
	_, err := s.db.Exec(
		`UPDATE messages SET content = ? WHERE id = ?`,
		content, messageID,
	)
	return err
}

// UpdateAssistantMessage updates assistant content together with the
// generation snapshot that produced it.
func (s *Store) UpdateAssistantMessage(messageID int64, msg chat.Message) error {
	if msg.Role == "" {
		msg.Role = "assistant"
	}
	if err := validateMessageSnapshot(msg); err != nil {
		return err
	}
	_, err := s.db.Exec(
		`UPDATE messages
		 SET content = ?, snapshot_provider = ?, snapshot_model = ?, snapshot_api_format = ?, snapshot_role_name = ?, snapshot_role_prompt = ?
		 WHERE id = ?`,
		msg.Content, msg.SnapshotProvider, msg.SnapshotModel, msg.SnapshotAPIFormat, msg.SnapshotRoleName, msg.SnapshotRolePrompt, messageID,
	)
	return err
}

func (s *Store) SetConversationModelBinding(name, providerName, modelName string) error {
	_, err := s.db.Exec(
		`INSERT INTO conversations(name, provider_name, model_name, updated_at)
		 VALUES(?, ?, ?, CURRENT_TIMESTAMP)
		 ON CONFLICT(name) DO UPDATE SET
		 provider_name = excluded.provider_name,
		 model_name = excluded.model_name,
		 updated_at = CURRENT_TIMESTAMP`,
		name, providerName, modelName,
	)
	return err
}

func (s *Store) GetConversationModelBinding(name string) (string, string, error) {
	var providerName string
	var modelName string
	err := s.db.QueryRow(
		`SELECT provider_name, model_name FROM conversations WHERE name = ?`,
		name,
	).Scan(&providerName, &modelName)
	if err == sql.ErrNoRows {
		return "", "", ErrNotFound
	}
	if err != nil {
		return "", "", err
	}
	return providerName, modelName, nil
}

func (s *Store) RenameConversationProviderBindings(oldName, newName string) error {
	if oldName == "" || newName == "" || oldName == newName {
		return nil
	}
	_, err := s.db.Exec(
		`UPDATE conversations
		 SET provider_name = ?, updated_at = CURRENT_TIMESTAMP
		 WHERE provider_name = ?`,
		newName, oldName,
	)
	return err
}

func (s *Store) RenameConversationModelBindings(providerName, oldName, newName string) error {
	if providerName == "" || oldName == "" || newName == "" || oldName == newName {
		return nil
	}
	_, err := s.db.Exec(
		`UPDATE conversations
		 SET model_name = ?, updated_at = CURRENT_TIMESTAMP
		 WHERE provider_name = ? AND model_name = ?`,
		newName, providerName, oldName,
	)
	return err
}

func (s *Store) RenameSnapshotProvider(oldName, newName string) error {
	if oldName == "" || newName == "" || oldName == newName {
		return nil
	}
	_, err := s.db.Exec(
		`UPDATE messages
		 SET snapshot_provider = ?
		 WHERE snapshot_provider = ?`,
		newName, oldName,
	)
	return err
}

// MarkTurnEdited sets edited_after_generation=1 for the assistant message
// and stale_after_user_edit=1 for the user message.
func (s *Store) MarkTurnEdited(userID, assistantID int64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(
		`UPDATE messages SET edited_after_generation = 1 WHERE id = ?`,
		userID,
	); err != nil {
		return err
	}

	if _, err := tx.Exec(
		`UPDATE messages SET stale_after_user_edit = 1 WHERE id = ?`,
		assistantID,
	); err != nil {
		return err
	}

	return tx.Commit()
}

// ClearTurnEdited clears the edited flags for a user/assistant pair.
func (s *Store) ClearTurnEdited(userID, assistantID int64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(
		`UPDATE messages SET edited_after_generation = 0 WHERE id = ?`,
		userID,
	); err != nil {
		return err
	}

	if _, err := tx.Exec(
		`UPDATE messages SET stale_after_user_edit = 0 WHERE id = ?`,
		assistantID,
	); err != nil {
		return err
	}

	return tx.Commit()
}
