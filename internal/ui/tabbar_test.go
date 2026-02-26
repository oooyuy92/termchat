package ui

import (
	"testing"

	"github.com/termchat/termchat/internal/chat"
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
