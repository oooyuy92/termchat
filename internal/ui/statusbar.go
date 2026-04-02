// internal/ui/statusbar.go
package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	xansi "github.com/charmbracelet/x/ansi"
)

func (m Model) renderStatusBar() string {
	barStyle := lipgloss.NewStyle().
		Background(lipgloss.Color(m.theme.StatusBarBg)).
		Foreground(lipgloss.Color(m.theme.StatusBarFg))

	if m.mode == modeTabRename {
		prompt := "Rename tab: " + m.tabRename + "█"
		lines := packStyledLines([]string{prompt}, m.width, 2)
		for i, line := range lines {
			if m.width > 0 {
				lines[i] = barStyle.Width(m.width).Render(line)
			} else {
				lines[i] = barStyle.Render(line)
			}
		}
		return strings.Join(lines, "\n")
	}

	tab := m.activeTabSession()
	model := m.theme.StatusKeyStyle().Render("model") + m.theme.StatusBarStyle().Render(tab.client.Model())
	tokens := m.theme.StatusKeyStyle().Render("tokens") + m.theme.StatusBarStyle().Render(fmt.Sprintf("%d", tab.totalTokens))
	msgs := m.theme.StatusKeyStyle().Render("msgs") + m.theme.StatusBarStyle().Render(fmt.Sprintf("%d", tab.history.Count()))

	segments := []string{model, tokens, msgs}
	if m.activeRole != "" {
		role := m.theme.StatusKeyStyle().Render("角色") + m.theme.StatusBarStyle().Render(m.activeRole)
		segments = append(segments, role)
	}

	if m.statusMsg != "" {
		segments = append(segments, m.theme.StatusBarStyle().Render(m.statusMsg))
	}

	lines := packStyledLines(segments, m.width, 2)
	for i, line := range lines {
		if m.width > 0 {
			lines[i] = barStyle.Width(m.width).Render(line)
		} else {
			lines[i] = barStyle.Render(line)
		}
	}

	return strings.Join(lines, "\n")
}

func packStyledLines(segments []string, width, maxLines int) []string {
	if len(segments) == 0 {
		return []string{""}
	}
	if width <= 0 {
		return []string{strings.Join(segments, " ")}
	}

	var lines []string
	current := ""
	for _, segment := range segments {
		candidate := segment
		if current != "" {
			candidate = current + " " + segment
		}
		if current == "" && xansi.StringWidth(segment) > width {
			lines = append(lines, xansi.TruncateWc(segment, width, ""))
			continue
		}
		if xansi.StringWidth(candidate) <= width {
			current = candidate
			continue
		}
		if current != "" {
			lines = append(lines, current)
		}
		current = segment
	}
	if current != "" {
		lines = append(lines, current)
	}

	if maxLines > 0 && len(lines) > maxLines {
		overflow := lines[maxLines-1]
		for _, segment := range lines[maxLines:] {
			overflow += " " + segment
		}
		lines = append(lines[:maxLines-1], xansi.TruncateWc(overflow, width, ""))
	}
	return lines
}
