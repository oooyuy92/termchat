# Conversation Management Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add double-Esc message browsing mode to termchat, allowing rollback, single-message delete, branch, and clipboard copy from any message in the active conversation.

**Architecture:** A new `modeMessageBrowse` uiMode is entered via double-Esc in chat mode (counter-based, no timer). The view shows the full content of the selected message on its own page; ↑↓ navigates between messages. Operations (Enter=rollback, d=delete, b=branch, c=copy) modify the in-memory `History` and trigger auto-save.

**Tech Stack:** Go, BubbleTea (TUI), glamour (Markdown rendering), github.com/atotto/clipboard (clipboard), SQLite via modernc.org/sqlite

---

### Task 1: Add `DeleteAt` and `Truncate` to `chat.History`

**Files:**
- Modify: `internal/chat/history.go`
- Modify: `internal/chat/history_test.go`

**Step 1: Write the failing tests**

Add to `internal/chat/history_test.go`:

```go
func TestHistory_DeleteAt(t *testing.T) {
	h := NewHistory()
	h.Add(Message{Role: "user", Content: "a"})
	h.Add(Message{Role: "assistant", Content: "b"})
	h.Add(Message{Role: "user", Content: "c"})

	h.DeleteAt(1) // remove "b"

	msgs := h.Messages()
	if len(msgs) != 2 {
		t.Fatalf("len = %d, want 2", len(msgs))
	}
	if msgs[0].Content != "a" || msgs[1].Content != "c" {
		t.Errorf("got %v, want [a c]", msgs)
	}
}

func TestHistory_DeleteAt_OutOfBounds(t *testing.T) {
	h := NewHistory()
	h.Add(Message{Role: "user", Content: "a"})
	h.DeleteAt(-1)  // no-op
	h.DeleteAt(5)   // no-op
	if len(h.Messages()) != 1 {
		t.Errorf("len = %d, want 1 (out-of-bounds delete should be no-op)", len(h.Messages()))
	}
}

func TestHistory_Truncate(t *testing.T) {
	h := NewHistory()
	h.Add(Message{Role: "user", Content: "a"})
	h.Add(Message{Role: "assistant", Content: "b"})
	h.Add(Message{Role: "user", Content: "c"})

	h.Truncate(2) // keep first 2

	msgs := h.Messages()
	if len(msgs) != 2 {
		t.Fatalf("len = %d, want 2", len(msgs))
	}
	if msgs[0].Content != "a" || msgs[1].Content != "b" {
		t.Errorf("got %v, want [a b]", msgs)
	}
}

func TestHistory_Truncate_Noop(t *testing.T) {
	h := NewHistory()
	h.Add(Message{Role: "user", Content: "a"})
	h.Truncate(5) // n > len → no-op
	if len(h.Messages()) != 1 {
		t.Errorf("len = %d, want 1", len(h.Messages()))
	}
}
```

**Step 2: Run tests to verify they fail**

```bash
go test ./internal/chat/... -run "TestHistory_DeleteAt|TestHistory_Truncate" -v
```

Expected: FAIL with "undefined: (*History).DeleteAt" or similar.

**Step 3: Implement the methods**

Add to `internal/chat/history.go` (after the `Count` method):

```go
// DeleteAt removes the message at index i. No-op if i is out of bounds.
func (h *History) DeleteAt(i int) {
	if i < 0 || i >= len(h.messages) {
		return
	}
	h.messages = append(h.messages[:i], h.messages[i+1:]...)
}

// Truncate keeps only the first n messages, discarding the rest.
// No-op if n >= len(messages).
func (h *History) Truncate(n int) {
	if n >= len(h.messages) {
		return
	}
	h.messages = h.messages[:n]
}
```

**Step 4: Run tests to verify they pass**

```bash
go test ./internal/chat/... -v
```

Expected: all tests PASS.

**Step 5: Commit**

```bash
git add internal/chat/history.go internal/chat/history_test.go
git commit -m "feat: add History.DeleteAt and History.Truncate"
```

---

### Task 2: Add `modeMessageBrowse`, `escCount`, `browseCursor` to model

**Files:**
- Modify: `internal/ui/model.go`

**Step 1: Add `modeMessageBrowse` to the uiMode iota**

In `internal/ui/model.go`, find the `const (` block starting with `modeChat uiMode = iota`. Add `modeMessageBrowse` after `modeSlashComplete`:

```go
const (
	modeChat uiMode = iota
	modeConfig
	modeResume
	modeShortcuts
	modeRolePicker
	modeRoles
	modeOnboard
	modeSlashComplete
	modeMessageBrowse  // ← add this
)
```

**Step 2: Add fields to `Model` struct**

In the `Model` struct (after `slashAC slashComplete`), add:

```go
escCount     int // consecutive Esc presses in chat mode for double-Esc detection
browseCursor int // index of selected message in modeMessageBrowse
```

**Step 3: Verify the project still builds**

```bash
go build ./...
```

Expected: builds with no errors.

**Step 4: Commit**

```bash
git add internal/ui/model.go
git commit -m "feat: add modeMessageBrowse, escCount, browseCursor to model"
```

---

### Task 3: Add `github.com/atotto/clipboard` dependency

**Files:**
- Modify: `go.mod`, `go.sum`

**Step 1: Fetch the dependency**

```bash
go get github.com/atotto/clipboard
go mod tidy
```

**Step 2: Verify it downloads cleanly**

```bash
go build ./...
```

Expected: builds with no errors.

**Step 3: Commit**

```bash
git add go.mod go.sum
git commit -m "chore: add atotto/clipboard dependency"
```

---

### Task 4: Create `messagebrowse.go` with update and view handlers

**Files:**
- Create: `internal/ui/messagebrowse.go`

**Step 1: Create the file**

Create `internal/ui/messagebrowse.go` with this exact content:

```go
// internal/ui/messagebrowse.go
package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/atotto/clipboard"
	tea "github.com/charmbracelet/bubbletea"
)

func (m Model) updateMessageBrowse(msg tea.KeyMsg) (Model, tea.Cmd) {
	// Reset confirmQuit on any key other than ctrl+c
	if msg.String() != "ctrl+c" {
		m.confirmQuit = false
	}

	msgs := m.history.Messages()

	switch msg.String() {
	case "esc":
		m.mode = modeChat
		m.escCount = 0
		return m, nil

	case "ctrl+c":
		if m.confirmQuit {
			return m, tea.Quit
		}
		m.confirmQuit = true
		m.statusMsg = "Press Ctrl+C again to quit"
		return m, nil

	case "up", "k":
		if m.browseCursor > 0 {
			m.browseCursor--
		}

	case "down", "j":
		if m.browseCursor < len(msgs)-1 {
			m.browseCursor++
		}

	case "enter":
		// Rollback: keep messages[0..cursor] inclusive
		n := m.browseCursor + 1
		m.history.Truncate(n)
		m.statusMsg = fmt.Sprintf("Rolled back to message %d", n)
		m.mode = modeChat
		return m, m.autoSaveCmd()

	case "d":
		if len(msgs) == 0 {
			return m, nil
		}
		m.history.DeleteAt(m.browseCursor)
		remaining := m.history.Messages()
		if len(remaining) == 0 {
			m.mode = modeChat
			m.statusMsg = "All messages deleted"
			return m, m.autoSaveCmd()
		}
		if m.browseCursor >= len(remaining) {
			m.browseCursor = len(remaining) - 1
		}
		return m, m.autoSaveCmd()

	case "b":
		// Branch: save history[0..cursor] as a brand-new conversation
		newName := time.Now().Format("2006-01-02_150405")
		m.history.Truncate(m.browseCursor + 1)
		m.autoSaveName = newName
		m.statusMsg = "Branched: " + newName
		m.mode = modeChat
		return m, m.autoSaveCmd()

	case "c":
		if len(msgs) > m.browseCursor {
			if err := clipboard.WriteAll(msgs[m.browseCursor].Content); err != nil {
				m.statusMsg = "Copy failed: " + err.Error()
			} else {
				m.statusMsg = "Copied"
			}
		}
	}

	return m, nil
}

func (m Model) viewMessageBrowse() string {
	msgs := m.history.Messages()
	if len(msgs) == 0 {
		var b strings.Builder
		b.WriteString(m.theme.ConfigTitleStyle().Render("Browse Messages") + "\n\n")
		b.WriteString(m.theme.ConfigHelpStyle().Render("  No messages.") + "\n\n")
		b.WriteString(m.theme.ConfigHelpStyle().Render("  Esc: back") + "\n")
		return b.String() + "\n" + m.renderStatusBar()
	}

	cur := m.browseCursor
	if cur >= len(msgs) {
		cur = len(msgs) - 1
	}
	selected := msgs[cur]

	var b strings.Builder

	// Header line
	header := fmt.Sprintf("── Message %d / %d ── %s ", cur+1, len(msgs), selected.Role)
	if m.width > len(header)+2 {
		header += strings.Repeat("─", m.width-len(header)-1)
	}
	b.WriteString(m.theme.ConfigTitleStyle().Render(header) + "\n\n")

	// Full message content
	var content string
	if selected.Role == "assistant" {
		rendered, err := m.renderer.Render(selected.Content)
		if err != nil {
			content = selected.Content
		} else {
			content = cleanGlamourOutput(rendered)
		}
	} else {
		content = selected.Content
	}

	// Clip content to available height to avoid overflow
	// Available lines = total height - header(2) - blank(1) - divider(1) - help(1) - status(1) - blank(1)
	availableLines := m.height - 7
	if availableLines < 1 {
		availableLines = 1
	}
	contentLines := strings.Split(content, "\n")
	clipped := false
	if len(contentLines) > availableLines {
		contentLines = contentLines[:availableLines]
		clipped = true
	}
	b.WriteString(strings.Join(contentLines, "\n"))
	if clipped {
		b.WriteString("\n" + m.theme.ConfigHelpStyle().Render("(↓ more…)"))
	}
	b.WriteString("\n\n")

	// Bottom divider
	divider := strings.Repeat("─", m.width-1)
	if m.width <= 1 {
		divider = "─"
	}
	b.WriteString(divider + "\n")
	b.WriteString(m.theme.ConfigHelpStyle().Render(
		"↑↓: prev/next  Enter: rollback  d: delete  b: branch  c: copy  Esc: back",
	) + "\n")

	return b.String() + "\n" + m.renderStatusBar()
}
```

**Step 2: Build to verify it compiles**

```bash
go build ./...
```

Expected: builds with no errors. (No tests for this file — behavior is tested end-to-end via wiring in Task 5.)

**Step 3: Commit**

```bash
git add internal/ui/messagebrowse.go
git commit -m "feat: add message browse update and view handlers"
```

---

### Task 5: Wire double-Esc detection and routing into `update.go` and `view.go`

**Files:**
- Modify: `internal/ui/update.go`
- Modify: `internal/ui/view.go`

**Step 1: Add `modeMessageBrowse` routing to `Update()` in `update.go`**

In `internal/ui/update.go`, find the `case tea.KeyMsg:` block. After the `modeSlashComplete` check (around line 46), add:

```go
if m.mode == modeMessageBrowse {
    return m.updateMessageBrowse(msg)
}
```

**Step 2: Add double-Esc detection in chat mode**

Still in `update.go`, inside the `switch msg.String()` block (around line 72), add a new case **before** `case "ctrl+c":`:

```go
case "esc":
    if m.streaming {
        return m, nil
    }
    m.escCount++
    if m.escCount >= 2 {
        m.escCount = 0
        if len(m.history.Messages()) == 0 {
            m.statusMsg = "No messages to browse"
            return m, nil
        }
        m.browseCursor = len(m.history.Messages()) - 1
        m.mode = modeMessageBrowse
    } else {
        m.statusMsg = "Press Esc again to browse messages"
    }
    return m, nil
```

Also update the `escCount` reset: in the existing line that reads:
```go
if msg.String() != "ctrl+c" {
    m.confirmQuit = false
}
```
change it to also reset `escCount` on non-Esc keys:
```go
if msg.String() != "ctrl+c" {
    m.confirmQuit = false
}
if msg.String() != "ctrl+c" && msg.String() != "esc" {
    m.escCount = 0
}
```

**Step 3: Add `modeMessageBrowse` routing to `View()` in `view.go`**

In `internal/ui/view.go`, after the `modeSlashComplete` check, add:

```go
if m.mode == modeMessageBrowse {
    return m.viewMessageBrowse()
}
```

**Step 4: Run all tests**

```bash
go test ./...
```

Expected: all packages PASS.

**Step 5: Build**

```bash
go build ./...
```

Expected: builds with no errors.

**Step 6: Commit**

```bash
git add internal/ui/update.go internal/ui/view.go
git commit -m "feat: wire message browse mode with double-Esc trigger"
```
