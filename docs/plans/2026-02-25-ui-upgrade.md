# UI Upgrade Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Upgrade termchat UI with bubbles/textarea (multi-line input), bubbles/spinner (above input), lipgloss dynamic layout, and fix mouse scroll on macOS Terminal.app.

**Target layout:**
```
┌─────────────────────────────────────────┐
│  chat viewport  (dynamic height)        │
│  scrollable message history             │
├─────────────────────────────────────────┤
│  ⠋ 生成中...   (streaming 时显示)       │  ← spinner line
├─────────────────────────────────────────┤
│  > _                                    │  ← textarea (1-N lines)
│    Shift+Enter 换行，Enter 发送          │
├─────────────────────────────────────────┤
│  model  tokens  role  status            │  ← status bar
└─────────────────────────────────────────┘
```

**viewport height = total - lipgloss.Height(spinnerLine) - lipgloss.Height(inputArea) - lipgloss.Height(statusBar)**

**Branch:** `feature/viewport-migration`
**Working directory:** `/home/cc/Github/llm-chat-in-terminal/.worktrees/viewport-migration`

---

### Task 1: Fix mouse scroll (remove setAlternateScroll conflict)

**Files:** `main.go`

Always use `tea.WithMouseCellMotion()`. Remove `setAlternateScroll` entirely — macOS Terminal.app intercepts right-click at OS level so right-click still works.

Find:
```go
var opts []tea.ProgramOption
if useAltScreen {
    opts = append(opts, tea.WithAltScreen())
    setAlternateScroll(os.Stdout, true)
    defer setAlternateScroll(os.Stdout, false)
} else {
    opts = append(opts, tea.WithMouseCellMotion())
}
```

Replace with:
```go
var opts []tea.ProgramOption
if useAltScreen {
    opts = append(opts, tea.WithAltScreen())
}
opts = append(opts, tea.WithMouseCellMotion())
```

Also delete the `setAlternateScroll` function entirely (lines ~51-60).

Build and commit:
```bash
go build ./...
git add main.go
git commit -m "fix(ui): always use MouseCellMotion, remove alternate scroll conflict"
```

---

### Task 2: Add bubbles/spinner to Model

**Files:** `internal/ui/model.go`, `internal/ui/update.go`

**Step 1: Add import and field**

In `model.go`, add import:
```go
"github.com/charmbracelet/bubbles/spinner"
```

Add to `Model` struct:
```go
spinner  spinner.Model
```

In `NewModel` return literal, add:
```go
spinner: func() spinner.Model {
    s := spinner.New()
    s.Spinner = spinner.Dot
    return s
}(),
```

**Step 2: Wire spinner tick in update.go**

Add import:
```go
"github.com/charmbracelet/bubbles/spinner"
```

Add `spinner.TickMsg` case in `Update()`:
```go
case spinner.TickMsg:
    if m.streaming {
        var cmd tea.Cmd
        m.spinner, cmd = m.spinner.Update(msg)
        return m, cmd
    }
    return m, nil
```

In `streamStartMsg` case, after setting `m.streamCh`:
```go
return m, tea.Batch(m.readNextChunk(), m.spinner.Tick)
```

Wait — `streamStartMsg` currently returns `m.readNextChunk()`. Change to:
```go
case streamStartMsg:
    m.streamCh = msg.chunks
    m.streamErr = msg.errs
    return m, tea.Batch(m.readNextChunk(), m.spinner.Tick)
```

**Step 3: Build check**
```bash
go build ./...
```

---

### Task 3: Add bubbles/textarea, replace m.input

**Files:** `internal/ui/model.go`, `internal/ui/update.go`, and any file referencing `m.input`

**Step 1: Add textarea to model.go**

Add import:
```go
"github.com/charmbracelet/bubbles/textarea"
```

Replace in `Model` struct:
```go
input string
```
with:
```go
textarea textarea.Model
```

Initialize in `NewModel`:
```go
textarea: func() textarea.Model {
    ta := textarea.New()
    ta.Placeholder = "Message... (Enter to send, Shift+Enter for newline)"
    ta.Focus()
    ta.SetWidth(80)  // will be resized on WindowSizeMsg
    ta.SetHeight(3)
    ta.ShowLineNumbers = false
    ta.KeyMap.InsertNewline.SetKeys("shift+enter")
    return ta
}(),
```

Remove `input: ""` from the return literal (field no longer exists).

**Step 2: Update WindowSizeMsg in update.go**

In the `tea.WindowSizeMsg` case, after setting viewport size, also resize textarea:
```go
m.textarea.SetWidth(msg.Width)
```

**Step 3: Replace key handling for input**

In `Update()`, the `tea.KeyMsg` case currently handles `"enter"`, `"backspace"`, `"ctrl+v"`, and `default` (runes). Replace the entire input-related key handling:

Remove these cases:
- `"backspace"` handler
- `default` rune handler that appends to `m.input`
- The `m.input == "/"` slash autocomplete trigger

Replace the `"enter"` case with:
```go
case "enter":
    input := strings.TrimSpace(m.textarea.Value())
    if input == "" {
        return m, nil
    }
    m.textarea.Reset()

    if strings.HasPrefix(input, "/") {
        return m.handleCommand(input)
    }

    m.history.Add(chat.Message{Role: "user", Content: input, Images: m.pendingImages})
    m.pendingImages = nil
    m.streaming = true
    m.chatFollowBottom = true
    m.currentResp = ""
    m.currentThinking = ""
    m.err = nil

    ctx, cancel := context.WithCancel(context.Background())
    m.streamCtrl = &streamControl{cancel: cancel}
    m.viewport.SetContent(m.buildChatContent())
    m.viewport.GotoBottom()

    return m, tea.Batch(m.sendStreamCmd(ctx), m.spinner.Tick)
```

For all other key events (non-scroll, non-special), delegate to textarea:
```go
default:
    var cmd tea.Cmd
    m.textarea, cmd = m.textarea.Update(msg)
    // Check for slash autocomplete trigger
    if m.textarea.Value() == "/" {
        m.mode = modeSlashComplete
        m.slashAC = slashComplete{
            matches: filterSlashCmds("/"),
            cursor:  0,
            offset:  0,
        }
    }
    return m, cmd
```

**Step 4: Update ctrl+v paste handler**

The `"ctrl+v"` case calls `m.pasteFromClipboard()`. Update `clipboardImageMsg` handler to insert into textarea instead of `m.input`:

In `clipboardImageMsg` case, replace:
```go
m.input += msg.Text
```
with:
```go
m.textarea.InsertString(msg.Text)
```

And replace:
```go
m.input += fmt.Sprintf("[image %d]", m.imageCounter)
```
with:
```go
m.textarea.InsertString(fmt.Sprintf("[image %d]", m.imageCounter))
```

**Step 5: Fix slash autocomplete**

In `updateSlashComplete`, when a shortcut is selected and inserted back into input, replace `m.input = ...` with `m.textarea.SetValue(...)`. Read `internal/ui/slashcomplete.go` first to understand the exact lines.

**Step 6: Fix messagebrowse and resumepicker**

Search for any remaining `m.input` references:
```bash
grep -rn "m\.input" /home/cc/Github/llm-chat-in-terminal/.worktrees/viewport-migration/internal/
```
Fix each one — replace reads of `m.input` with `m.textarea.Value()` and writes with `m.textarea.SetValue(...)` or `m.textarea.InsertString(...)`.

**Step 7: Build check**
```bash
go build ./...
```

---

### Task 4: Update View() with dynamic layout + spinner line

**Files:** `internal/ui/view.go`

**Step 1: Update View() for chat mode**

Replace the current chat mode rendering at the bottom of `View()`:
```go
if m.streaming {
    return m.viewport.View() + "\n" + m.renderStatusBar()
}
return m.viewport.View() + "\n" + m.theme.InputPromptStyle().Render("> ") + m.input + "\n" + m.renderStatusBar()
```

With dynamic layout:
```go
statusBar := m.renderStatusBar()
inputArea := m.textarea.View()

// Spinner line: shown only during streaming
spinnerLine := ""
if m.streaming {
    spinnerLine = m.theme.SpinnerStyle().Render(m.spinner.View() + " 生成中...")
}

// Dynamic viewport height
vpHeight := m.height - lipgloss.Height(statusBar) - lipgloss.Height(inputArea)
if spinnerLine != "" {
    vpHeight -= lipgloss.Height(spinnerLine)
}
if vpHeight < 1 {
    vpHeight = 1
}
if m.viewport.Height != vpHeight {
    m.viewport.Height = vpHeight
    if m.chatFollowBottom {
        m.viewport.GotoBottom()
    }
}

parts := []string{m.viewport.View()}
if spinnerLine != "" {
    parts = append(parts, spinnerLine)
}
parts = append(parts, inputArea, statusBar)
return lipgloss.JoinVertical(lipgloss.Left, parts...)
```

**Step 2: Add lipgloss import**

Add to imports in `view.go`:
```go
"github.com/charmbracelet/lipgloss"
```

**Step 3: Add SpinnerStyle to theme**

Read `internal/ui/theme.go` and add a `SpinnerStyle()` method that returns a dimmed/muted style. Example:
```go
func (t Theme) SpinnerStyle() lipgloss.Style {
    return lipgloss.NewStyle().Foreground(t.SubtleColor())
}
```

(Read theme.go first to understand the existing pattern and color names.)

**Step 4: Remove █ cursor from buildChatContent()**

In `buildChatContent()`, find and remove:
```go
b.WriteString("\u2588\n")
```
The spinner now handles the "generating" indicator.

**Step 5: Build check**
```bash
go build ./...
```

---

### Task 5: Lipgloss styling improvements

**Files:** `internal/ui/view.go`, `internal/ui/theme.go`

**Step 1: Style the textarea**

In the textarea initialization (model.go), add styling via lipgloss:
```go
ta.FocusedStyle.Base = lipgloss.NewStyle().
    Border(lipgloss.RoundedBorder()).
    BorderForeground(lipgloss.Color("62"))
ta.BlurredStyle.Base = lipgloss.NewStyle().
    Border(lipgloss.RoundedBorder()).
    BorderForeground(lipgloss.Color("240"))
```

**Step 2: Style message labels**

In `buildChatContent()`, the user label currently uses `m.theme.UserLabelStyle()`. Ensure it has a distinct color (e.g. green/blue). Read `theme.go` to check current colors and adjust if needed.

**Step 3: Build and run tests**
```bash
go build ./...
go test -run "TestParseCLI|TestResolveAltScreen" -v .
```

**Step 4: Commit all UI changes**
```bash
git add internal/ui/model.go internal/ui/update.go internal/ui/view.go internal/ui/theme.go internal/ui/slashcomplete.go
git commit -m "feat(ui): textarea input, spinner, dynamic layout, lipgloss styling"
```

---

### Task 6: Build test binary and smoke test

```bash
cd /home/cc/Github/llm-chat-in-terminal/.worktrees/viewport-migration
go build -o /tmp/termchat-test .
echo "Binary ready at /tmp/termchat-test"
```

Manual checklist:
- [ ] Mouse wheel scrolls chat (not terminal scrollback)
- [ ] Right-click shows context menu
- [ ] Textarea shows cursor, left/right arrow works
- [ ] Shift+Enter adds newline in input
- [ ] Enter sends message
- [ ] Spinner appears above input during streaming
- [ ] Spinner disappears after response
- [ ] Terminal resize doesn't break layout
- [ ] `/clear` resets correctly
- [ ] Slash autocomplete still works

**Final commit and push:**
```bash
git add -A
git commit -m "chore: build artifacts" --allow-empty
git push origin feature/viewport-migration
```
