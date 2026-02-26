# Viewport Migration Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Replace the custom scroll implementation in the chat UI with `charmbracelet/bubbles/viewport`, fixing scrolling drift, mouse wheel issues, and resize bugs.

**Architecture:** Add `viewport.Model` to `Model`, remove `chatScrollTop`/`chatFollowBottom` and all manual scroll helpers. Separate the input line from chat content so viewport owns only the message area. Delegate key/mouse events to viewport directly.

**Tech Stack:** Go 1.24, charmbracelet/bubbletea v1.3.10, charmbracelet/bubbles (new dep), charmbracelet/glamour, charmbracelet/lipgloss

---

### Task 1: Add bubbles dependency

**Files:**
- Modify: `go.mod`, `go.sum`

**Step 1: Add the dependency**

```bash
cd /home/cc/Github/llm-chat-in-terminal
go get github.com/charmbracelet/bubbles@latest
```

Expected: `go.mod` now lists `github.com/charmbracelet/bubbles` under `require`.

**Step 2: Verify it compiles**

```bash
go build ./...
```

Expected: no errors.

**Step 3: Commit**

```bash
git add go.mod go.sum
git commit -m "chore(deps): add charmbracelet/bubbles for viewport"
```

---

### Task 2: Add viewport to Model struct

**Files:**
- Modify: `internal/ui/model.go`

**Step 1: Add import**

In `internal/ui/model.go`, add to the import block:

```go
"github.com/charmbracelet/bubbles/viewport"
```

**Step 2: Replace scroll fields with viewport**

In the `Model` struct, replace:

```go
chatScrollTop    int
chatFollowBottom bool
```

with:

```go
viewport         viewport.Model
chatFollowBottom bool  // still needed: tracks whether to auto-scroll on new content
```

**Step 3: Initialize viewport in NewModel**

At the end of `NewModel`, before `return Model{...}`, the viewport will be sized on first `WindowSizeMsg`. Initialize with zero size for now — it gets sized in Update. In the returned `Model{}` literal, add:

```go
viewport:         viewport.New(0, 0),
chatFollowBottom: true,
```

Remove the old `chatScrollTop` field from the literal (it no longer exists).

**Step 4: Verify it compiles**

```bash
go build ./...
```

Expected: compile errors about `chatScrollTop` being used elsewhere — that's fine, we'll fix them in the next tasks.

---

### Task 3: Update WindowSizeMsg handler to size the viewport

**Files:**
- Modify: `internal/ui/update.go`

**Step 1: Update the WindowSizeMsg case**

Find the `tea.WindowSizeMsg` case in `Update()`:

```go
case tea.WindowSizeMsg:
    m.width = msg.Width
    m.height = msg.Height
    if msg.Width > 0 {
        m.recreateRenderer(msg.Width)
    }
    return m, nil
```

Replace with:

```go
case tea.WindowSizeMsg:
    m.width = msg.Width
    m.height = msg.Height
    if msg.Width > 0 {
        m.recreateRenderer(msg.Width)
    }
    // viewport height = total height - 1 (input line) - 1 (status bar)
    vpHeight := msg.Height - 2
    if vpHeight < 1 {
        vpHeight = 1
    }
    m.viewport.Width = msg.Width
    m.viewport.Height = vpHeight
    // Re-render content at new size
    m.viewport.SetContent(m.buildChatContent())
    if m.chatFollowBottom {
        m.viewport.GotoBottom()
    }
    return m, nil
```

**Step 2: Verify it compiles**

```bash
go build ./...
```

---

### Task 4: Replace scroll key handling with viewport delegation

**Files:**
- Modify: `internal/ui/update.go`

**Step 1: Replace handleChatScrollKey calls**

Find the two places `handleChatScrollKey` is called in `Update()`:

1. In the `m.streaming` block:
```go
if m.handleChatScrollKey(msg.String()) {
    return m, nil
}
```

Replace with:
```go
if isScrollKey(msg.String()) {
    var cmd tea.Cmd
    m.viewport, cmd = m.viewport.Update(msg)
    // Update follow-bottom state based on viewport position
    m.chatFollowBottom = m.viewport.AtBottom()
    return m, cmd
}
```

2. In the non-streaming key switch:
```go
case "up", "down", "pgup", "pgdown", "home", "end":
    m.handleChatScrollKey(msg.String())
    return m, nil
```

Replace with:
```go
case "up", "down", "pgup", "pgdown", "home", "end":
    var cmd tea.Cmd
    m.viewport, cmd = m.viewport.Update(msg)
    m.chatFollowBottom = m.viewport.AtBottom()
    return m, cmd
```

**Step 2: Add isScrollKey helper**

Add this small helper near the bottom of `update.go` (before the existing helpers):

```go
func isScrollKey(key string) bool {
    switch key {
    case "up", "down", "pgup", "pgdown", "home", "end":
        return true
    }
    return false
}
```

**Step 3: Handle mouse events for viewport**

In `Update()`, add a new case for `tea.MouseMsg` (add it after the `tea.KeyMsg` case):

```go
case tea.MouseMsg:
    if m.mode == modeChat {
        var cmd tea.Cmd
        m.viewport, cmd = m.viewport.Update(msg)
        m.chatFollowBottom = m.viewport.AtBottom()
        return m, cmd
    }
```

**Step 4: Delete dead scroll functions**

Delete these functions entirely from `update.go`:
- `handleMouseFallbackRunes`
- `maxChatScrollTop`
- `scrollChatBy`
- `scrollChatToTop`
- `scrollChatToBottom`
- `handleChatScrollKey`

**Step 5: Verify it compiles**

```bash
go build ./...
```

---

### Task 5: Update streaming to push content into viewport

**Files:**
- Modify: `internal/ui/update.go`

**Step 1: Update streamChunkMsg handler**

Find:
```go
case streamChunkMsg:
    m.currentResp += msg.Content
    m.currentThinking += msg.Thinking
    return m, m.readNextChunk()
```

Replace with:
```go
case streamChunkMsg:
    m.currentResp += msg.Content
    m.currentThinking += msg.Thinking
    m.viewport.SetContent(m.buildChatContent())
    if m.chatFollowBottom {
        m.viewport.GotoBottom()
    }
    return m, m.readNextChunk()
```

**Step 2: Update streamDoneMsg handler**

Find:
```go
case streamDoneMsg:
    m.streaming = false
    m.escCount = 0
    if m.currentResp != "" {
        m.history.Add(chat.Message{Role: "assistant", Content: m.currentResp})
    }
    m.currentResp = ""
    m.currentThinking = ""
    return m, m.autoSaveCmd()
```

Replace with:
```go
case streamDoneMsg:
    m.streaming = false
    m.escCount = 0
    if m.currentResp != "" {
        m.history.Add(chat.Message{Role: "assistant", Content: m.currentResp})
    }
    m.currentResp = ""
    m.currentThinking = ""
    m.viewport.SetContent(m.buildChatContent())
    if m.chatFollowBottom {
        m.viewport.GotoBottom()
    }
    return m, m.autoSaveCmd()
```

**Step 3: Update the Enter key handler (sending a message)**

Find where `m.streaming = true` is set (in the `"enter"` case). After setting `m.chatFollowBottom = true`, add:

```go
m.viewport.SetContent(m.buildChatContent())
m.viewport.GotoBottom()
```

**Step 4: Verify it compiles**

```bash
go build ./...
```

---

### Task 6: Update View() to use viewport

**Files:**
- Modify: `internal/ui/view.go`

**Step 1: Update the main View() function**

Find:
```go
content := m.buildChatContent()
return m.renderChatViewport(content) + "\n" + m.renderStatusBar()
```

Replace with:
```go
inputLine := m.theme.InputPromptStyle().Render("> ") + m.input
if m.streaming {
    inputLine = ""
}
return m.viewport.View() + "\n" + inputLine + "\n" + m.renderStatusBar()
```

**Step 2: Remove input area from buildChatContent()**

In `buildChatContent()`, find and delete the input area section at the bottom:

```go
// Input area.
if !m.streaming {
    b.WriteString(m.theme.InputPromptStyle().Render("> ") + m.input)
}
```

**Step 3: Delete dead view helpers**

Delete these functions from `view.go`:
- `chatViewportHeight()`
- `maxChatScrollForContent()`
- `renderChatViewport()`

**Step 4: Verify it compiles**

```bash
go build ./...
```

---

### Task 7: Update /clear command and resume to reset viewport

**Files:**
- Modify: `internal/ui/update.go`

**Step 1: Update /clear handler**

Find the `/clear` case in `handleCommand`:

```go
case "/clear":
    m.history.Clear()
    m.totalTokens = 0
    m.pendingImages = nil
    m.imageCounter = 0
    m.chatScrollTop = 0
    m.chatFollowBottom = true
    m.statusMsg = "Conversation cleared"
    return m, nil
```

Replace with:

```go
case "/clear":
    m.history.Clear()
    m.totalTokens = 0
    m.pendingImages = nil
    m.imageCounter = 0
    m.chatFollowBottom = true
    m.viewport.SetContent("")
    m.viewport.GotoBottom()
    m.statusMsg = "Conversation cleared"
    return m, nil
```

**Step 2: Search for any remaining chatScrollTop references**

```bash
grep -rn "chatScrollTop" /home/cc/Github/llm-chat-in-terminal/
```

Expected: no results. If any remain, remove them.

**Step 3: Final compile check**

```bash
go build ./...
```

Expected: clean build, no errors.

**Step 4: Run existing tests**

```bash
go test ./...
```

Expected: all pass (existing tests don't test UI scroll logic).

**Step 5: Commit**

```bash
git add internal/ui/model.go internal/ui/update.go internal/ui/view.go go.mod go.sum
git commit -m "feat(ui): replace custom scroll with bubbles/viewport"
```

---

### Task 8: Enable mouse support in main.go

**Files:**
- Modify: `main.go`

**Step 1: Read main.go to find where tea.Program is created**

The viewport's mouse wheel support requires `tea.WithMouseCellMotion()` or `tea.WithMouseAllMotion()` to be passed to the program.

Find the `tea.NewProgram(...)` call and add mouse support:

```go
tea.WithMouseCellMotion(),
```

**Step 2: Verify and build**

```bash
go build ./...
```

**Step 3: Manual smoke test**

Run the app and verify:
- [ ] Chat scrolls with arrow keys / PgUp / PgDown
- [ ] Mouse wheel scrolls
- [ ] New messages auto-scroll to bottom
- [ ] Scrolling up pauses auto-follow
- [ ] Scrolling back to bottom re-enables auto-follow
- [ ] Terminal resize doesn't break scroll position
- [ ] `/clear` resets the viewport

**Step 4: Commit**

```bash
git add main.go
git commit -m "feat(ui): enable mouse support for viewport scrolling"
```
