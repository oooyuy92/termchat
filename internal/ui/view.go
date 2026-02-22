// internal/ui/view.go
package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	userLabelStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("86")).
		Bold(true)

	assistantLabelStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("212")).
		Bold(true)

	inputPromptStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("241"))

	errStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("196")).
		Bold(true)
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
			b.WriteString(userLabelStyle.Render("You:") + "\n")
			b.WriteString(msg.Content + "\n\n")
		case "assistant":
			b.WriteString(assistantLabelStyle.Render("Assistant:") + "\n")
			rendered, err := m.renderer.Render(msg.Content)
			if err != nil {
				b.WriteString(msg.Content + "\n\n")
			} else {
				b.WriteString(rendered + "\n")
			}
		}
	}

	// Render current streaming response
	if m.streaming && m.currentResp != "" {
		b.WriteString(assistantLabelStyle.Render("Assistant:") + "\n")
		rendered, err := m.renderer.Render(m.currentResp)
		if err != nil {
			b.WriteString(m.currentResp)
		} else {
			b.WriteString(rendered)
		}
		b.WriteString("\u2588\n")
	}

	// Render error
	if m.err != nil {
		b.WriteString(errStyle.Render(fmt.Sprintf("Error: %v", m.err)) + "\n\n")
	}

	// Input area
	if !m.streaming {
		b.WriteString(inputPromptStyle.Render("> ") + m.input)
	}

	content := b.String()
	return content + "\n" + m.renderStatusBar()
}
