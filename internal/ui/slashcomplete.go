// internal/ui/slashcomplete.go
package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type slashCmd struct {
	Name string // e.g. "/resume"
	Desc string // e.g. "Browse conversations"
}

type slashComplete struct {
	matches []slashCmd
	cursor  int
	offset  int // first visible item index (viewport)
}

// slashCmds is the canonical list of all slash commands with descriptions.
var slashCmds = []slashCmd{
	{"/clear", "Clear conversation"},
	{"/exit", "Quit termchat"},
	{"/history", "Conversation history"},
	{"/roles", "Edit role presets"},
	{"/settings", "Model & parameters"},
	{"/shortcuts", "Edit keyboard shortcuts"},
}

// filterSlashCmds returns commands whose Name has input as a prefix.
// input must start with "/".
func filterSlashCmds(input string) []slashCmd {
	var out []slashCmd
	for _, c := range slashCmds {
		if strings.HasPrefix(c.Name, input) {
			out = append(out, c)
		}
	}
	return out
}

const slashACMaxVisible = 5

// clampSlashAC adjusts cursor and offset so cursor stays within the visible window.
func (ac *slashComplete) clampSlashAC() {
	n := len(ac.matches)
	if n == 0 {
		ac.cursor = 0
		ac.offset = 0
		return
	}
	// Wrap cursor
	if ac.cursor < 0 {
		ac.cursor = n - 1
	}
	if ac.cursor >= n {
		ac.cursor = 0
	}
	// Adjust viewport
	if ac.cursor < ac.offset {
		ac.offset = ac.cursor
	}
	if ac.cursor >= ac.offset+slashACMaxVisible {
		ac.offset = ac.cursor - slashACMaxVisible + 1
	}
	if ac.offset < 0 {
		ac.offset = 0
	}
}

func (m Model) updateSlashComplete(msg tea.KeyMsg) (Model, tea.Cmd) {
	// Reset confirmQuit on any key other than ctrl+c (same pattern as Update())
	if msg.String() != "ctrl+c" {
		m.confirmQuit = false
	}

	switch msg.String() {
	case "esc":
		m.tabs[m.activeTab].textarea.SetValue("")
		m.mode = modeChat
		return m, nil

	case "ctrl+c":
		if m.confirmQuit {
			return m, tea.Quit
		}
		m.confirmQuit = true
		m.statusMsg = "Press Ctrl+C again to quit"
		return m, nil

	case "up":
		m.slashAC.cursor--
		m.slashAC.clampSlashAC()
		return m, nil

	case "down":
		m.slashAC.cursor++
		m.slashAC.clampSlashAC()
		return m, nil

	case "tab":
		// Fill selected command name into input, return to chat to edit args
		if len(m.slashAC.matches) > 0 {
			m.tabs[m.activeTab].textarea.SetValue(m.slashAC.matches[m.slashAC.cursor].Name)
		}
		m.mode = modeChat
		return m, nil

	case "enter":
		// Execute selected command immediately
		if len(m.slashAC.matches) > 0 {
			cmd := m.slashAC.matches[m.slashAC.cursor].Name
			m.tabs[m.activeTab].textarea.SetValue("")
			m.mode = modeChat
			newModel, teaCmd := m.handleCommand(cmd)
			if updated, ok := newModel.(Model); ok {
				return updated, teaCmd
			}
			return m, teaCmd
		}
		m.mode = modeChat
		return m, nil

	case "backspace":
		val := m.tabs[m.activeTab].textarea.Value()
		runes := []rune(val)
		if len(runes) > 0 {
			m.tabs[m.activeTab].textarea.SetValue(string(runes[:len(runes)-1]))
		}
		if m.tabs[m.activeTab].textarea.Value() == "" {
			m.mode = modeChat
			return m, nil
		}
		m.slashAC.matches = filterSlashCmds(m.tabs[m.activeTab].textarea.Value())
		if len(m.slashAC.matches) == 0 {
			m.mode = modeChat
			return m, nil
		}
		if m.slashAC.cursor >= len(m.slashAC.matches) {
			m.slashAC.cursor = 0
		}
		m.slashAC.clampSlashAC()
		return m, nil

	default:
		if msg.Type != tea.KeyRunes {
			return m, nil
		}
		m.tabs[m.activeTab].textarea.SetValue(m.tabs[m.activeTab].textarea.Value() + string(msg.Runes))
		m.slashAC.matches = filterSlashCmds(m.tabs[m.activeTab].textarea.Value())
		if len(m.slashAC.matches) == 0 {
			// No matches — fall back to chat mode
			m.mode = modeChat
			return m, nil
		}
		if m.slashAC.cursor >= len(m.slashAC.matches) {
			m.slashAC.cursor = 0
		}
		m.slashAC.clampSlashAC()
		return m, nil
	}
}

func (m Model) viewSlashComplete() string {
	statusBar := m.renderStatusBar()
	inputArea := m.tabs[m.activeTab].textarea.View()

	// Build dropdown lines
	var dropdownLines []string
	ac := m.slashAC
	end := ac.offset + slashACMaxVisible
	if end > len(ac.matches) {
		end = len(ac.matches)
	}
	for i := ac.offset; i < end; i++ {
		c := ac.matches[i]
		if i == ac.cursor {
			prefix := m.theme.ConfigCursorStyle().Render("> ")
			line := prefix + m.theme.ConfigLabelStyle().Render(fmt.Sprintf("%-12s", c.Name)) + " " + m.theme.ConfigHelpStyle().Render(c.Desc)
			dropdownLines = append(dropdownLines, line)
		} else {
			line := m.theme.ConfigHelpStyle().Render(fmt.Sprintf("  %-12s %s", c.Name, c.Desc))
			dropdownLines = append(dropdownLines, line)
		}
	}
	dropdown := strings.Join(dropdownLines, "\n")

	// Dynamic viewport height — same calculation as View()
	vpHeight := m.height - lipgloss.Height(statusBar) - lipgloss.Height(inputArea)
	if dropdown != "" {
		vpHeight -= lipgloss.Height(dropdown)
	}
	if vpHeight < 1 {
		vpHeight = 1
	}
	tab := &m.tabs[m.activeTab]
	if tab.viewport.Height != vpHeight {
		tab.viewport.Height = vpHeight
		if tab.chatFollowBottom {
			tab.viewport.GotoBottom()
		}
	}

	parts := []string{tab.viewport.View()}
	if dropdown != "" {
		parts = append(parts, dropdown)
	}
	parts = append(parts, inputArea, statusBar)
	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}
