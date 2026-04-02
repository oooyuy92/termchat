// internal/ui/onboard.go
package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func (m Model) updateOnboardMode(msg tea.KeyMsg) (Model, tea.Cmd) {
	if msg.String() == "esc" || msg.String() == "s" {
		m.mode = modeChat
		m.statusMsg = "Skipped setup. Use /model to configure provider and model."
		return m, nil
	}
	if msg.String() == "enter" {
		m.modelSel = newModelSelectorState(m.modelRegistry, modelSelectorManage)
		m.mode = modeModelSelector
		m.statusMsg = "Configure a provider and model"
		return m, nil
	}

	if msg.String() != "ctrl+c" {
		m.confirmQuit = false
	}
	if msg.String() == "ctrl+c" {
		if m.confirmQuit {
			return m, tea.Quit
		}
		m.confirmQuit = true
		m.statusMsg = "Press Ctrl+C again to quit"
		return m, nil
	}
	return m, nil
}

func (m Model) viewOnboard() string {
	tabBar := (&m).renderTabBar()
	var b strings.Builder

	b.WriteString(m.theme.ConfigTitleStyle().Render("Welcome to termchat!"))
	b.WriteString("\n")
	b.WriteString(m.theme.ConfigHelpStyle().Render("Set up your first provider and model to get started."))
	b.WriteString("\n\n")
	b.WriteString(m.theme.ConfigLabelStyle().Render("Model Registry"))
	b.WriteString("\n")
	b.WriteString(m.theme.ConfigHelpStyle().Render("  termchat now manages providers and models through /model only."))
	b.WriteString("\n")
	b.WriteString(m.theme.ConfigHelpStyle().Render("  Add a provider, then add one or more models under it, then select the model to use in this chat."))
	b.WriteString("\n")
	b.WriteString(m.theme.ConfigHelpStyle().Render("  Enter: open model manager  |  S/Esc: skip for now  |  Ctrl+C: quit"))
	b.WriteString("\n")

	return lipgloss.JoinVertical(lipgloss.Left, tabBar, b.String()+"\n"+m.renderStatusBar())
}
