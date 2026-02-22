package ui

import (
	"fmt"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/termchat/termchat/internal/config"
)

func buildConfigFields(cfg config.Config) []configField {
	return []configField{
		{Label: "API Base URL", Key: "base_url", Value: cfg.API.BaseURL},
		{Label: "API Key", Key: "api_key", Value: cfg.API.APIKey, Masked: true},
		{Label: "Model", Key: "model", Value: cfg.API.Model},
		{Label: "Temperature", Key: "temperature", Value: fmt.Sprintf("%.2f", cfg.Parameters.Temperature)},
		{Label: "Max Tokens", Key: "max_tokens", Value: strconv.Itoa(cfg.Parameters.MaxTokens)},
		{Label: "Reasoning Effort", Key: "reasoning_effort", Value: cfg.Parameters.ReasoningEffort, Options: []string{"", "low", "medium", "high"}},
		{Label: "Theme", Key: "theme", Value: cfg.Settings.Theme, Options: []string{"dark", "light"}},
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
	case "theme":
		v := strings.ToLower(strings.TrimSpace(value))
		if v != "dark" && v != "light" {
			return "must be dark or light"
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
	case "theme":
		cfg.Settings.Theme = strings.ToLower(strings.TrimSpace(value))
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

	// Reset confirmQuit on any key other than ctrl+c
	if msg.String() != "ctrl+c" {
		m.confirmQuit = false
	}

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
			// Sync theme if theme field was changed
			m.theme = ThemeByName(m.cfg.Settings.Theme)
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
		field := &ed.fields[ed.cursor]
		if len(field.Options) > 0 {
			// Cycle through options
			idx := 0
			for i, opt := range field.Options {
				if opt == field.Value {
					idx = i
					break
				}
			}
			idx = (idx + 1) % len(field.Options)
			field.Value = field.Options[idx]
			applyFieldToConfig(&m.cfg, field.Key, field.Value)
			applyConfigToClient(m.client, m.cfg)
			m.theme = ThemeByName(m.cfg.Settings.Theme)
			return m, m.saveConfigCmd()
		}
		ed.editing = true
		ed.editBuf = field.Value
		ed.editErr = ""
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

	b.WriteString(m.theme.ConfigTitleStyle().Render("Model & Parameters Configuration"))
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
		b.WriteString(m.theme.ConfigHelpStyle().Render("  Up/Down: navigate  |  Enter: edit/cycle  |  Esc: back to chat"))
	}
	b.WriteString("\n")

	return b.String() + "\n" + m.renderStatusBar()
}
