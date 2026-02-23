# Slash Command Autocomplete — Design

**Date:** 2026-02-23

## Goal

Show a dropdown of matching slash commands above the input line whenever the user types `/` as the first character.

## Requirements

- Trigger only when `/` is the first character of input
- Display candidates above the input line (popup style)
- Navigate with ↑↓ arrows
- Tab → fill selected command into input (continue editing args)
- Enter → execute selected command immediately
- Each candidate shows command name + short description
- Maximum 5 visible candidates at a time (scroll with cursor)

---

## Architecture

### New mode: `modeSlashComplete`

Added to `uiMode` iota after `modeOnboard`. Follows the same pattern as `modeConfig`, `modeRoles`, etc.

### New types (in `internal/ui/slashcomplete.go`)

```go
type slashCmd struct {
    Name string // e.g. "/resume"
    Desc string // e.g. "Browse conversations"
}

type slashComplete struct {
    matches []slashCmd
    cursor  int
}
```

Package-level `var slashCmds = []slashCmd{...}` with all 9 commands and descriptions.

Filtering: `strings.HasPrefix(cmd.Name, m.input)` — input is the prefix.

---

## Mode Transitions

In `updateChatMode` character input handling:

| Event | Action |
|-------|--------|
| First char typed is `/` | Enter `modeSlashComplete`, compute initial matches (all cmds) |
| More chars typed | Stay in `modeSlashComplete`, refilter matches |
| Backspace → input empty | Return to `modeChat` |
| Esc | Clear input, return to `modeChat` |
| ↑ / ↓ | Move `slashAC.cursor` (wraps top↔bottom) |
| Tab | Fill `matches[cursor].Name` into `m.input`, return to `modeChat` |
| Enter | Execute `matches[cursor].Name` via `handleCommand`, return to `modeChat` |
| `matches` becomes empty | Return to `modeChat`, continue normal input |

---

## Rendering (`viewSlashComplete`)

Layout (top to bottom):

```
[conversation history — same as chat mode]

  /resume  Browse conversations      ← non-highlighted (dim)
▶ /roles   Edit roles                ← highlighted (accent + ▶)
  /settings  Model & parameters

> /ro_                               ← input line
[status bar]
```

Details:
- Max 5 candidates visible; viewport scrolls with cursor
- Command name column: `%-12s` padding for alignment
- Non-highlighted rows: `ConfigHelpStyle`
- Highlighted row: `ConfigCursorStyle` for `▶`, full row highlighted
- Description text: subdued color (same as help text)

---

## Files

| File | Change |
|------|--------|
| `internal/ui/model.go` | Add `modeSlashComplete` to iota; add `slashAC slashComplete` field to `Model` |
| `internal/ui/slashcomplete.go` | New file: `slashCmd`, `slashComplete` types, `slashCmds` var, `updateSlashComplete`, `viewSlashComplete` |
| `internal/ui/update.go` | In `updateChatMode`: detect `/` first char → enter `modeSlashComplete`; route `modeSlashComplete` in `Update()` |
| `internal/ui/view.go` | Route `modeSlashComplete` to `viewSlashComplete()` |
