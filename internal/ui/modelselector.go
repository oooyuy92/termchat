package ui

import (
	"fmt"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/termchat/termchat/internal/chat"
	"github.com/termchat/termchat/internal/config"
	"github.com/termchat/termchat/internal/storage"
)

func buildProviderEntryFields(entry config.ProviderEntry) []configField {
	return []configField{
		{Label: "Name", Key: "name", Value: entry.Name},
		{Label: "API Base URL", Key: "base_url", Value: entry.BaseURL},
		{Label: "API Key", Key: "api_key", Value: entry.APIKey, Masked: true},
	}
}

func buildModelEntryFields(entry config.ModelEntry, cfg config.Config) []configField {
	return []configField{
		{Label: "Model ID", Key: "model", Value: entry.Model},
		{Label: "API Format", Key: "api_format", Value: entry.APIFormat, Options: []string{"openai", "openai-compatible", "anthropic", "gemini"}},
		{Label: "Temperature", Key: "temperature", Value: strconv.FormatFloat(entry.Temperature, 'f', 2, 64)},
		{Label: "Max Tokens", Key: "max_tokens", Value: strconv.Itoa(entry.MaxTokens)},
		{Label: "Reasoning Effort", Key: "reasoning_effort", Value: entry.ReasoningEffort, Options: []string{"", "minimal", "low", "medium", "high"}},
		{Label: "Budget Tokens", Key: "budget_tokens", Value: strconv.Itoa(entry.BudgetTokens)},
	}
}

func defaultNewProviderEntry(count int, cfg config.Config) config.ProviderEntry {
	return config.ProviderEntry{
		Name:    fmt.Sprintf("provider-%d", count+1),
		BaseURL: strings.TrimSpace(cfg.API.BaseURL),
		APIKey:  cfg.API.APIKey,
	}
}

func nextAvailableProviderName(providers []config.ProviderEntry) string {
	for i := 1; ; i++ {
		name := fmt.Sprintf("provider-%d", i)
		if !providerNameExists(providers, name, -1) {
			return name
		}
	}
}

func nextAvailableModelName(models []config.ModelEntry) string {
	for i := 1; ; i++ {
		name := fmt.Sprintf("model-%d", i)
		if !modelNameExists(models, name, -1) {
			return name
		}
	}
}

func defaultNewModelEntry(count int, cfg config.Config) config.ModelEntry {
	modelID := fmt.Sprintf("model-%d", count+1)
	return config.ModelEntry{
		Name:            modelID,
		Model:           modelID,
		APIFormat:       cfg.API.Provider,
		Temperature:     cfg.Parameters.Temperature,
		MaxTokens:       cfg.Parameters.MaxTokens,
		ReasoningEffort: cfg.Parameters.ReasoningEffort,
		BudgetTokens:    cfg.Parameters.BudgetTokens,
	}
}

func applyFieldsToProviderEntry(entry *config.ProviderEntry, fields []configField) {
	for _, field := range fields {
		switch field.Key {
		case "name":
			entry.Name = strings.TrimSpace(field.Value)
		case "base_url":
			entry.BaseURL = strings.TrimSpace(field.Value)
		case "api_key":
			entry.APIKey = field.Value
		}
	}
}

func applyFieldsToModelEntry(entry *config.ModelEntry, fields []configField) {
	for _, field := range fields {
		switch field.Key {
		case "model":
			entry.Model = strings.TrimSpace(field.Value)
		case "api_format":
			entry.APIFormat = strings.TrimSpace(field.Value)
		case "temperature":
			f, _ := strconv.ParseFloat(field.Value, 64)
			entry.Temperature = f
		case "max_tokens":
			n, _ := strconv.Atoi(field.Value)
			entry.MaxTokens = n
		case "reasoning_effort":
			entry.ReasoningEffort = strings.ToLower(strings.TrimSpace(field.Value))
		case "budget_tokens":
			n, _ := strconv.Atoi(field.Value)
			entry.BudgetTokens = n
		}
	}
	entry.Name = entry.Model
}

func providerNameExists(providers []config.ProviderEntry, name string, skip int) bool {
	for i, provider := range providers {
		if i == skip {
			continue
		}
		if provider.Name == name {
			return true
		}
	}
	return false
}

func modelNameExists(models []config.ModelEntry, name string, skip int) bool {
	for i, model := range models {
		if i == skip {
			continue
		}
		if model.Name == name {
			return true
		}
	}
	return false
}

func (m *Model) selectedProviderEntry() *config.ProviderEntry {
	if len(m.modelSel.providers) == 0 {
		return nil
	}
	if m.modelSel.providerCursor < 0 || m.modelSel.providerCursor >= len(m.modelSel.providers) {
		return nil
	}
	return &m.modelSel.providers[m.modelSel.providerCursor]
}

func (m *Model) selectedModelEntry() *config.ModelEntry {
	provider := m.selectedProviderEntry()
	if provider == nil || len(provider.Models) == 0 {
		return nil
	}
	if m.modelSel.modelCursor < 0 || m.modelSel.modelCursor >= len(provider.Models) {
		return nil
	}
	return &provider.Models[m.modelSel.modelCursor]
}

func (m Model) newProviderClient(apiFormat, baseURL, apiKey, model string) chat.Provider {
	if m.providerFactory != nil {
		return m.providerFactory(apiFormat, baseURL, apiKey, model)
	}
	return chat.NewProvider(apiFormat, baseURL, apiKey, model)
}

func lookupModelEntry(provider config.ProviderEntry, modelName string) (config.ModelEntry, bool) {
	for _, model := range provider.Models {
		if model.Name == modelName {
			return model, true
		}
	}
	return config.ModelEntry{}, false
}

func (m *Model) validateProviderEntry(entry config.ProviderEntry, skip int) error {
	if strings.TrimSpace(entry.Name) == "" {
		return fmt.Errorf("provider name is empty")
	}
	if providerNameExists(m.modelSel.providers, entry.Name, skip) {
		return fmt.Errorf("provider name %q already exists", entry.Name)
	}
	return nil
}

func (m *Model) validateModelEntry(provider *config.ProviderEntry, entry config.ModelEntry, skip int) error {
	if provider == nil {
		return fmt.Errorf("no provider selected")
	}
	if err := validateGenerationModelEntry(entry); err != nil {
		return err
	}
	if modelNameExists(provider.Models, entry.Name, skip) {
		return fmt.Errorf("model id %q already exists under provider %q", entry.Name, provider.Name)
	}
	return nil
}

func (m *Model) applyModelSelection(provider config.ProviderEntry, model config.ModelEntry, persist bool) error {
	tab := m.activeTabSession()
	if tab == nil {
		return fmt.Errorf("no active tab")
	}
	if err := validateGenerationModelEntry(model); err != nil {
		return err
	}

	apiFormat := model.APIFormat
	tab.client = m.newProviderClient(apiFormat, provider.BaseURL, provider.APIKey, model.Model)
	tab.providerConfigName = provider.Name
	tab.apiFormat = apiFormat
	tab.modelConfigName = model.Name
	tab.name = model.Model

	if persist && m.store != nil && tab.autoSaveName != "" {
		if err := m.store.SetConversationModelBinding(tab.autoSaveName, provider.Name, model.Name); err != nil {
			return err
		}
	}

	return nil
}

func (m *Model) restoreConversationBindingToActiveTab(convName string) error {
	if m.store == nil {
		return nil
	}
	providerName, modelName, err := m.store.GetConversationModelBinding(convName)
	if err != nil {
		if err == storage.ErrNotFound {
			return nil
		}
		return err
	}
	if providerName == "" || modelName == "" {
		return nil
	}

	provider, ok := m.lookupProviderConfig(providerName)
	if !ok {
		return fmt.Errorf("provider %q not found", providerName)
	}
	model, ok := lookupModelEntry(provider, modelName)
	if !ok {
		return fmt.Errorf("model %q not found under provider %q", modelName, providerName)
	}
	return m.applyModelSelection(provider, model, false)
}

func (m *Model) saveModelRegistryState() error {
	m.modelRegistry = config.ModelRegistry{Providers: append([]config.ProviderEntry(nil), m.modelSel.providers...)}
	if m.modelRegistryPath == "" {
		return nil
	}
	return config.SaveModelRegistry(m.modelRegistryPath, m.modelRegistry)
}

func (m *Model) updateTabsForProvider(provider config.ProviderEntry, oldProviderName string) {
	for i := range m.tabs {
		tab := &m.tabs[i]
		if tab.providerConfigName != provider.Name && tab.providerConfigName != oldProviderName {
			continue
		}
		if oldProviderName != "" && tab.providerConfigName == oldProviderName {
			tab.providerConfigName = provider.Name
		}
		model, ok := lookupModelEntry(provider, tab.modelConfigName)
		if !ok {
			continue
		}
		tab.client = m.newProviderClient(model.APIFormat, provider.BaseURL, provider.APIKey, model.Model)
		tab.apiFormat = model.APIFormat
		tab.name = model.Model
	}
}

func (m *Model) renameSnapshotsInTabs(oldProviderName, newProviderName string) {
	if oldProviderName == "" || newProviderName == "" || oldProviderName == newProviderName {
		return
	}
	for i := range m.tabs {
		msgs := append([]chat.Message(nil), m.tabs[i].history.Messages()...)
		changed := false
		for j := range msgs {
			if msgs[j].SnapshotProvider == oldProviderName {
				msgs[j].SnapshotProvider = newProviderName
				changed = true
			}
		}
		if changed {
			m.tabs[i].history.ReplaceMessages(msgs)
		}
	}
}

func (m *Model) syncProviderEdit(oldProviderName string, provider config.ProviderEntry) error {
	m.updateTabsForProvider(provider, oldProviderName)
	m.renameSnapshotsInTabs(oldProviderName, provider.Name)
	if m.store == nil {
		return nil
	}
	if err := m.store.RenameConversationProviderBindings(oldProviderName, provider.Name); err != nil {
		return err
	}
	if err := m.store.RenameSnapshotProvider(oldProviderName, provider.Name); err != nil {
		return err
	}
	return nil
}

func (m *Model) syncModelEdit(provider config.ProviderEntry, oldModelName string, model config.ModelEntry) error {
	for i := range m.tabs {
		tab := &m.tabs[i]
		if tab.providerConfigName != provider.Name {
			continue
		}
		if tab.modelConfigName != oldModelName && tab.modelConfigName != model.Name {
			continue
		}
		tab.modelConfigName = model.Name
		tab.client = m.newProviderClient(model.APIFormat, provider.BaseURL, provider.APIKey, model.Model)
		tab.apiFormat = model.APIFormat
		tab.name = model.Model
	}
	if m.store == nil {
		return nil
	}
	return m.store.RenameConversationModelBindings(provider.Name, oldModelName, model.Name)
}

func (m *Model) createModelSelectorProvider() error {
	entry := defaultNewProviderEntry(len(m.modelSel.providers), m.cfg)
	entry.Name = nextAvailableProviderName(m.modelSel.providers)
	if err := m.validateProviderEntry(entry, -1); err != nil {
		return err
	}
	m.modelSel.providers = append(m.modelSel.providers, entry)
	m.modelSel.providerCursor = len(m.modelSel.providers) - 1
	m.modelSel.selectingModels = false
	return m.saveModelRegistryState()
}

func (m *Model) createModelSelectorModel() error {
	provider := m.selectedProviderEntry()
	if provider == nil {
		return fmt.Errorf("no provider selected")
	}

	entry := defaultNewModelEntry(len(provider.Models), m.cfg)
	entry.Name = nextAvailableModelName(provider.Models)
	entry.Model = entry.Name
	if err := m.validateModelEntry(provider, entry, -1); err != nil {
		return err
	}
	provider.Models = append(provider.Models, entry)
	m.modelSel.modelCursor = len(provider.Models) - 1
	m.modelSel.selectingModels = true
	return m.saveModelRegistryState()
}

func (m *Model) beginModelSelectorEdit(targetModel bool, isNew bool) {
	m.modelSel.editing = true
	m.modelSel.editTargetModel = targetModel
	m.modelSel.editIsNew = isNew
	m.modelSel.editDirty = false
	m.modelSel.editReadyToSave = false

	if targetModel {
		provider := m.selectedProviderEntry()
		entry := defaultNewModelEntry(0, m.cfg)
		if provider != nil {
			entry = defaultNewModelEntry(len(provider.Models), m.cfg)
			if !isNew && m.modelSel.modelCursor >= 0 && m.modelSel.modelCursor < len(provider.Models) {
				entry = provider.Models[m.modelSel.modelCursor]
			}
		}
		m.modelSel.editor = configEditor{fields: buildModelEntryFields(entry, m.cfg)}
		return
	}

	entry := defaultNewProviderEntry(len(m.modelSel.providers), m.cfg)
	if !isNew && m.modelSel.providerCursor >= 0 && m.modelSel.providerCursor < len(m.modelSel.providers) {
		entry = m.modelSel.providers[m.modelSel.providerCursor]
	}
	m.modelSel.editor = configEditor{fields: buildProviderEntryFields(entry)}
}

func (m *Model) saveModelSelectorEdit() error {
	if m.modelSel.editTargetModel {
		provider := m.selectedProviderEntry()
		if provider == nil {
			return fmt.Errorf("no provider selected")
		}
		oldModelName := ""
		if m.modelSel.modelCursor >= 0 && m.modelSel.modelCursor < len(provider.Models) {
			oldModelName = provider.Models[m.modelSel.modelCursor].Name
		}
		entry := config.ModelEntry{}
		applyFieldsToModelEntry(&entry, m.modelSel.editor.fields)
		skip := -1
		if !m.modelSel.editIsNew {
			skip = m.modelSel.modelCursor
		}
		if err := m.validateModelEntry(provider, entry, skip); err != nil {
			return err
		}
		if m.modelSel.editIsNew {
			provider.Models = append(provider.Models, entry)
			m.modelSel.modelCursor = len(provider.Models) - 1
		} else if m.modelSel.modelCursor >= 0 && m.modelSel.modelCursor < len(provider.Models) {
			provider.Models[m.modelSel.modelCursor] = entry
		}
		m.modelSel.selectingModels = true
		if err := m.saveModelRegistryState(); err != nil {
			return err
		}
		if !m.modelSel.editIsNew {
			if err := m.syncModelEdit(*provider, oldModelName, provider.Models[m.modelSel.modelCursor]); err != nil {
				return err
			}
		}
	} else {
		oldProviderName := ""
		if m.modelSel.providerCursor >= 0 && m.modelSel.providerCursor < len(m.modelSel.providers) {
			oldProviderName = m.modelSel.providers[m.modelSel.providerCursor].Name
		}
		entry := config.ProviderEntry{}
		applyFieldsToProviderEntry(&entry, m.modelSel.editor.fields)
		skip := -1
		if !m.modelSel.editIsNew {
			skip = m.modelSel.providerCursor
		}
		if err := m.validateProviderEntry(entry, skip); err != nil {
			return err
		}
		if m.modelSel.editIsNew {
			m.modelSel.providers = append(m.modelSel.providers, entry)
			m.modelSel.providerCursor = len(m.modelSel.providers) - 1
		} else if m.modelSel.providerCursor >= 0 && m.modelSel.providerCursor < len(m.modelSel.providers) {
			existingModels := m.modelSel.providers[m.modelSel.providerCursor].Models
			entry.Models = existingModels
			m.modelSel.providers[m.modelSel.providerCursor] = entry
		}
		m.modelSel.selectingModels = false
		m.modelSel.modelCursor = 0
		if err := m.saveModelRegistryState(); err != nil {
			return err
		}
		if !m.modelSel.editIsNew {
			if err := m.syncProviderEdit(oldProviderName, m.modelSel.providers[m.modelSel.providerCursor]); err != nil {
				return err
			}
		}
	}
	return nil
}

func (m *Model) resetModelSelectorEditorAfterSave() {
	m.modelSel.editDirty = false
	m.modelSel.editReadyToSave = false
}

func (m *Model) closeModelSelectorEditor() {
	m.modelSel.editing = false
	m.modelSel.editDirty = false
	m.modelSel.editReadyToSave = false
	m.modelSel.editor = configEditor{}
}

func (m *Model) deleteModelSelectorSelection() error {
	if m.modelSel.selectingModels {
		provider := m.selectedProviderEntry()
		if provider == nil || len(provider.Models) == 0 {
			return nil
		}
		if m.activeTabSession().providerConfigName == provider.Name &&
			m.activeTabSession().modelConfigName == provider.Models[m.modelSel.modelCursor].Name {
			return fmt.Errorf("cannot delete active session model")
		}
		provider.Models = append(provider.Models[:m.modelSel.modelCursor], provider.Models[m.modelSel.modelCursor+1:]...)
		if m.modelSel.modelCursor >= len(provider.Models) && m.modelSel.modelCursor > 0 {
			m.modelSel.modelCursor--
		}
		if len(provider.Models) == 0 {
			m.modelSel.modelCursor = 0
			m.modelSel.selectingModels = false
		}
	} else {
		if len(m.modelSel.providers) == 0 {
			return nil
		}
		if m.activeTabSession().providerConfigName == m.modelSel.providers[m.modelSel.providerCursor].Name {
			return fmt.Errorf("cannot delete active session provider")
		}
		m.modelSel.providers = append(m.modelSel.providers[:m.modelSel.providerCursor], m.modelSel.providers[m.modelSel.providerCursor+1:]...)
		if m.modelSel.providerCursor >= len(m.modelSel.providers) && m.modelSel.providerCursor > 0 {
			m.modelSel.providerCursor--
		}
		m.modelSel.modelCursor = 0
		m.modelSel.selectingModels = false
	}
	return m.saveModelRegistryState()
}

func (m Model) updateModelSelectorEditor(msg tea.KeyMsg) (Model, tea.Cmd) {
	ed := &m.modelSel.editor
	if ed.editing {
		switch msg.String() {
	case "enter":
		field := &ed.fields[ed.cursor]
			errMsg := validateField(field.Key, ed.editBuf)
			if field.Key == "name" && strings.TrimSpace(ed.editBuf) == "" {
				errMsg = "must not be empty"
			}
			if errMsg != "" {
				ed.editErr = errMsg
				return m, nil
			}
			field.Value = ed.editBuf
			ed.editing = false
			ed.editErr = ""
			m.modelSel.editDirty = true
			m.modelSel.editReadyToSave = ed.cursor == len(ed.fields)-1
			for _, field := range ed.fields {
				if field.Key == "name" && strings.TrimSpace(field.Value) == "" {
					ed.editErr = "must not be empty"
					return m, nil
				}
				if errMsg := validateField(field.Key, field.Value); errMsg != "" {
					ed.editErr = errMsg
					return m, nil
				}
			}
			if err := m.saveModelSelectorEdit(); err != nil {
				m.statusMsg = "Save failed: " + err.Error()
				return m, nil
			}
			m.resetModelSelectorEditorAfterSave()
			m.statusMsg = "Model registry saved"
			return m, nil
		case "esc":
			ed.editing = false
			ed.editErr = ""
			m.modelSel.editReadyToSave = false
			m.statusMsg = "Edit canceled"
			return m, nil
		case "backspace", "ctrl+h":
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

	switch msg.String() {
	case "up", "k":
		if ed.cursor > 0 {
			ed.cursor--
		}
		m.modelSel.editReadyToSave = false
	case "down", "j":
		if ed.cursor < len(ed.fields)-1 {
			ed.cursor++
		}
		m.modelSel.editReadyToSave = false
	case "enter":
		if m.modelSel.editReadyToSave && ed.cursor == len(ed.fields)-1 {
			for _, field := range ed.fields {
				if field.Key == "name" && strings.TrimSpace(field.Value) == "" {
					ed.editErr = "must not be empty"
					return m, nil
				}
				if errMsg := validateField(field.Key, field.Value); errMsg != "" {
					ed.editErr = errMsg
					return m, nil
				}
			}
			if err := m.saveModelSelectorEdit(); err != nil {
				m.statusMsg = "Save failed: " + err.Error()
				return m, nil
			}
			m.resetModelSelectorEditorAfterSave()
			m.statusMsg = "Model registry saved"
			return m, nil
		}
		field := &ed.fields[ed.cursor]
		if len(field.Options) > 0 {
			idx := 0
			for i, opt := range field.Options {
				if opt == field.Value {
					idx = i
					break
				}
			}
			idx = (idx + 1) % len(field.Options)
			field.Value = field.Options[idx]
			m.modelSel.editDirty = true
			m.modelSel.editReadyToSave = ed.cursor == len(ed.fields)-1
			for _, field := range ed.fields {
				if field.Key == "name" && strings.TrimSpace(field.Value) == "" {
					ed.editErr = "must not be empty"
					return m, nil
				}
				if errMsg := validateField(field.Key, field.Value); errMsg != "" {
					ed.editErr = errMsg
					return m, nil
				}
			}
			if err := m.saveModelSelectorEdit(); err != nil {
				m.statusMsg = "Save failed: " + err.Error()
				return m, nil
			}
			m.resetModelSelectorEditorAfterSave()
			m.statusMsg = "Model registry saved"
			return m, nil
		}
		ed.editing = true
		ed.editBuf = field.Value
		ed.editErr = ""
		m.modelSel.editReadyToSave = false
	case "esc":
		m.closeModelSelectorEditor()
		m.statusMsg = "Edit canceled"
		return m, nil
	}

	return m, nil
}

func (m Model) updateModelSelector(msg tea.KeyMsg) (Model, tea.Cmd) {
	if m.modelSel.editing {
		return m.updateModelSelectorEditor(msg)
	}

	switch msg.String() {
	case "esc":
		m.mode = modeChat
		return m, nil
	case "n":
		if m.modelSel.purpose != modelSelectorManage {
			return m, nil
		}
		if m.modelSel.selectingModels {
			if err := m.createModelSelectorModel(); err != nil {
				m.statusMsg = "Create failed: " + err.Error()
				return m, nil
			}
			m.beginModelSelectorEdit(true, false)
			m.statusMsg = "Model created"
			return m, nil
		}
		if err := m.createModelSelectorProvider(); err != nil {
			m.statusMsg = "Create failed: " + err.Error()
			return m, nil
		}
		m.beginModelSelectorEdit(false, false)
		m.statusMsg = "Provider created"
		return m, nil
	case "e":
		if m.modelSel.purpose != modelSelectorManage {
			return m, nil
		}
		if m.modelSel.selectingModels {
			if provider := m.selectedProviderEntry(); provider == nil || len(provider.Models) == 0 {
				return m, nil
			}
		} else if len(m.modelSel.providers) == 0 {
			return m, nil
		}
		m.beginModelSelectorEdit(m.modelSel.selectingModels, false)
		return m, nil
	case "d":
		if m.modelSel.purpose != modelSelectorManage {
			return m, nil
		}
		if err := m.deleteModelSelectorSelection(); err != nil {
			m.statusMsg = "Delete failed: " + err.Error()
			return m, nil
		}
		m.statusMsg = "Deleted"
		return m, nil
	case "left", "h":
		m.modelSel.selectingModels = false
		return m, nil
	case "right", "l":
		if provider := m.selectedProviderEntry(); provider != nil && (len(provider.Models) > 0 || m.modelSel.purpose == modelSelectorManage) {
			m.modelSel.selectingModels = true
		}
		return m, nil
	case "up", "k":
		if m.modelSel.selectingModels {
			if m.modelSel.modelCursor > 0 {
				m.modelSel.modelCursor--
			}
			return m, nil
		}
		if m.modelSel.providerCursor > 0 {
			m.modelSel.providerCursor--
			m.modelSel.modelCursor = 0
		}
		return m, nil
	case "down", "j":
		if m.modelSel.selectingModels {
			if provider := m.selectedProviderEntry(); provider != nil && m.modelSel.modelCursor < len(provider.Models)-1 {
				m.modelSel.modelCursor++
			}
			return m, nil
		}
		if m.modelSel.providerCursor < len(m.modelSel.providers)-1 {
			m.modelSel.providerCursor++
			m.modelSel.modelCursor = 0
		}
		return m, nil
	case "enter":
		if !m.modelSel.selectingModels {
			return m, nil
		}
		provider := m.selectedProviderEntry()
		model := m.selectedModelEntry()
		if provider == nil || model == nil {
			return m, nil
		}
		if m.modelSel.purpose == modelSelectorPickForNewVersion {
			return m.appendAssistantVersionFromSelection(*provider, *model)
		}
		if err := m.applyModelSelection(*provider, *model, true); err != nil {
			m.statusMsg = "Switch failed: " + err.Error()
			return m, nil
		}
		m.statusMsg = fmt.Sprintf("Switched to %s / %s", provider.Name, model.Model)
		m.mode = modeChat
		return m, nil
	}

	return m, nil
}

func (m Model) viewModelSelector() string {
	tabBar := (&m).renderTabBar()
	statusBar := ""
	if len(m.tabs) > 0 {
		statusBar = m.renderStatusBar()
	}

	var b strings.Builder
	b.WriteString(m.theme.ConfigTitleStyle().Render("Model Selector"))
	b.WriteString("\n\n")

	if m.modelSel.editing {
		for i, field := range m.modelSel.editor.fields {
			cursor := "  "
			if i == m.modelSel.editor.cursor {
				cursor = m.theme.ConfigCursorStyle().Render("> ")
			}
			value := field.Value
			if field.Masked && !m.modelSel.editor.editing {
				value = maskValue(value)
			}
			if i == m.modelSel.editor.cursor && m.modelSel.editor.editing {
				value = m.theme.ConfigEditStyle().Render(m.modelSel.editor.editBuf + "\u2588")
			} else {
				value = m.theme.ConfigValueStyle().Render(value)
			}
			b.WriteString(cursor + m.theme.ConfigLabelStyle().Render(field.Label+": ") + value + "\n")
		}
		b.WriteString("\n")
		if m.modelSel.editor.editErr != "" {
			b.WriteString(m.theme.ConfigErrStyle().Render("  Error: " + m.modelSel.editor.editErr))
			b.WriteString("\n\n")
		}
		b.WriteString(m.theme.ConfigHelpStyle().Render("  ↑↓: move  Enter: edit/cycle/save  Esc: cancel"))
		body := b.String()
		if statusBar != "" {
			return lipgloss.JoinVertical(lipgloss.Left, tabBar, body, statusBar)
		}
		return lipgloss.JoinVertical(lipgloss.Left, tabBar, body)
	}

	if len(m.modelSel.providers) == 0 {
		b.WriteString(m.theme.ConfigHelpStyle().Render("  No configured providers."))
		b.WriteString("\n\n")
		if m.modelSel.purpose == modelSelectorManage {
			b.WriteString(m.theme.ConfigHelpStyle().Render("  n: new provider  Esc: back"))
		} else {
			b.WriteString(m.theme.ConfigHelpStyle().Render("  Esc: back"))
		}
		body := b.String()
		if statusBar != "" {
			return lipgloss.JoinVertical(lipgloss.Left, tabBar, body, statusBar)
		}
		return lipgloss.JoinVertical(lipgloss.Left, tabBar, body)
	}

	providerPaneWidth := m.width / 2
	if providerPaneWidth < 24 {
		providerPaneWidth = 24
	}
	modelPaneWidth := m.width - providerPaneWidth - 3
	if modelPaneWidth < 24 {
		modelPaneWidth = 24
	}

	var providerLines []string
	providerLines = append(providerLines, m.theme.ConfigLabelStyle().Render("Providers"))
	for i, provider := range m.modelSel.providers {
		cursor := "  "
		if !m.modelSel.selectingModels && i == m.modelSel.providerCursor {
			cursor = m.theme.ConfigCursorStyle().Render("> ")
		}
		host := provider.BaseURL
		host = strings.TrimPrefix(host, "https://")
		host = strings.TrimPrefix(host, "http://")
		host = strings.TrimSuffix(host, "/")
		line := cursor + provider.Name
		if host != "" {
			line += "  " + m.theme.ConfigHelpStyle().Render(host)
		}
		providerLines = append(providerLines, line)
	}

	var modelLines []string
	modelLines = append(modelLines, m.theme.ConfigLabelStyle().Render("Models"))
	if provider := m.selectedProviderEntry(); provider != nil {
		if len(provider.Models) == 0 {
			prefix := "  "
			if m.modelSel.selectingModels {
				prefix = m.theme.ConfigCursorStyle().Render("> ")
			}
			modelLines = append(modelLines, prefix+m.theme.ConfigHelpStyle().Render("No models. Press n to add one."))
		}
		for j, model := range provider.Models {
			cursor := "  "
			if m.modelSel.selectingModels && j == m.modelSel.modelCursor {
				cursor = m.theme.ConfigCursorStyle().Render("> ")
			}
			line := cursor + model.Name
			if model.Model != "" {
				line = cursor + model.Model
			}
			meta := model.Model
			if model.APIFormat != "" {
				if meta == "" {
					meta = model.APIFormat
				} else {
					meta += " [" + model.APIFormat + "]"
				}
			}
			modelLines = append(modelLines, line)
			if meta != "" && meta != model.Model {
				modelLines = append(modelLines, "   "+m.theme.ConfigHelpStyle().Render(meta))
			}
		}
	}

	left := lipgloss.NewStyle().Width(providerPaneWidth).Render(strings.Join(providerLines, "\n"))
	right := lipgloss.NewStyle().Width(modelPaneWidth).Render(strings.Join(modelLines, "\n"))
	b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, left, "   ", right))

	b.WriteString("\n")
	help := "  ←→: focus  ↑↓: move  Enter: select  Esc: back"
	if m.modelSel.purpose == modelSelectorManage {
		help = "  ←→: focus  ↑↓: move  n: new  e: edit  d: delete  Enter: select  Esc: back"
	}
	b.WriteString(m.theme.ConfigHelpStyle().Render(help))

	body := b.String()
	if statusBar != "" {
		return lipgloss.JoinVertical(lipgloss.Left, tabBar, body, statusBar)
	}
	return lipgloss.JoinVertical(lipgloss.Left, tabBar, body)
}
