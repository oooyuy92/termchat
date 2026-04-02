// internal/ui/roleeditor.go
package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/termchat/termchat/internal/roles"
)

// editScrollMax returns the maximum scrollTop so the last line is visible.
func editScrollMax(text string, height, overhead int) int {
	lines := strings.Split(text, "\n")
	available := height - overhead
	if available < 3 {
		available = 3
	}
	max := len(lines) - available
	if max < 0 {
		max = 0
	}
	return max
}

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
			ed.scrollTop = 0
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
			ed.scrollTop = 0
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
			if strings.TrimSpace(ed.editBuf) == "" {
				return m, nil
			}
			ed.items[ed.cursor].Name = ed.editBuf
			ed.editBuf = ed.items[ed.cursor].Prompt
			// Start at bottom so cursor (end of text) is visible
			ed.scrollTop = editScrollMax(ed.editBuf, m.height, 7)
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
		case "shift+enter":
			ed.editBuf += "\n"
			ed.scrollTop = editScrollMax(ed.editBuf, m.height, 7)
		case "up", "k":
			if ed.scrollTop > 0 {
				ed.scrollTop--
			}
		case "down", "j":
			max := editScrollMax(ed.editBuf, m.height, 7)
			if ed.scrollTop < max {
				ed.scrollTop++
			}
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
				ed.scrollTop = editScrollMax(ed.editBuf, m.height, 7)
			}
		default:
			if msg.Type == tea.KeyRunes {
				text := string(msg.Runes)
				if text == "\n" {
					ed.editBuf += "\n"
				} else {
					ed.editBuf += text
				}
				ed.scrollTop = editScrollMax(ed.editBuf, m.height, 7)
			}
		}
	}

	return m, nil
}

func (m Model) viewRolesEditor() string {
	tabBar := (&m).renderTabBar()
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
			return lipgloss.JoinVertical(lipgloss.Left, tabBar, b.String()+"\n"+m.renderStatusBar())
		}

		for i, role := range ed.items {
			cursor := "  "
			if i == ed.cursor {
				cursor = m.theme.ConfigCursorStyle().Render("> ")
			}
			name := m.theme.ConfigValueStyle().Render(role.Name)
			if role.Prompt != "" {
				// Replace newlines with spaces for single-line preview in list
				flat := strings.ReplaceAll(strings.ReplaceAll(role.Prompt, "\n", " "), "  ", " ")
				preview := m.theme.ConfigHelpStyle().Render("  " + truncate(flat, 40))
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
		flatPrompt := strings.ReplaceAll(strings.ReplaceAll(role.Prompt, "\n", " "), "  ", " ")
		b.WriteString(m.theme.ConfigLabelStyle().Render("  提示词: ") + m.theme.ConfigValueStyle().Render(truncate(flatPrompt, 60)) + "\n")
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
		b.WriteString(m.theme.ConfigLabelStyle().Render("  名称: ") + m.theme.ConfigValueStyle().Render(role.Name) + "\n")

		// Windowed scrollable content display
		// overhead: title(1)+blank(1)+name(1)+label(1)+blank(1)+help(1)+status(1) = 7
		const overhead = 7
		available := m.height - overhead
		if available < 3 {
			available = 3
		}

		allLines := strings.Split(ed.editBuf, "\n")
		totalLines := len(allLines)

		// Clamp scrollTop
		scrollTop := ed.scrollTop
		maxScroll := totalLines - available
		if maxScroll < 0 {
			maxScroll = 0
		}
		if scrollTop > maxScroll {
			scrollTop = maxScroll
		}
		if scrollTop < 0 {
			scrollTop = 0
		}
		endLine := scrollTop + available
		if endLine > totalLines {
			endLine = totalLines
		}

		if totalLines > available {
			label := fmt.Sprintf("  提示词 (%d–%d / %d 行, ↑↓ 滚动):", scrollTop+1, endLine, totalLines)
			b.WriteString(m.theme.ConfigLabelStyle().Render(label) + "\n")
		} else {
			b.WriteString(m.theme.ConfigLabelStyle().Render("  提示词:") + "\n")
		}

		cursorLine := totalLines - 1
		for i, line := range allLines[scrollTop:endLine] {
			display := line
			if scrollTop+i == cursorLine {
				display += "\u2588"
			}
			b.WriteString(m.theme.ConfigEditStyle().Render("  "+display) + "\n")
		}

		b.WriteString("\n")
		b.WriteString(m.theme.ConfigHelpStyle().Render("  Enter: 保存  |  Shift+Enter: 换行  |  ↑↓: 滚动  |  Esc: 取消"))
		b.WriteString("\n")
	}

	return lipgloss.JoinVertical(lipgloss.Left, tabBar, b.String()+"\n"+m.renderStatusBar())
}
