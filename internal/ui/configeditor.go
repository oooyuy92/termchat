package ui

import (
	"fmt"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/termchat/termchat/internal/config"
)

var (
	configTitleStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("86"))

	configCursorStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("86")).
		Bold(true)

	configLabelStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("252")).
		Width(20)

	configValueStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("117"))

	configEditStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("229"))

	configErrStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("196"))

	configHelpStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("241"))
)

func buildConfigFields(cfg config.Config) []configField {
	return []configField{
		{Label: "API Base URL", Key: "base_url", Value: cfg.API.BaseURL},
		{Label: "API Key", Key: "api_key", Value: cfg.API.APIKey, Masked: true},
		{Label: "Model", Key: "model", Value: cfg.API.Model},
		{Label: "Temperature", Key: "temperature", Value: fmt.Sprintf("%.2f", cfg.Parameters.Temperature)},
		{Label: "Max Tokens", Key: "max_tokens", Value: strconv.Itoa(cfg.Parameters.MaxTokens)},
		{Label: "Reasoning Effort", Key: "reasoning_effort", Value: cfg.Parameters.ReasoningEffort},
	}
}

func maskValue(s string) string {
	if len(s) <= 4 {
		return strings.Repeat("*", len(s))
	}
	return strings.Repeat("*", len(s)-4) + s[len(s)-4:]
}

func validateField(key, value string) string {
	switch key {
	case "base_url":
		if value == "" {
			return "must not be empty"
		}
		if !strings.HasPrefix(value, "http://") && !strings.HasPrefix(value, "https://") {
			return "must start with http:// or https://"
		}
	case "model":
		if value == "" {
			return "must not be empty"
		}
	case "temperature":
		f, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return "must be a number (e.g., 0.7)"
		}
		if f < 0 || f > 2 {
			return "must be between 0.0 and 2.0"
		}
	case "max_tokens":
		n, err := strconv.Atoi(value)
		if err != nil || n <= 0 {
			return "must be a positive integer"
		}
	case "reasoning_effort":
		v := strings.ToLower(strings.TrimSpace(value))
		if v != "" && v != "low" && v != "medium" && v != "high" {
			return "must be low, medium, high, or empty"
		}
	}
	return ""
}

func applyFieldToConfig(cfg *config.Config, key, value string) {
	switch key {
	case "base_url":
		cfg.API.BaseURL = value
	case "api_key":
		cfg.API.APIKey = value
	case "model":
		cfg.API.Model = value
	case "temperature":
		f, _ := strconv.ParseFloat(value, 64)
		cfg.Parameters.Temperature = f
	case "max_tokens":
		n, _ := strconv.Atoi(value)
		cfg.Parameters.MaxTokens = n
	case "reasoning_effort":
		cfg.Parameters.ReasoningEffort = strings.ToLower(strings.TrimSpace(value))
	}
}

func applyConfigToClient(client interface {
	SetBaseURL(string)
	SetAPIKey(string)
	SetModel(string)
}, cfg config.Config) {
	client.SetBaseURL(cfg.API.BaseURL)
	client.SetAPIKey(cfg.API.APIKey)
	client.SetModel(cfg.API.Model)
}

func (m Model) updateConfigMode(msg tea.KeyMsg) (Model, tea.Cmd) {
	ed := &m.configEd

	if ed.editing {
		switch msg.String() {
		case "enter":
			field := &ed.fields[ed.cursor]
			errMsg := validateField(field.Key, ed.editBuf)
			if errMsg != "" {
				ed.editErr = errMsg
				return m, nil
			}
			field.Value = ed.editBuf
			applyFieldToConfig(&m.cfg, field.Key, ed.editBuf)
			applyConfigToClient(m.client, m.cfg)
			ed.editing = false
			ed.editErr = ""
			return m, m.saveConfigCmd()

		case "esc":
			ed.editing = false
			ed.editErr = ""
			return m, nil

		case "backspace":
			runes := []rune(ed.editBuf)
			if len(runes) > 0 {
				ed.editBuf = string(runes[:len(runes)-1])
			}
			ed.editErr = ""
			return m, nil

		default:
			if msg.Type == tea.KeyRunes {
				ed.editBuf += string(msg.Runes)
				ed.editErr = ""
			}
			return m, nil
		}
	}

	// Non-editing mode
	switch msg.String() {
	case "up", "k":
		if ed.cursor > 0 {
			ed.cursor--
		}
	case "down", "j":
		if ed.cursor < len(ed.fields)-1 {
			ed.cursor++
		}
	case "enter":
		field := ed.fields[ed.cursor]
		ed.editing = true
		ed.editBuf = field.Value
		ed.editErr = ""
	case "esc":
		m.mode = modeChat
		m.statusMsg = "Back to chat"
	case "ctrl+c":
		return m, tea.Quit
	}
	return m, nil
}

func (m Model) saveConfigCmd() tea.Cmd {
	cfg := m.cfg
	path := m.cfgPath
	return func() tea.Msg {
		err := config.Save(path, cfg)
		return configSavedMsg{Err: err}
	}
}

func (m Model) viewConfigEditor() string {
	var b strings.Builder
	ed := m.configEd

	b.WriteString(configTitleStyle.Render("Model & Parameters Configuration"))
	b.WriteString("\n\n")

	for i, field := range ed.fields {
		cursor := "  "
		if i == ed.cursor {
			cursor = configCursorStyle.Render("> ")
		}

		label := configLabelStyle.Render(field.Label + ":")

		var value string
		if ed.editing && i == ed.cursor {
			value = configEditStyle.Render(ed.editBuf + "\u2588")
		} else {
			displayVal := field.Value
			if displayVal == "" {
				displayVal = "(not set)"
			} else if field.Masked {
				displayVal = maskValue(displayVal)
			}
			value = configValueStyle.Render(displayVal)
		}

		b.WriteString(cursor + label + value + "\n")
	}

	b.WriteString("\n")

	if ed.editErr != "" {
		b.WriteString(configErrStyle.Render("  Error: "+ed.editErr) + "\n\n")
	}

	if ed.editing {
		b.WriteString(configHelpStyle.Render("  Enter: confirm  |  Esc: cancel"))
	} else {
		b.WriteString(configHelpStyle.Render("  Up/Down: navigate  |  Enter: edit  |  Esc: back to chat"))
	}
	b.WriteString("\n")

	return b.String() + "\n" + m.renderStatusBar()
}
