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
				Name:    "gateway",
				BaseURL: "https://example.test/v1",
				Models: []config.ModelEntry{
					{Name: "flash", Model: "gemini-3-flash-preview", APIFormat: "gemini"},
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
	if !strings.Contains(out, "Providers") {
		t.Fatalf("View() missing providers header: %q", out)
	}
	if !strings.Contains(out, "Models") {
		t.Fatalf("View() missing models header: %q", out)
	}
	if !strings.Contains(out, "gateway") {
		t.Fatalf("View() missing provider name: %q", out)
	}
	if !strings.Contains(out, "gemini-3-flash-preview") {
		t.Fatalf("View() missing model name: %q", out)
	}
	if strings.Contains(out, "flash (") {
		t.Fatalf("View() unexpectedly shows separate model alias: %q", out)
	}
}

func TestViewModelSelectorShowsEmptyModelPaneForSelectedProvider(t *testing.T) {
	reg := config.ModelRegistry{
		Providers: []config.ProviderEntry{
			{
				Name:    "gateway",
				BaseURL: "https://example.test/v1",
				Models:  nil,
			},
		},
	}

	m := Model{
		theme:         DarkTheme,
		modelRegistry: reg,
		modelSel: modelSelectorState{
			providers:       reg.Providers,
			selectingModels: true,
		},
		mode:  modeModelSelector,
		width: 80,
	}

	out := m.View()
	if !strings.Contains(out, "No models. Press n to add one.") {
		t.Fatalf("View() missing empty models state: %q", out)
	}
}
