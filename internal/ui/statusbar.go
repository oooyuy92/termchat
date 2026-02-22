// internal/ui/statusbar.go
package ui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

func (m Model) renderStatusBar() string {
	model := m.theme.StatusKeyStyle().Render("model") + m.theme.StatusBarStyle().Render(m.client.Model())
	tokens := m.theme.StatusKeyStyle().Render("tokens") + m.theme.StatusBarStyle().Render(fmt.Sprintf("%d", m.totalTokens))
	msgs := m.theme.StatusKeyStyle().Render("msgs") + m.theme.StatusBarStyle().Render(fmt.Sprintf("%d", m.history.Count()))

	status := model + " " + tokens + " " + msgs

	if m.statusMsg != "" {
		status += "  " + m.theme.StatusBarStyle().Render(m.statusMsg)
	}

	bar := lipgloss.NewStyle().
		Width(m.width).
		Background(lipgloss.Color(m.theme.StatusBarBg)).
		Render(status)

	return bar
}
