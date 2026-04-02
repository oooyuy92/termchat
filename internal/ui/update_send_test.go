package ui

import (
	"context"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/termchat/termchat/internal/chat"
	"github.com/termchat/termchat/internal/config"
)

func TestSendStreamCmdUsesBoundModelParameters(t *testing.T) {
	store := newBrowserTestStore(t)
	m := newBrowserTestModel(t, store)
	provider := &streamingStubProvider{
		stubProvider: stubProvider{model: "gemini-3-flash-preview"},
		response:     "ok",
	}

	m.modelRegistry = config.ModelRegistry{
		Providers: []config.ProviderEntry{
			{
				Name:    "gateway",
				BaseURL: "https://example.test/v1",
				APIKey:  "secret",
				Models: []config.ModelEntry{
					{
						Name:            "flash",
						Model:           "gemini-3-flash-preview",
						APIFormat:       "openai-compatible",
						Temperature:     0.25,
						MaxTokens:       8192,
						ReasoningEffort: "medium",
						BudgetTokens:    1024,
					},
				},
			},
		},
	}
	m.tabs[0].client = provider
	m.tabs[0].providerConfigName = "gateway"
	m.tabs[0].modelConfigName = "flash"
	m.tabs[0].history.Add(chat.Message{Seq: 1, Role: "user", Content: "hello"})

	cmd := m.sendStreamCmd(context.Background(), 0)
	if cmd == nil {
		t.Fatal("sendStreamCmd() returned nil")
	}
	_ = cmd()

	if provider.temperature != 0.25 {
		t.Fatalf("temperature = %f, want 0.25", provider.temperature)
	}
	if provider.maxTokens != 8192 {
		t.Fatalf("maxTokens = %d, want 8192", provider.maxTokens)
	}
	if provider.reasoningEffort != "medium" {
		t.Fatalf("reasoningEffort = %q, want medium", provider.reasoningEffort)
	}
	if provider.budgetTokens != 1024 {
		t.Fatalf("budgetTokens = %d, want 1024", provider.budgetTokens)
	}
}

func TestChatEnterWithoutRegistryBindingShowsErrorAndDoesNotPersistMessage(t *testing.T) {
	store := newBrowserTestStore(t)
	m := newBrowserTestModel(t, store)
	m.modelRegistry = config.ModelRegistry{
		Providers: []config.ProviderEntry{
			{
				Name:    "gateway",
				BaseURL: "https://example.test/v1",
				APIKey:  "secret",
				Models: []config.ModelEntry{
					{Name: "flash", Model: "gemini-3-flash-preview", APIFormat: "openai-compatible", Temperature: 0.25, MaxTokens: 8192},
				},
			},
		},
	}
	m.tabs[0].autoSaveName = "conv"
	m.tabs[0].providerConfigName = "gateway"
	m.tabs[0].modelConfigName = ""
	m.tabs[0].apiFormat = "openai-compatible"
	m.tabs[0].textarea.SetValue("hello")

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(Model)

	if got := m.statusMsg; got != "Send failed: current session model is not selected" {
		t.Fatalf("statusMsg = %q, want missing model binding error", got)
	}
	if got := m.tabs[0].textarea.Value(); got != "hello" {
		t.Fatalf("textarea = %q, want original input preserved", got)
	}
	if got := len(m.tabs[0].history.Messages()); got != 0 {
		t.Fatalf("history messages len = %d, want 0", got)
	}
	list, err := store.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if got := len(list); got != 0 {
		t.Fatalf("stored conversations len = %d, want 0", got)
	}
}
