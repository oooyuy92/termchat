// internal/ui/statusbar.go
package ui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
	xansi "github.com/charmbracelet/x/ansi"
)

func (m Model) renderStatusBar() string {
	model := m.theme.StatusKeyStyle().Render("model") + m.theme.StatusBarStyle().Render(m.client.Model())
	tokens := m.theme.StatusKeyStyle().Render("tokens") + m.theme.StatusBarStyle().Render(fmt.Sprintf("%d", m.totalTokens))
	msgs := m.theme.StatusKeyStyle().Render("msgs") + m.theme.StatusBarStyle().Render(fmt.Sprintf("%d", m.history.Count()))

	roleDisplay := ""
	if m.activeRole != "" {
		roleDisplay = " " + m.theme.StatusKeyStyle().Render("角色") + m.theme.StatusBarStyle().Render(m.activeRole)
	}
	status := model + " " + tokens + " " + msgs + roleDisplay

	if m.statusMsg != "" {
		status += "  " + m.theme.StatusBarStyle().Render(m.statusMsg)
	}
	if m.width > 0 {
		// Keep status bar to a single terminal line so it doesn't steal chat rows.
		status = xansi.TruncateWc(status, m.width, "")
	}

	bar := lipgloss.NewStyle().
		Width(m.width).
		Background(lipgloss.Color(m.theme.StatusBarBg)).
		Render(status)

	return bar
}
