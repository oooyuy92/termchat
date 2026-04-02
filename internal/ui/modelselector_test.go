package ui

import (
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/termchat/termchat/internal/chat"
	"github.com/termchat/termchat/internal/config"
)

func saveModelSelectorEditorWithEnter(t *testing.T, m Model, lastField int) Model {
	t.Helper()
	var next tea.Model
	for i := 0; i < lastField; i++ {
		next, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
		m = next.(Model)
	}
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(Model)
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(Model)
	return m
}

func TestModelSelectorEnterSwitchesCurrentTabClient(t *testing.T) {
	store := newBrowserTestStore(t)
	m := newBrowserTestModel(t, store)
	m.cfg.API.Provider = "openai"
	m.cfg.API.BaseURL = "https://api.openai.com/v1"
	m.cfg.API.Model = "gpt-4o"
	var gotAPIFormat string
	m.providerFactory = func(apiFormat, baseURL, apiKey, model string) chat.Provider {
		gotAPIFormat = apiFormat
		return &stubProvider{model: model}
	}
	m.modelRegistry = config.ModelRegistry{
		Providers: []config.ProviderEntry{
			{
				Name:     "anthropic-direct",
				BaseURL:  "https://api.anthropic.com",
				APIKey:   "k",
				Models: []config.ModelEntry{
					{Name: "sonnet", Model: "claude-sonnet-4-20250514", APIFormat: "anthropic", Temperature: 0.2, MaxTokens: 8192},
				},
			},
		},
	}
	m.mode = modeModelSelector
	m.modelSel = newModelSelectorState(m.modelRegistry, modelSelectorPickForSwitch)

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRight})
	m = next.(Model)
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(Model)

	if got := m.tabs[0].client.Model(); got != "claude-sonnet-4-20250514" {
		t.Fatalf("client model = %q, want claude-sonnet-4-20250514", got)
	}
	if gotAPIFormat != "anthropic" {
		t.Fatalf("apiFormat = %q, want anthropic", gotAPIFormat)
	}
}

func TestModelSelectorRightAndLeftSwitchFocusBetweenColumns(t *testing.T) {
	store := newBrowserTestStore(t)
	m := newBrowserTestModel(t, store)
	m.modelRegistry = config.ModelRegistry{
		Providers: []config.ProviderEntry{
			{
				Name: "gateway",
				Models: []config.ModelEntry{
					{Name: "flash", Model: "gemini-3-flash-preview", APIFormat: "gemini"},
				},
			},
		},
	}
	m.mode = modeModelSelector
	m.modelSel = newModelSelectorState(m.modelRegistry, modelSelectorManage)

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRight})
	m = next.(Model)
	if !m.modelSel.selectingModels {
		t.Fatalf("selectingModels = false, want true after right")
	}

	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyLeft})
	m = next.(Model)
	if m.modelSel.selectingModels {
		t.Fatalf("selectingModels = true, want false after left")
	}
}

func TestModelSelectorDownMovesInsideFocusedModelColumn(t *testing.T) {
	store := newBrowserTestStore(t)
	m := newBrowserTestModel(t, store)
	m.modelRegistry = config.ModelRegistry{
		Providers: []config.ProviderEntry{
			{
				Name: "gateway",
				Models: []config.ModelEntry{
					{Name: "flash", Model: "gemini-3-flash-preview", APIFormat: "gemini"},
					{Name: "pro", Model: "gemini-3.1-pro-preview", APIFormat: "gemini"},
				},
			},
			{
				Name: "anthropic-direct",
				Models: []config.ModelEntry{
					{Name: "sonnet", Model: "claude-sonnet-4", APIFormat: "anthropic"},
				},
			},
		},
	}
	m.mode = modeModelSelector
	m.modelSel = newModelSelectorState(m.modelRegistry, modelSelectorManage)

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRight})
	m = next.(Model)
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = next.(Model)

	if got := m.modelSel.providerCursor; got != 0 {
		t.Fatalf("providerCursor = %d, want 0", got)
	}
	if got := m.modelSel.modelCursor; got != 1 {
		t.Fatalf("modelCursor = %d, want 1", got)
	}
}

func TestModelSelectorNAddsProviderAndPersistsRegistry(t *testing.T) {
	store := newBrowserTestStore(t)
	m := newBrowserTestModel(t, store)
	m.modelRegistryPath = filepath.Join(t.TempDir(), "models.yaml")
	m.mode = modeModelSelector
	m.modelSel = newModelSelectorState(config.ModelRegistry{}, modelSelectorManage)

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	m = next.(Model)
	if !m.modelSel.editing {
		t.Fatal("editing = false, want true after n")
	}
	if got := len(m.modelSel.providers); got != 1 {
		t.Fatalf("providers len after n = %d, want 1", got)
	}
	reg, _, err := config.LoadModelRegistryOrDefault(m.modelRegistryPath)
	if err != nil {
		t.Fatalf("LoadModelRegistryOrDefault() after n error = %v", err)
	}
	if got := len(reg.Providers); got != 1 {
		t.Fatalf("saved providers len after n = %d, want 1", got)
	}

	m = saveModelSelectorEditorWithEnter(t, m, 2)

	if len(m.modelSel.providers) != 1 {
		t.Fatalf("providers len = %d, want 1", len(m.modelSel.providers))
	}
	if !m.modelSel.editing {
		t.Fatalf("editing = false, want true after saving provider field")
	}
	if m.modelSel.selectingModels {
		t.Fatalf("selectingModels = true, want false while still editing provider")
	}
	if got := m.modelSel.providers[0].BaseURL; got != m.cfg.API.BaseURL {
		t.Fatalf("provider base_url = %q, want %q", got, m.cfg.API.BaseURL)
	}
	reg, _, err = config.LoadModelRegistryOrDefault(m.modelRegistryPath)
	if err != nil {
		t.Fatalf("LoadModelRegistryOrDefault() error = %v", err)
	}
	if len(reg.Providers) != 1 {
		t.Fatalf("saved providers len = %d, want 1", len(reg.Providers))
	}
	if got := reg.Providers[0].BaseURL; got != m.cfg.API.BaseURL {
		t.Fatalf("saved provider base_url = %q, want %q", got, m.cfg.API.BaseURL)
	}
}

func TestModelSelectorProviderEscKeepsNewlyCreatedProvider(t *testing.T) {
	store := newBrowserTestStore(t)
	m := newBrowserTestModel(t, store)
	m.modelRegistryPath = filepath.Join(t.TempDir(), "models.yaml")
	m.mode = modeModelSelector
	m.modelSel = newModelSelectorState(config.ModelRegistry{}, modelSelectorManage)

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	m = next.(Model)
	if !m.modelSel.editing {
		t.Fatal("editing = false, want true after n")
	}
	if got := len(m.modelSel.providers); got != 1 {
		t.Fatalf("providers len after n = %d, want 1", got)
	}

	m.modelSel.editor.fields[0].Value = "custom-provider"
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = next.(Model)

	if m.modelSel.editing {
		t.Fatal("editing = true, want false after esc")
	}
	if got := len(m.modelSel.providers); got != 1 {
		t.Fatalf("providers len after esc = %d, want 1", got)
	}
	if got := m.modelSel.providers[0].Name; got != "provider-1" {
		t.Fatalf("provider name after esc = %q, want provider-1", got)
	}
	reg, _, err := config.LoadModelRegistryOrDefault(m.modelRegistryPath)
	if err != nil {
		t.Fatalf("LoadModelRegistryOrDefault() error = %v", err)
	}
	if got := len(reg.Providers); got != 1 {
		t.Fatalf("saved providers len after esc = %d, want 1", got)
	}
	if got := reg.Providers[0].Name; got != "provider-1" {
		t.Fatalf("saved provider name after esc = %q, want provider-1", got)
	}
}

func TestModelSelectorNAddsModelAndPersistsRegistry(t *testing.T) {
	store := newBrowserTestStore(t)
	m := newBrowserTestModel(t, store)
	m.modelRegistryPath = filepath.Join(t.TempDir(), "models.yaml")
	m.modelRegistry = config.ModelRegistry{
		Providers: []config.ProviderEntry{
			{
				Name:    "gateway",
				BaseURL: "https://example.test/v1",
				APIKey:  "secret",
			},
		},
	}
	m.mode = modeModelSelector
	m.modelSel = newModelSelectorState(m.modelRegistry, modelSelectorManage)

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRight})
	m = next.(Model)
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	m = next.(Model)
	if !m.modelSel.editing {
		t.Fatal("editing = false, want true after n")
	}
	if got := len(m.modelSel.providers[0].Models); got != 1 {
		t.Fatalf("models len after n = %d, want 1", got)
	}
	reg, _, err := config.LoadModelRegistryOrDefault(m.modelRegistryPath)
	if err != nil {
		t.Fatalf("LoadModelRegistryOrDefault() after n error = %v", err)
	}
	if got := len(reg.Providers[0].Models); got != 1 {
		t.Fatalf("saved models len after n = %d, want 1", got)
	}

	m = saveModelSelectorEditorWithEnter(t, m, 6)

	if got := len(m.modelSel.providers[0].Models); got != 1 {
		t.Fatalf("models len = %d, want 1", got)
	}
	reg, _, err = config.LoadModelRegistryOrDefault(m.modelRegistryPath)
	if err != nil {
		t.Fatalf("LoadModelRegistryOrDefault() error = %v", err)
	}
	if got := len(reg.Providers[0].Models); got != 1 {
		t.Fatalf("saved models len = %d, want 1", got)
	}
}

func TestModelSelectorEscKeepsNewlyCreatedModel(t *testing.T) {
	store := newBrowserTestStore(t)
	m := newBrowserTestModel(t, store)
	m.modelRegistryPath = filepath.Join(t.TempDir(), "models.yaml")
	m.modelRegistry = config.ModelRegistry{
		Providers: []config.ProviderEntry{
			{
				Name:    "gateway",
				BaseURL: "https://example.test/v1",
				APIKey:  "secret",
			},
		},
	}
	if err := config.SaveModelRegistry(m.modelRegistryPath, m.modelRegistry); err != nil {
		t.Fatalf("SaveModelRegistry() error = %v", err)
	}
	m.mode = modeModelSelector
	m.modelSel = newModelSelectorState(m.modelRegistry, modelSelectorManage)

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRight})
	m = next.(Model)
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	m = next.(Model)
	if !m.modelSel.editing {
		t.Fatal("editing = false, want true after n")
	}
	if got := len(m.modelSel.providers[0].Models); got != 1 {
		t.Fatalf("models len after n = %d, want 1", got)
	}

	m.modelSel.editor.fields[0].Value = "custom-model"
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = next.(Model)

	if m.modelSel.editing {
		t.Fatal("editing = true, want false after esc")
	}
	if got := len(m.modelSel.providers[0].Models); got != 1 {
		t.Fatalf("models len after esc = %d, want 1", got)
	}
	if got := m.modelSel.providers[0].Models[0].Model; got != "model-1" {
		t.Fatalf("model id after esc = %q, want original default model-1", got)
	}
	reg, _, err := config.LoadModelRegistryOrDefault(m.modelRegistryPath)
	if err != nil {
		t.Fatalf("LoadModelRegistryOrDefault() error = %v", err)
	}
	if got := len(reg.Providers[0].Models); got != 1 {
		t.Fatalf("saved models len after esc = %d, want 1", got)
	}
	if got := reg.Providers[0].Models[0].Model; got != "model-1" {
		t.Fatalf("saved model id after esc = %q, want model-1", got)
	}
}

func TestModelSelectorEnterSavesFieldBeforeEsc(t *testing.T) {
	store := newBrowserTestStore(t)
	m := newBrowserTestModel(t, store)
	m.modelRegistryPath = filepath.Join(t.TempDir(), "models.yaml")
	m.modelRegistry = config.ModelRegistry{
		Providers: []config.ProviderEntry{
			{
				Name:    "gateway",
				BaseURL: "https://example.test/v1",
				APIKey:  "secret",
				Models: []config.ModelEntry{
					{Name: "model-1", Model: "model-1", APIFormat: "openai-compatible", Temperature: 0.20, MaxTokens: 4096},
				},
			},
		},
	}
	if err := config.SaveModelRegistry(m.modelRegistryPath, m.modelRegistry); err != nil {
		t.Fatalf("SaveModelRegistry() error = %v", err)
	}
	m.mode = modeModelSelector
	m.modelSel = newModelSelectorState(m.modelRegistry, modelSelectorManage)

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRight})
	m = next.(Model)
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("e")})
	m = next.(Model)
	if !m.modelSel.editing {
		t.Fatal("editing = false, want true after e")
	}

	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(Model)
	if !m.modelSel.editor.editing {
		t.Fatal("editor.editing = false, want true after enter")
	}

	m.modelSel.editor.editBuf = "custom-model"
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(Model)

	if !m.modelSel.editing {
		t.Fatal("editing = false, want true after field save")
	}
	if m.modelSel.editor.editing {
		t.Fatal("editor.editing = true, want false after field save")
	}
	if got := m.modelSel.providers[0].Models[0].Model; got != "custom-model" {
		t.Fatalf("model id after enter = %q, want custom-model", got)
	}

	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = next.(Model)

	if m.modelSel.editing {
		t.Fatal("editing = true, want false after esc")
	}
	if got := m.modelSel.providers[0].Models[0].Model; got != "custom-model" {
		t.Fatalf("model id after esc = %q, want custom-model", got)
	}
	reg, _, err := config.LoadModelRegistryOrDefault(m.modelRegistryPath)
	if err != nil {
		t.Fatalf("LoadModelRegistryOrDefault() error = %v", err)
	}
	if got := reg.Providers[0].Models[0].Model; got != "custom-model" {
		t.Fatalf("saved model id = %q, want custom-model", got)
	}
}

func TestModelSelectorProviderEnterSavesFieldBeforeEsc(t *testing.T) {
	store := newBrowserTestStore(t)
	m := newBrowserTestModel(t, store)
	m.modelRegistryPath = filepath.Join(t.TempDir(), "models.yaml")
	m.modelRegistry = config.ModelRegistry{
		Providers: []config.ProviderEntry{
			{
				Name:    "gateway",
				BaseURL: "https://example.test/v1",
				APIKey:  "secret",
			},
		},
	}
	if err := config.SaveModelRegistry(m.modelRegistryPath, m.modelRegistry); err != nil {
		t.Fatalf("SaveModelRegistry() error = %v", err)
	}
	m.mode = modeModelSelector
	m.modelSel = newModelSelectorState(m.modelRegistry, modelSelectorManage)

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("e")})
	m = next.(Model)
	if !m.modelSel.editing {
		t.Fatal("editing = false, want true after e")
	}

	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = next.(Model)
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(Model)
	if !m.modelSel.editor.editing {
		t.Fatal("editor.editing = false, want true after enter")
	}

	m.modelSel.editor.editBuf = "https://gateway.example/v2"
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(Model)

	if !m.modelSel.editing {
		t.Fatal("editing = false, want true after field save")
	}
	if m.modelSel.editor.editing {
		t.Fatal("editor.editing = true, want false after field save")
	}
	if got := m.modelSel.providers[0].BaseURL; got != "https://gateway.example/v2" {
		t.Fatalf("provider base_url after enter = %q, want https://gateway.example/v2", got)
	}

	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = next.(Model)

	if m.modelSel.editing {
		t.Fatal("editing = true, want false after esc")
	}
	if got := m.modelSel.providers[0].BaseURL; got != "https://gateway.example/v2" {
		t.Fatalf("provider base_url after esc = %q, want https://gateway.example/v2", got)
	}
	reg, _, err := config.LoadModelRegistryOrDefault(m.modelRegistryPath)
	if err != nil {
		t.Fatalf("LoadModelRegistryOrDefault() error = %v", err)
	}
	if got := reg.Providers[0].BaseURL; got != "https://gateway.example/v2" {
		t.Fatalf("saved provider base_url = %q, want https://gateway.example/v2", got)
	}
}

func TestModelSelectorOptionFieldEnterSavesBeforeEsc(t *testing.T) {
	store := newBrowserTestStore(t)
	m := newBrowserTestModel(t, store)
	m.modelRegistryPath = filepath.Join(t.TempDir(), "models.yaml")
	m.modelRegistry = config.ModelRegistry{
		Providers: []config.ProviderEntry{
			{
				Name:    "gateway",
				BaseURL: "https://example.test/v1",
				APIKey:  "secret",
				Models: []config.ModelEntry{
					{Name: "model-1", Model: "model-1", APIFormat: "openai-compatible", Temperature: 0.20, MaxTokens: 4096},
				},
			},
		},
	}
	if err := config.SaveModelRegistry(m.modelRegistryPath, m.modelRegistry); err != nil {
		t.Fatalf("SaveModelRegistry() error = %v", err)
	}
	m.mode = modeModelSelector
	m.modelSel = newModelSelectorState(m.modelRegistry, modelSelectorManage)

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRight})
	m = next.(Model)
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("e")})
	m = next.(Model)
	if !m.modelSel.editing {
		t.Fatal("editing = false, want true after e")
	}

	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = next.(Model)
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(Model)

	if !m.modelSel.editing {
		t.Fatal("editing = false, want true after option save")
	}
	if m.modelSel.editor.editing {
		t.Fatal("editor.editing = true, want false after option save")
	}
	if got := m.modelSel.providers[0].Models[0].APIFormat; got != "anthropic" {
		t.Fatalf("api_format after enter = %q, want anthropic", got)
	}

	reg, _, err := config.LoadModelRegistryOrDefault(m.modelRegistryPath)
	if err != nil {
		t.Fatalf("LoadModelRegistryOrDefault() error = %v", err)
	}
	if got := reg.Providers[0].Models[0].APIFormat; got != "anthropic" {
		t.Fatalf("saved api_format = %q, want anthropic", got)
	}
}

func TestModelSelectorRejectsDuplicateProviderName(t *testing.T) {
	store := newBrowserTestStore(t)
	m := newBrowserTestModel(t, store)
	m.modelRegistry = config.ModelRegistry{
		Providers: []config.ProviderEntry{
			{Name: "gateway", BaseURL: "https://example.test/v1", APIKey: "a"},
			{Name: "gateway-2", BaseURL: "https://example.test/v2", APIKey: "b"},
		},
	}
	m.mode = modeModelSelector
	m.modelSel = newModelSelectorState(m.modelRegistry, modelSelectorManage)
	m.modelSel.providerCursor = 1

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("e")})
	m = next.(Model)
	if !m.modelSel.editing {
		t.Fatal("editing = false, want true after e")
	}

	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(Model)
	m.modelSel.editor.editBuf = "gateway"
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(Model)

	if got := m.modelSel.providers[1].Name; got != "gateway-2" {
		t.Fatalf("provider name = %q, want gateway-2", got)
	}
	if got := m.statusMsg; got != "Save failed: provider name \"gateway\" already exists" {
		t.Fatalf("statusMsg = %q, want duplicate provider error", got)
	}
}

func TestModelSelectorRejectsDuplicateModelName(t *testing.T) {
	store := newBrowserTestStore(t)
	m := newBrowserTestModel(t, store)
	m.modelRegistry = config.ModelRegistry{
		Providers: []config.ProviderEntry{
			{
				Name:    "gateway",
				BaseURL: "https://example.test/v1",
				APIKey:  "secret",
				Models: []config.ModelEntry{
					{Name: "model-1", Model: "model-1", APIFormat: "openai-compatible", Temperature: 0.20, MaxTokens: 4096},
					{Name: "model-2", Model: "model-2", APIFormat: "openai-compatible", Temperature: 0.20, MaxTokens: 4096},
				},
			},
		},
	}
	m.mode = modeModelSelector
	m.modelSel = newModelSelectorState(m.modelRegistry, modelSelectorManage)
	m.modelSel.selectingModels = true
	m.modelSel.modelCursor = 1

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("e")})
	m = next.(Model)
	if !m.modelSel.editing {
		t.Fatal("editing = false, want true after e")
	}

	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(Model)
	m.modelSel.editor.editBuf = "model-1"
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(Model)

	if got := m.modelSel.providers[0].Models[1].Model; got != "model-2" {
		t.Fatalf("model id = %q, want model-2", got)
	}
	if got := m.statusMsg; got != "Save failed: model id \"model-1\" already exists under provider \"gateway\"" {
		t.Fatalf("statusMsg = %q, want duplicate model error", got)
	}
}

func TestModelSelectorProviderRenameSyncsTabBindingsAndSnapshots(t *testing.T) {
	store := newBrowserTestStore(t)
	m := newBrowserTestModel(t, store)
	m.modelRegistry = config.ModelRegistry{
		Providers: []config.ProviderEntry{
			{
				Name:    "gateway",
				BaseURL: "https://example.test/v1",
				APIKey:  "secret",
				Models: []config.ModelEntry{
					{Name: "flash", Model: "flash", APIFormat: "openai-compatible", Temperature: 0.2, MaxTokens: 4096},
				},
			},
		},
	}
	if err := store.Save("conv", []chat.Message{{Role: "assistant", Content: "hi", SnapshotProvider: "gateway", SnapshotModel: "flash", SnapshotAPIFormat: "openai-compatible"}}); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	if err := store.SetConversationModelBinding("conv", "gateway", "flash"); err != nil {
		t.Fatalf("SetConversationModelBinding() error = %v", err)
	}
	m.tabs[0].autoSaveName = "conv"
	m.tabs[0].providerConfigName = "gateway"
	m.tabs[0].modelConfigName = "flash"
	m.tabs[0].history.ReplaceMessages([]chat.Message{{Role: "assistant", Content: "hi", SnapshotProvider: "gateway", SnapshotModel: "flash", SnapshotAPIFormat: "openai-compatible"}})
	m.mode = modeModelSelector
	m.modelSel = newModelSelectorState(m.modelRegistry, modelSelectorManage)

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("e")})
	m = next.(Model)
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(Model)
	m.modelSel.editor.editBuf = "gateway-renamed"
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(Model)

	if got := m.tabs[0].providerConfigName; got != "gateway-renamed" {
		t.Fatalf("providerConfigName = %q, want gateway-renamed", got)
	}
	if got := m.tabs[0].history.Messages()[0].SnapshotProvider; got != "gateway-renamed" {
		t.Fatalf("history snapshot provider = %q, want gateway-renamed", got)
	}
	providerName, modelName, err := store.GetConversationModelBinding("conv")
	if err != nil {
		t.Fatalf("GetConversationModelBinding() error = %v", err)
	}
	if providerName != "gateway-renamed" || modelName != "flash" {
		t.Fatalf("binding = %q/%q, want gateway-renamed/flash", providerName, modelName)
	}
	loaded, err := store.Load("conv")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got := loaded[0].SnapshotProvider; got != "gateway-renamed" {
		t.Fatalf("stored snapshot provider = %q, want gateway-renamed", got)
	}
	if _, _, err := m.resolveCurrentTabModelSelection(&m.tabs[0]); err != nil {
		t.Fatalf("resolveCurrentTabModelSelection() error = %v", err)
	}
}

func TestModelSelectorModelRenameSyncsTabAndConversationBinding(t *testing.T) {
	store := newBrowserTestStore(t)
	m := newBrowserTestModel(t, store)
	m.modelRegistry = config.ModelRegistry{
		Providers: []config.ProviderEntry{
			{
				Name:    "gateway",
				BaseURL: "https://example.test/v1",
				APIKey:  "secret",
				Models: []config.ModelEntry{
					{Name: "flash", Model: "flash", APIFormat: "openai-compatible", Temperature: 0.2, MaxTokens: 4096},
				},
			},
		},
	}
	if err := store.SetConversationModelBinding("conv", "gateway", "flash"); err != nil {
		t.Fatalf("SetConversationModelBinding() error = %v", err)
	}
	m.tabs[0].autoSaveName = "conv"
	m.tabs[0].providerConfigName = "gateway"
	m.tabs[0].modelConfigName = "flash"
	m.mode = modeModelSelector
	m.modelSel = newModelSelectorState(m.modelRegistry, modelSelectorManage)
	m.modelSel.selectingModels = true

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("e")})
	m = next.(Model)
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(Model)
	m.modelSel.editor.editBuf = "flash-v2"
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(Model)

	if got := m.tabs[0].modelConfigName; got != "flash-v2" {
		t.Fatalf("modelConfigName = %q, want flash-v2", got)
	}
	if got := m.tabs[0].name; got != "flash-v2" {
		t.Fatalf("tab name = %q, want flash-v2", got)
	}
	providerName, modelName, err := store.GetConversationModelBinding("conv")
	if err != nil {
		t.Fatalf("GetConversationModelBinding() error = %v", err)
	}
	if providerName != "gateway" || modelName != "flash-v2" {
		t.Fatalf("binding = %q/%q, want gateway/flash-v2", providerName, modelName)
	}
	if _, _, err := m.resolveCurrentTabModelSelection(&m.tabs[0]); err != nil {
		t.Fatalf("resolveCurrentTabModelSelection() error = %v", err)
	}
}

func TestModelSelectorDDeletesSelectedModelAndPersistsRegistry(t *testing.T) {
	store := newBrowserTestStore(t)
	m := newBrowserTestModel(t, store)
	m.modelRegistryPath = filepath.Join(t.TempDir(), "models.yaml")
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
	if err := config.SaveModelRegistry(m.modelRegistryPath, m.modelRegistry); err != nil {
		t.Fatalf("SaveModelRegistry() error = %v", err)
	}
	m.mode = modeModelSelector
	m.modelSel = newModelSelectorState(m.modelRegistry, modelSelectorManage)

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRight})
	m = next.(Model)
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	m = next.(Model)

	if got := len(m.modelSel.providers[0].Models); got != 0 {
		t.Fatalf("models len = %d, want 0", got)
	}
	reg, _, err := config.LoadModelRegistryOrDefault(m.modelRegistryPath)
	if err != nil {
		t.Fatalf("LoadModelRegistryOrDefault() error = %v", err)
	}
	if got := len(reg.Providers[0].Models); got != 0 {
		t.Fatalf("saved models len = %d, want 0", got)
	}
}

func TestModelSelectorEEditsSelectedModelAndPersistsRegistry(t *testing.T) {
	store := newBrowserTestStore(t)
	m := newBrowserTestModel(t, store)
	m.modelRegistryPath = filepath.Join(t.TempDir(), "models.yaml")
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
	if err := config.SaveModelRegistry(m.modelRegistryPath, m.modelRegistry); err != nil {
		t.Fatalf("SaveModelRegistry() error = %v", err)
	}
	m.mode = modeModelSelector
	m.modelSel = newModelSelectorState(m.modelRegistry, modelSelectorManage)

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRight})
	m = next.(Model)
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("e")})
	m = next.(Model)
	if !m.modelSel.editing {
		t.Fatal("editing = false, want true after e")
	}
	m.modelSel.editor.fields[0].Value = "gemini-3-flash-thinking"
	if err := m.saveModelSelectorEdit(); err != nil {
		t.Fatalf("saveModelSelectorEdit() error = %v", err)
	}

	if got := m.modelSel.providers[0].Models[0].Model; got != "gemini-3-flash-thinking" {
		t.Fatalf("model id = %q, want gemini-3-flash-thinking", got)
	}
	reg, _, err := config.LoadModelRegistryOrDefault(m.modelRegistryPath)
	if err != nil {
		t.Fatalf("LoadModelRegistryOrDefault() error = %v", err)
	}
	if got := reg.Providers[0].Models[0].Model; got != "gemini-3-flash-thinking" {
		t.Fatalf("saved model id = %q, want gemini-3-flash-thinking", got)
	}
	if got := reg.Providers[0].Models[0].Name; got != "gemini-3-flash-thinking" {
		t.Fatalf("saved model key = %q, want gemini-3-flash-thinking", got)
	}
}

func TestModelSelectorEscCancelsEditAndKeepsOriginalModel(t *testing.T) {
	store := newBrowserTestStore(t)
	m := newBrowserTestModel(t, store)
	m.modelRegistry = config.ModelRegistry{
		Providers: []config.ProviderEntry{
			{
				Name:    "gateway",
				BaseURL: "https://example.test/v1",
				APIKey:  "secret",
				Models: []config.ModelEntry{
					{Name: "flash", Model: "gemini-3-flash-preview", APIFormat: "gemini"},
				},
			},
		},
	}
	m.mode = modeModelSelector
	m.modelSel = newModelSelectorState(m.modelRegistry, modelSelectorManage)

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRight})
	m = next.(Model)
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("e")})
	m = next.(Model)
	if !m.modelSel.editing {
		t.Fatal("editing = false, want true after e")
	}
	m.modelSel.editor.fields[0].Value = "gemini-3-flash-thinking"

	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = next.(Model)

	if m.modelSel.editing {
		t.Fatal("editing = true, want false after esc")
	}
	if got := m.modelSel.providers[0].Models[0].Model; got != "gemini-3-flash-preview" {
		t.Fatalf("model id = %q, want original gemini-3-flash-preview", got)
	}
	if got := m.statusMsg; got != "Edit canceled" {
		t.Fatalf("statusMsg = %q, want Edit canceled", got)
	}
}

func TestModelSelectorDDeletesSelectedProviderAndPersistsRegistry(t *testing.T) {
	store := newBrowserTestStore(t)
	m := newBrowserTestModel(t, store)
	m.modelRegistryPath = filepath.Join(t.TempDir(), "models.yaml")
	m.modelRegistry = config.ModelRegistry{
		Providers: []config.ProviderEntry{
			{Name: "gateway", BaseURL: "https://example.test/v1", APIKey: "secret"},
			{Name: "anthropic-direct", BaseURL: "https://api.anthropic.com", APIKey: "k"},
		},
	}
	if err := config.SaveModelRegistry(m.modelRegistryPath, m.modelRegistry); err != nil {
		t.Fatalf("SaveModelRegistry() error = %v", err)
	}
	m.mode = modeModelSelector
	m.modelSel = newModelSelectorState(m.modelRegistry, modelSelectorManage)
	m.tabs[0].providerConfigName = "active"

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	m = next.(Model)

	if got := len(m.modelSel.providers); got != 1 {
		t.Fatalf("providers len = %d, want 1", got)
	}
	reg, _, err := config.LoadModelRegistryOrDefault(m.modelRegistryPath)
	if err != nil {
		t.Fatalf("LoadModelRegistryOrDefault() error = %v", err)
	}
	if got := len(reg.Providers); got != 1 {
		t.Fatalf("saved providers len = %d, want 1", got)
	}
	if got := reg.Providers[0].Name; got != "anthropic-direct" {
		t.Fatalf("remaining provider = %q, want anthropic-direct", got)
	}
}
