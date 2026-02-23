// internal/ui/roleeditor.go
package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/termchat/termchat/internal/roles"
)

func (m Model) saveRolesCmd() tea.Cmd {
	items := make([]roles.Role, len(m.roleEd.items))
	copy(items, m.roleEd.items)
	path := m.rolesPath
	return func() tea.Msg {
		err := roles.Save(path, items)
		return rolesSavedMsg{Err: err}
	}
}

func (m Model) updateRolesMode(msg tea.KeyMsg) (Model, tea.Cmd) {
	ed := &m.roleEd

	if msg.String() != "ctrl+c" {
		m.confirmQuit = false
	}

	switch ed.subMode {

	case roleModeList:
		switch msg.String() {
		case "up", "k":
			if ed.cursor > 0 {
				ed.cursor--
			}
		case "down", "j":
			if ed.cursor < len(ed.items)-1 {
				ed.cursor++
			}
		case "e":
			if len(ed.items) == 0 {
				return m, nil
			}
			ed.savedName = ed.items[ed.cursor].Name
			ed.savedPrompt = ed.items[ed.cursor].Prompt
			ed.editBuf = ed.items[ed.cursor].Name
			ed.isNew = false
			ed.subMode = roleModeEditName
		case "n":
			newItems := make([]roles.Role, len(ed.items)+1)
			copy(newItems, ed.items)
			ed.items = newItems
			ed.cursor = len(ed.items) - 1
			ed.savedName = ""
			ed.savedPrompt = ""
			ed.editBuf = ""
			ed.isNew = true
			ed.subMode = roleModeEditName
		case "d":
			if len(ed.items) == 0 {
				return m, nil
			}
			ed.items = append(ed.items[:ed.cursor], ed.items[ed.cursor+1:]...)
			if ed.cursor >= len(ed.items) && ed.cursor > 0 {
				ed.cursor--
			}
			return m, m.saveRolesCmd()
		case "esc":
			m.mode = modeChat
			m.statusMsg = "Back to chat"
		case "ctrl+c":
			if m.confirmQuit {
				return m, tea.Quit
			}
			m.confirmQuit = true
			m.statusMsg = "Press Ctrl+C again to quit"
		}

	case roleModeEditName:
		switch msg.String() {
		case "enter":
			ed.items[ed.cursor].Name = ed.editBuf
			ed.editBuf = ed.items[ed.cursor].Prompt
			ed.subMode = roleModeEditPrompt
		case "esc":
			if ed.isNew {
				ed.items = ed.items[:len(ed.items)-1]
				if ed.cursor >= len(ed.items) && ed.cursor > 0 {
					ed.cursor--
				}
			} else {
				ed.items[ed.cursor].Name = ed.savedName
				ed.items[ed.cursor].Prompt = ed.savedPrompt
			}
			ed.subMode = roleModeList
		case "backspace":
			runes := []rune(ed.editBuf)
			if len(runes) > 0 {
				ed.editBuf = string(runes[:len(runes)-1])
			}
		default:
			if msg.Type == tea.KeyRunes {
				ed.editBuf += string(msg.Runes)
			}
		}

	case roleModeEditPrompt:
		switch msg.String() {
		case "enter":
			ed.items[ed.cursor].Prompt = ed.editBuf
			ed.subMode = roleModeList
			return m, m.saveRolesCmd()
		case "esc":
			if ed.isNew {
				ed.items = ed.items[:len(ed.items)-1]
				if ed.cursor >= len(ed.items) && ed.cursor > 0 {
					ed.cursor--
				}
			} else {
				ed.items[ed.cursor].Name = ed.savedName
				ed.items[ed.cursor].Prompt = ed.savedPrompt
			}
			ed.subMode = roleModeList
		case "backspace":
			runes := []rune(ed.editBuf)
			if len(runes) > 0 {
				ed.editBuf = string(runes[:len(runes)-1])
			}
		default:
			if msg.Type == tea.KeyRunes {
				ed.editBuf += string(msg.Runes)
			}
		}
	}

	return m, nil
}

func (m Model) viewRolesEditor() string {
	var b strings.Builder
	ed := m.roleEd

	switch ed.subMode {
	case roleModeList:
		b.WriteString(m.theme.ConfigTitleStyle().Render("角色"))
		b.WriteString("\n\n")

		if len(ed.items) == 0 {
			b.WriteString(m.theme.ConfigHelpStyle().Render("  (empty — press 'n' to add one)"))
			b.WriteString("\n\n")
			b.WriteString(m.theme.ConfigHelpStyle().Render("  n: new  |  Esc: back"))
			b.WriteString("\n")
			return b.String() + "\n" + m.renderStatusBar()
		}

		for i, role := range ed.items {
			cursor := "  "
			if i == ed.cursor {
				cursor = m.theme.ConfigCursorStyle().Render("> ")
			}
			name := m.theme.ConfigValueStyle().Render(role.Name)
			if role.Prompt != "" {
				preview := m.theme.ConfigHelpStyle().Render("  " + truncate(role.Prompt, 40))
				b.WriteString(cursor + name + preview + "\n")
			} else {
				hint := m.theme.ConfigHelpStyle().Render("  (无系统提示)")
				b.WriteString(cursor + name + hint + "\n")
			}
		}

		b.WriteString("\n")
		b.WriteString(m.theme.ConfigHelpStyle().Render("  ↑↓: navigate  |  e: edit  |  n: new  |  d: delete  |  Esc: back"))
		b.WriteString("\n")

	case roleModeEditName:
		if len(ed.items) == 0 {
			ed.subMode = roleModeList
			break
		}
		b.WriteString(m.theme.ConfigTitleStyle().Render("角色 — 编辑名称"))
		b.WriteString("\n\n")
		role := ed.items[ed.cursor]
		b.WriteString(m.theme.ConfigLabelStyle().Render("  名称:   ") + m.theme.ConfigEditStyle().Render(ed.editBuf+"\u2588") + "\n")
		b.WriteString(m.theme.ConfigLabelStyle().Render("  提示词: ") + m.theme.ConfigValueStyle().Render(truncate(role.Prompt, 60)) + "\n")
		b.WriteString("\n")
		b.WriteString(m.theme.ConfigHelpStyle().Render("  Enter: next field  |  Esc: cancel"))
		b.WriteString("\n")

	case roleModeEditPrompt:
		if len(ed.items) == 0 {
			ed.subMode = roleModeList
			break
		}
		b.WriteString(m.theme.ConfigTitleStyle().Render("角色 — 编辑提示词"))
		b.WriteString("\n\n")
		role := ed.items[ed.cursor]
		b.WriteString(m.theme.ConfigLabelStyle().Render("  名称:   ") + m.theme.ConfigValueStyle().Render(role.Name) + "\n")
		b.WriteString(m.theme.ConfigLabelStyle().Render("  提示词: ") + m.theme.ConfigEditStyle().Render(ed.editBuf+"\u2588") + "\n")
		b.WriteString("\n")
		b.WriteString(m.theme.ConfigHelpStyle().Render("  Enter: save  |  Esc: cancel"))
		b.WriteString("\n")
	}

	return b.String() + "\n" + m.renderStatusBar()
}
