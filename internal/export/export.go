// internal/export/export.go
package export

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/signintech/gopdf"
	"github.com/termchat/termchat/internal/chat"
)

//go:embed fonts/NotoSansSC-Regular.ttf
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
	pdf := gopdf.GoPdf{}
	pdf.Start(gopdf.Config{PageSize: *gopdf.PageSizeA4, Unit: gopdf.UnitPT})
	if err := pdf.AddTTFFontData("NotoSansSC", notoSansSC); err != nil {
		return fmt.Errorf("load font: %w", err)
	}

	margin := 40.0
	textW := gopdf.PageSizeA4.W - 2*margin

	pdf.AddPage()
	pdf.SetMargins(margin, margin, margin, margin)
	pdf.SetX(margin)
	pdf.SetY(margin)

	for _, m := range msgs {
		role := "User"
		if m.Role == "assistant" {
			role = "Assistant"
		}
		// Role label
		if err := pdf.SetFont("NotoSansSC", "", 13); err != nil {
			return err
		}
		if err := pdf.Cell(nil, role); err != nil {
			return err
		}
		pdf.Br(18)
		pdf.SetX(margin)
		// Content
		if err := pdf.SetFont("NotoSansSC", "", 11); err != nil {
			return err
		}
		if err := pdf.MultiCell(&gopdf.Rect{W: textW, H: 15}, m.Content); err != nil {
			return err
		}
		pdf.Br(10)
		pdf.SetX(margin)
	}

	return pdf.WritePdf(path)
}
