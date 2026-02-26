# History Rename + Conversation Export Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Rename `/resume` to `/history`, update display text, and add `s` key in history list to export the selected conversation as txt/md/pdf.

**Architecture:** The export logic lives in a new `internal/export` package. The history picker gains an export sub-state (`exportPick bool` + `exportFmt int`) rendered as a left/right selector at the bottom. PDF uses `go-pdf/fpdf` with an embedded NotoSansSC font for CJK support. Export dir is configurable via `/settings`.

**Tech Stack:** Go 1.24, BubbleTea TUI, `github.com/go-pdf/fpdf` for PDF, `go:embed` for font, SQLite storage (existing).

---

### Task 1: Rename /resume → /history

**Files:**
- Modify: `internal/ui/slashcomplete.go:26`
- Modify: `internal/ui/update.go:471`
- Modify: `internal/ui/resumepicker.go:108`

**Step 1: Apply the three renames**

In `internal/ui/slashcomplete.go` line 26, change:
```go
{"/resume", "Browse conversations by date"},
```
to:
```go
{"/history", "Conversation history"},
```

In `internal/ui/update.go` line 471, change:
```go
case "/resume":
```
to:
```go
case "/history":
```

In `internal/ui/resumepicker.go` line 108, change:
```go
b.WriteString(m.theme.ConfigTitleStyle().Render("Resume a Conversation"))
```
to:
```go
b.WriteString(m.theme.ConfigTitleStyle().Render("Conversation History"))
```

**Step 2: Build to verify no compile errors**

```bash
go build ./...
```
Expected: no output (success)

**Step 3: Commit**

```bash
git add internal/ui/slashcomplete.go internal/ui/update.go internal/ui/resumepicker.go
git commit -m "feat(ui): rename /resume to /history"
```

---

### Task 2: Add ExportDir to config

**Files:**
- Modify: `internal/config/config.go`
- Modify: `internal/ui/configeditor.go`

**Step 1: Add ExportDir field to SettingsConfig**

In `internal/config/config.go`, change `SettingsConfig`:
```go
type SettingsConfig struct {
	Theme           string `yaml:"theme"`
	AlternateScreen string `yaml:"alternate_screen"`
	ExportDir       string `yaml:"export_dir,omitempty"`
}
```

Default stays empty (means current working directory). No change needed in `DefaultConfig()`.

**Step 2: Add Export Dir field to settings editor**

In `internal/ui/configeditor.go`, in `buildConfigFields`, add after the `Alt Screen` line:
```go
{Label: "Export Dir", Key: "export_dir", Value: cfg.Settings.ExportDir},
```

**Step 3: Add validation in validateField**

In `validateField`, add a case (no-op — any string is valid, empty means cwd):
```go
case "export_dir":
    // any value is valid; empty means current working directory
```

**Step 4: Add apply in applyFieldToConfig**

```go
case "export_dir":
    cfg.Settings.ExportDir = value
```

**Step 5: Build**

```bash
go build ./...
```

**Step 6: Commit**

```bash
git add internal/config/config.go internal/ui/configeditor.go
git commit -m "feat(config): add export_dir setting"
```

---

### Task 3: Create export package (txt + md)

**Files:**
- Create: `internal/export/export.go`

**Step 1: Write the file**

```go
// internal/export/export.go
package export

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/termchat/termchat/internal/chat"
)

// ExportTxt writes messages to path as plain text.
func ExportTxt(path string, msgs []chat.Message) error {
	var b strings.Builder
	for _, m := range msgs {
		role := "User"
		if m.Role == "assistant" {
			role = "Assistant"
		}
		fmt.Fprintf(&b, "[%s]: %s\n\n", role, m.Content)
	}
	return os.WriteFile(path, []byte(b.String()), 0644)
}

// ExportMd writes messages to path as Markdown.
func ExportMd(path string, msgs []chat.Message) error {
	var b strings.Builder
	for _, m := range msgs {
		heading := "## User"
		if m.Role == "assistant" {
			heading = "## Assistant"
		}
		fmt.Fprintf(&b, "%s\n\n%s\n\n", heading, m.Content)
	}
	return os.WriteFile(path, []byte(b.String()), 0644)
}

// ResolvePath returns the full export path for a conversation name and extension.
// If exportDir is empty, uses the current working directory.
func ResolvePath(exportDir, convName, ext string) (string, error) {
	dir := exportDir
	if dir == "" {
		var err error
		dir, err = os.Getwd()
		if err != nil {
			return "", err
		}
	}
	// Expand ~ if present
	if strings.HasPrefix(dir, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		dir = filepath.Join(home, dir[2:])
	}
	return filepath.Join(dir, convName+"."+ext), nil
}
```

**Step 2: Build**

```bash
go build ./...
```

**Step 3: Commit**

```bash
git add internal/export/export.go
git commit -m "feat(export): add txt and md export"
```

---

### Task 4: Add PDF export with embedded CJK font

**Files:**
- Create: `internal/export/fonts/` (directory + font file)
- Modify: `internal/export/export.go`

**Step 1: Download NotoSansSC font**

```bash
mkdir -p internal/export/fonts
curl -L "https://github.com/notofonts/noto-cjk/raw/main/Sans/SubsetOTF/SC/NotoSansSC-Regular.otf" \
  -o internal/export/fonts/NotoSansSC-Regular.otf
```

Note: fpdf accepts TTF or OTF. If the above URL fails, use any NotoSansSC-Regular.ttf/otf from the system or noto-fonts package.

**Step 2: Add PDF dependency**

```bash
go get github.com/go-pdf/fpdf@latest
```

**Step 3: Add ExportPdf to export.go**

Add to the imports:
```go
import (
    _ "embed"
    // ... existing imports
    "github.com/go-pdf/fpdf"
)

//go:embed fonts/NotoSansSC-Regular.otf
var notoSansSC []byte
```

Add the function:
```go
// ExportPdf writes messages to path as a PDF with CJK font support.
func ExportPdf(path string, msgs []chat.Message) error {
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.AddUTF8FontFromBytes("NotoSansSC", "", notoSansSC)
	pdf.SetFont("NotoSansSC", "", 11)
	pdf.AddPage()

	lineH := 6.0
	pageW, _, _ := pdf.PageSize(0)
	margin := 15.0
	textW := pageW - 2*margin
	pdf.SetMargins(margin, margin, margin)
	pdf.SetAutoPageBreak(true, margin)

	for _, m := range msgs {
		role := "User"
		if m.Role == "assistant" {
			role = "Assistant"
		}
		// Role heading in bold-ish (larger size)
		pdf.SetFont("NotoSansSC", "", 12)
		pdf.MultiCell(textW, lineH, role, "", "L", false)
		pdf.SetFont("NotoSansSC", "", 11)
		pdf.MultiCell(textW, lineH, m.Content, "", "L", false)
		pdf.Ln(4)
	}

	return pdf.OutputFileAndClose(path)
}
```

**Step 4: Build**

```bash
go build ./...
```

**Step 5: Commit**

```bash
git add internal/export/ go.mod go.sum
git commit -m "feat(export): add PDF export with embedded NotoSansSC font"
```

---

### Task 5: Add export sub-state to resumePicker

**Files:**
- Modify: `internal/ui/resumepicker.go`
- Modify: `internal/ui/model.go` (resumePicker struct)

**Step 1: Add exportPick fields to resumePicker struct**

In `internal/ui/model.go`, find the `resumePicker` struct (it's defined in resumepicker.go — check there). In `resumepicker.go`, the struct is implicit via the `resumePicker` type. Add fields:

The `resumePicker` struct is defined in `resumepicker.go`. Add two fields:

```go
type resumePicker struct {
	groups     []dateGroup
	dateIdx    int
	convIdx    int
	exporting  bool // true when format selector is active
	exportFmt  int  // 0=txt, 1=md, 2=pdf
}
```

**Step 2: Handle `s` key and export navigation in updateResumeMode**

In `updateResumeMode`, add before the existing switch:

```go
// If export picker is active, handle its keys first
if p.exporting {
    return m.updateExportPick(msg)
}
```

Add `case "s":` inside the switch:
```go
case "s":
    if len(p.groups) > 0 {
        p.exporting = true
    }
```

**Step 3: Add updateExportPick method**

```go
func (m Model) updateExportPick(msg tea.KeyMsg) (Model, tea.Cmd) {
	p := &m.resumePick
	switch msg.String() {
	case "left", "h":
		if p.exportFmt > 0 {
			p.exportFmt--
		}
	case "right", "l":
		if p.exportFmt < 2 {
			p.exportFmt++
		}
	case "esc":
		p.exporting = false
	case "enter":
		p.exporting = false
		conv := p.groups[p.dateIdx].convs[p.convIdx]
		msgs, err := m.store.Load(conv.Name)
		if err != nil {
			m.statusMsg = "Export failed: " + err.Error()
			return m, nil
		}
		exts := []string{"txt", "md", "pdf"}
		ext := exts[p.exportFmt]
		path, err := export.ResolvePath(m.cfg.Settings.ExportDir, conv.Name, ext)
		if err != nil {
			m.statusMsg = "Export failed: " + err.Error()
			return m, nil
		}
		switch p.exportFmt {
		case 0:
			err = export.ExportTxt(path, msgs)
		case 1:
			err = export.ExportMd(path, msgs)
		case 2:
			err = export.ExportPdf(path, msgs)
		}
		if err != nil {
			m.statusMsg = "Export failed: " + err.Error()
		} else {
			m.statusMsg = "Saved to " + path
		}
	}
	return m, nil
}
```

Add import `"github.com/termchat/termchat/internal/export"` to resumepicker.go.

**Step 4: Update viewResumePicker to show export selector**

At the bottom of `viewResumePicker`, before the help line, add:

```go
if p.exporting {
    fmts := []string{"txt", "md", "pdf"}
    var parts []string
    for i, f := range fmts {
        if i == p.exportFmt {
            parts = append(parts, m.theme.ConfigCursorStyle().Render("["+f+"]"))
        } else {
            parts = append(parts, m.theme.ConfigHelpStyle().Render(f))
        }
    }
    b.WriteString(m.theme.ConfigHelpStyle().Render("  Export: ") +
        strings.Join(parts, m.theme.ConfigHelpStyle().Render(" | ")))
    b.WriteString("\n")
    b.WriteString(m.theme.ConfigHelpStyle().Render("  ←/→: select format  Enter: export  Esc: cancel"))
    b.WriteString("\n")
} else {
    b.WriteString(m.theme.ConfigHelpStyle().Render("  ↑↓: select  |  ←→: change date  |  Enter: resume  |  s: export  |  Esc: back"))
    b.WriteString("\n")
}
```

Replace the existing help line.

**Step 5: Build**

```bash
go build ./...
```

**Step 6: Manual smoke test**

1. Run `./termchat`
2. Type `/history` → enter
3. Navigate to a conversation, press `s`
4. Verify format selector appears at bottom
5. Use `←/→` to switch formats, press Enter
6. Verify status bar shows `Saved to ./...`
7. Verify file exists and has correct content

**Step 7: Commit**

```bash
git add internal/ui/resumepicker.go
git commit -m "feat(ui): add export picker to history list (s key)"
```

---

### Task 6: Push

```bash
git push
```
