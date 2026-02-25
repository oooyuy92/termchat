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

//go:embed fonts/DroidSansFallback.ttf
var cjkFont []byte

// ExportTxt writes messages to path as plain text.
func ExportTxt(path string, msgs []chat.Message) error {
	var b strings.Builder
	for _, m := range msgs {
		var label string
		switch m.Role {
		case "user":
			label = "You"
		case "assistant":
			label = "AI"
		default:
			continue // skip system and other roles
		}
		fmt.Fprintf(&b, "[%s]: %s\n\n", label, m.Content)
	}
	return os.WriteFile(path, []byte(b.String()), 0644)
}

// ExportMd writes messages to path as Markdown.
func ExportMd(path string, msgs []chat.Message) error {
	var b strings.Builder
	for _, m := range msgs {
		var heading string
		switch m.Role {
		case "user":
			heading = "## You"
		case "assistant":
			heading = "## AI"
		default:
			continue // skip system and other roles
		}
		fmt.Fprintf(&b, "%s\n\n%s\n\n", heading, m.Content)
	}
	return os.WriteFile(path, []byte(b.String()), 0644)
}

// ErrNoDownloadsDir is returned when the Downloads directory cannot be found.
// The caller should prompt the user to configure export_dir in settings.
var ErrNoDownloadsDir = fmt.Errorf("Downloads folder not found; set export_dir in /settings")

// defaultDownloadsDir returns the platform-appropriate Downloads directory,
// or an error if it cannot be determined or does not exist.
func defaultDownloadsDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", ErrNoDownloadsDir
	}
	// Windows: %USERPROFILE%\Downloads
	// macOS/Linux: ~/Downloads
	dir := filepath.Join(home, "Downloads")
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return "", ErrNoDownloadsDir
	}
	return dir, nil
}

// ResolvePath returns the full export path for a conversation name and extension.
// If exportDir is empty, uses the platform Downloads folder.
// Returns ErrNoDownloadsDir if Downloads doesn't exist and no exportDir is configured.
func ResolvePath(exportDir, convName, ext string) (string, error) {
	dir := exportDir
	if dir == "" {
		var err error
		dir, err = defaultDownloadsDir()
		if err != nil {
			return "", err
		}
	} else if strings.HasPrefix(dir, "~/") {
		// Expand ~ if present in user-configured path
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
	if err := pdf.AddTTFFontData("CJK", cjkFont); err != nil {
		return fmt.Errorf("load font: %w", err)
	}

	margin := 40.0
	textW := gopdf.PageSizeA4.W - 2*margin

	pdf.AddPage()
	pdf.SetMargins(margin, margin, margin, margin)
	pdf.SetX(margin)
	pdf.SetY(margin)

	for _, m := range msgs {
		var label string
		switch m.Role {
		case "user":
			label = "You"
		case "assistant":
			label = "AI"
		default:
			continue // skip system and other roles
		}
		// Role label
		if err := pdf.SetFont("CJK", "", 13); err != nil {
			return err
		}
		if err := pdf.Cell(nil, label); err != nil {
			return err
		}
		pdf.Br(18)
		pdf.SetX(margin)
		// Content
		if err := pdf.SetFont("CJK", "", 11); err != nil {
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
