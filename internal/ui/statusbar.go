// internal/ui/statusbar.go
package ui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

var statusBarStyle = lipgloss.NewStyle().
	Background(lipgloss.Color("236")).
	Foreground(lipgloss.Color("252")).
	Padding(0, 1)

var statusKeyStyle = lipgloss.NewStyle().
	Background(lipgloss.Color("63")).
	Foreground(lipgloss.Color("230")).
	Padding(0, 1).
	Bold(true)

func (m Model) renderStatusBar() string {
	model := statusKeyStyle.Render("model") + statusBarStyle.Render(m.client.Model())
	tokens := statusKeyStyle.Render("tokens") + statusBarStyle.Render(fmt.Sprintf("%d", m.totalTokens))
	msgs := statusKeyStyle.Render("msgs") + statusBarStyle.Render(fmt.Sprintf("%d", m.history.Count()))

	status := model + " " + tokens + " " + msgs

	if m.statusMsg != "" {
		status += "  " + statusBarStyle.Render(m.statusMsg)
	}

	bar := lipgloss.NewStyle().
		Width(m.width).
		Background(lipgloss.Color("236")).
		Render(status)

	return bar
}
