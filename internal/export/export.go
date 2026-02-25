// internal/export/export.go
package export

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-pdf/fpdf"
	"github.com/termchat/termchat/internal/chat"
)

//go:embed fonts/NotoSansSC-Regular.otf
var notoSansSC []byte

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
		pdf.SetFont("NotoSansSC", "", 12)
		pdf.MultiCell(textW, lineH, role, "", "L", false)
		pdf.SetFont("NotoSansSC", "", 11)
		pdf.MultiCell(textW, lineH, m.Content, "", "L", false)
		pdf.Ln(4)
	}

	return pdf.OutputFileAndClose(path)
}
