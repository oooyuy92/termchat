// internal/ui/messagebrowse.go
package ui

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// writeToClipboard copies text to the system clipboard using platform-native commands.
func writeToClipboard(text string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("pbcopy")
	case "windows":
		cmd = exec.Command("clip")
	default:
		// Linux: try wl-copy (Wayland), then xclip, then xsel
		if _, err := exec.LookPath("wl-copy"); err == nil {
			cmd = exec.Command("wl-copy")
		} else if _, err := exec.LookPath("xclip"); err == nil {
			cmd = exec.Command("xclip", "-selection", "clipboard")
		} else if _, err := exec.LookPath("xsel"); err == nil {
			cmd = exec.Command("xsel", "--clipboard", "--input")
		} else {
			return fmt.Errorf("no clipboard tool found (install xclip, xsel, or wl-clipboard)")
		}
	}
	cmd.Stdin = strings.NewReader(text)
	return cmd.Run()
}

func (m Model) updateMessageBrowse(msg tea.KeyMsg) (Model, tea.Cmd) {
	// Reset confirmQuit on any key other than ctrl+c
	if msg.String() != "ctrl+c" {
		m.confirmQuit = false
	}

	msgs := m.history.Messages()

	switch msg.String() {
	case "esc":
		m.mode = modeChat
		m.escCount = 0
		return m, nil

	case "ctrl+c":
		if m.confirmQuit {
			return m, tea.Quit
		}
		m.confirmQuit = true
		m.statusMsg = "Press Ctrl+C again to quit"
		return m, nil

	case "up", "k":
		if m.browseCursor > 0 {
			m.browseCursor--
		}

	case "down", "j":
		if m.browseCursor < len(msgs)-1 {
			m.browseCursor++
		}

	case "enter":
		// Rollback: keep messages[0..cursor] inclusive
		n := m.browseCursor + 1
		m.history.Truncate(n)
		m.statusMsg = fmt.Sprintf("Rolled back to message %d", n)
		m.mode = modeChat
		return m, m.autoSaveCmd()

	case "d":
		if len(msgs) == 0 {
			return m, nil
		}
		m.history.DeleteAt(m.browseCursor)
		remaining := m.history.Messages()
		if len(remaining) == 0 {
			m.mode = modeChat
			m.statusMsg = "All messages deleted"
			return m, m.autoSaveCmd()
		}
		if m.browseCursor >= len(remaining) {
			m.browseCursor = len(remaining) - 1
		}
		return m, m.autoSaveCmd()

	case "b":
		// Branch: save history[0..cursor] as a brand-new conversation
		newName := time.Now().Format("2006-01-02_150405")
		m.history.Truncate(m.browseCursor + 1)
		m.autoSaveName = newName
		m.statusMsg = "Branched: " + newName
		m.mode = modeChat
		return m, m.autoSaveCmd()

	case "c":
		if len(msgs) > m.browseCursor {
			if err := writeToClipboard(msgs[m.browseCursor].Content); err != nil {
				m.statusMsg = "Copy failed: " + err.Error()
			} else {
				m.statusMsg = "Copied"
			}
		}
	}

	return m, nil
}

func (m Model) viewMessageBrowse() string {
	msgs := m.history.Messages()
	if len(msgs) == 0 {
		var b strings.Builder
		b.WriteString(m.theme.ConfigTitleStyle().Render("Browse Messages") + "\n\n")
		b.WriteString(m.theme.ConfigHelpStyle().Render("  No messages.") + "\n\n")
		b.WriteString(m.theme.ConfigHelpStyle().Render("  Esc: back") + "\n")
		return b.String() + "\n" + m.renderStatusBar()
	}

	cur := m.browseCursor
	if cur >= len(msgs) {
		cur = len(msgs) - 1
	}
	selected := msgs[cur]

	var b strings.Builder

	// Header line
	header := fmt.Sprintf("── Message %d / %d ── %s ", cur+1, len(msgs), selected.Role)
	if m.width > len(header)+2 {
		header += strings.Repeat("─", m.width-len(header)-1)
	}
	b.WriteString(m.theme.ConfigTitleStyle().Render(header) + "\n\n")

	// Full message content
	var content string
	if selected.Role == "assistant" {
		rendered, err := m.renderer.Render(selected.Content)
		if err != nil {
			content = selected.Content
		} else {
			content = cleanGlamourOutput(rendered)
		}
	} else {
		content = selected.Content
	}

	// Clip content to available height to avoid overflow
	// Available lines = total height - header(2) - blank(1) - divider(1) - help(1) - status(1) - blank(1)
	availableLines := m.height - 7
	if availableLines < 1 {
		availableLines = 1
	}
	contentLines := strings.Split(content, "\n")
	clipped := false
	if len(contentLines) > availableLines {
		contentLines = contentLines[:availableLines]
		clipped = true
	}
	b.WriteString(strings.Join(contentLines, "\n"))
	if clipped {
		b.WriteString("\n" + m.theme.ConfigHelpStyle().Render("(↓ more…)"))
	}
	b.WriteString("\n\n")

	// Bottom divider
	divider := strings.Repeat("─", m.width-1)
	if m.width <= 1 {
		divider = "─"
	}
	b.WriteString(divider + "\n")
	b.WriteString(m.theme.ConfigHelpStyle().Render(
		"↑↓: prev/next  Enter: rollback  d: delete  b: branch  c: copy  Esc: back",
	) + "\n")

	return b.String() + "\n" + m.renderStatusBar()
}
