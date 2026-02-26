// internal/ui/onboard.go
package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/termchat/termchat/internal/chat"
)

// updateOnboardMode handles key events in onboarding mode.
// It delegates field editing to updateConfigMode, but intercepts non-editing
// Esc to save the current config (even defaults) and enter chat.
func (m Model) updateOnboardMode(msg tea.KeyMsg) (Model, tea.Cmd) {
	// Non-editing Esc: save whatever we have and enter chat.
	if msg.String() == "esc" && !m.configEd.editing {
		m.mode = modeChat
		m.statusMsg = "Ready! Type a message to start chatting."
		m.activeTabSession().client = chat.NewProvider(m.cfg.API.Provider, m.cfg.API.BaseURL, m.cfg.API.APIKey, m.cfg.API.Model)
		return m, m.saveConfigCmd()
	}

	// Ctrl+C: double-quit pattern (same as other modes).
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

	// Delegate all other keys to the config editor logic.
	// updateConfigMode's own "esc" handler (non-editing) sets m.mode = modeChat,
	// but we've already handled that case above. In editing mode, esc cancels
	// the edit — that's correct for onboarding too.
	return m.updateConfigMode(msg)
}

// viewOnboard renders the first-run onboarding wizard.
func (m Model) viewOnboard() string {
	var b strings.Builder
	ed := m.configEd

	b.WriteString(m.theme.ConfigTitleStyle().Render("Welcome to termchat!"))
	b.WriteString("\n")
	b.WriteString(m.theme.ConfigHelpStyle().Render("Configure your API access to get started."))
	b.WriteString("\n\n")

	for i, field := range ed.fields {
		cursor := "  "
		if i == ed.cursor {
			cursor = m.theme.ConfigCursorStyle().Render("> ")
		}

		label := m.theme.ConfigLabelStyle().Render(field.Label + ":")

		var value string
		if ed.editing && i == ed.cursor {
			value = m.theme.ConfigEditStyle().Render(ed.editBuf + "\u2588")
		} else {
			displayVal := field.Value
			if displayVal == "" {
				displayVal = "(not set)"
			} else if field.Masked {
				displayVal = maskValue(displayVal)
			}
			value = m.theme.ConfigValueStyle().Render(displayVal)
			if len(field.Options) > 0 {
				optStrs := make([]string, len(field.Options))
				for j, opt := range field.Options {
					if opt == "" {
						optStrs[j] = "(not set)"
					} else {
						optStrs[j] = opt
					}
				}
				value += "  [" + strings.Join(optStrs, " | ") + "]"
			}
		}

		b.WriteString(cursor + label + value + "\n")
	}

	b.WriteString("\n")

	if ed.editErr != "" {
		b.WriteString(m.theme.ConfigErrStyle().Render("  Error: "+ed.editErr) + "\n\n")
	}

	if ed.editing {
		b.WriteString(m.theme.ConfigHelpStyle().Render("  Enter: confirm  |  Esc: cancel"))
	} else {
		b.WriteString(m.theme.ConfigHelpStyle().Render("  ↑↓: navigate  |  Enter: edit/cycle  |  Esc: skip and start chatting"))
	}
	b.WriteString("\n")

	return b.String() + "\n" + m.renderStatusBar()
}
