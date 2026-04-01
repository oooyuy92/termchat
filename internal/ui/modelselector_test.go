package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/termchat/termchat/internal/config"
)

func TestModelSelectorEnterSwitchesCurrentTabClient(t *testing.T) {
	store := newBrowserTestStore(t)
	m := newBrowserTestModel(t, store)
	m.cfg.API.Provider = "openai"
	m.cfg.API.BaseURL = "https://api.openai.com/v1"
	m.cfg.API.Model = "gpt-4o"
	m.modelRegistry = config.ModelRegistry{
		Providers: []config.ProviderEntry{
			{
				Name:     "anthropic-direct",
				Provider: "anthropic",
				BaseURL:  "https://api.anthropic.com",
				APIKey:   "k",
				Models: []config.ModelEntry{
					{Name: "sonnet", Model: "claude-sonnet-4-20250514"},
				},
			},
		},
	}
	m.mode = modeModelSelector
	m.modelSel = newModelSelectorState(m.modelRegistry, modelSelectorPickForSwitch)

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(Model)
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(Model)

	if got := m.tabs[0].client.Model(); got != "claude-sonnet-4-20250514" {
		t.Fatalf("client model = %q, want claude-sonnet-4-20250514", got)
	}
}
