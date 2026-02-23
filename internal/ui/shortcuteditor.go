// internal/ui/shortcuteditor.go
package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/termchat/termchat/internal/shortcuts"
)

func (m Model) saveShortcutsCmd() tea.Cmd {
	items := make([]shortcuts.Shortcut, len(m.shortcutEd.items))
	copy(items, m.shortcutEd.items)
	path := m.shortcutsPath
	return func() tea.Msg {
		err := shortcuts.Save(path, items)
		return shortcutsSavedMsg{Err: err}
	}
}

func (m Model) updateShortcutsMode(msg tea.KeyMsg) (Model, tea.Cmd) {
	ed := &m.shortcutEd

	if msg.String() != "ctrl+c" {
		m.confirmQuit = false
	}

	switch ed.subMode {

	case shortcutModeList:
		switch msg.String() {
		case "up", "k":
			if ed.cursor > 0 {
				ed.cursor--
			}
		case "down", "j":
			if ed.cursor < len(ed.items)-1 {
				ed.cursor++
			}
		case "enter":
			if len(ed.items) == 0 {
				return m, nil
			}
			m.input = ed.items[ed.cursor].Content
			m.mode = modeChat
			m.statusMsg = "Shortcut loaded"
			return m, nil
		case "e":
			if len(ed.items) == 0 {
				return m, nil
			}
			ed.savedName = ed.items[ed.cursor].Name
			ed.savedContent = ed.items[ed.cursor].Content
			ed.editBuf = ed.items[ed.cursor].Name
			ed.isNew = false
			ed.subMode = shortcutModeEditName
		case "n":
			newItems := make([]shortcuts.Shortcut, len(ed.items)+1)
			copy(newItems, ed.items)
			ed.items = newItems
			ed.cursor = len(ed.items) - 1
			ed.savedName = ""
			ed.savedContent = ""
			ed.editBuf = ""
			ed.isNew = true
			ed.subMode = shortcutModeEditName
		case "d":
			if len(ed.items) == 0 {
				return m, nil
			}
			ed.items = append(ed.items[:ed.cursor], ed.items[ed.cursor+1:]...)
			if ed.cursor >= len(ed.items) && ed.cursor > 0 {
				ed.cursor--
			}
			return m, m.saveShortcutsCmd()
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

	case shortcutModeEditName:
		switch msg.String() {
		case "enter":
			ed.items[ed.cursor].Name = ed.editBuf
			ed.editBuf = ed.items[ed.cursor].Content
			ed.subMode = shortcutModeEditContent
		case "esc":
			if ed.isNew {
				ed.items = ed.items[:len(ed.items)-1]
				if ed.cursor >= len(ed.items) && ed.cursor > 0 {
					ed.cursor--
				}
			} else {
				ed.items[ed.cursor].Name = ed.savedName
				ed.items[ed.cursor].Content = ed.savedContent
			}
			ed.subMode = shortcutModeList
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

	case shortcutModeEditContent:
		switch msg.String() {
		case "enter":
			ed.items[ed.cursor].Content = ed.editBuf
			ed.subMode = shortcutModeList
			return m, m.saveShortcutsCmd()
		case "esc":
			ed.items[ed.cursor].Name = ed.savedName
			ed.items[ed.cursor].Content = ed.savedContent
			ed.subMode = shortcutModeList
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

func (m Model) viewShortcutsEditor() string {
	var b strings.Builder
	ed := m.shortcutEd

	switch ed.subMode {
	case shortcutModeList:
		b.WriteString(m.theme.ConfigTitleStyle().Render("Shortcuts"))
		b.WriteString("\n\n")

		if len(ed.items) == 0 {
			b.WriteString(m.theme.ConfigHelpStyle().Render("  (empty — press 'n' to add one)"))
			b.WriteString("\n\n")
			b.WriteString(m.theme.ConfigHelpStyle().Render("  n: new  |  Esc: back"))
			b.WriteString("\n")
			return b.String() + "\n" + m.renderStatusBar()
		}

		for i, sc := range ed.items {
			cursor := "  "
			if i == ed.cursor {
				cursor = m.theme.ConfigCursorStyle().Render("> ")
			}
			name := m.theme.ConfigValueStyle().Render(sc.Name)
			preview := m.theme.ConfigHelpStyle().Render("  " + truncate(sc.Content, 40))
			b.WriteString(cursor + name + preview + "\n")
		}

		b.WriteString("\n")
		b.WriteString(m.theme.ConfigHelpStyle().Render("  ↑↓: navigate  |  Enter: use  |  e: edit  |  n: new  |  d: delete  |  Esc: back"))
		b.WriteString("\n")

	case shortcutModeEditName:
		if len(ed.items) == 0 {
			ed.subMode = shortcutModeList
			break
		}
		b.WriteString(m.theme.ConfigTitleStyle().Render("Shortcuts — edit name"))
		b.WriteString("\n\n")
		sc := ed.items[ed.cursor]
		b.WriteString(m.theme.ConfigLabelStyle().Render("  Name:    ") + m.theme.ConfigEditStyle().Render(ed.editBuf+"\u2588") + "\n")
		b.WriteString(m.theme.ConfigLabelStyle().Render("  Content: ") + m.theme.ConfigValueStyle().Render(sc.Content) + "\n")
		b.WriteString("\n")
		b.WriteString(m.theme.ConfigHelpStyle().Render("  Enter: next field  |  Esc: cancel"))
		b.WriteString("\n")

	case shortcutModeEditContent:
		if len(ed.items) == 0 {
			ed.subMode = shortcutModeList
			break
		}
		b.WriteString(m.theme.ConfigTitleStyle().Render("Shortcuts — edit content"))
		b.WriteString("\n\n")
		sc := ed.items[ed.cursor]
		b.WriteString(m.theme.ConfigLabelStyle().Render("  Name:    ") + m.theme.ConfigValueStyle().Render(sc.Name) + "\n")
		b.WriteString(m.theme.ConfigLabelStyle().Render("  Content: ") + m.theme.ConfigEditStyle().Render(ed.editBuf+"\u2588") + "\n")
		b.WriteString("\n")
		b.WriteString(m.theme.ConfigHelpStyle().Render("  Enter: save  |  Esc: cancel"))
		b.WriteString("\n")
	}

	return b.String() + "\n" + m.renderStatusBar()
}
