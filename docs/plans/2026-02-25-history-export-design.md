# Design: /history rename + conversation export

Date: 2026-02-25

## Summary

1. Rename `/resume` slash command to `/history` with updated display text.
2. Add `s` key in history list to export selected conversation as txt/md/pdf.

---

## Part 1: Rename

| Location | Change |
|---|---|
| `internal/ui/slashcomplete.go` | `/resume` → `/history`, description → `Conversation history` |
| `internal/ui/update.go` | `case "/resume"` → `case "/history"` |
| `internal/ui/resumepicker.go` | Title text `Resume a Conversation` → `Conversation History` |

---

## Part 2: Export Feature

### Interaction Flow

In history list mode, press `s` to enter export format picker (sub-state).
Bottom bar shows left/right selector:

```
Export: < txt | [md] | pdf >   ←/→: select  enter: confirm  esc: cancel
```

- `h` / `left` / `l` / `right`: cycle through formats
- `enter`: export and return to history list
- `esc`: cancel, return to history list
- After export: status bar shows `Saved to /path/to/file.md`

### Export Path

- Default: current working directory (`./`)
- Configurable via `/settings` → `Export Dir` field
- Filename: `<conversation-name>.<ext>` (conversation name is already a timestamp)

### Export Formats

**txt**: Plain text, each message as `[User]: content` or `[Assistant]: content`, blank line between messages.

**md**: Markdown with `## User` / `## Assistant` headings, content as-is.

**pdf**: Uses `go-pdf/fpdf` library. NotoSansSC-Regular.ttf embedded via `go:embed` for CJK support.

### New Files

- `internal/export/export.go` — `ExportTxt`, `ExportMd`, `ExportPdf` functions
- `internal/export/fonts/NotoSansSC-Regular.ttf` — embedded CJK font

### Modified Files

- `internal/config/config.go` — add `ExportDir string` to `SettingsConfig`
- `internal/ui/configeditor.go` — add `Export Dir` field in settings editor
- `internal/ui/resumepicker.go` — add export sub-state and key handling
- `internal/ui/slashcomplete.go` — rename
- `internal/ui/update.go` — rename
