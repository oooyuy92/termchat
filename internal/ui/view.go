// internal/ui/view.go
package ui

import (
	"fmt"
	"strings"

	"github.com/muesli/reflow/wrap"
)

// cleanGlamourOutput strips glamour's leading/trailing blank lines and
// collapses inter-paragraph blank lines.
// The glamour paragraph renderer hardcodes "\n" before each non-first paragraph
// and "\n" after each paragraph, producing "\n\n" (a blank line) between them.
// Code block blank lines have ANSI codes between newlines and are unaffected.
// The MarginWriter pads every line to terminal width with spaces; stripping
// trailing spaces first converts those padded "blank" lines into empty strings
// so the subsequent \n\n collapse catches them.
func cleanGlamourOutput(s string) string {
	s = strings.TrimLeft(s, "\n")
	// Strip trailing spaces from each line to remove padding-writer artifacts.
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimRight(line, " ")
	}
	s = strings.Join(lines, "\n")
	s = strings.ReplaceAll(s, "\n\n", "\n")
	s = strings.TrimRight(s, " \n")
	return s
}

func (m Model) View() string {
	if m.mode == modeConfig {
		return m.viewConfigEditor()
	}
	if m.mode == modeResume {
		return m.viewResumePicker()
	}
	if m.mode == modeShortcuts {
		return m.viewShortcutsEditor()
	}

	var b strings.Builder

	// Render conversation history
	for _, msg := range m.history.Messages() {
		switch msg.Role {
		case "user":
			b.WriteString(m.theme.UserLabelStyle().Render("You:") + "\n")
			b.WriteString(msg.Content + "\n\n")
		case "assistant":
			b.WriteString(m.theme.AssistantLabelStyle().Render(m.client.Model()+":") + "\n")
			rendered, err := m.renderer.Render(msg.Content)
			if err != nil {
				b.WriteString(msg.Content + "\n\n")
			} else {
				cleaned := cleanGlamourOutput(rendered)
				// Hard-wrap long lines (e.g. Chinese text with no spaces) that
				// glamour's word-wrapper cannot break at word boundaries.
				if m.width > 0 {
					cleaned = wrap.String(cleaned, m.width)
				}
				b.WriteString(cleaned + "\n\n")
			}
		}
	}

	// Render current streaming response
	if m.streaming {
		b.WriteString(m.theme.AssistantLabelStyle().Render(m.client.Model()+":") + "\n")
		if m.currentThinking != "" {
			thinking := strings.ReplaceAll(strings.TrimSpace(m.currentThinking), "\n\n", "\n")
			b.WriteString(m.theme.ThinkingStyle().Render("\U0001f4ad "+thinking) + "\n")
		}
		if m.currentResp != "" {
			rendered, err := m.renderer.Render(m.currentResp)
			if err != nil {
				b.WriteString(m.currentResp)
			} else {
				cleaned := cleanGlamourOutput(rendered)
				if m.width > 0 {
					cleaned = wrap.String(cleaned, m.width)
				}
				b.WriteString(cleaned + "\n")
			}
		}
		b.WriteString("\u2588\n")
	}

	// Render error
	if m.err != nil {
		b.WriteString(m.theme.ErrStyle().Render(fmt.Sprintf("Error: %v", m.err)) + "\n")
	}

	// Input area
	if !m.streaming {
		b.WriteString(m.theme.InputPromptStyle().Render("> ") + m.input)
	}

	content := b.String()
	return content + "\n" + m.renderStatusBar()
}
