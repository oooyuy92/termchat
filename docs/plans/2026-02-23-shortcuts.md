# Shortcuts Feature Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add a `/shortcuts` command that opens a keyboard-navigable editor for user-defined prompt shortcuts; selecting a shortcut fills the chat input box.

**Architecture:** Shortcuts are stored as YAML at `~/.config/termchat/shortcuts.yaml` (same dir as config). A new `internal/shortcuts` package handles Load/Save. A new `modeShortcuts` UI mode follows the exact same pattern as `modeConfig` and `modeResume` — state struct in `model.go`, logic + view in `shortcuteditor.go`, wired up in `update.go` and `view.go`.

**Tech Stack:** Go, BubbleTea (mode-based UI), lipgloss (via existing Theme), `gopkg.in/yaml.v3` (already in go.mod).

---

## UI layout

### List mode

```
Shortcuts

> translate     请翻译以下内容：
  code-review   请审查以下代码并指出潜在问题
  explain       请解释以下代码

  ↑↓: navigate  |  Enter: use  |  e: edit  |  n: new  |  d: delete  |  Esc: back
```

- `>` marks the selected item
- Name is left-aligned, content shown inline (truncated to 40 runes) as dim help text
- If no shortcuts: `  (empty — press 'n' to add one)`

### Edit mode — editing name

```
Shortcuts — edit name

  Name:    translate█
  Content: 请翻译以下内容：

  Enter: next field  |  Esc: cancel
```

### Edit mode — editing content

```
Shortcuts — edit content

  Name:    translate
  Content: 请翻译以下内容：█

  Enter: save  |  Esc: cancel
```

---

## Key bindings

| Mode | Key | Action |
|------|-----|--------|
| list | `up`/`k`, `down`/`j` | navigate |
| list | `enter` | fill `m.input` with content, return to chat |
| list | `e` | enter edit-name mode for selected item |
| list | `n` | append empty item, cursor → it, enter edit-name mode |
| list | `d` | delete selected item, save, clamp cursor |
| list | `esc` | return to chat |
| list | `ctrl+c` | double-press quit |
| edit-name | runes / backspace | edit name buffer |
| edit-name | `enter` | save name → move to edit-content |
| edit-name | `esc` | cancel; if new and empty, remove item; return to list |
| edit-content | runes / backspace | edit content buffer |
| edit-content | `enter` | save content → save file → return to list |
| edit-content | `esc` | cancel (revert both fields) → return to list |

---

## Key files

| File | Change |
|------|--------|
| `internal/shortcuts/shortcuts.go` | New package: `Shortcut`, `Load`, `Save` |
| `internal/shortcuts/shortcuts_test.go` | Tests for Load/Save |
| `internal/ui/model.go` | Add `modeShortcuts`, `shortcutEditor` type, `shortcutsPath`/`shortcutEd` fields, `shortcutsSavedMsg` |
| `internal/ui/shortcuteditor.go` | New file: `updateShortcutsMode`, `viewShortcutsEditor`, `saveShortcutsCmd` |
| `internal/ui/update.go` | Route `modeShortcuts`; add `/shortcuts` case; handle `shortcutsSavedMsg` |
| `internal/ui/view.go` | Add `modeShortcuts` branch |

---

### Task 1: shortcuts storage package

**Files:**
- Create: `internal/shortcuts/shortcuts.go`
- Create: `internal/shortcuts/shortcuts_test.go`

**Step 1: Write failing tests first**

Create `internal/shortcuts/shortcuts_test.go`:

```go
package shortcuts

import (
	"path/filepath"
	"testing"
)

func TestLoadNonExistent(t *testing.T) {
	dir := t.TempDir()
	items, err := Load(filepath.Join(dir, "shortcuts.yaml"))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(items) != 0 {
		t.Errorf("Load() = %v, want empty", items)
	}
}

func TestSaveAndLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "shortcuts.yaml")
	input := []Shortcut{
		{Name: "translate", Content: "请翻译以下内容："},
		{Name: "review", Content: "请审查以下代码："},
	}
	if err := Save(path, input); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(loaded) != 2 {
		t.Fatalf("Load() len = %d, want 2", len(loaded))
	}
	if loaded[0].Name != "translate" || loaded[0].Content != "请翻译以下内容：" {
		t.Errorf("loaded[0] = %+v", loaded[0])
	}
	if loaded[1].Name != "review" {
		t.Errorf("loaded[1].Name = %q, want %q", loaded[1].Name, "review")
	}
}

func TestSaveEmpty(t *testing.T) {
	path := filepath.Join(t.TempDir(), "shortcuts.yaml")
	if err := Save(path, []Shortcut{}); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(loaded) != 0 {
		t.Errorf("Load() = %v, want empty", loaded)
	}
}
```

**Step 2: Run tests to verify they fail**

```bash
GOPATH=/home/cc/gopath GOMODCACHE=/home/cc/gopath/pkg/mod /home/cc/go/bin/go test ./internal/shortcuts/... -v
```

Expected: FAIL (package does not exist yet).

**Step 3: Create `internal/shortcuts/shortcuts.go`**

```go
// internal/shortcuts/shortcuts.go
package shortcuts

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Shortcut is a named prompt template.
type Shortcut struct {
	Name    string `yaml:"name"`
	Content string `yaml:"content"`
}

type file struct {
	Shortcuts []Shortcut `yaml:"shortcuts"`
}

// Load reads shortcuts from path. Returns an empty slice (not an error)
// if the file does not exist yet.
func Load(path string) ([]Shortcut, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []Shortcut{}, nil
		}
		return nil, err
	}
	var f file
	if err := yaml.Unmarshal(data, &f); err != nil {
		return nil, err
	}
	if f.Shortcuts == nil {
		return []Shortcut{}, nil
	}
	return f.Shortcuts, nil
}

// Save writes shortcuts to path, creating parent directories as needed.
func Save(path string, items []Shortcut) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := yaml.Marshal(file{Shortcuts: items})
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
```

**Step 4: Run tests — all must pass**

```bash
GOPATH=/home/cc/gopath GOMODCACHE=/home/cc/gopath/pkg/mod /home/cc/go/bin/go test ./internal/shortcuts/... -v
```

Expected: all 3 tests PASS.

**Step 5: Build**

```bash
GOPATH=/home/cc/gopath GOMODCACHE=/home/cc/gopath/pkg/mod /home/cc/go/bin/go build ./...
```

Expected: no errors.

**Step 6: Commit**

```bash
git add internal/shortcuts/
git commit -m "feat: add shortcuts storage package"
```

---

### Task 2: Add shortcut editor state to model.go

**Files:**
- Modify: `internal/ui/model.go`

**Step 1: Read the file**

```bash
cat /home/cc/Github/llm-chat-in-terminal/internal/ui/model.go
```

**Step 2: Add `modeShortcuts` constant**

Find:
```go
const (
	modeChat uiMode = iota
	modeConfig
	modeResume
)
```

Change to:
```go
const (
	modeChat uiMode = iota
	modeConfig
	modeResume
	modeShortcuts
)
```

**Step 3: Add `shortcutsSavedMsg` type**

After the `autoSavedMsg` declaration:
```go
// autoSavedMsg carries the result of a background auto-save (Err may be nil).
type autoSavedMsg struct{ Err error }
```

Add:
```go
// shortcutsSavedMsg carries the result of saving shortcuts to disk.
type shortcutsSavedMsg struct{ Err error }
```

**Step 4: Add `shortcutEditor` type**

The `shortcutEditor` tracks which item is selected, which sub-mode is active, and the in-progress edit buffer.

Add after the `resumePicker` struct:

```go
// shortcutSubMode describes what the shortcut editor is currently doing.
type shortcutSubMode int

const (
	shortcutModeList        shortcutSubMode = iota
	shortcutModeEditName                    // editing the name field
	shortcutModeEditContent                 // editing the content field
)

type shortcutEditor struct {
	items      []shortcuts.Shortcut
	cursor     int             // index of selected item
	subMode    shortcutSubMode
	editBuf    string          // current text being typed
	savedName  string          // name before edit started (for cancel)
	savedContent string        // content before edit started (for cancel)
	isNew      bool            // true when 'n' added a new item
}
```

**Step 5: Add import for shortcuts package**

In the import block of model.go, add:
```go
"github.com/termchat/termchat/internal/shortcuts"
```

**Step 6: Add fields to `Model` struct**

After `resumePick resumePicker`:
```go
	shortcutsPath string
	shortcutEd    shortcutEditor
```

**Step 7: Set `shortcutsPath` in `NewModel`**

Add `"path/filepath"` to the import block.

In `NewModel`, before `return Model{...}`:
```go
	shortcutsPath := filepath.Join(filepath.Dir(cfgPath), "shortcuts.yaml")
```

In the returned `Model{}` literal, add:
```go
		shortcutsPath: shortcutsPath,
```

**Step 8: Build to verify compilation**

```bash
GOPATH=/home/cc/gopath GOMODCACHE=/home/cc/gopath/pkg/mod /home/cc/go/bin/go build ./...
```

Expected: no errors.

**Step 9: Commit**

```bash
git add internal/ui/model.go
git commit -m "feat: add modeShortcuts, shortcutEditor types to model"
```

---

### Task 3: Implement shortcut editor — new file

**Files:**
- Create: `internal/ui/shortcuteditor.go`

**Step 1: Create `internal/ui/shortcuteditor.go`**

```go
// internal/ui/shortcuteditor.go
package ui

import (
	"strings"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/termchat/termchat/internal/shortcuts"
)

func (m Model) saveShortcutsCmd() tea.Cmd {
	items := make([]shortcuts.Shortcut, len(m.shortcutEd.items))
	copy(items, m.shortcutEd.items)
	path := m.shortcutsPath
	return func() tea.Msg {
		err := shortcuts.Save(path, items)
		return shortcutsSavedMsg{Err: err}
	}
}

func (m Model) updateShortcutsMode(msg tea.KeyMsg) (Model, tea.Cmd) {
	ed := &m.shortcutEd

	if msg.String() != "ctrl+c" {
		m.confirmQuit = false
	}

	switch ed.subMode {

	case shortcutModeList:
		switch msg.String() {
		case "up", "k":
			if ed.cursor > 0 {
				ed.cursor--
			}
		case "down", "j":
			if ed.cursor < len(ed.items)-1 {
				ed.cursor++
			}
		case "enter":
			if len(ed.items) == 0 {
				m.mode = modeChat
				m.statusMsg = "No shortcuts defined"
				return m, nil
			}
			m.input = ed.items[ed.cursor].Content
			m.mode = modeChat
			m.statusMsg = "Shortcut loaded"
			return m, nil
		case "e":
			if len(ed.items) == 0 {
				return m, nil
			}
			ed.savedName = ed.items[ed.cursor].Name
			ed.savedContent = ed.items[ed.cursor].Content
			ed.editBuf = ed.items[ed.cursor].Name
			ed.isNew = false
			ed.subMode = shortcutModeEditName
		case "n":
			ed.items = append(ed.items, shortcuts.Shortcut{})
			ed.cursor = len(ed.items) - 1
			ed.savedName = ""
			ed.savedContent = ""
			ed.editBuf = ""
			ed.isNew = true
			ed.subMode = shortcutModeEditName
		case "d":
			if len(ed.items) == 0 {
				return m, nil
			}
			ed.items = append(ed.items[:ed.cursor], ed.items[ed.cursor+1:]...)
			if ed.cursor >= len(ed.items) && ed.cursor > 0 {
				ed.cursor--
			}
			return m, m.saveShortcutsCmd()
		case "esc":
			m.mode = modeChat
			m.statusMsg = "Back to chat"
		case "ctrl+c":
			if m.confirmQuit {
				return m, tea.Quit
			}
			m.confirmQuit = true
			m.statusMsg = "Press Ctrl+C again to quit"
		}

	case shortcutModeEditName:
		switch msg.String() {
		case "enter":
			ed.items[ed.cursor].Name = ed.editBuf
			ed.editBuf = ed.items[ed.cursor].Content
			ed.subMode = shortcutModeEditContent
		case "esc":
			if ed.isNew {
				// Remove the newly added empty item
				ed.items = ed.items[:len(ed.items)-1]
				if ed.cursor >= len(ed.items) && ed.cursor > 0 {
					ed.cursor--
				}
			} else {
				ed.items[ed.cursor].Name = ed.savedName
			}
			ed.subMode = shortcutModeList
		case "backspace":
			runes := []rune(ed.editBuf)
			if len(runes) > 0 {
				ed.editBuf = string(runes[:len(runes)-1])
			}
		default:
			if msg.Type == tea.KeyRunes {
				ed.editBuf += string(msg.Runes)
			}
		}

	case shortcutModeEditContent:
		switch msg.String() {
		case "enter":
			ed.items[ed.cursor].Content = ed.editBuf
			ed.subMode = shortcutModeList
			return m, m.saveShortcutsCmd()
		case "esc":
			ed.items[ed.cursor].Name = ed.savedName
			ed.items[ed.cursor].Content = ed.savedContent
			ed.subMode = shortcutModeList
		case "backspace":
			runes := []rune(ed.editBuf)
			if len(runes) > 0 {
				ed.editBuf = string(runes[:len(runes)-1])
			}
		default:
			if msg.Type == tea.KeyRunes {
				ed.editBuf += string(msg.Runes)
			}
		}
	}

	return m, nil
}

// truncateShortcut shortens s to at most n runes, appending "…" if truncated.
// (The ui package already has truncate() in resumepicker.go; use a different name
// to avoid a duplicate identifier in the same package.)
func truncateShortcut(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	return string([]rune(s)[:n]) + "…"
}

func (m Model) viewShortcutsEditor() string {
	var b strings.Builder
	ed := m.shortcutEd

	switch ed.subMode {
	case shortcutModeList:
		b.WriteString(m.theme.ConfigTitleStyle().Render("Shortcuts"))
		b.WriteString("\n\n")

		if len(ed.items) == 0 {
			b.WriteString(m.theme.ConfigHelpStyle().Render("  (empty — press 'n' to add one)"))
			b.WriteString("\n\n")
			b.WriteString(m.theme.ConfigHelpStyle().Render("  n: new  |  Esc: back"))
			b.WriteString("\n")
			return b.String() + "\n" + m.renderStatusBar()
		}

		for i, sc := range ed.items {
			cursor := "  "
			if i == ed.cursor {
				cursor = m.theme.ConfigCursorStyle().Render("> ")
			}
			name := m.theme.ConfigValueStyle().Render(sc.Name)
			preview := m.theme.ConfigHelpStyle().Render("  " + truncateShortcut(sc.Content, 40))
			b.WriteString(cursor + name + preview + "\n")
		}

		b.WriteString("\n")
		b.WriteString(m.theme.ConfigHelpStyle().Render("  ↑↓: navigate  |  Enter: use  |  e: edit  |  n: new  |  d: delete  |  Esc: back"))
		b.WriteString("\n")

	case shortcutModeEditName:
		b.WriteString(m.theme.ConfigTitleStyle().Render("Shortcuts — edit name"))
		b.WriteString("\n\n")
		sc := ed.items[ed.cursor]
		b.WriteString(m.theme.ConfigLabelStyle().Render("  Name:    ") + m.theme.ConfigEditStyle().Render(ed.editBuf+"\u2588") + "\n")
		b.WriteString(m.theme.ConfigLabelStyle().Render("  Content: ") + m.theme.ConfigValueStyle().Render(sc.Content) + "\n")
		b.WriteString("\n")
		b.WriteString(m.theme.ConfigHelpStyle().Render("  Enter: next field  |  Esc: cancel"))
		b.WriteString("\n")

	case shortcutModeEditContent:
		b.WriteString(m.theme.ConfigTitleStyle().Render("Shortcuts — edit content"))
		b.WriteString("\n\n")
		sc := ed.items[ed.cursor]
		b.WriteString(m.theme.ConfigLabelStyle().Render("  Name:    ") + m.theme.ConfigValueStyle().Render(sc.Name) + "\n")
		b.WriteString(m.theme.ConfigLabelStyle().Render("  Content: ") + m.theme.ConfigEditStyle().Render(ed.editBuf+"\u2588") + "\n")
		b.WriteString("\n")
		b.WriteString(m.theme.ConfigHelpStyle().Render("  Enter: save  |  Esc: cancel"))
		b.WriteString("\n")
	}

	return b.String() + "\n" + m.renderStatusBar()
}
```

**Note:** `truncate()` already exists in `resumepicker.go` in the same `ui` package. Use `truncateShortcut()` here to avoid a duplicate function name.

**Step 2: Build**

```bash
GOPATH=/home/cc/gopath GOMODCACHE=/home/cc/gopath/pkg/mod /home/cc/go/bin/go build ./...
```

Expected: no errors.

**Step 3: Commit**

```bash
git add internal/ui/shortcuteditor.go
git commit -m "feat: implement shortcut editor — update and view logic"
```

---

### Task 4: Wire up update.go and view.go

**Files:**
- Modify: `internal/ui/update.go`
- Modify: `internal/ui/view.go`

**Step 1: Read both files**

```bash
cat /home/cc/Github/llm-chat-in-terminal/internal/ui/update.go
cat /home/cc/Github/llm-chat-in-terminal/internal/ui/view.go
```

**Step 2: Add `modeShortcuts` routing in `Update()` in update.go**

Find:
```go
	case tea.KeyMsg:
		if m.mode == modeConfig {
			return m.updateConfigMode(msg)
		}
		if m.mode == modeResume {
			return m.updateResumeMode(msg)
		}
```

Change to:
```go
	case tea.KeyMsg:
		if m.mode == modeConfig {
			return m.updateConfigMode(msg)
		}
		if m.mode == modeResume {
			return m.updateResumeMode(msg)
		}
		if m.mode == modeShortcuts {
			return m.updateShortcutsMode(msg)
		}
```

**Step 3: Add `/shortcuts` command in `handleCommand()` in update.go**

Find (before `default:`):
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

	default:
```

Add before `default:`:
```go
	case "/shortcuts":
		items, err := shortcuts.Load(m.shortcutsPath)
		if err != nil {
			m.statusMsg = "Failed to load shortcuts: " + err.Error()
			return m, nil
		}
		m.shortcutEd = shortcutEditor{items: items}
		m.mode = modeShortcuts
		return m, nil
```

Add the import for the shortcuts package to update.go:
```go
import (
	"context"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/termchat/termchat/internal/chat"
	"github.com/termchat/termchat/internal/shortcuts"
)
```

**Step 4: Handle `shortcutsSavedMsg` in `Update()` in update.go**

Find:
```go
	case autoSavedMsg:
		if msg.Err != nil {
			m.statusMsg = "Auto-save failed: " + msg.Err.Error()
		}
		return m, nil
	}
```

Add before the closing `}`:
```go
	case shortcutsSavedMsg:
		if msg.Err != nil {
			m.statusMsg = "Shortcuts save failed: " + msg.Err.Error()
		}
		return m, nil
```

**Step 5: Add `modeShortcuts` branch in `View()` in view.go**

Find:
```go
func (m Model) View() string {
	if m.mode == modeConfig {
		return m.viewConfigEditor()
	}
	if m.mode == modeResume {
		return m.viewResumePicker()
	}
```

Change to:
```go
func (m Model) View() string {
	if m.mode == modeConfig {
		return m.viewConfigEditor()
	}
	if m.mode == modeResume {
		return m.viewResumePicker()
	}
	if m.mode == modeShortcuts {
		return m.viewShortcutsEditor()
	}
```

**Step 6: Build**

```bash
GOPATH=/home/cc/gopath GOMODCACHE=/home/cc/gopath/pkg/mod /home/cc/go/bin/go build ./...
```

Expected: no errors.

**Step 7: Run all tests**

```bash
GOPATH=/home/cc/gopath GOMODCACHE=/home/cc/gopath/pkg/mod /home/cc/go/bin/go test ./...
```

Expected: all tests pass.

**Step 8: Commit**

```bash
git add internal/ui/update.go internal/ui/view.go
git commit -m "feat: wire up /shortcuts command and modeShortcuts"
```

---

## Verification checklist

1. `go build ./...` succeeds
2. `go test ./...` all pass
3. Run app: `/shortcuts` opens the list (empty on first run)
4. Press `n`, type a name, press Enter, type prompt content, press Enter → item appears in list, saved to `~/.config/termchat/shortcuts.yaml`
5. Navigate to item, press `e` → edit name then content; changes persist after restart
6. Press `d` → item deleted
7. Navigate to item, press Enter → input box filled with content, returned to chat
8. Press Esc → returns to chat without changes
