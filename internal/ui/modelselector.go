package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/termchat/termchat/internal/chat"
	"github.com/termchat/termchat/internal/config"
)

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

func (m Model) newProviderClient(providerType, baseURL, apiKey, model string) chat.Provider {
	if m.providerFactory != nil {
		return m.providerFactory(providerType, baseURL, apiKey, model)
	}
	return chat.NewProvider(providerType, baseURL, apiKey, model)
}

func (m Model) updateModelSelector(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		if m.modelSel.selectingModels {
			m.modelSel.selectingModels = false
			return m, nil
		}
		m.mode = modeChat
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
			m.modelSel.selectingModels = true
			return m, nil
		}
		provider := m.selectedProviderEntry()
		model := m.selectedModelEntry()
		if provider == nil || model == nil {
			return m, nil
		}
		m.tabs[m.activeTab].client = m.newProviderClient(provider.Provider, provider.BaseURL, provider.APIKey, model.Model)
		m.tabs[m.activeTab].name = model.Model
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

	if len(m.modelSel.providers) == 0 {
		b.WriteString(m.theme.ConfigHelpStyle().Render("  No configured providers."))
		b.WriteString("\n\n")
		b.WriteString(m.theme.ConfigHelpStyle().Render("  Esc: back"))
		body := b.String()
		if statusBar != "" {
			return lipgloss.JoinVertical(lipgloss.Left, tabBar, body, statusBar)
		}
		return lipgloss.JoinVertical(lipgloss.Left, tabBar, body)
	}

	for i, provider := range m.modelSel.providers {
		cursor := "  "
		if !m.modelSel.selectingModels && i == m.modelSel.providerCursor {
			cursor = m.theme.ConfigCursorStyle().Render("> ")
		}
		b.WriteString(cursor + provider.Name + " [" + provider.Provider + "]\n")
		if i == m.modelSel.providerCursor {
			for j, model := range provider.Models {
				modelCursor := "    "
				if m.modelSel.selectingModels && j == m.modelSel.modelCursor {
					modelCursor = m.theme.ConfigCursorStyle().Render("  -> ")
				}
				b.WriteString(modelCursor + model.Name + " (" + model.Model + ")\n")
			}
		}
	}

	b.WriteString("\n")
	b.WriteString(m.theme.ConfigHelpStyle().Render("  ↑↓: move  Enter: select  Esc: back"))

	body := b.String()
	if statusBar != "" {
		return lipgloss.JoinVertical(lipgloss.Left, tabBar, body, statusBar)
	}
	return lipgloss.JoinVertical(lipgloss.Left, tabBar, body)
}
