package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func (m Model) updateTabOverflow(msg tea.KeyMsg) (Model, tea.Cmd) {
	// Count how many tabs are visible (from hit zones)
	zones := m.computeTabBarZones()
	visibleCount := 0
	for _, z := range zones {
		if z.action == tabHitSelect && z.tabIdx+1 > visibleCount {
			visibleCount = z.tabIdx + 1
		}
	}
	n := len(m.tabs) - visibleCount

	switch msg.String() {
	case "esc", "alt+0":
		m.mode = modeChat
		m.tabOverflowOffset = 0
	case "up", "k":
		if m.tabOverflowOffset > 0 {
			m.tabOverflowOffset--
		}
	case "down", "j":
		if m.tabOverflowOffset < n-1 {
			m.tabOverflowOffset++
		}
	case "enter":
		m.switchTab(visibleCount + m.tabOverflowOffset)
		m.tabOverflowOffset = 0
		m.mode = modeChat
	}
	return m, nil
}

func (m Model) viewTabOverflow() string {
	// Count visible tabs from hit zones
	zones := m.computeTabBarZones()
	visibleCount := 0
	for _, z := range zones {
		if z.action == tabHitSelect && z.tabIdx+1 > visibleCount {
			visibleCount = z.tabIdx + 1
		}
	}
	hiddenTabs := m.tabs[visibleCount:]

	var lines []string
	for i, t := range hiddenTabs {
		cursor := "  "
		if i == m.tabOverflowOffset {
			cursor = "> "
		}
		lines = append(lines, fmt.Sprintf("%s[%d] %s", cursor, visibleCount+i+1, t.name))
	}
	dropdown := strings.Join(lines, "\n")

	tabBar := (&m).renderTabBar()
	statusBar := m.renderStatusBar()
	inputArea := m.tabs[m.activeTab].textarea.View()
	vpHeight := m.height -
		lipgloss.Height(tabBar) -
		lipgloss.Height(statusBar) -
		lipgloss.Height(inputArea) -
		lipgloss.Height(dropdown)
	if vpHeight < 1 {
		vpHeight = 1
	}
	m.tabs[m.activeTab].viewport.Height = vpHeight

	parts := []string{tabBar, m.tabs[m.activeTab].viewport.View(), dropdown, inputArea, statusBar}
	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}
