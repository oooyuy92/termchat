package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	xansi "github.com/charmbracelet/x/ansi"
)

// renderTabBar renders the tab bar and records hit zones for mouse interaction.
// Returns the rendered string (exactly 1 terminal line wide).
func (m *Model) renderTabBar() string {
	const maxNameRunes = 14
	const newBtn = " + "
	newBtnWidth := xansi.StringWidth(newBtn)

	type tabInfo struct {
		label string
		width int
	}
	infos := make([]tabInfo, len(m.tabs))
	for i, t := range m.tabs {
		name := t.name
		runes := []rune(name)
		if len(runes) > maxNameRunes {
			name = string(runes[:maxNameRunes-1]) + "…"
		}
		label := name + " ×"
		w := xansi.StringWidth(label) + 2 // +2 for padding(0,1) adds 1 on each side
		infos[i] = tabInfo{label: label, width: w}
	}

	// Determine how many tabs fit; reserve room for [+] and optionally […]
	overflowBtnWidth := xansi.StringWidth(" … 99 ")
	available := m.width - newBtnWidth
	visibleCount := 0
	used := 0
	for i, info := range infos {
		need := info.width
		remainingWidth := 0
		for _, fi := range infos[i+1:] {
			remainingWidth += fi.width
		}
		if remainingWidth > 0 && used+need+overflowBtnWidth > available {
			break
		}
		if used+need > available {
			break
		}
		used += need
		visibleCount++
	}
	if visibleCount == 0 && len(infos) > 0 {
		visibleCount = 1
	}
	hiddenCount := len(infos) - visibleCount

	var sb strings.Builder
	zones := make([]tabHitZone, 0, visibleCount*2+2)
	x := 0

	for i := 0; i < visibleCount; i++ {
		info := infos[i]
		startX := x

		var rendered string
		if i == m.activeTab {
			rendered = m.theme.TabActiveStyle().Render(info.label)
		} else {
			rendered = m.theme.TabInactiveStyle().Render(info.label)
		}
		sb.WriteString(rendered)

		endX := x + info.width
		// The "×" is the last 2 chars of label (space + ×), closeOffset points to where " ×" starts
		closeOffset := info.width - 2
		zones = append(zones, tabHitZone{
			startX: startX,
			endX:   startX + closeOffset - 1,
			action: tabHitSelect,
			tabIdx: i,
		})
		zones = append(zones, tabHitZone{
			startX: startX + closeOffset,
			endX:   endX - 1,
			action: tabHitClose,
			tabIdx: i,
		})
		x = endX
	}

	// [+] button
	newRendered := lipgloss.NewStyle().
		Background(lipgloss.Color(m.theme.TabBarBg)).
		Foreground(lipgloss.Color(m.theme.TabInactiveFg)).
		Render(newBtn)
	zones = append(zones, tabHitZone{startX: x, endX: x + newBtnWidth - 1, action: tabHitNew})
	sb.WriteString(newRendered)
	x += newBtnWidth

	// […] overflow button
	if hiddenCount > 0 {
		overflowLabel := fmt.Sprintf(" … %d ", hiddenCount)
		overflowW := xansi.StringWidth(overflowLabel)
		overflowRendered := lipgloss.NewStyle().
			Background(lipgloss.Color(m.theme.TabBarBg)).
			Foreground(lipgloss.Color(m.theme.TabInactiveFg)).
			Render(overflowLabel)
		zones = append(zones, tabHitZone{startX: x, endX: x + overflowW - 1, action: tabHitOverflow})
		sb.WriteString(overflowRendered)
	}

	m.tabBarZones = zones

	// Pad to full width
	bar := lipgloss.NewStyle().
		Background(lipgloss.Color(m.theme.TabBarBg)).
		Width(m.width).
		Render(sb.String())

	return bar
}

// hitTestTabBar returns the hit zone at column x on the tab bar row.
// Returns nil if no zone matches.
func (m *Model) hitTestTabBar(x int) *tabHitZone {
	for i := range m.tabBarZones {
		z := &m.tabBarZones[i]
		if x >= z.startX && x <= z.endX {
			return z
		}
	}
	return nil
}
