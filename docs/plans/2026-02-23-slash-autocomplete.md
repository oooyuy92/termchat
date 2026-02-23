# Slash Command Autocomplete Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Show a dropdown of matching slash commands above the input line whenever the user's input starts with `/`.

**Architecture:** Add `modeSlashComplete` as a new `uiMode`; add a `slashAC slashComplete` field to `Model`; all autocomplete logic lives in a new `internal/ui/slashcomplete.go` file; `update.go` and `view.go` get minimal routing additions.

**Tech Stack:** Go, BubbleTea (existing), no new dependencies.

---

## Background: existing code you must understand

### `internal/ui/model.go`

- `uiMode` iota: `modeChat`, `modeConfig`, `modeResume`, `modeShortcuts`, `modeRolePicker`, `modeRoles`, `modeOnboard`.
- `Model` struct has `input string`, `mode uiMode`, `theme Theme`, etc.

### `internal/ui/update.go`

- `Update()` routes modes at the top of `case tea.KeyMsg:`.
- Chat input is handled inline (no separate `updateChatMode` function). The relevant section:
  ```go
  case "backspace":
      runes := []rune(m.input)
      if len(runes) > 0 {
          m.input = string(runes[:len(runes)-1])
      }

  default:
      if msg.Type == tea.KeyRunes {
          m.input += string(msg.Runes)
      }
  ```
- `handleCommand(input)` executes slash commands.

### `internal/ui/view.go`

- `View()` routes modes at the top, then falls through to chat rendering.
- Chat rendering: history → streaming → error → `"> " + m.input` → status bar.

### `internal/ui/onboard.go`

Good reference for the pattern: a new `updateXMode` + `viewX` pair in its own file.

---

## Task 1: Types, command table, and model fields

**Files:**
- Create: `internal/ui/slashcomplete.go`
- Modify: `internal/ui/model.go`

### Step 1: Add the new types and command table to `slashcomplete.go`

Create `internal/ui/slashcomplete.go`:

```go
// internal/ui/slashcomplete.go
package ui

import "strings"

type slashCmd struct {
	Name string // e.g. "/resume"
	Desc string // e.g. "Browse conversations"
}

type slashComplete struct {
	matches []slashCmd
	cursor  int
	offset  int // first visible item index (viewport)
}

// slashCmds is the canonical list of all slash commands with descriptions.
var slashCmds = []slashCmd{
	{"/clear", "Clear conversation"},
	{"/exit", "Quit termchat"},
	{"/list", "List saved conversations"},
	{"/load", "Load a conversation"},
	{"/resume", "Browse conversations by date"},
	{"/roles", "Edit role presets"},
	{"/save", "Save conversation"},
	{"/settings", "Model & parameters"},
	{"/shortcuts", "Edit keyboard shortcuts"},
}

// filterSlashCmds returns commands whose Name has input as a prefix.
// input must start with "/".
func filterSlashCmds(input string) []slashCmd {
	var out []slashCmd
	for _, c := range slashCmds {
		if strings.HasPrefix(c.Name, input) {
			out = append(out, c)
		}
	}
	return out
}
```

### Step 2: Write tests for `filterSlashCmds`

There are no existing tests in `internal/ui/` — that's OK. The `internal/config/` package has tests as a reference (uses standard `testing` package). Since `filterSlashCmds` is a pure function, it's the only testable piece in this feature.

Create `internal/ui/slashcomplete_test.go`:

```go
package ui

import (
	"testing"
)

func TestFilterSlashCmds_slash(t *testing.T) {
	// "/" alone should return all commands
	got := filterSlashCmds("/")
	if len(got) != len(slashCmds) {
		t.Errorf("filterSlashCmds(\"/\") = %d results, want %d", len(got), len(slashCmds))
	}
}

func TestFilterSlashCmds_prefix(t *testing.T) {
	got := filterSlashCmds("/r")
	// Should match /resume and /roles
	if len(got) != 2 {
		t.Errorf("filterSlashCmds(\"/r\") = %d results, want 2", len(got))
	}
	for _, c := range got {
		if c.Name != "/resume" && c.Name != "/roles" {
			t.Errorf("unexpected match: %s", c.Name)
		}
	}
}

func TestFilterSlashCmds_exact(t *testing.T) {
	got := filterSlashCmds("/exit")
	if len(got) != 1 || got[0].Name != "/exit" {
		t.Errorf("filterSlashCmds(\"/exit\") = %v, want [{/exit ...}]", got)
	}
}

func TestFilterSlashCmds_noMatch(t *testing.T) {
	got := filterSlashCmds("/zzz")
	if len(got) != 0 {
		t.Errorf("filterSlashCmds(\"/zzz\") = %d results, want 0", len(got))
	}
}
```

### Step 3: Run tests to verify they fail

```bash
cd /home/cc/Github/llm-chat-in-terminal && GOPATH=/home/cc/gopath /home/cc/go/bin/go test ./internal/ui/...
```

Expected: FAIL — `slashcomplete_test.go` won't compile because `slashcomplete.go` does not exist yet.

Actually wait — you already created `slashcomplete.go` in Step 1. So the tests should compile. Run them to confirm they PASS:

```bash
cd /home/cc/Github/llm-chat-in-terminal && GOPATH=/home/cc/gopath /home/cc/go/bin/go test ./internal/ui/...
```

Expected: PASS (the logic is simple enough to get right first try).

### Step 4: Add `modeSlashComplete` and `slashAC` to `model.go`

Read `internal/ui/model.go` first.

**4a. Add `modeSlashComplete` to the iota** — append after `modeOnboard`:

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
)
```

**4b. Add `slashAC slashComplete` to `Model` struct** — add after `input string`:

```go
input   string
slashAC slashComplete
```

### Step 5: Build to verify

```bash
cd /home/cc/Github/llm-chat-in-terminal && GOPATH=/home/cc/gopath /home/cc/go/bin/go build ./...
```

Expected: PASS

### Step 6: Commit

```bash
cd /home/cc/Github/llm-chat-in-terminal && git add internal/ui/slashcomplete.go internal/ui/slashcomplete_test.go internal/ui/model.go && git commit -m "feat: add slash autocomplete types, command table, model fields"
```

---

## Task 2: `updateSlashComplete` and `viewSlashComplete`

**Files:**
- Modify: `internal/ui/slashcomplete.go`

Add both functions to the bottom of `internal/ui/slashcomplete.go`.

### Step 1: Add `updateSlashComplete`

```go
const slashACMaxVisible = 5

// clampSlashAC adjusts offset so cursor is within the visible window.
func (ac *slashComplete) clampSlashAC() {
	n := len(ac.matches)
	if n == 0 {
		ac.cursor = 0
		ac.offset = 0
		return
	}
	if ac.cursor < 0 {
		ac.cursor = n - 1
	}
	if ac.cursor >= n {
		ac.cursor = 0
	}
	if ac.cursor < ac.offset {
		ac.offset = ac.cursor
	}
	if ac.cursor >= ac.offset+slashACMaxVisible {
		ac.offset = ac.cursor - slashACMaxVisible + 1
	}
	if ac.offset < 0 {
		ac.offset = 0
	}
}

func (m Model) updateSlashComplete(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.input = ""
		m.mode = modeChat
		return m, nil

	case "ctrl+c":
		if m.confirmQuit {
			return m, tea.Quit
		}
		m.confirmQuit = true
		m.statusMsg = "Press Ctrl+C again to quit"
		return m, nil

	case "up":
		m.slashAC.cursor--
		m.slashAC.clampSlashAC()
		return m, nil

	case "down":
		m.slashAC.cursor++
		m.slashAC.clampSlashAC()
		return m, nil

	case "tab":
		// Fill selected command name into input, return to chat to edit args
		if len(m.slashAC.matches) > 0 {
			m.input = m.slashAC.matches[m.slashAC.cursor].Name
		}
		m.mode = modeChat
		return m, nil

	case "enter":
		// Execute selected command immediately
		if len(m.slashAC.matches) > 0 {
			cmd := m.slashAC.matches[m.slashAC.cursor].Name
			m.input = ""
			m.mode = modeChat
			return m.handleCommand(cmd)
		}
		m.mode = modeChat
		return m, nil

	case "backspace":
		runes := []rune(m.input)
		if len(runes) > 0 {
			m.input = string(runes[:len(runes)-1])
		}
		if m.input == "" {
			m.mode = modeChat
			return m, nil
		}
		m.slashAC.matches = filterSlashCmds(m.input)
		if len(m.slashAC.matches) == 0 {
			m.mode = modeChat
			return m, nil
		}
		if m.slashAC.cursor >= len(m.slashAC.matches) {
			m.slashAC.cursor = 0
		}
		m.slashAC.clampSlashAC()
		return m, nil

	default:
		if msg.Type != tea.KeyRunes {
			return m, nil
		}
		m.input += string(msg.Runes)
		m.slashAC.matches = filterSlashCmds(m.input)
		if len(m.slashAC.matches) == 0 {
			// No matches — fall back to chat mode (user can keep typing freely)
			m.mode = modeChat
			return m, nil
		}
		if m.slashAC.cursor >= len(m.slashAC.matches) {
			m.slashAC.cursor = 0
		}
		m.slashAC.clampSlashAC()
		return m, nil
	}
}
```

### Step 2: Add `viewSlashComplete`

```go
func (m Model) viewSlashComplete() string {
	var b strings.Builder

	// Render conversation history (same as chat mode)
	for _, msg := range m.history.Messages() {
		switch msg.Role {
		case "user":
			b.WriteString(m.theme.UserLabelStyle().Render("You:") + "\n")
			b.WriteString(msg.Content + "\n\n")
		case "assistant":
			b.WriteString(m.theme.AssistantLabelStyle().Render(m.client.Model()+":") + "\n")
			rendered, err := m.renderer.Render(msg.Content)
			if err != nil {
				b.WriteString(msg.Content + "\n\n")
			} else {
				b.WriteString(cleanGlamourOutput(rendered) + "\n\n")
			}
		}
	}

	// Dropdown: show up to slashACMaxVisible candidates
	ac := m.slashAC
	end := ac.offset + slashACMaxVisible
	if end > len(ac.matches) {
		end = len(ac.matches)
	}
	for i := ac.offset; i < end; i++ {
		c := ac.matches[i]
		line := fmt.Sprintf("  %-12s %s", c.Name, c.Desc)
		if i == ac.cursor {
			prefix := m.theme.ConfigCursorStyle().Render("> ")
			line = prefix + m.theme.ConfigLabelStyle().Render(fmt.Sprintf("%-12s", c.Name)) + " " + m.theme.ConfigHelpStyle().Render(c.Desc)
		} else {
			line = m.theme.ConfigHelpStyle().Render(line)
		}
		b.WriteString(line + "\n")
	}

	// Input line
	b.WriteString(m.theme.InputPromptStyle().Render("> ") + m.input)

	return b.String() + "\n" + m.renderStatusBar()
}
```

Note: `viewSlashComplete` imports `"fmt"` — add it to the import block of `slashcomplete.go`:

```go
import (
    "fmt"
    "strings"

    tea "github.com/charmbracelet/bubbletea"
)
```

Also add the `tea` import since `updateSlashComplete` uses `tea.KeyMsg`, `tea.Cmd`, `tea.Quit`, `tea.KeyRunes`.

### Step 3: Build

```bash
cd /home/cc/Github/llm-chat-in-terminal && GOPATH=/home/cc/gopath /home/cc/go/bin/go build ./...
```

Expected: PASS

### Step 4: Run tests

```bash
cd /home/cc/Github/llm-chat-in-terminal && GOPATH=/home/cc/gopath /home/cc/go/bin/go test ./...
```

Expected: all PASS

### Step 5: Commit

```bash
cd /home/cc/Github/llm-chat-in-terminal && git add internal/ui/slashcomplete.go && git commit -m "feat: add updateSlashComplete and viewSlashComplete"
```

---

## Task 3: Wire into `update.go` and `view.go`

**Files:**
- Modify: `internal/ui/update.go`
- Modify: `internal/ui/view.go`

Read both files before making changes.

### Step 1: Route `modeSlashComplete` in `update.go`

**3a.** Add routing at the top of `case tea.KeyMsg:`, after the `modeOnboard` check:

```go
if m.mode == modeOnboard {
    return m.updateOnboardMode(msg)
}
if m.mode == modeSlashComplete {
    return m.updateSlashComplete(msg)
}
```

**3b.** In the `default` case of the chat key handler, after appending to `m.input`, detect the `/` trigger:

Current code:
```go
default:
    // Handle rune input for multi-byte characters
    if msg.Type == tea.KeyRunes {
        m.input += string(msg.Runes)
    }
```

Replace with:
```go
default:
    if msg.Type == tea.KeyRunes {
        m.input += string(msg.Runes)
        // If the user just typed "/" as the very first character, enter autocomplete.
        if m.input == "/" {
            m.mode = modeSlashComplete
            m.slashAC = slashComplete{
                matches: filterSlashCmds("/"),
                cursor:  0,
                offset:  0,
            }
        }
    }
```

### Step 2: Route `modeSlashComplete` in `view.go`

After the `modeOnboard` check:

```go
if m.mode == modeOnboard {
    return m.viewOnboard()
}
if m.mode == modeSlashComplete {
    return m.viewSlashComplete()
}
```

### Step 3: Build and run all tests

```bash
cd /home/cc/Github/llm-chat-in-terminal && GOPATH=/home/cc/gopath /home/cc/go/bin/go build ./... && GOPATH=/home/cc/gopath /home/cc/go/bin/go test ./...
```

Expected: all PASS

### Step 4: Commit

```bash
cd /home/cc/Github/llm-chat-in-terminal && git add internal/ui/update.go internal/ui/view.go && git commit -m "feat: wire slash autocomplete into update and view"
```

---

## Manual Testing Checklist

1. Type `/` → dropdown appears with all 9 commands
2. Type `/r` → only `/resume` and `/roles` shown
3. Type `/zzz` → no matches → returns to normal chat input, cursor stays at `/zzz`
4. Press ↑↓ → cursor moves through list, wraps around
5. Press Tab on `/resume` → input becomes `/resume`, mode returns to chat
6. Press Enter on `/roles` → opens roles editor, input cleared
7. Press Backspace to delete back to `/` → all commands shown again
8. Press Backspace on `/` → back to empty input, normal chat mode
9. Press Esc → input cleared, normal chat mode
10. Normal message (no `/`) → no autocomplete triggered

---

## Run all tests

```bash
cd /home/cc/Github/llm-chat-in-terminal && GOPATH=/home/cc/gopath /home/cc/go/bin/go test ./...
```

Expected: all packages pass.
