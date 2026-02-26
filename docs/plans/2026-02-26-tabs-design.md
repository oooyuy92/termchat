# Multi-Tab Chat Design

Date: 2026-02-26
Status: Approved

## Overview

Add a tab bar to the top of the UI so users can run multiple independent chat sessions simultaneously, each with its own model/provider, history, and streaming state.

## Requirements

- Each tab has its own model/provider (independent of other tabs)
- Default tab name = model name; user can rename via F2
- Switch tabs: mouse click, Alt+1–9, Alt+0 for overflow menu
- New tab: Ctrl+T or `+` button; Close tab: Ctrl+W or `×` button on tab
- Overflow: tabs that don't fit collapse into a `…` menu
- Background tabs continue receiving stream output while user is on another tab

## Architecture: Single Model + TabSession (Approach B)

Keep a single BubbleTea root `Model`. Extract all per-tab state into a `TabSession` struct. `Model` holds `[]TabSession` and `activeTab int`.

### TabSession (new)

```go
type TabSession struct {
    name            string
    client          chat.Provider
    history         *chat.History
    viewport        viewport.Model
    textarea        textarea.Model
    spinner         spinner.Model
    renderer        *glamour.TermRenderer
    streaming       bool
    streamCh        <-chan chat.StreamChunk
    streamErr       <-chan error
    streamCtrl      *streamControl
    currentResp     string
    currentThinking string
    chatFollowBottom bool
    pendingImages   []chat.ImageData
    imageCounter    int
    autoSaveName    string
    err             error
}
```

### Model changes

Fields moved out of `Model` into `TabSession`:
`history`, `client`, `viewport`, `textarea`, `spinner`, `renderer`, `streaming`, `streamCh`, `streamErr`, `streamCtrl`, `currentResp`, `currentThinking`, `chatFollowBottom`, `pendingImages`, `imageCounter`, `autoSaveName`, `err`

Fields added to `Model`:
```go
tabs      []TabSession
activeTab int
tabRename string  // rename input buffer (used in modeTabRename)
```

Global fields staying in `Model`:
`cfg`, `theme`, `store`, `mode`, `configEd`, `resumePick`, `shortcutEd`, `roleEd`, `rolePick`, `cfgPath`, `shortcutsPath`, `rolesPath`, `activeRole`, `slashAC`, `escCount`, `browseCursor`, `confirmQuit`, `statusMsg`, `totalTokens`, `width`, `height`

### Stream messages carry TabIdx

```go
type streamChunkMsg struct { TabIdx int; Content string; Thinking string }
type streamDoneMsg  struct { TabIdx int }
type streamErrMsg   struct { TabIdx int; Err error }
type autoSavedMsg   struct { TabIdx int; Err error }
```

This allows background tabs to continue receiving stream output.

## UI Layout

```
[ gpt-4o × ][ claude-3 × ][ gemini × ][ + ][ … ]   ← tab bar (1 line)
─────────────────────────────────────────────────
(viewport)
(spinner line — only during streaming)
(textarea)
(status bar)
```

- Active tab: bold + highlighted background (StatusKeyBg colors)
- Inactive tabs: muted foreground
- `×` on each tab to close; `+` at end to create; `…` when overflow
- `vpHeight = height - 1(tabBar) - statusBarHeight - inputAreaHeight - spinnerHeight`

### Tab bar overflow

Render tabs left-to-right until remaining width is exhausted. If any tabs are hidden, show `[ … N ]` (N = hidden count) at the end. `[+]` always appears before `[…]`.

### Mouse hit detection

Render time: record `tabBarHitZones []hitZone` with start/end X for each clickable element. On `tea.MouseMsg` with Y=0, run hit test to determine clicked element.

## Interactions

### Keyboard shortcuts

| Action | Key |
|--------|-----|
| New tab | Ctrl+T |
| Close tab | Ctrl+W |
| Jump to tab N | Alt+1 … Alt+9 |
| Open overflow menu | Alt+0 |
| Rename tab | F2 |

### New tab

1. Create `TabSession` copying current tab's provider/model config
2. Name defaults to current model name
3. Switch to new tab (`activeTab = len(tabs) - 1`)
4. New tab's textarea gets focus

### Close tab

1. If streaming → cancel stream first, then close
2. If only 1 tab → refuse, show `"Cannot close last tab"` in status bar
3. After close, switch to left neighbour (`activeTab = max(0, activeTab-1)`)

### Rename tab (modeTabRename)

- F2 enters rename mode; `tabRename` initialized to current tab name
- Typing updates `tabRename`; Enter saves, Esc cancels
- Status bar shows `Rename tab: <input>█`

### Per-tab model configuration

`/settings` modifies only `tabs[activeTab].client` at runtime. Global `cfg` stores the default config used when creating new tabs. New tabs inherit the current model/provider from `cfg` (or from the active tab — TBD at implementation time).

## New UI Mode

Add `modeTabRename` to the `uiMode` enum.
Add `modeTabOverflow` for the overflow `…` dropdown.
