# Model Registry And Version Regeneration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a persistent provider/model registry, a `/model` workflow for registry management and current-tab model switching, and complete the `e`/`r`/`g` assistant versioning behavior using per-version generation snapshots.

**Architecture:** Persist provider/model registry in `models.yaml`, persist assistant generation snapshots on each assistant row, and route both `/model` and browser `g` through one reusable provider/model selector state in the UI. Regeneration paths (`e` and `r`) read the snapshot stored on each assistant version so results are reproducible even after the active tab's current client changes.

**Tech Stack:** Go, Bubble Tea, YAML config, SQLite via `modernc.org/sqlite`

---

## File Map

### New files

- `internal/config/models.go`
- `internal/config/models_test.go`
- `internal/ui/modelselector.go`
- `internal/ui/modelselector_test.go`
- `internal/ui/modelselector_view_test.go`

### Modified files

- `internal/chat/history.go`
- `internal/storage/conversation.go`
- `internal/storage/conversation_test.go`
- `internal/ui/model.go`
- `internal/ui/update.go`
- `internal/ui/slashcomplete.go`
- `internal/ui/slashcomplete_test.go`
- `internal/ui/messagebrowse.go`
- `internal/ui/messagebrowse_test.go`
- `internal/ui/messagebrowse_view_test.go`
- `internal/ui/rolepicker.go`
- `internal/ui/statusbar.go`
- `internal/ui/statusbar_test.go`

## Task 1: Add persistent model registry config

**Files:**
- Create: `internal/config/models.go`
- Create: `internal/config/models_test.go`
- Test: `internal/config/models_test.go`

- [ ] **Step 1: Write failing tests for registry load/save and path helpers**

Add these tests in `internal/config/models_test.go`:

```go
package config

import (
	"path/filepath"
	"testing"
)

func TestDefaultModelRegistry(t *testing.T) {
	reg := DefaultModelRegistry()
	if len(reg.Providers) != 0 {
		t.Fatalf("Providers len = %d, want 0", len(reg.Providers))
	}
}

func TestModelRegistryPathUsesConfigDir(t *testing.T) {
	got := ModelRegistryPath("/tmp/termchat/config.yaml")
	want := filepath.Join("/tmp/termchat", "models.yaml")
	if got != want {
		t.Fatalf("ModelRegistryPath() = %q, want %q", got, want)
	}
}

func TestLoadModelRegistryMissingReturnsDefault(t *testing.T) {
	path := filepath.Join(t.TempDir(), "models.yaml")
	reg, missing, err := LoadModelRegistryOrDefault(path)
	if err != nil {
		t.Fatalf("LoadModelRegistryOrDefault() error = %v", err)
	}
	if !missing {
		t.Fatalf("missing = false, want true")
	}
	if len(reg.Providers) != 0 {
		t.Fatalf("Providers len = %d, want 0", len(reg.Providers))
	}
}

func TestSaveAndLoadModelRegistry(t *testing.T) {
	path := filepath.Join(t.TempDir(), "models.yaml")
	want := ModelRegistry{
		Providers: []ProviderEntry{
			{
				Name:     "gateway",
				Provider: "openai-compatible",
				BaseURL:  "https://example.test/v1",
				APIKey:   "secret",
				Models: []ModelEntry{
					{Name: "flash", Model: "gemini-3-flash-preview"},
					{Name: "pro", Model: "gemini-3.1-pro-preview"},
				},
			},
		},
	}
	if err := SaveModelRegistry(path, want); err != nil {
		t.Fatalf("SaveModelRegistry() error = %v", err)
	}

	got, missing, err := LoadModelRegistryOrDefault(path)
	if err != nil {
		t.Fatalf("LoadModelRegistryOrDefault() error = %v", err)
	}
	if missing {
		t.Fatalf("missing = true, want false")
	}
	if got.Providers[0].Name != "gateway" {
		t.Fatalf("provider name = %q, want gateway", got.Providers[0].Name)
	}
	if got.Providers[0].Models[1].Model != "gemini-3.1-pro-preview" {
		t.Fatalf("model = %q, want gemini-3.1-pro-preview", got.Providers[0].Models[1].Model)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run:

```bash
env GOCACHE=/tmp/go-build go test ./internal/config -run 'Test(DefaultModelRegistry|ModelRegistryPathUsesConfigDir|LoadModelRegistryMissingReturnsDefault|SaveAndLoadModelRegistry)' -count=1
```

Expected: FAIL with undefined `ModelRegistry`, `ProviderEntry`, `ModelEntry`, and helper functions.

- [ ] **Step 3: Write minimal registry implementation**

Create `internal/config/models.go` with:

```go
package config

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type ModelRegistry struct {
	Providers []ProviderEntry `yaml:"providers"`
}

type ProviderEntry struct {
	Name     string       `yaml:"name"`
	Provider string       `yaml:"provider"`
	BaseURL  string       `yaml:"base_url"`
	APIKey   string       `yaml:"api_key"`
	Models   []ModelEntry `yaml:"models"`
}

type ModelEntry struct {
	Name  string `yaml:"name"`
	Model string `yaml:"model"`
}

func DefaultModelRegistry() ModelRegistry {
	return ModelRegistry{}
}

func ModelRegistryPath(cfgPath string) string {
	return filepath.Join(filepath.Dir(cfgPath), "models.yaml")
}

func LoadModelRegistry(path string) (ModelRegistry, error) {
	reg := DefaultModelRegistry()
	data, err := os.ReadFile(path)
	if err != nil {
		return reg, err
	}
	if err := yaml.Unmarshal(data, &reg); err != nil {
		return reg, err
	}
	return reg, nil
}

func LoadModelRegistryOrDefault(path string) (reg ModelRegistry, missing bool, err error) {
	reg, err = LoadModelRegistry(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return DefaultModelRegistry(), true, nil
		}
		return reg, false, err
	}
	return reg, false, nil
}

func SaveModelRegistry(path string, reg ModelRegistry) error {
	data, err := yaml.Marshal(reg)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run:

```bash
env GOCACHE=/tmp/go-build go test ./internal/config -run 'Test(DefaultModelRegistry|ModelRegistryPathUsesConfigDir|LoadModelRegistryMissingReturnsDefault|SaveAndLoadModelRegistry)' -count=1
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/config/models.go internal/config/models_test.go
git commit -m "feat: add persistent model registry config"
```

## Task 2: Persist assistant generation snapshots in chat/storage

**Files:**
- Modify: `internal/chat/history.go`
- Modify: `internal/storage/conversation.go`
- Modify: `internal/storage/conversation_test.go`
- Test: `internal/storage/conversation_test.go`

- [ ] **Step 1: Write failing storage tests for snapshot round-trip and legacy fallback**

Append these tests in `internal/storage/conversation_test.go`:

```go
func TestAppendAssistantMessagePersistsGenerationSnapshot(t *testing.T) {
	store := newTestStore(t)
	const name = "conv"

	_, err := store.AppendMessage(name, chat.Message{Seq: 1, Role: "user", Content: "q1"})
	if err != nil {
		t.Fatalf("AppendMessage(user) error = %v", err)
	}
	_, err = store.AppendMessage(name, chat.Message{
		Seq:                1,
		Role:               "assistant",
		Content:            "a1",
		VersionNumber:      1,
		SnapshotProvider:   "anthropic-direct",
		SnapshotModel:      "claude-sonnet-4-20250514",
		SnapshotRoleName:   "writer",
		SnapshotRolePrompt: "be concise",
	})
	if err != nil {
		t.Fatalf("AppendMessage(assistant) error = %v", err)
	}

	got, err := store.LoadBrowseMessages(name)
	if err != nil {
		t.Fatalf("LoadBrowseMessages() error = %v", err)
	}
	if got[1].SnapshotProvider != "anthropic-direct" {
		t.Fatalf("SnapshotProvider = %q, want anthropic-direct", got[1].SnapshotProvider)
	}
	if got[1].SnapshotRolePrompt != "be concise" {
		t.Fatalf("SnapshotRolePrompt = %q, want be concise", got[1].SnapshotRolePrompt)
	}
	if !got[1].HasGenerationSnapshot() {
		t.Fatalf("HasGenerationSnapshot = false, want true")
	}
}

func TestLegacyAssistantWithoutSnapshotIsNonReproducible(t *testing.T) {
	store := newTestStore(t)
	const name = "conv"

	_, err := store.AppendMessage(name, chat.Message{Seq: 1, Role: "user", Content: "q1"})
	if err != nil {
		t.Fatalf("AppendMessage(user) error = %v", err)
	}
	_, err = store.AppendMessage(name, chat.Message{
		Seq:           1,
		Role:          "assistant",
		Content:       "legacy",
		VersionNumber: 1,
	})
	if err != nil {
		t.Fatalf("AppendMessage(assistant) error = %v", err)
	}

	got, err := store.LoadBrowseMessages(name)
	if err != nil {
		t.Fatalf("LoadBrowseMessages() error = %v", err)
	}
	if got[1].HasGenerationSnapshot() {
		t.Fatalf("HasGenerationSnapshot = true, want false")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run:

```bash
env GOCACHE=/tmp/go-build go test ./internal/storage -run 'Test(AppendAssistantMessagePersistsGenerationSnapshot|LegacyAssistantWithoutSnapshotIsNonReproducible)' -count=1
```

Expected: FAIL with missing snapshot fields and helper.

- [ ] **Step 3: Add snapshot fields and schema migration**

Update `internal/chat/history.go` `Message` with:

```go
	SnapshotProvider   string `json:"snapshot_provider"` // provider config name from models.yaml
	SnapshotModel      string `json:"snapshot_model"`
	SnapshotRoleName   string `json:"snapshot_role_name"`
	SnapshotRolePrompt string `json:"snapshot_role_prompt"`
```

Add helper:

```go
func (m Message) HasGenerationSnapshot() bool {
	return m.SnapshotProvider != "" &&
		m.SnapshotModel != "" &&
		m.SnapshotRolePrompt != ""
}
```

Update `internal/storage/conversation.go` schema:

```sql
snapshot_provider    TEXT NOT NULL DEFAULT '',
snapshot_model       TEXT NOT NULL DEFAULT '',
snapshot_role_name   TEXT NOT NULL DEFAULT '',
snapshot_role_prompt TEXT NOT NULL DEFAULT ''
```

Add matching `ALTER TABLE` entries in `ensureMessagesColumns`, include fields in:

- `AppendMessage`
- `AppendAssistantVersion`
- `LoadActiveTimeline`
- `ListVersions`
- `LoadBrowseMessages`

Use insert/read shapes like:

```go
`INSERT INTO messages(
	conversation_id, role, content, seq,
	version_group_id, version_number, is_active_version,
	edited_after_generation, stale_after_user_edit,
	is_deleted, deleted_batch_id,
	snapshot_provider, snapshot_model, snapshot_role_name, snapshot_role_prompt
) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
```

and:

```go
SELECT id, role, content, seq, version_group_id, version_number,
       is_active_version, edited_after_generation, stale_after_user_edit,
       is_deleted, deleted_batch_id,
       snapshot_provider, snapshot_model, snapshot_role_name, snapshot_role_prompt
```

- [ ] **Step 4: Run tests to verify they pass**

Run:

```bash
env GOCACHE=/tmp/go-build go test ./internal/storage -run 'Test(AppendAssistantMessagePersistsGenerationSnapshot|LegacyAssistantWithoutSnapshotIsNonReproducible)' -count=1
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/chat/history.go internal/storage/conversation.go internal/storage/conversation_test.go
git commit -m "feat: persist assistant generation snapshots"
```

## Task 3: Load model registry into the UI and add `/model` command entry

**Files:**
- Modify: `internal/ui/model.go`
- Modify: `internal/ui/slashcomplete.go`
- Modify: `internal/ui/slashcomplete_test.go`
- Modify: `internal/ui/update.go`
- Test: `internal/ui/slashcomplete_test.go`

- [ ] **Step 1: Write failing tests for `/model` command discovery**

Extend `internal/ui/slashcomplete_test.go` with:

```go
func TestFilterSlashCmdsIncludesModel(t *testing.T) {
	got := filterSlashCmds("/mo")
	if len(got) == 0 {
		t.Fatalf("filterSlashCmds(/mo) returned no results")
	}
	if got[0].Name != "/model" {
		t.Fatalf("got[0].Name = %q, want /model", got[0].Name)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run:

```bash
env GOCACHE=/tmp/go-build go test ./internal/ui -run TestFilterSlashCmdsIncludesModel -count=1
```

Expected: FAIL because `/model` is not in `slashCmds`.

- [ ] **Step 3: Add registry state to `Model` and add `/model` command routing**

In `internal/ui/model.go`, add state:

```go
type modelSelectorPurpose int

const (
	modelSelectorManage modelSelectorPurpose = iota
	modelSelectorPickForSwitch
	modelSelectorPickForNewVersion
)

type modelSelectorState struct {
	purpose         modelSelectorPurpose
	providers       []config.ProviderEntry
	providerCursor  int
	modelCursor     int
	selectingModels bool
}
```

Add fields to `Model`:

```go
	modelRegistryPath string
	modelRegistry     config.ModelRegistry
	modelSel          modelSelectorState
	providerFactory   func(providerType, baseURL, apiKey, model string) chat.Provider
```

In `NewModel`, load registry:

```go
	modelsPath := config.ModelRegistryPath(cfgPath)
	reg, _, err := config.LoadModelRegistryOrDefault(modelsPath)
	if err != nil {
		return Model{}, fmt.Errorf("load model registry: %w", err)
	}
```

Store `modelsPath` and `reg` on the model and initialize:

```go
	providerFactory: func(providerType, baseURL, apiKey, model string) chat.Provider {
		return chat.NewProvider(providerType, baseURL, apiKey, model)
	},
```

In `internal/ui/slashcomplete.go`, add:

```go
	{"/model", "Manage and switch models"},
```

In `internal/ui/update.go`, route:

```go
	case "/model":
		m.modelSel = newModelSelectorState(m.modelRegistry, modelSelectorManage)
		m.mode = modeModelSelector
		return m, nil
```

- [ ] **Step 4: Run tests to verify they pass**

Run:

```bash
env GOCACHE=/tmp/go-build go test ./internal/ui -run TestFilterSlashCmdsIncludesModel -count=1
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/ui/model.go internal/ui/slashcomplete.go internal/ui/slashcomplete_test.go internal/ui/update.go
git commit -m "feat: add /model command entry and registry state"
```

## Task 4: Build reusable model selector UI for `/model` and `g`

**Files:**
- Create: `internal/ui/modelselector.go`
- Create: `internal/ui/modelselector_test.go`
- Create: `internal/ui/modelselector_view_test.go`
- Modify: `internal/ui/model.go`
- Modify: `internal/ui/update.go`
- Test: `internal/ui/modelselector_test.go`
- Test: `internal/ui/modelselector_view_test.go`

- [ ] **Step 1: Write failing tests for provider/model navigation and tab switching**

Add `internal/ui/modelselector_test.go`:

```go
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
```

Add `internal/ui/modelselector_view_test.go`:

```go
package ui

import (
	"strings"
	"testing"

	"github.com/termchat/termchat/internal/config"
)

func TestViewModelSelectorShowsProviderAndModels(t *testing.T) {
	m := Model{
		theme: DarkTheme,
		modelRegistry: config.ModelRegistry{
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
		},
		modelSel: newModelSelectorState(config.ModelRegistry{
			Providers: []config.ProviderEntry{
				{
					Name:     "gateway",
					Provider: "openai-compatible",
					BaseURL:  "https://example.test/v1",
					Models: []config.ModelEntry{{Name: "flash", Model: "gemini-3-flash-preview"}},
				},
			},
		}, modelSelectorManage),
		mode:  modeModelSelector,
		width: 80,
	}

	out := m.View()
	if !strings.Contains(out, "gateway") {
		t.Fatalf("View() missing provider name: %q", out)
	}
	if !strings.Contains(out, "gemini-3-flash-preview") {
		t.Fatalf("View() missing model name: %q", out)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run:

```bash
env GOCACHE=/tmp/go-build go test ./internal/ui -run 'Test(ModelSelectorEnterSwitchesCurrentTabClient|ViewModelSelectorShowsProviderAndModels)' -count=1
```

Expected: FAIL with missing `modeModelSelector`, `newModelSelectorState`, and view/update handlers.

- [ ] **Step 3: Implement reusable model selector state and UI**

Create `internal/ui/modelselector.go` with:

```go
package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/termchat/termchat/internal/chat"
	"github.com/termchat/termchat/internal/config"
)

func newModelSelectorState(reg config.ModelRegistry, purpose modelSelectorPurpose) modelSelectorState {
	return modelSelectorState{
		purpose:   purpose,
		providers: reg.Providers,
	}
}

func (m *Model) selectedProviderEntry() *config.ProviderEntry {
	if len(m.modelSel.providers) == 0 {
		return nil
	}
	return &m.modelSel.providers[m.modelSel.providerCursor]
}

func (m *Model) selectedModelEntry() *config.ModelEntry {
	p := m.selectedProviderEntry()
	if p == nil || len(p.Models) == 0 {
		return nil
	}
	return &p.Models[m.modelSel.modelCursor]
}

func (m Model) updateModelSelector(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = modeChat
		return m, nil
	case "up", "k":
		if m.modelSel.selectingModels {
			if m.modelSel.modelCursor > 0 {
				m.modelSel.modelCursor--
			}
		} else if m.modelSel.providerCursor > 0 {
			m.modelSel.providerCursor--
			m.modelSel.modelCursor = 0
		}
	case "down", "j":
		if m.modelSel.selectingModels {
			if p := m.selectedProviderEntry(); p != nil && m.modelSel.modelCursor < len(p.Models)-1 {
				m.modelSel.modelCursor++
			}
		} else if m.modelSel.providerCursor < len(m.modelSel.providers)-1 {
			m.modelSel.providerCursor++
			m.modelSel.modelCursor = 0
		}
	case "enter":
		if !m.modelSel.selectingModels {
			m.modelSel.selectingModels = true
			return m, nil
		}
		p := m.selectedProviderEntry()
		model := m.selectedModelEntry()
		if p == nil || model == nil {
			return m, nil
		}
		m.tabs[m.activeTab].client = m.providerFactory(p.Provider, p.BaseURL, p.APIKey, model.Model)
		m.tabs[m.activeTab].name = model.Model
		m.statusMsg = fmt.Sprintf("Switched to %s / %s", p.Name, model.Model)
		m.mode = modeChat
	}
	return m, nil
}

func (m Model) viewModelSelector() string {
	tabBar := (&m).renderTabBar()
	statusBar := m.renderStatusBar()
	var b strings.Builder
	b.WriteString(m.theme.ConfigTitleStyle().Render("Model Selector") + "\n\n")
	for i, p := range m.modelSel.providers {
		cursor := "  "
		if !m.modelSel.selectingModels && i == m.modelSel.providerCursor {
			cursor = "> "
		}
		b.WriteString(cursor + p.Name + " [" + p.Provider + "]\n")
		if i == m.modelSel.providerCursor {
			for j, model := range p.Models {
				modelCursor := "    "
				if m.modelSel.selectingModels && j == m.modelSel.modelCursor {
					modelCursor = "  -> "
				}
				b.WriteString(modelCursor + model.Name + " (" + model.Model + ")\n")
			}
		}
	}
	b.WriteString("\n")
	b.WriteString(m.theme.ConfigHelpStyle().Render("  ↑↓: move  Enter: select  Esc: back"))
	return lipgloss.JoinVertical(lipgloss.Left, tabBar, b.String(), statusBar)
}
```

Also add `modeModelSelector` to `uiMode` and route it from `View()` and `Update()`.

- [ ] **Step 4: Run tests to verify they pass**

Run:

```bash
env GOCACHE=/tmp/go-build go test ./internal/ui -run 'Test(ModelSelectorEnterSwitchesCurrentTabClient|ViewModelSelectorShowsProviderAndModels)' -count=1
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/ui/modelselector.go internal/ui/modelselector_test.go internal/ui/modelselector_view_test.go internal/ui/model.go internal/ui/update.go
git commit -m "feat: add reusable provider model selector UI"
```

## Task 5: Complete browser `e` and `r` using per-version snapshots

**Files:**
- Modify: `internal/ui/messagebrowse.go`
- Modify: `internal/ui/messagebrowse_test.go`
- Modify: `internal/ui/messagebrowse_view_test.go`
- Test: `internal/ui/messagebrowse_test.go`

- [ ] **Step 1: Write failing tests for `r` and full-version `e` regeneration**

Append these tests in `internal/ui/messagebrowse_test.go`:

```go
type erroringProvider struct {
	model string
	err   error
}

func (p *erroringProvider) SendStreamChan(_ context.Context, _ []chat.Message, _ float64, _ int, _ string, _ int) (<-chan chat.StreamChunk, <-chan error) {
	chunks := make(chan chat.StreamChunk)
	errs := make(chan error, 1)
	errs <- p.err
	close(chunks)
	close(errs)
	return chunks, errs
}

func (p *erroringProvider) Model() string         { return p.model }
func (p *erroringProvider) SetModel(model string) { p.model = model }
func (p *erroringProvider) SetBaseURL(string)     {}
func (p *erroringProvider) SetAPIKey(string)      {}
func (p *erroringProvider) BaseURL() string       { return "" }
func (p *erroringProvider) APIKey() string        { return "" }
func (p *erroringProvider) SupportsVision() bool  { return false }

func TestMessageBrowse_RegenerateCurrentPreviewVersionUsesStoredSnapshot(t *testing.T) {
	store := newBrowserTestStore(t)
	tab, err := newTabSession(config.DefaultConfig(), &stubProvider{model: "live-model"}, 80)
	if err != nil {
		t.Fatalf("newTabSession() error = %v", err)
	}
	m := Model{
		cfg:       config.DefaultConfig(),
		store:     store,
		theme:     DarkTheme,
		tabs:      []TabSession{tab},
		activeTab: 0,
		width:     80,
		height:    24,
		providerFactory: func(providerType, baseURL, apiKey, model string) chat.Provider {
			return &streamingStubProvider{
				stubProvider: stubProvider{model: model},
				response:     "rewritten by snapshot",
			}
		},
		modelRegistry: config.ModelRegistry{
			Providers: []config.ProviderEntry{
				{
					Name:     "anthropic-direct",
					Provider: "anthropic",
					BaseURL:  "https://api.anthropic.com",
					APIKey:   "k",
					Models:   []config.ModelEntry{{Name: "sonnet", Model: "claude-sonnet-4"}},
				},
			},
		},
	}
	const name = "conv"
	seedConversationLegacy(t, store, name, []chat.Message{
		{Seq: 1, Role: "user", Content: "q1"},
		{Seq: 1, Role: "assistant", Content: "v1", VersionNumber: 1, SnapshotProvider: "anthropic-direct", SnapshotModel: "claude-sonnet-4", SnapshotRoleName: "writer", SnapshotRolePrompt: "be concise"},
	})
	m.tabs[0].autoSaveName = name
	m.tabs[0].history.ReplaceMessages(mustLoadActiveTimeline(t, store, name))
	if err := m.buildBrowserState(); err != nil {
		t.Fatalf("buildBrowserState() error = %v", err)
	}
	m.mode = modeMessageBrowse

	m, _ = m.updateMessageBrowse(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")})

	reloaded := mustLoadActiveTimeline(t, store, name)
	if got := reloaded[1].Content; got != "rewritten by snapshot" {
		t.Fatalf("assistant content = %q, want rewritten by snapshot", got)
	}
}

func TestMessageBrowse_EditRegenerateAllVersionsRewritesEachVersionOrError(t *testing.T) {
	store := newBrowserTestStore(t)
	cfg := config.DefaultConfig()
	tab, err := newTabSession(cfg, &stubProvider{model: "unused-live-model"}, 80)
	if err != nil {
		t.Fatalf("newTabSession() error = %v", err)
	}
	m := Model{
		cfg:       cfg,
		store:     store,
		theme:     DarkTheme,
		tabs:      []TabSession{tab},
		activeTab: 0,
		width:     80,
		height:    24,
		providerFactory: func(providerType, baseURL, apiKey, model string) chat.Provider {
			switch model {
			case "claude-sonnet-4":
				return &streamingStubProvider{
					stubProvider: stubProvider{model: model},
					response:     "claude rewrite",
				}
			case "gemini-3-flash-preview":
				return &erroringProvider{model: model, err: errors.New("429")}
			default:
				return &streamingStubProvider{
					stubProvider: stubProvider{model: model},
					response:     "fallback rewrite",
				}
			}
		},
		modelRegistry: config.ModelRegistry{
			Providers: []config.ProviderEntry{
				{
					Name:     "anthropic-direct",
					Provider: "anthropic",
					BaseURL:  "https://api.anthropic.com",
					APIKey:   "k1",
					Models:   []config.ModelEntry{{Name: "sonnet", Model: "claude-sonnet-4"}},
				},
				{
					Name:     "gateway",
					Provider: "openai-compatible",
					BaseURL:  "https://example.test/v1",
					APIKey:   "k2",
					Models:   []config.ModelEntry{{Name: "flash", Model: "gemini-3-flash-preview"}},
				},
			},
		},
	}
	const name = "conv"
	seedConversationLegacy(t, store, name, []chat.Message{
		{Seq: 1, Role: "user", Content: "old question"},
		{Seq: 1, Role: "assistant", Content: "old a", VersionNumber: 1, SnapshotProvider: "anthropic-direct", SnapshotModel: "claude-sonnet-4", SnapshotRoleName: "writer", SnapshotRolePrompt: "be concise"},
		{Seq: 1, Role: "assistant", Content: "old b", VersionNumber: 2, SnapshotProvider: "gateway", SnapshotModel: "gemini-3-flash-preview", SnapshotRoleName: "writer", SnapshotRolePrompt: "be concise"},
	})
	m.tabs[0].autoSaveName = name
	m.tabs[0].history.ReplaceMessages(mustLoadActiveTimeline(t, store, name))
	if err := m.buildBrowserState(); err != nil {
		t.Fatalf("buildBrowserState() error = %v", err)
	}
	m.mode = modeMessageBrowse

	m, _ = m.updateMessageBrowse(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("e")})
	m.messageBrowse.editBuffer = "edited question"
	m.messageBrowse.editDirty = true
	m, _ = m.updateMessageBrowse(tea.KeyMsg{Type: tea.KeyEnter})
	m.messageBrowse.pendingConfirm.cursor = 0
	m, _ = m.updateMessageBrowse(tea.KeyMsg{Type: tea.KeyEnter})

	reloaded, err := store.LoadBrowseMessages(name)
	if err != nil {
		t.Fatalf("LoadBrowseMessages() error = %v", err)
	}
	if got := reloaded[0].Content; got != "edited question" {
		t.Fatalf("user content = %q, want edited question", got)
	}
	if got := reloaded[1].Content; got != "claude rewrite" {
		t.Fatalf("assistant v1 content = %q, want claude rewrite", got)
	}
	if got := reloaded[2].Content; got != "Error: 429" {
		t.Fatalf("assistant v2 content = %q, want Error: 429", got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run:

```bash
env GOCACHE=/tmp/go-build go test ./internal/ui -run 'TestMessageBrowse_(RegenerateCurrentPreviewVersionUsesStoredSnapshot|EditRegenerateAllVersionsRewritesEachVersionOrError)' -count=1
```

Expected: FAIL because `r` is missing and `e` still only regenerates one active assistant using the live tab client.

- [ ] **Step 3: Add snapshot-driven provider creation and regeneration paths**

In `internal/ui/messagebrowse.go`, add helpers:

```go
func (m *Model) lookupProviderConfig(name string) (config.ProviderEntry, bool) {
	for _, entry := range m.modelRegistry.Providers {
		if entry.Name == name {
			return entry, true
		}
	}
	return config.ProviderEntry{}, false
}

func (m *Model) providerFromAssistantSnapshot(msg chat.Message) (chat.Provider, error) {
	entry, ok := m.lookupProviderConfig(msg.SnapshotProvider)
	if !ok {
		return nil, fmt.Errorf("snapshot provider %q not found", msg.SnapshotProvider)
	}
	return m.providerFactory(entry.Provider, entry.BaseURL, entry.APIKey, msg.SnapshotModel), nil
}
```

Implement `r`:

```go
case "r":
	return m.regeneratePreviewVersion()
```

with behavior:

```go
func (m Model) regeneratePreviewVersion() (Model, tea.Cmd) {
	turn := m.currentBrowseTurn()
	if len(turn.AssistantVersions) == 0 {
		return m, nil
	}
	target := turn.AssistantVersions[turn.PreviewVersion]
	if !target.HasGenerationSnapshot() {
		m.statusMsg = "This version cannot be regenerated"
		return m, nil
	}
	return m.regenerateSingleAssistantVersion(turn, target)
}
```

Refactor `e -> regenerate all versions` into:

```go
func (m Model) regenerateAllAssistantVersions(turn browseTurn, editedContent string) (Model, tea.Cmd) {
	for _, version := range turn.AssistantVersions {
		if !version.HasGenerationSnapshot() {
			m.statusMsg = "Turn contains legacy versions that cannot be regenerated"
			return m, nil
		}
		content, err := m.regenerateAssistantVersionContent(turn, version, editedContent)
		if err != nil {
			m.statusMsg = "Regenerate failed: " + err.Error()
			return m, nil
		}
		if err := m.store.UpdateMessageContent(version.ID, content); err != nil {
			m.statusMsg = "Update failed: " + err.Error()
			return m, nil
		}
	}
	return m, nil
}
```

Add one shared content helper:

```go
func (m Model) regenerateAssistantVersionContent(turn browseTurn, target chat.Message, editedContent string) (string, error) {
	client, err := m.providerFromAssistantSnapshot(target)
	if err != nil {
		return "", err
	}
	apiMessages := m.buildTurnRegenerationMessages(turn, editedContent, target.SnapshotRolePrompt)
	chunks, errs := client.SendStreamChan(
		context.Background(),
		apiMessages,
		m.cfg.Parameters.Temperature,
		m.cfg.Parameters.MaxTokens,
		m.cfg.Parameters.ReasoningEffort,
		m.cfg.Parameters.BudgetTokens,
	)
	if chunks == nil || errs == nil {
		return "", fmt.Errorf("provider returned no stream")
	}
	var resp strings.Builder
	for chunks != nil || errs != nil {
		select {
		case chunk, ok := <-chunks:
			if !ok {
				chunks = nil
				continue
			}
			resp.WriteString(chunk.Content)
		case err, ok := <-errs:
			if !ok {
				errs = nil
				continue
			}
			if err != nil {
				return "Error: " + err.Error(), nil
			}
		}
	}
	return resp.String(), nil
}
```

Both `r` and `e` must:

- rebuild request from active timeline before current turn + current user
- use the target version snapshot role prompt
- overwrite content in place
- overwrite failures with `Error: ...`

- [ ] **Step 4: Run tests to verify they pass**

Run:

```bash
env GOCACHE=/tmp/go-build go test ./internal/ui -run 'TestMessageBrowse_(RegenerateCurrentPreviewVersionUsesStoredSnapshot|EditRegenerateAllVersionsRewritesEachVersionOrError)' -count=1
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/ui/messagebrowse.go internal/ui/messagebrowse_test.go internal/ui/messagebrowse_view_test.go
git commit -m "feat: regenerate versions from stored snapshots"
```

## Task 6: Implement `g` new-version flow using the provider/model selector

**Files:**
- Modify: `internal/ui/model.go`
- Modify: `internal/ui/messagebrowse.go`
- Modify: `internal/ui/messagebrowse_test.go`
- Modify: `internal/ui/messagebrowse_view_test.go`
- Modify: `internal/ui/update.go`
- Test: `internal/ui/messagebrowse_test.go`

- [ ] **Step 1: Write failing test for `g` appending a version from selected provider/model**

Add this test in `internal/ui/messagebrowse_test.go`:

```go
func TestMessageBrowse_GCreatesAssistantVersionFromSelectedRegistryModel(t *testing.T) {
	store := newBrowserTestStore(t)
	m := newBrowserTestModel(t, store)
	m.providerFactory = func(providerType, baseURL, apiKey, model string) chat.Provider {
		return &streamingStubProvider{
			stubProvider: stubProvider{model: model},
			response:     "new version from registry",
		}
	}
	m.modelRegistry = config.ModelRegistry{
		Providers: []config.ProviderEntry{
			{
				Name:     "gateway",
				Provider: "openai-compatible",
				BaseURL:  "https://example.test/v1",
				APIKey:   "k",
				Models: []config.ModelEntry{
					{Name: "flash", Model: "gemini-3-flash-preview"},
				},
			},
		},
	}
	seedConversation(t, store, "conv", []seedTurn{{user: "q1", assistant: []string{"v1"}}})
	m.tabs[0].autoSaveName = "conv"
	m.tabs[0].history.ReplaceMessages(mustLoadActiveTimeline(t, store, "conv"))
	if err := m.buildBrowserState(); err != nil {
		t.Fatalf("buildBrowserState() error = %v", err)
	}
	m.mode = modeMessageBrowse

	m, _ = m.updateMessageBrowse(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("g")})
	if m.mode != modeModelSelector {
		t.Fatalf("mode = %v, want modeModelSelector", m.mode)
	}
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(Model)
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(Model)

	reloaded := mustLoadActiveTimeline(t, store, "conv")
	if len(reloaded) != 2 {
		t.Fatalf("active timeline len = %d, want 2", len(reloaded))
	}
	browse, err := store.LoadBrowseMessages("conv")
	if err != nil {
		t.Fatalf("LoadBrowseMessages() error = %v", err)
	}
	if len(browse) != 3 {
		t.Fatalf("browse len = %d, want 3", len(browse))
	}
	if got := browse[2].Content; got != "new version from registry" {
		t.Fatalf("new version content = %q, want new version from registry", got)
	}
	if got := browse[2].SnapshotProvider; got != "gateway" {
		t.Fatalf("SnapshotProvider = %q, want gateway", got)
	}
	if got := browse[2].SnapshotModel; got != "gemini-3-flash-preview" {
		t.Fatalf("SnapshotModel = %q, want gemini-3-flash-preview", got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run:

```bash
env GOCACHE=/tmp/go-build go test ./internal/ui -run TestMessageBrowse_GCreatesAssistantVersionFromSelectedRegistryModel -count=1
```

Expected: FAIL because `g` is not implemented and does not open the selector.

- [ ] **Step 3: Implement `g` selection flow and append-version persistence**

In `internal/ui/messagebrowse.go`:

```go
case "g":
	m.modelSel = newModelSelectorState(m.modelRegistry, modelSelectorPickForNewVersion)
	m.mode = modeModelSelector
	return m, nil
```

In `internal/ui/modelselector.go`, extend enter handling:

```go
if m.modelSel.purpose == modelSelectorPickForNewVersion {
	return m.appendAssistantVersionFromSelection(*p, *model)
}
```

Implement:

```go
func collectAssistantResponseOrError(chunks <-chan chat.StreamChunk, errs <-chan error) string {
	var resp strings.Builder
	for chunks != nil || errs != nil {
		select {
		case chunk, ok := <-chunks:
			if !ok {
				chunks = nil
				continue
			}
			resp.WriteString(chunk.Content)
		case err, ok := <-errs:
			if !ok {
				errs = nil
				continue
			}
			if err != nil {
				return "Error: " + err.Error()
			}
		}
	}
	return resp.String()
}

func (m Model) appendAssistantVersionFromSelection(p config.ProviderEntry, model config.ModelEntry) (Model, tea.Cmd) {
	turn := m.currentBrowseTurn()
	client := m.providerFactory(p.Provider, p.BaseURL, p.APIKey, model.Model)
	rolePrompt := m.tabs[m.activeTab].history.SystemPrompt()
	roleName := m.activeRole
	apiMessages := m.buildTurnRegenerationMessages(turn, turn.User.Content, rolePrompt)
	chunks, errs := client.SendStreamChan(
		context.Background(),
		apiMessages,
		m.cfg.Parameters.Temperature,
		m.cfg.Parameters.MaxTokens,
		m.cfg.Parameters.ReasoningEffort,
		m.cfg.Parameters.BudgetTokens,
	)
	if chunks == nil || errs == nil {
		m.statusMsg = "New version failed: provider returned no stream"
		m.mode = modeMessageBrowse
		return m, nil
	}
	content := collectAssistantResponseOrError(chunks, errs)
	anchor := turn.AssistantVersions[0].ID
	newMsg := chat.Message{
		Seq:                turn.User.Seq,
		Role:               "assistant",
		Content:            content,
		VersionNumber:      len(turn.AssistantVersions) + 1,
		SnapshotProvider:   p.Name,
		SnapshotModel:      model.Model,
		SnapshotRoleName:   roleName,
		SnapshotRolePrompt: rolePrompt,
	}
	if _, err := m.store.AppendAssistantVersion(m.tabs[m.activeTab].autoSaveName, anchor, newMsg); err != nil {
		m.statusMsg = "Append version failed: " + err.Error()
		m.mode = modeMessageBrowse
		return m, nil
	}
	if err := m.rebuildBrowserStateAtSeq(turn.TurnSeq); err != nil {
		m.statusMsg = "Reload failed: " + err.Error()
		m.mode = modeMessageBrowse
		return m, nil
	}
	m.mode = modeMessageBrowse
	return m, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run:

```bash
env GOCACHE=/tmp/go-build go test ./internal/ui -run TestMessageBrowse_GCreatesAssistantVersionFromSelectedRegistryModel -count=1
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/ui/model.go internal/ui/messagebrowse.go internal/ui/messagebrowse_test.go internal/ui/messagebrowse_view_test.go internal/ui/update.go
git commit -m "feat: add new assistant version flow from model registry"
```

## Task 7: Surface provider/model labels and legacy-state guards in the browser

**Files:**
- Modify: `internal/ui/messagebrowse.go`
- Modify: `internal/ui/messagebrowse_view_test.go`
- Test: `internal/ui/messagebrowse_view_test.go`

- [ ] **Step 1: Write failing render tests for provider/model labels and disabled legacy regeneration**

Add these tests:

```go
func TestViewMessageBrowseShowsProviderAndModelLabels(t *testing.T) {
	m := newVersionedBrowseModel(t)
	m.mode = modeMessageBrowse
	m.messageBrowse.turns[0].AssistantVersions[0].SnapshotProvider = "gateway"
	m.messageBrowse.turns[0].AssistantVersions[0].SnapshotModel = "gemini-3-flash-preview"

	out := m.viewMessageBrowse()
	if !strings.Contains(out, "gateway") {
		t.Fatalf("missing provider label: %q", out)
	}
	if !strings.Contains(out, "gemini-3-flash-preview") {
		t.Fatalf("missing model label: %q", out)
	}
}
```

And a behavior test:

```go
func TestMessageBrowse_RDisabledForLegacyVersion(t *testing.T) {
	m := newVersionedBrowseModel(t)
	m.mode = modeMessageBrowse
	m.messageBrowse.turns[0].AssistantVersions[0].SnapshotProvider = ""
	m.messageBrowse.turns[0].AssistantVersions[0].SnapshotModel = ""
	m, _ = m.updateMessageBrowse(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")})
	if m.statusMsg == "" {
		t.Fatalf("statusMsg empty, want legacy warning")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run:

```bash
env GOCACHE=/tmp/go-build go test ./internal/ui -run 'Test(ViewMessageBrowseShowsProviderAndModelLabels|MessageBrowse_RDisabledForLegacyVersion)' -count=1
```

Expected: FAIL because provider/model labels are not rendered and legacy guard is incomplete.

- [ ] **Step 3: Render labels in message and compare mode headers**

In `internal/ui/messagebrowse.go`, build labels like:

```go
func assistantVersionTitle(msg chat.Message) string {
	label := fmt.Sprintf("v%d/%d", msg.VersionNumber, msg.TotalVersions)
	if msg.SnapshotProvider != "" || msg.SnapshotModel != "" {
		label += fmt.Sprintf(" %s / %s", msg.SnapshotProvider, msg.SnapshotModel)
	}
	return label
}
```

Use the helper in:

- compare card titles
- message-mode right pane title

Also make legacy warnings explicit:

```go
if !target.HasGenerationSnapshot() {
	m.statusMsg = "Legacy version cannot be regenerated"
	return m, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run:

```bash
env GOCACHE=/tmp/go-build go test ./internal/ui -run 'Test(ViewMessageBrowseShowsProviderAndModelLabels|MessageBrowse_RDisabledForLegacyVersion)' -count=1
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/ui/messagebrowse.go internal/ui/messagebrowse_view_test.go
git commit -m "feat: show model labels and guard legacy versions"
```

## Task 8: Full verification pass

**Files:**
- Modify: `internal/config/models.go`
- Modify: `internal/config/models_test.go`
- Modify: `internal/chat/history.go`
- Modify: `internal/storage/conversation.go`
- Modify: `internal/storage/conversation_test.go`
- Modify: `internal/ui/model.go`
- Modify: `internal/ui/update.go`
- Modify: `internal/ui/slashcomplete.go`
- Modify: `internal/ui/slashcomplete_test.go`
- Modify: `internal/ui/modelselector.go`
- Modify: `internal/ui/modelselector_test.go`
- Modify: `internal/ui/modelselector_view_test.go`
- Modify: `internal/ui/messagebrowse.go`
- Modify: `internal/ui/messagebrowse_test.go`
- Modify: `internal/ui/messagebrowse_view_test.go`
- Test: `internal/config/models_test.go`
- Test: `internal/storage/conversation_test.go`
- Test: `internal/ui/slashcomplete_test.go`
- Test: `internal/ui/modelselector_test.go`
- Test: `internal/ui/modelselector_view_test.go`
- Test: `internal/ui/messagebrowse_test.go`
- Test: `internal/ui/messagebrowse_view_test.go`
- Test: `internal/ui/statusbar_test.go`

- [ ] **Step 1: Run targeted package tests**

Run:

```bash
env GOCACHE=/tmp/go-build go test ./internal/config ./internal/storage ./internal/ui
```

Expected: PASS

- [ ] **Step 2: Run broader project tests**

Run:

```bash
env GOCACHE=/tmp/go-build go test ./...
```

Expected: PASS

- [ ] **Step 3: Manual smoke test checklist**

Verify in the app:

```text
1. Start a chat after selecting a role.
2. Run /model and switch current tab to a configured model.
3. Enter browser, press g, choose provider then model, confirm new version appears.
4. Press r on the preview version and confirm only that version changes.
5. Press e, modify the user message, choose save only, confirm versions remain and become stale.
6. Press e again, choose regenerate all versions, confirm each version updates or becomes Error: ...
7. Confirm compare mode and message mode both show provider/model labels.
8. Confirm a legacy row without snapshot blocks r and full e-regenerate but still allows g and delete.
```

- [ ] **Step 4: Commit final fixes**

```bash
git add internal/config/models.go internal/config/models_test.go internal/chat/history.go internal/storage/conversation.go internal/storage/conversation_test.go internal/ui/model.go internal/ui/update.go internal/ui/slashcomplete.go internal/ui/slashcomplete_test.go internal/ui/modelselector.go internal/ui/modelselector_test.go internal/ui/modelselector_view_test.go internal/ui/messagebrowse.go internal/ui/messagebrowse_test.go internal/ui/messagebrowse_view_test.go internal/ui/statusbar.go internal/ui/statusbar_test.go
git commit -m "feat: add model registry and snapshot driven regeneration"
```
