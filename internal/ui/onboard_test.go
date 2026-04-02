package ui

import (
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/termchat/termchat/internal/config"
)

func TestOnboardEnterOpensModelSelectorManage(t *testing.T) {
	store := newBrowserTestStore(t)
	m := newBrowserTestModel(t, store)
	m.mode = modeOnboard

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(Model)

	if m.mode != modeModelSelector {
		t.Fatalf("mode = %v, want modeModelSelector", m.mode)
	}
	if m.modelSel.purpose != modelSelectorManage {
		t.Fatalf("purpose = %v, want modelSelectorManage", m.modelSel.purpose)
	}
}

func TestViewOnboardExplainsModelRegistrySetup(t *testing.T) {
	store := newBrowserTestStore(t)
	m := newBrowserTestModel(t, store)
	m.mode = modeOnboard

	out := m.View()
	if !strings.Contains(out, "Model") {
		t.Fatalf("View() missing model setup text: %q", out)
	}
	if strings.Contains(out, "API Base URL") {
		t.Fatalf("View() unexpectedly shows legacy API field: %q", out)
	}
}

func TestOnboardSkipReturnsToChat(t *testing.T) {
	store := newBrowserTestStore(t)
	m := newBrowserTestModel(t, store)
	m.mode = modeOnboard

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})
	m = next.(Model)

	if m.mode != modeChat {
		t.Fatalf("mode = %v, want modeChat", m.mode)
	}
	if got := m.statusMsg; got != "Skipped setup. Use /model to configure provider and model." {
		t.Fatalf("statusMsg = %q, want skipped setup message", got)
	}
}

func TestNewModelShowsOnboardWhenModelRegistryHasNoProviders(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	cfg := config.DefaultConfig()
	if err := config.Save(cfgPath, cfg); err != nil {
		t.Fatalf("config.Save() error = %v", err)
	}
	modelsPath := config.ModelRegistryPath(cfgPath)
	if err := config.SaveModelRegistry(modelsPath, config.ModelRegistry{}); err != nil {
		t.Fatalf("SaveModelRegistry() error = %v", err)
	}

	m, err := NewModel(cfg, cfgPath, false, &stubProvider{model: "test-model"})
	if err != nil {
		t.Fatalf("NewModel() error = %v", err)
	}
	defer m.Close()

	if m.mode != modeOnboard {
		t.Fatalf("mode = %v, want modeOnboard", m.mode)
	}
}

func TestSlashModelWorksWithoutCurrentModelBinding(t *testing.T) {
	store := newBrowserTestStore(t)
	m := newBrowserTestModel(t, store)
	m.tabs[0].providerConfigName = "openai-compatible"
	m.tabs[0].modelConfigName = ""
	m.tabs[0].textarea.SetValue("/model")

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(Model)

	if m.mode != modeModelSelector {
		t.Fatalf("mode = %v, want modeModelSelector", m.mode)
	}
	if got := m.tabs[0].textarea.Value(); got != "" {
		t.Fatalf("textarea = %q, want empty after command execution", got)
	}
	if strings.Contains(m.statusMsg, "Send failed") {
		t.Fatalf("statusMsg = %q, want no send failure", m.statusMsg)
	}
}
