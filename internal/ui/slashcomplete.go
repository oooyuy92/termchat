// internal/ui/slashcomplete.go
package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
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
	{"/list", "List saved conversations"},
	{"/load", "Load a conversation"},
	{"/resume", "Browse conversations by date"},
	{"/roles", "Edit role presets"},
	{"/save", "Save conversation"},
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
	switch msg.String() {
	case "esc":
		m.input = ""
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
			m.input = m.slashAC.matches[m.slashAC.cursor].Name
		}
		m.mode = modeChat
		return m, nil

	case "enter":
		// Execute selected command immediately
		if len(m.slashAC.matches) > 0 {
			cmd := m.slashAC.matches[m.slashAC.cursor].Name
			m.input = ""
			m.mode = modeChat
			newModel, teaCmd := m.handleCommand(cmd)
			return newModel.(Model), teaCmd
		}
		m.mode = modeChat
		return m, nil

	case "backspace":
		runes := []rune(m.input)
		if len(runes) > 0 {
			m.input = string(runes[:len(runes)-1])
		}
		if m.input == "" {
			m.mode = modeChat
			return m, nil
		}
		m.slashAC.matches = filterSlashCmds(m.input)
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
		m.input += string(msg.Runes)
		m.slashAC.matches = filterSlashCmds(m.input)
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
	var b strings.Builder

	// Render conversation history (same as chat mode)
	for _, msg := range m.history.Messages() {
		switch msg.Role {
		case "user":
			b.WriteString(m.theme.UserLabelStyle().Render("You:") + "\n")
			b.WriteString(msg.Content + "\n\n")
		case "assistant":
			b.WriteString(m.theme.AssistantLabelStyle().Render(m.client.Model()+":") + "\n")
			rendered, err := m.renderer.Render(msg.Content)
			if err != nil {
				b.WriteString(msg.Content + "\n\n")
			} else {
				b.WriteString(cleanGlamourOutput(rendered) + "\n\n")
			}
		}
	}

	// Dropdown: show up to slashACMaxVisible candidates
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
			b.WriteString(line + "\n")
		} else {
			line := m.theme.ConfigHelpStyle().Render(fmt.Sprintf("  %-12s %s", c.Name, c.Desc))
			b.WriteString(line + "\n")
		}
	}

	// Input line
	b.WriteString(m.theme.InputPromptStyle().Render("> ") + m.input)

	return b.String() + "\n" + m.renderStatusBar()
}
