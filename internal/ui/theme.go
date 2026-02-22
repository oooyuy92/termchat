package ui

import "github.com/charmbracelet/lipgloss"

// Theme holds all color values for a UI theme and provides lipgloss.Style methods.
type Theme struct {
	UserLabel      string
	AssistantLabel string
	ThinkingText   string
	InputPrompt    string
	ErrorText      string
	ConfigTitle    string
	ConfigCursor   string
	ConfigLabel    string
	ConfigValue    string
	ConfigEdit     string
	ConfigHelp     string
	StatusBarBg    string
	StatusBarFg    string
	StatusKeyBg    string
	StatusKeyFg    string
}

var DarkTheme = Theme{
	UserLabel:      "#B8C4B8",
	AssistantLabel: "#C4A8B0",
	ThinkingText:   "#A0A0A0",
	InputPrompt:    "#8A8A8A",
	ErrorText:      "#C4867A",
	ConfigTitle:    "#B8C4B8",
	ConfigCursor:   "#B8C4B8",
	ConfigLabel:    "#A8A8A8",
	ConfigValue:    "#A0B8C4",
	ConfigEdit:     "#C4C0A0",
	ConfigHelp:     "#8A8A8A",
	StatusBarBg:    "#2A2A2A",
	StatusBarFg:    "#9A9A9A",
	StatusKeyBg:    "#3D3D3D",
	StatusKeyFg:    "#A8A8A8",
}

var LightTheme = Theme{
	UserLabel:      "#5A6B5A",
	AssistantLabel: "#7A5A64",
	ThinkingText:   "#787878",
	InputPrompt:    "#8A8A8A",
	ErrorText:      "#8B5A50",
	ConfigTitle:    "#5A6B5A",
	ConfigCursor:   "#5A6B5A",
	ConfigLabel:    "#6A6A6A",
	ConfigValue:    "#506878",
	ConfigEdit:     "#787050",
	ConfigHelp:     "#8A8A8A",
	StatusBarBg:    "#E0DDD8",
	StatusBarFg:    "#6A6A6A",
	StatusKeyBg:    "#D0CCC6",
	StatusKeyFg:    "#5A5A5A",
}

func ThemeByName(name string) Theme {
	switch name {
	case "light":
		return LightTheme
	default:
		return DarkTheme
	}
}

func (t Theme) UserLabelStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(t.UserLabel)).
		Bold(true)
}

func (t Theme) AssistantLabelStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(t.AssistantLabel)).
		Bold(true)
}

func (t Theme) ThinkingStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(t.ThinkingText)).
		Italic(true)
}

func (t Theme) InputPromptStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(t.InputPrompt))
}

func (t Theme) ErrStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(t.ErrorText)).
		Bold(true)
}

func (t Theme) ConfigTitleStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(t.ConfigTitle)).
		Bold(true)
}

func (t Theme) ConfigCursorStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(t.ConfigCursor)).
		Bold(true)
}

func (t Theme) ConfigLabelStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(t.ConfigLabel)).
		Width(20)
}

func (t Theme) ConfigValueStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(t.ConfigValue))
}

func (t Theme) ConfigEditStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(t.ConfigEdit))
}

func (t Theme) ConfigErrStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(t.ErrorText))
}

func (t Theme) ConfigHelpStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(t.ConfigHelp))
}

func (t Theme) StatusBarStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Background(lipgloss.Color(t.StatusBarBg)).
		Foreground(lipgloss.Color(t.StatusBarFg)).
		Padding(0, 1)
}

func (t Theme) StatusKeyStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Background(lipgloss.Color(t.StatusKeyBg)).
		Foreground(lipgloss.Color(t.StatusKeyFg)).
		Padding(0, 1)
}
