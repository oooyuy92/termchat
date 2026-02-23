# Conversation Management Design

## Goal

Add message-level management to termchat: browse all messages in the active conversation via a full-screen viewer (triggered by double-Esc), and perform rollback, single-message delete, branch, and copy operations.

## Trigger: Double-Esc

In chat mode, pressing Esc twice consecutively enters `modeMessageBrowse`. The first Esc increments `escCount` and shows a status hint (`"Press Esc again to browse messages"`). The second Esc enters browse mode. Any other key (except `ctrl+c`) resets `escCount` to 0.

If history is empty, double-Esc shows `"No messages to browse"` and does not enter browse mode.

## Message Browsing View

Full-screen independent page showing the **complete content** of the currently selected message:

```
─── Message 3 / 5 ─── user ──────────────────────────────

那量子纠缠能用于通信吗？…（完整内容）

─────────────────────────────────────────────────────────
↑↓: prev/next  Enter: rollback  d: delete  b: branch  c: copy  Esc: back
```

- Header: `Message N / Total`, role (`user` / `assistant`)
- Body: full message content; assistant messages rendered with glamour Markdown
- If content exceeds available height, vertically clip and show `(↓ more)` indicator
- Fixed help bar at bottom
- ↑↓ jumps between messages (not line-level scrolling within a message)

## Operations

| Key | Operation |
|-----|-----------|
| `↑` / `k` | Previous message |
| `↓` / `j` | Next message |
| `enter` | **Rollback**: `Truncate(cursor+1)`, exit browse mode, auto-save; status: `"Rolled back to message N"` |
| `d` | **Delete single**: `DeleteAt(cursor)`, clamp cursor, auto-save; exit if history becomes empty |
| `b` | **Branch**: generate new timestamp `autoSaveName`, `Truncate(cursor+1)`, update `m.autoSaveName`, auto-save as new conversation; original conversation in DB untouched; status: `"Branched: <new-name>"` |
| `c` | **Copy**: `clipboard.WriteAll(messages[cursor].Content)` via `atotto/clipboard`; status: `"Copied"` |
| `esc` | Exit to `modeChat` |
| `ctrl+c` | Double-press to quit (reuses existing `confirmQuit` logic) |

## Data Model Changes

### `chat.History` new methods
```go
func (h *History) DeleteAt(i int)  // remove message at index i
func (h *History) Truncate(n int)  // keep only first n messages
```

### `Model` new fields
```go
escCount     int // consecutive Esc presses for double-Esc detection
browseCursor int // selected message index in modeMessageBrowse
```

### New uiMode
```go
modeMessageBrowse
```

## Files Changed

| File | Change |
|------|--------|
| `internal/chat/history.go` | Add `DeleteAt`, `Truncate` |
| `internal/chat/history_test.go` | Tests for new methods |
| `internal/ui/model.go` | Add `modeMessageBrowse` to iota, `escCount` + `browseCursor` to `Model` |
| `internal/ui/messagebrowse.go` | New: `updateMessageBrowse`, `viewMessageBrowse` |
| `internal/ui/update.go` | Double-Esc detection in chat mode, route `modeMessageBrowse` |
| `internal/ui/view.go` | Route `modeMessageBrowse` |
| `go.mod` / `go.sum` | Add `github.com/atotto/clipboard` |

## Approach Chosen

**Counter-based double-Esc** (no timer): consistent with existing `confirmQuit` pattern in codebase.
