# Settings Rename + Resume Picker Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Rename `/model` to `/settings` and add a `/resume` command with a date-grouped, keyboard-navigable conversation picker.

**Architecture:** `/settings` is a one-line rename. `/resume` adds a new `modeResume` UI mode following the same pattern as `modeConfig` — a dedicated `resumepicker.go` file with state types, update logic, and view rendering. The storage layer gets a new `ListWithDate()` method that returns conversations with their date string for grouping. The picker is loaded synchronously (SQLite query is fast) when entering the mode.

**Tech Stack:** Go, BubbleTea (mode-based UI), lipgloss (styling), SQLite via `database/sql` + `modernc.org/sqlite`.

---

## UI layout for `/resume`

```
Resume a Conversation

  ◀ 2026-02-22 ▶   (1/3 dates)

  2026-02-22_103000
> 2026-02-22_150405    ← cursor (up/down to move)
  myconv

  ↑↓: select  |  ←→: change date  |  Enter: resume  |  Esc: back
```

- `◀`/`▶` appear only when there are adjacent dates
- Left/right keys cycle between date groups (clamp, no wrap)
- Up/down keys move the cursor within the current date group
- `Enter` loads the conversation, returns to chat, sets `autoSaveName` to that name
- `Esc` returns to chat without loading anything
- `h/l` also work as left/right, `k/j` as up/down (vim-style)

---

## Key files

| File | Change |
|------|--------|
| `internal/ui/update.go` | Rename `/model` → `/settings`; add `/resume` handler; route `modeResume` keys |
| `internal/ui/view.go` | Add `modeResume` branch |
| `internal/ui/model.go` | Add `modeResume` const; `dateGroup`, `resumePicker` types; `resumePick` field |
| `internal/ui/resumepicker.go` | New file: `buildResumePicker`, `updateResumeMode`, `viewResumePicker` |
| `internal/storage/conversation.go` | Add `ConvInfo` type + `ListWithDate()` method |
| `internal/storage/conversation_test.go` | Add `TestListWithDate` test |

---

### Task 1: Rename `/model` to `/settings`

**Files:**
- Modify: `internal/ui/update.go` (line ~206)

**Step 1: Read the file to find the exact text**

```bash
grep -n "/model" /home/cc/Github/llm-chat-in-terminal/internal/ui/update.go
```

**Step 2: Change `case "/model":` to `case "/settings":`**

In `handleCommand` in `update.go`, find:
```go
	case "/model":
		if len(parts) < 2 {
			// No argument: enter config editor mode
			m.mode = modeConfig
			m.configEd = configEditor{
				fields: buildConfigFields(m.cfg),
			}
			return m, nil
		}
		m.client.SetModel(parts[1])
		m.cfg.API.Model = parts[1]
		m.statusMsg = "Model set to " + parts[1]
		return m, m.saveConfigCmd()
```

Change `case "/model":` to `case "/settings":`.

**Step 3: Build**

```bash
GOPATH=/home/cc/gopath GOMODCACHE=/home/cc/gopath/pkg/mod /home/cc/go/bin/go build ./...
```

Expected: no errors.

**Step 4: Commit**

```bash
cd /home/cc/Github/llm-chat-in-terminal
git add internal/ui/update.go
git commit -m "feat: rename /model command to /settings"
```

---

### Task 2: Add `ConvInfo` + `ListWithDate()` to storage

**Files:**
- Modify: `internal/storage/conversation.go`
- Modify: `internal/storage/conversation_test.go`

**Step 1: Add `ConvInfo` and `ListWithDate()` to `conversation.go`**

Add after the `ErrNotFound` declaration:

```go
// ConvInfo holds summary information for a conversation.
type ConvInfo struct {
	Name string
	Date string // "YYYY-MM-DD" (date portion of updated_at)
}
```

Add after the `List()` method:

```go
// ListWithDate returns all conversations with their date string (YYYY-MM-DD),
// ordered by most recently updated.
func (s *Store) ListWithDate() ([]ConvInfo, error) {
	rows, err := s.db.Query(
		`SELECT name, date(updated_at) FROM conversations ORDER BY updated_at DESC, id DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var convs []ConvInfo
	for rows.Next() {
		var c ConvInfo
		if err := rows.Scan(&c.Name, &c.Date); err != nil {
			return nil, err
		}
		convs = append(convs, c)
	}
	return convs, rows.Err()
}
```

**Step 2: Write failing test**

Add to `conversation_test.go`:

```go
func TestListWithDate(t *testing.T) {
	store := newTestStore(t)

	if err := store.Save("conv-a", []chat.Message{{Role: "user", Content: "a"}}); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	if err := store.Save("conv-b", []chat.Message{{Role: "user", Content: "b"}}); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	convs, err := store.ListWithDate()
	if err != nil {
		t.Fatalf("ListWithDate() error = %v", err)
	}
	if len(convs) != 2 {
		t.Fatalf("ListWithDate() len = %d, want 2", len(convs))
	}
	// Date must be a non-empty string in YYYY-MM-DD format (10 chars).
	for _, c := range convs {
		if len(c.Date) != 10 {
			t.Errorf("conv %q: Date = %q, want 10-char YYYY-MM-DD", c.Name, c.Date)
		}
		if c.Name == "" {
			t.Error("expected non-empty Name")
		}
	}
	// conv-b was saved last — must appear first.
	if convs[0].Name != "conv-b" {
		t.Errorf("convs[0].Name = %q, want %q", convs[0].Name, "conv-b")
	}
}
```

**Step 3: Run test to verify it fails first**

```bash
cd /home/cc/Github/llm-chat-in-terminal
GOPATH=/home/cc/gopath GOMODCACHE=/home/cc/gopath/pkg/mod /home/cc/go/bin/go test ./internal/storage/... -run TestListWithDate -v
```

Expected: FAIL (method does not exist yet).

**Step 4: Implement `ListWithDate()` (as in Step 1 above)**

**Step 5: Run tests — all must pass**

```bash
GOPATH=/home/cc/gopath GOMODCACHE=/home/cc/gopath/pkg/mod /home/cc/go/bin/go test ./internal/storage/... -v
```

Expected: all 6 tests pass.

**Step 6: Commit**

```bash
git add internal/storage/conversation.go internal/storage/conversation_test.go
git commit -m "feat: add ConvInfo type and ListWithDate() to storage"
```

---

### Task 3: Add resume picker state to model.go

**Files:**
- Modify: `internal/ui/model.go`

**Step 1: Add `modeResume` to the mode constants**

Find:
```go
const (
	modeChat uiMode = iota
	modeConfig
)
```

Change to:
```go
const (
	modeChat uiMode = iota
	modeConfig
	modeResume
)
```

**Step 2: Add `dateGroup` and `resumePicker` types**

Add after the `configEditor` struct:

```go
type dateGroup struct {
	date  string   // "YYYY-MM-DD"
	convs []string // conversation names in this group (most recent first)
}

type resumePicker struct {
	groups  []dateGroup
	dateIdx int // index into groups (left/right navigation)
	convIdx int // index within groups[dateIdx].convs (up/down navigation)
}
```

**Step 3: Add `resumePick` field to `Model` struct**

Add after `configEd configEditor`:
```go
	resumePick      resumePicker
```

**Step 4: Build to verify compilation**

```bash
GOPATH=/home/cc/gopath GOMODCACHE=/home/cc/gopath/pkg/mod /home/cc/go/bin/go build ./...
```

Expected: no errors.

**Step 5: Commit**

```bash
git add internal/ui/model.go
git commit -m "feat: add modeResume, dateGroup, resumePicker types to model"
```

---

### Task 4: Implement resume picker — new file + wire up

**Files:**
- Create: `internal/ui/resumepicker.go`
- Modify: `internal/ui/update.go` (add `/resume` command + `modeResume` routing)
- Modify: `internal/ui/view.go` (add `modeResume` branch)

**Step 1: Create `internal/ui/resumepicker.go`**

```go
// internal/ui/resumepicker.go
package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/termchat/termchat/internal/storage"
)

// buildResumePicker groups ConvInfo by date (preserving most-recent-first order)
// and returns an initialized resumePicker.
func buildResumePicker(convs []storage.ConvInfo) resumePicker {
	if len(convs) == 0 {
		return resumePicker{}
	}
	var groups []dateGroup
	groupIdx := map[string]int{}
	for _, c := range convs {
		idx, ok := groupIdx[c.Date]
		if !ok {
			idx = len(groups)
			groups = append(groups, dateGroup{date: c.Date})
			groupIdx[c.Date] = idx
		}
		groups[idx].convs = append(groups[idx].convs, c.Name)
	}
	return resumePicker{groups: groups}
}

func (m Model) updateResumeMode(msg tea.KeyMsg) (Model, tea.Cmd) {
	p := &m.resumePick

	if msg.String() != "ctrl+c" {
		m.confirmQuit = false
	}

	switch msg.String() {
	case "left", "h":
		if p.dateIdx > 0 {
			p.dateIdx--
			p.convIdx = 0
		}
	case "right", "l":
		if p.dateIdx < len(p.groups)-1 {
			p.dateIdx++
			p.convIdx = 0
		}
	case "up", "k":
		if p.convIdx > 0 {
			p.convIdx--
		}
	case "down", "j":
		if len(p.groups) > 0 && p.convIdx < len(p.groups[p.dateIdx].convs)-1 {
			p.convIdx++
		}
	case "enter":
		if len(p.groups) == 0 {
			m.mode = modeChat
			m.statusMsg = "No conversations"
			return m, nil
		}
		name := p.groups[p.dateIdx].convs[p.convIdx]
		msgs, err := m.store.Load(name)
		if err != nil {
			m.statusMsg = "Load failed: " + err.Error()
			m.mode = modeChat
			return m, nil
		}
		m.autoSaveName = name
		m.history.Clear()
		for _, msg := range msgs {
			m.history.Add(msg)
		}
		m.mode = modeChat
		m.statusMsg = "Resumed: " + name
	case "esc":
		m.mode = modeChat
		m.statusMsg = "Cancelled"
	case "ctrl+c":
		if m.confirmQuit {
			return m, tea.Quit
		}
		m.confirmQuit = true
		m.statusMsg = "Press Ctrl+C again to quit"
	}
	return m, nil
}

func (m Model) viewResumePicker() string {
	var b strings.Builder
	p := m.resumePick

	b.WriteString(m.theme.ConfigTitleStyle().Render("Resume a Conversation"))
	b.WriteString("\n\n")

	if len(p.groups) == 0 {
		b.WriteString(m.theme.ConfigHelpStyle().Render("  No saved conversations."))
		b.WriteString("\n\n")
		b.WriteString(m.theme.ConfigHelpStyle().Render("  Esc: back to chat"))
		b.WriteString("\n")
		return b.String() + "\n" + m.renderStatusBar()
	}

	// Date navigation row
	prev := "  "
	if p.dateIdx > 0 {
		prev = "◀ "
	}
	next := "  "
	if p.dateIdx < len(p.groups)-1 {
		next = " ▶"
	}
	dateStr := p.groups[p.dateIdx].date
	b.WriteString("  " + prev + m.theme.ConfigTitleStyle().Render(dateStr) + next)
	b.WriteString("\n\n")

	// Conversation list for current date group
	group := p.groups[p.dateIdx]
	for i, name := range group.convs {
		cursor := "  "
		if i == p.convIdx {
			cursor = m.theme.ConfigCursorStyle().Render("> ")
		}
		b.WriteString(cursor + m.theme.ConfigValueStyle().Render(name) + "\n")
	}

	b.WriteString("\n")
	b.WriteString(m.theme.ConfigHelpStyle().Render("  ↑↓: select  |  ←→: change date  |  Enter: resume  |  Esc: back"))
	b.WriteString("\n")

	return b.String() + "\n" + m.renderStatusBar()
}
```

**Step 2: Add `modeResume` routing to `update.go`**

In `Update()`, after the existing `modeConfig` check:
```go
	case tea.KeyMsg:
		if m.mode == modeConfig {
			return m.updateConfigMode(msg)
		}
```

Add:
```go
	case tea.KeyMsg:
		if m.mode == modeConfig {
			return m.updateConfigMode(msg)
		}
		if m.mode == modeResume {
			return m.updateResumeMode(msg)
		}
```

In `handleCommand`, add the `/resume` case before `default`:

```go
	case "/resume":
		convs, err := m.store.ListWithDate()
		if err != nil {
			m.statusMsg = "Failed to load conversations: " + err.Error()
			return m, nil
		}
		m.resumePick = buildResumePicker(convs)
		m.mode = modeResume
		return m, nil
```

**Step 3: Add `modeResume` branch to `view.go`**

In `View()`, after the existing `modeConfig` check:
```go
func (m Model) View() string {
	if m.mode == modeConfig {
		return m.viewConfigEditor()
	}
```

Add:
```go
func (m Model) View() string {
	if m.mode == modeConfig {
		return m.viewConfigEditor()
	}
	if m.mode == modeResume {
		return m.viewResumePicker()
	}
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
git add internal/ui/resumepicker.go internal/ui/update.go internal/ui/view.go
git commit -m "feat: add /resume command with date-grouped conversation picker"
```

---

## Verification checklist

After all tasks complete:

1. `go build ./...` succeeds
2. `go test ./...` all pass
3. Run app: `/settings` opens the config editor (same as old `/model`)
4. Run app: `/resume` shows the picker with date headers
5. Left/right keys switch between dates; up/down move the cursor
6. Enter loads the selected conversation and returns to chat
7. Esc returns to chat without changes
