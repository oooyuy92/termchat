package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/termchat/termchat/internal/chat"
	"github.com/termchat/termchat/internal/config"
)

func TestResumeConversationRestoresStoredModelBinding(t *testing.T) {
	store := newBrowserTestStore(t)
	m := newBrowserTestModel(t, store)
	m.modelRegistry = config.ModelRegistry{
		Providers: []config.ProviderEntry{
			{
				Name:    "gateway",
				BaseURL: "https://example.test/v1",
				APIKey:  "secret",
				Models: []config.ModelEntry{
					{Name: "flash", Model: "gemini-3-flash-preview", APIFormat: "gemini", Temperature: 0.2, MaxTokens: 4096},
				},
			},
		},
	}

	if err := store.Save("conv", []chat.Message{{Role: "user", Content: "hello"}}); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	if err := store.SetConversationModelBinding("conv", "gateway", "flash"); err != nil {
		t.Fatalf("SetConversationModelBinding() error = %v", err)
	}

	convs, err := store.ListWithDate()
	if err != nil {
		t.Fatalf("ListWithDate() error = %v", err)
	}
	m.resumePick = buildResumePicker(convs)
	m.mode = modeResume

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(Model)

	if got := m.tabs[0].providerConfigName; got != "gateway" {
		t.Fatalf("providerConfigName = %q, want gateway", got)
	}
	if got := m.tabs[0].modelConfigName; got != "flash" {
		t.Fatalf("modelConfigName = %q, want flash", got)
	}
	if got := m.tabs[0].client.Model(); got != "gemini-3-flash-preview" {
		t.Fatalf("client model = %q, want gemini-3-flash-preview", got)
	}
}
