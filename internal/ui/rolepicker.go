// internal/ui/rolepicker.go
package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func (m Model) updateRolePickerMode(msg tea.KeyMsg) (Model, tea.Cmd) {
	p := &m.rolePick

	if msg.String() != "ctrl+c" {
		m.confirmQuit = false
	}

	switch msg.String() {
	case "up", "k":
		if p.cursor > 0 {
			p.cursor--
		}
	case "down", "j":
		if p.cursor < len(p.items)-1 {
			p.cursor++
		}
	case "enter":
		if len(p.items) == 0 || p.cursor >= len(p.items) {
			m.mode = modeChat
			return m, nil
		}
		role := p.items[p.cursor]
		m.activeTabSession().history.SetSystemPrompt(role.Prompt)
		m.activeRole = role.Name
		m.mode = modeChat
		return m, nil
	case "esc":
		// Use blank system prompt (no role selected)
		m.activeTabSession().history.SetSystemPrompt("")
		m.activeRole = ""
		m.mode = modeChat
		return m, nil
	case "ctrl+c":
		if m.confirmQuit {
			return m, tea.Quit
		}
		m.confirmQuit = true
		m.statusMsg = "Press Ctrl+C again to quit"
	}
	return m, nil
}

func (m Model) viewRolePicker() string {
	tabBar := (&m).renderTabBar()
	var b strings.Builder
	p := m.rolePick

	b.WriteString(m.theme.ConfigTitleStyle().Render("选择角色"))
	b.WriteString("\n\n")

	if len(p.items) == 0 {
		b.WriteString(m.theme.ConfigHelpStyle().Render("  无可用角色。"))
		b.WriteString("\n\n")
		b.WriteString(m.theme.ConfigHelpStyle().Render("  Esc: 跳过"))
		b.WriteString("\n")
		return lipgloss.JoinVertical(lipgloss.Left, tabBar, b.String()+"\n"+m.renderStatusBar())
	}

	for i, role := range p.items {
		cursor := "  "
		if i == p.cursor {
			cursor = m.theme.ConfigCursorStyle().Render("> ")
		}
		name := m.theme.ConfigValueStyle().Render(role.Name)
		if role.Prompt != "" {
			preview := m.theme.ConfigHelpStyle().Render("  " + truncate(role.Prompt, 50))
			b.WriteString(cursor + name + preview + "\n")
		} else {
			hint := m.theme.ConfigHelpStyle().Render("  (无系统提示)")
			b.WriteString(cursor + name + hint + "\n")
		}
	}

	b.WriteString("\n")
	b.WriteString(m.theme.ConfigHelpStyle().Render("  ↑↓: 选择  |  Enter: 确认  |  Esc: 跳过（不使用角色）"))
	b.WriteString("\n")

	return lipgloss.JoinVertical(lipgloss.Left, tabBar, b.String()+"\n"+m.renderStatusBar())
}
