// internal/ui/view.go
package ui

import (
	"fmt"
	"strings"
)

// cleanGlamourOutput strips glamour's leading/trailing blank lines and
// collapses inter-paragraph blank lines.
// The glamour paragraph renderer hardcodes "\n" before each non-first paragraph
// and "\n" after each paragraph, producing "\n\n" (a blank line) between them.
// Code block blank lines have ANSI codes between newlines and are unaffected.
func cleanGlamourOutput(s string) string {
	s = strings.TrimLeft(s, "\n")
	s = strings.ReplaceAll(s, "\n\n", "\n")
	s = strings.TrimRight(s, " \n")
	return s
}

func (m Model) View() string {
	if m.mode == modeConfig {
		return m.viewConfigEditor()
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
				b.WriteString(cleanGlamourOutput(rendered) + "\n\n")
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
				b.WriteString(cleanGlamourOutput(rendered) + "\n")
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
