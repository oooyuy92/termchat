package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/termchat/termchat/internal/chat"
	"github.com/termchat/termchat/internal/config"
)

func makeTabModel(tabNames []string, activeTab int, width int) Model {
	tabs := make([]TabSession, len(tabNames))
	for i, name := range tabNames {
		tabs[i] = TabSession{
			name:    name,
			client:  &stubProvider{model: name},
			history: chat.NewHistory(),
		}
	}
	return Model{
		theme:     DarkTheme,
		tabs:      tabs,
		activeTab: activeTab,
		width:     width,
	}
}

func TestRenderTabBarNoOverflow(t *testing.T) {
	m := makeTabModel([]string{"gpt-4o", "claude-3"}, 0, 80)
	bar := m.renderTabBar()
	if bar == "" {
		t.Fatal("expected non-empty tab bar")
	}
	// 2 tabs × (select + close) = 4 zones + 1 new button = 5 minimum
	if len(m.tabBarZones) < 5 {
		t.Fatalf("expected at least 5 hit zones, got %d", len(m.tabBarZones))
	}
}

func TestRenderTabBarShowsOverflow(t *testing.T) {
	names := make([]string, 10)
	for i := range names {
		names[i] = "very-long-model-name"
	}
	m := makeTabModel(names, 0, 60)
	m.renderTabBar()
	hasOverflow := false
	for _, z := range m.tabBarZones {
		if z.action == tabHitOverflow {
			hasOverflow = true
			break
		}
	}
	if !hasOverflow {
		t.Fatal("expected overflow hit zone when tabs don't fit")
	}
}

func TestHitTestTabBarSelectFirstTab(t *testing.T) {
	m := makeTabModel([]string{"gpt-4o", "claude-3"}, 1, 80)
	m.renderTabBar()
	// Click at x=1 should hit the first tab (select zone, inside left padding)
	zone := m.hitTestTabBar(1)
	if zone == nil {
		t.Fatal("expected hit zone at x=1")
	}
	if zone.action != tabHitSelect || zone.tabIdx != 0 {
		t.Fatalf("expected tabHitSelect for tab 0, got action=%d tabIdx=%d", zone.action, zone.tabIdx)
	}
}

func TestHitTestTabBarMiss(t *testing.T) {
	m := makeTabModel([]string{"gpt-4o"}, 0, 80)
	m.renderTabBar()
	zone := m.hitTestTabBar(m.width + 5)
	if zone != nil {
		t.Fatal("expected nil for out-of-bounds x")
	}
}

func TestHitTestTabBarNewButton(t *testing.T) {
	m := makeTabModel([]string{"gpt-4o"}, 0, 80)
	m.renderTabBar()
	hasNew := false
	for _, z := range m.tabBarZones {
		if z.action == tabHitNew {
			hasNew = true
			// Test that clicking inside the + zone is detected
			zone := m.hitTestTabBar(z.startX)
			if zone == nil || zone.action != tabHitNew {
				t.Fatalf("hit test failed for new button at x=%d", z.startX)
			}
			break
		}
	}
	if !hasNew {
		t.Fatal("expected tabHitNew zone in tab bar")
	}
}

func TestRenderTabBarSingleTab(t *testing.T) {
	m := makeTabModel([]string{"my-model"}, 0, 20)
	bar := m.renderTabBar()
	if bar == "" {
		t.Fatal("expected non-empty bar even with very small width")
	}
	// With only 1 tab and small width, must still show at least the tab
	hasSelect := false
	for _, z := range m.tabBarZones {
		if z.action == tabHitSelect && z.tabIdx == 0 {
			hasSelect = true
			break
		}
	}
	if !hasSelect {
		t.Fatal("expected select zone for the single tab")
	}
}

func TestNewTabInheritsCurrentWindowDimensions(t *testing.T) {
	cfg := config.DefaultConfig()
	tab, err := newTabSession(cfg, &stubProvider{model: "test-model"}, 80)
	if err != nil {
		t.Fatalf("newTabSession() error = %v", err)
	}

	m := Model{
		cfg:       cfg,
		theme:     DarkTheme,
		tabs:      []TabSession{tab},
		activeTab: 0,
	}

	next, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = next.(Model)

	wantTextareaWidth := m.tabs[0].textarea.Width()
	wantViewportWidth := m.tabs[0].viewport.Width
	wantViewportHeight := m.tabs[0].viewport.Height

	m.newTab()

	got := m.tabs[m.activeTab]
	if got.textarea.Width() != wantTextareaWidth {
		t.Fatalf("new tab textarea width = %d, want %d", got.textarea.Width(), wantTextareaWidth)
	}
	if got.viewport.Width != wantViewportWidth {
		t.Fatalf("new tab viewport width = %d, want %d", got.viewport.Width, wantViewportWidth)
	}
	if got.viewport.Height != wantViewportHeight {
		t.Fatalf("new tab viewport height = %d, want %d", got.viewport.Height, wantViewportHeight)
	}
}

func TestMouseClickNewButtonWorksAfterView(t *testing.T) {
	cfg := config.DefaultConfig()
	tab, err := newTabSession(cfg, &stubProvider{model: "test-model"}, 80)
	if err != nil {
		t.Fatalf("newTabSession() error = %v", err)
	}

	m := Model{
		cfg:       cfg,
		theme:     DarkTheme,
		tabs:      []TabSession{tab},
		activeTab: 0,
	}

	next, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = next.(Model)

	// Runtime path: Bubble Tea renders the view before the user clicks.
	_ = m.View()

	var newZone *tabHitZone
	zones := m.computeTabBarZones()
	for i := range zones {
		if zones[i].action == tabHitNew {
			newZone = &zones[i]
			break
		}
	}
	if newZone == nil {
		t.Fatal("expected tabHitNew zone after computing tab layout")
	}

	next, _ = m.Update(tea.MouseMsg{
		X:      newZone.startX,
		Y:      0,
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonLeft,
	})
	updated := next.(Model)

	if len(updated.tabs) != 2 {
		t.Fatalf("expected click on new-tab button to append a tab, got %d tabs", len(updated.tabs))
	}
}

func TestMouseClickCloseButtonWorksAfterView(t *testing.T) {
	cfg := config.DefaultConfig()
	first, err := newTabSession(cfg, &stubProvider{model: "tab-1"}, 80)
	if err != nil {
		t.Fatalf("newTabSession() error = %v", err)
	}
	second, err := newTabSession(cfg, &stubProvider{model: "tab-2"}, 80)
	if err != nil {
		t.Fatalf("newTabSession() error = %v", err)
	}

	m := Model{
		cfg:       cfg,
		theme:     DarkTheme,
		tabs:      []TabSession{first, second},
		activeTab: 0,
	}

	next, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = next.(Model)
	_ = m.View()

	var closeZone *tabHitZone
	zones := m.computeTabBarZones()
	for i := range zones {
		if zones[i].action == tabHitClose && zones[i].tabIdx == 0 {
			closeZone = &zones[i]
			break
		}
	}
	if closeZone == nil {
		t.Fatal("expected close zone for first tab")
	}

	next, _ = m.Update(tea.MouseMsg{
		X:      closeZone.startX,
		Y:      0,
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonLeft,
	})
	updated := next.(Model)

	if len(updated.tabs) != 1 {
		t.Fatalf("expected click on close button to remove a tab, got %d tabs", len(updated.tabs))
	}
}
