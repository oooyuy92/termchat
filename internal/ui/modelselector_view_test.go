package ui

import (
	"strings"
	"testing"

	"github.com/termchat/termchat/internal/config"
)

func TestViewModelSelectorShowsProviderAndModels(t *testing.T) {
	reg := config.ModelRegistry{
		Providers: []config.ProviderEntry{
			{
				Name:     "gateway",
				Provider: "openai-compatible",
				BaseURL:  "https://example.test/v1",
				Models: []config.ModelEntry{
					{Name: "flash", Model: "gemini-3-flash-preview"},
				},
			},
		},
	}

	m := Model{
		theme:         DarkTheme,
		modelRegistry: reg,
		modelSel:      newModelSelectorState(reg, modelSelectorManage),
		mode:          modeModelSelector,
		width:         80,
	}

	out := m.View()
	if !strings.Contains(out, "gateway") {
		t.Fatalf("View() missing provider name: %q", out)
	}
	if !strings.Contains(out, "gemini-3-flash-preview") {
		t.Fatalf("View() missing model name: %q", out)
	}
}
