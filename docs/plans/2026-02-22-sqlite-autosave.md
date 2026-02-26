# SQLite Auto-Save Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Replace JSON file storage with SQLite (modernc.org/sqlite) and add automatic conversation persistence after each AI response.

**Architecture:** The `storage.Store` is rewritten to open a single `termchat.db` SQLite file. The existing `Save`/`Load`/`List` interface is preserved so slash commands continue working. A new `autoSaveName` field in `Model` (set to a session timestamp at startup) drives per-session auto-save triggered from the `streamDoneMsg` handler.

**Tech Stack:** `modernc.org/sqlite` (pure Go, no CGo), `database/sql`, `time` package for session naming.

---

## Key files

| File | Role |
|------|------|
| `internal/storage/conversation.go` | Complete rewrite — SQLite backend |
| `internal/storage/conversation_test.go` | Update tests — remove JSON path check |
| `internal/ui/model.go` | `New` now returns error; add `autoSaveName` field |
| `internal/ui/update.go` | Call `autoSaveCmd()` in `streamDoneMsg` handler |
| `go.mod` / `go.sum` | Add `modernc.org/sqlite` |

## SQLite schema

```sql
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
```

`seq` preserves insertion order (SQLite rowid order is reliable, but explicit is safer).

## Auto-save behavior

- `Model.autoSaveName` is set in `NewModel` to `time.Now().Format("2006-01-02_150405")`.
- After each `streamDoneMsg`, `autoSaveCmd()` calls `store.Save(m.autoSaveName, m.history.Messages())`.
- `/save [name]` saves under user-chosen name AND sets `m.autoSaveName = name` (future auto-saves update the same named conversation).
- `/load [name]` loads conversation AND sets `m.autoSaveName = name`.
- `/list` shows all conversations.

---

### Task 1: Add modernc.org/sqlite dependency

**Files:**
- Modify: `go.mod`, `go.sum`

**Step 1: Add the dependency**

```bash
cd /home/cc/Github/llm-chat-in-terminal
GOPATH=/home/cc/gopath /home/cc/go/bin/go get modernc.org/sqlite@latest
```

Expected output: `go: added modernc.org/sqlite v...`

If network is unavailable, try with direct proxy:
```bash
GOPATH=/home/cc/gopath GOPROXY=https://goproxy.cn,direct /home/cc/go/bin/go get modernc.org/sqlite@latest
```

**Step 2: Verify go.mod contains the dependency**

```bash
grep modernc /home/cc/Github/llm-chat-in-terminal/go.mod
```

Expected: line containing `modernc.org/sqlite v...`

**Step 3: Commit**

```bash
cd /home/cc/Github/llm-chat-in-terminal
git add go.mod go.sum
git commit -m "chore: add modernc.org/sqlite dependency"
```

---

### Task 2: Rewrite storage layer to SQLite

**Files:**
- Modify: `internal/storage/conversation.go` (complete rewrite)

**Step 1: Write the new implementation**

Replace the entire file with:

```go
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
```

**Step 2: Build to verify compilation**

```bash
cd /home/cc/Github/llm-chat-in-terminal
GOPATH=/home/cc/gopath GOMODCACHE=/home/cc/gopath/pkg/mod /home/cc/go/bin/go build ./internal/storage/...
```

Expected: no output (success). Fix any compilation errors before proceeding.

**Step 3: Commit**

```bash
git add internal/storage/conversation.go
git commit -m "feat: rewrite storage layer to use SQLite"
```

---

### Task 3: Update storage tests

**Files:**
- Modify: `internal/storage/conversation_test.go` (complete rewrite)

**Step 1: Write updated tests**

The old tests check for a `.json` file on disk — that no longer applies. Rewrite to test actual SQLite behavior:

```go
// internal/storage/conversation_test.go
package storage

import (
	"database/sql"
	"testing"

	"github.com/termchat/termchat/internal/chat"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	store, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	t.Cleanup(func() { store.Close() })
	return store
}

func TestSaveAndLoad(t *testing.T) {
	store := newTestStore(t)

	messages := []chat.Message{
		{Role: "user", Content: "hello"},
		{Role: "assistant", Content: "hi there"},
	}

	if err := store.Save("test-conv", messages); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	loaded, err := store.Load("test-conv")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if len(loaded) != 2 {
		t.Fatalf("loaded len = %d, want 2", len(loaded))
	}
	if loaded[0].Content != "hello" {
		t.Errorf("loaded[0].Content = %q, want %q", loaded[0].Content, "hello")
	}
	if loaded[1].Role != "assistant" {
		t.Errorf("loaded[1].Role = %q, want %q", loaded[1].Role, "assistant")
	}
}

func TestSaveOverwrites(t *testing.T) {
	store := newTestStore(t)

	store.Save("conv", []chat.Message{{Role: "user", Content: "first"}})
	store.Save("conv", []chat.Message{{Role: "user", Content: "second"}})

	loaded, err := store.Load("conv")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(loaded) != 1 || loaded[0].Content != "second" {
		t.Errorf("expected only second message, got %+v", loaded)
	}
}

func TestLoadNotFound(t *testing.T) {
	store := newTestStore(t)

	_, err := store.Load("nonexistent")
	if err != sql.ErrNoRows {
		t.Errorf("Load() error = %v, want sql.ErrNoRows", err)
	}
}

func TestList(t *testing.T) {
	store := newTestStore(t)

	store.Save("conv-a", []chat.Message{{Role: "user", Content: "a"}})
	store.Save("conv-b", []chat.Message{{Role: "user", Content: "b"}})

	names, err := store.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(names) != 2 {
		t.Fatalf("List() len = %d, want 2", len(names))
	}
}

func TestListEmpty(t *testing.T) {
	store := newTestStore(t)

	names, err := store.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(names) != 0 {
		t.Errorf("List() = %v, want empty", names)
	}
}
```

**Step 2: Run tests**

```bash
cd /home/cc/Github/llm-chat-in-terminal
GOPATH=/home/cc/gopath GOMODCACHE=/home/cc/gopath/pkg/mod /home/cc/go/bin/go test ./internal/storage/... -v
```

Expected: all 5 tests PASS.

**Step 3: Commit**

```bash
git add internal/storage/conversation_test.go
git commit -m "test: update storage tests for SQLite backend"
```

---

### Task 4: Update model.go — New() returns error, add autoSaveName

**Files:**
- Modify: `internal/ui/model.go`

**Step 1: Update imports and Model struct**

Add `"time"` to imports.

Add `autoSaveName string` field to `Model`:

```go
type Model struct {
    cfg      config.Config
    client   *chat.Client
    history  *chat.History
    store    *storage.Store
    renderer *glamour.TermRenderer

    // Theme
    theme Theme

    // Streaming channels
    streamCh  <-chan chat.StreamChunk
    streamErr <-chan error

    // Stream cancellation
    streamCtrl *streamControl

    // UI state
    mode            uiMode
    configEd        configEditor
    cfgPath         string
    autoSaveName    string  // ← new: name used for auto-saving current session
    input           string
    streaming       bool
    confirmQuit     bool
    currentResp     string
    currentThinking string
    statusMsg       string
    totalTokens     int
    width           int
    height          int
    err             error
}
```

**Step 2: Update NewModel to handle store error and set autoSaveName**

```go
func NewModel(cfg config.Config, cfgPath string) (Model, error) {
    initialWidth := 80
    if w, _, err := term.GetSize(int(os.Stdout.Fd())); err == nil && w > 0 {
        initialWidth = w
    }
    renderer, err := buildRenderer(cfg.Settings.Theme, initialWidth)
    if err != nil {
        return Model{}, err
    }

    storageDir := cfg.Storage.Dir
    if len(storageDir) > 0 && storageDir[0] == '~' {
        home, _ := os.UserHomeDir()
        storageDir = home + storageDir[1:]
    }

    store, err := storage.New(storageDir)
    if err != nil {
        return Model{}, fmt.Errorf("open storage: %w", err)
    }

    return Model{
        cfg:          cfg,
        client:       chat.NewClient(cfg.API.BaseURL, cfg.API.APIKey, cfg.API.Model),
        history:      chat.NewHistory(),
        store:        store,
        renderer:     renderer,
        cfgPath:      cfgPath,
        theme:        ThemeByName(cfg.Settings.Theme),
        autoSaveName: time.Now().Format("2006-01-02_150405"),
    }, nil
}
```

Also add `"fmt"` and `"time"` to the import block.

**Step 3: Build**

```bash
cd /home/cc/Github/llm-chat-in-terminal
GOPATH=/home/cc/gopath GOMODCACHE=/home/cc/gopath/pkg/mod /home/cc/go/bin/go build ./...
```

Expected: no errors. (main.go passes `NewModel` errors to `os.Exit(1)` already, so no change needed there.)

**Step 4: Commit**

```bash
git add internal/ui/model.go
git commit -m "feat: add autoSaveName field and propagate storage.New error"
```

---

### Task 5: Add auto-save and update slash commands

**Files:**
- Modify: `internal/ui/update.go`

**Step 1: Add autoSavedMsg type and autoSaveCmd method**

Add to the message types section at top of `update.go` (after other msg types or in model.go):

```go
// autoSavedMsg is a no-op result from auto-saving; it carries no UI state.
type autoSavedMsg struct{}
```

Add `autoSaveCmd` method on `Model` (add at the bottom of `update.go`):

```go
func (m Model) autoSaveCmd() tea.Cmd {
    msgs := m.history.Messages()
    store := m.store
    name := m.autoSaveName
    return func() tea.Msg {
        _ = store.Save(name, msgs)
        return autoSavedMsg{}
    }
}
```

**Step 2: Call autoSaveCmd in streamDoneMsg handler**

In the `Update` function, change `streamDoneMsg` case from:

```go
case streamDoneMsg:
    m.streaming = false
    if m.currentResp != "" {
        m.history.Add(chat.Message{Role: "assistant", Content: m.currentResp})
    }
    m.currentResp = ""
    m.currentThinking = ""
    return m, nil
```

To:

```go
case streamDoneMsg:
    m.streaming = false
    if m.currentResp != "" {
        m.history.Add(chat.Message{Role: "assistant", Content: m.currentResp})
    }
    m.currentResp = ""
    m.currentThinking = ""
    return m, m.autoSaveCmd()

case autoSavedMsg:
    return m, nil
```

**Step 3: Update /save and /load to set autoSaveName**

In `handleCommand`, update `/save`:

```go
case "/save":
    name := "default"
    if len(parts) >= 2 {
        name = parts[1]
    }
    err := m.store.Save(name, m.history.Messages())
    if err != nil {
        m.statusMsg = "Save failed: " + err.Error()
    } else {
        m.autoSaveName = name   // ← future auto-saves update this conversation
        m.statusMsg = "Saved as " + name
    }
    return m, nil
```

Update `/load`:

```go
case "/load":
    name := "default"
    if len(parts) >= 2 {
        name = parts[1]
    }
    msgs, err := m.store.Load(name)
    if err != nil {
        m.statusMsg = "Load failed: " + err.Error()
    } else {
        m.history.Clear()
        for _, msg := range msgs {
            m.history.Add(msg)
        }
        m.autoSaveName = name   // ← future auto-saves update this conversation
        m.statusMsg = "Loaded " + name
    }
    return m, nil
```

**Step 4: Build**

```bash
cd /home/cc/Github/llm-chat-in-terminal
GOPATH=/home/cc/gopath GOMODCACHE=/home/cc/gopath/pkg/mod /home/cc/go/bin/go build ./...
```

Expected: no errors.

**Step 5: Run all tests**

```bash
GOPATH=/home/cc/gopath GOMODCACHE=/home/cc/gopath/pkg/mod /home/cc/go/bin/go test ./...
```

Expected: all tests pass.

**Step 6: Commit**

```bash
git add internal/ui/update.go
git commit -m "feat: auto-save conversation to SQLite after each AI response"
```

---

## Verification checklist

After all tasks complete:

1. `go build ./...` succeeds with no errors
2. `go test ./...` all pass
3. Run the app, send a message, quit — relaunch and use `/list` to confirm the session appears
4. `/load <session-name>` restores the conversation
5. `/save myconv` followed by `/list` shows `myconv` in the list
6. After `/save myconv`, send another message — auto-save goes to `myconv` (verify with `/load myconv`)
