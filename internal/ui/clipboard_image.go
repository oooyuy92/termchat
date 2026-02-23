package ui

import (
	"fmt"
	"os/exec"
	"runtime"

	"github.com/termchat/termchat/internal/chat"
)

// readImageFromClipboard attempts to read image data from the system clipboard.
// Returns nil, nil if no image is found in the clipboard.
func readImageFromClipboard() (*chat.ImageData, error) {
	type attempt struct {
		mime string
		args []string
	}

	var attempts []attempt

	switch runtime.GOOS {
	case "darwin":
		attempts = []attempt{
			{"image/png", []string{"pngpaste", "-"}},
		}
	case "windows":
		attempts = []attempt{
			{"image/png", []string{"powershell", "-command", "Get-Clipboard -Format Image | ForEach-Object { $ms = New-Object System.IO.MemoryStream; $_.Save($ms, [System.Drawing.Imaging.ImageFormat]::Png); [Console]::OpenStandardOutput().Write($ms.ToArray(), 0, $ms.Length) }"}},
		}
	default:
		attempts = []attempt{
			{"image/png", []string{"wl-paste", "--type", "image/png"}},
			{"image/jpeg", []string{"wl-paste", "--type", "image/jpeg"}},
			{"image/png", []string{"xclip", "-selection", "clipboard", "-t", "image/png", "-o"}},
			{"image/jpeg", []string{"xclip", "-selection", "clipboard", "-t", "image/jpeg", "-o"}},
		}
	}

	for _, a := range attempts {
		if _, err := exec.LookPath(a.args[0]); err != nil {
			continue
		}
		cmd := exec.Command(a.args[0], a.args[1:]...)
		out, err := cmd.Output()
		if err == nil && len(out) > 0 {
			return &chat.ImageData{MimeType: a.mime, Data: out}, nil
		}
	}

	return nil, nil
}

// readTextFromClipboard reads text content from the system clipboard.
func readTextFromClipboard() (string, error) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("pbpaste")
	case "windows":
		cmd = exec.Command("powershell", "-command", "Get-Clipboard")
	default:
		if _, err := exec.LookPath("wl-paste"); err == nil {
			cmd = exec.Command("wl-paste", "--no-newline")
		} else if _, err := exec.LookPath("xclip"); err == nil {
			cmd = exec.Command("xclip", "-selection", "clipboard", "-o")
		} else if _, err := exec.LookPath("xsel"); err == nil {
			cmd = exec.Command("xsel", "--clipboard", "--output")
		} else {
			return "", fmt.Errorf("no clipboard tool found")
		}
	}
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}
