// internal/ui/view.go
package ui

import (
	"fmt"
	"strings"
)

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
			b.WriteString(msg.Content + "\n")
		case "assistant":
			b.WriteString(m.theme.AssistantLabelStyle().Render(m.client.Model()+":") + "\n")
			rendered, err := m.renderer.Render(msg.Content)
			if err != nil {
				b.WriteString(msg.Content + "\n")
			} else {
				b.WriteString(rendered)
			}
		}
	}

	// Render current streaming response
	if m.streaming {
		b.WriteString(m.theme.AssistantLabelStyle().Render(m.client.Model()+":") + "\n")
		if m.currentThinking != "" {
			b.WriteString(m.theme.ThinkingStyle().Render("\U0001f4ad "+m.currentThinking) + "\n")
		}
		if m.currentResp != "" {
			rendered, err := m.renderer.Render(m.currentResp)
			if err != nil {
				b.WriteString(m.currentResp)
			} else {
				b.WriteString(rendered)
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
