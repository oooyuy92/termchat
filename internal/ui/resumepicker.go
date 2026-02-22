// internal/ui/resumepicker.go
package ui

import (
	"strings"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/termchat/termchat/internal/storage"
)

// buildResumePicker groups ConvInfo by date (preserving most-recent-first order)
// and returns an initialized resumePicker.
func buildResumePicker(convs []storage.ConvInfo) resumePicker {
	if len(convs) == 0 {
		return resumePicker{}
	}
	var groups []dateGroup
	groupIdx := map[string]int{}
	for _, c := range convs {
		idx, ok := groupIdx[c.Date]
		if !ok {
			idx = len(groups)
			groups = append(groups, dateGroup{date: c.Date})
			groupIdx[c.Date] = idx
		}
		groups[idx].convs = append(groups[idx].convs, c)
	}
	return resumePicker{groups: groups}
}

// truncate shortens s to at most n runes, appending "…" if truncated.
func truncate(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	runes := []rune(s)
	return string(runes[:n]) + "…"
}

func (m Model) updateResumeMode(msg tea.KeyMsg) (Model, tea.Cmd) {
	p := &m.resumePick

	if msg.String() != "ctrl+c" {
		m.confirmQuit = false
	}

	switch msg.String() {
	case "left", "h":
		if p.dateIdx > 0 {
			p.dateIdx--
			p.convIdx = 0
		}
	case "right", "l":
		if p.dateIdx < len(p.groups)-1 {
			p.dateIdx++
			p.convIdx = 0
		}
	case "up", "k":
		if p.convIdx > 0 {
			p.convIdx--
		}
	case "down", "j":
		if len(p.groups) > 0 && p.convIdx < len(p.groups[p.dateIdx].convs)-1 {
			p.convIdx++
		}
	case "enter":
		if len(p.groups) == 0 {
			m.mode = modeChat
			m.statusMsg = "No conversations"
			return m, nil
		}
		conv := p.groups[p.dateIdx].convs[p.convIdx]
		msgs, err := m.store.Load(conv.Name)
		if err != nil {
			m.statusMsg = "Load failed: " + err.Error()
			m.mode = modeChat
			return m, nil
		}
		m.autoSaveName = conv.Name
		m.history.Clear()
		for _, msg := range msgs {
			m.history.Add(msg)
		}
		m.mode = modeChat
		m.statusMsg = "Resumed: " + conv.Name
	case "esc":
		m.mode = modeChat
		m.statusMsg = "Cancelled"
	case "ctrl+c":
		if m.confirmQuit {
			return m, tea.Quit
		}
		m.confirmQuit = true
		m.statusMsg = "Press Ctrl+C again to quit"
	}
	return m, nil
}

func (m Model) viewResumePicker() string {
	var b strings.Builder
	p := m.resumePick

	b.WriteString(m.theme.ConfigTitleStyle().Render("Resume a Conversation"))
	b.WriteString("\n\n")

	if len(p.groups) == 0 {
		b.WriteString(m.theme.ConfigHelpStyle().Render("  No saved conversations."))
		b.WriteString("\n\n")
		b.WriteString(m.theme.ConfigHelpStyle().Render("  Esc: back to chat"))
		b.WriteString("\n")
		return b.String() + "\n" + m.renderStatusBar()
	}

	// Date navigation row
	prev := "  "
	if p.dateIdx > 0 {
		prev = "◀ "
	}
	next := "  "
	if p.dateIdx < len(p.groups)-1 {
		next = " ▶"
	}
	dateStr := p.groups[p.dateIdx].date
	b.WriteString("  " + prev + m.theme.ConfigTitleStyle().Render(dateStr) + next)
	b.WriteString("\n\n")

	// Conversation list for current date group
	group := p.groups[p.dateIdx]
	for i, conv := range group.convs {
		cursor := "  "
		if i == p.convIdx {
			cursor = m.theme.ConfigCursorStyle().Render("> ")
		}
		name := m.theme.ConfigValueStyle().Render(conv.Name)
		if conv.Summary != "" {
			summary := m.theme.ConfigHelpStyle().Render("  " + truncate(conv.Summary, 50))
			b.WriteString(cursor + name + summary + "\n")
		} else {
			b.WriteString(cursor + name + "\n")
		}
	}

	b.WriteString("\n")
	b.WriteString(m.theme.ConfigHelpStyle().Render("  ↑↓: select  |  ←→: change date  |  Enter: resume  |  Esc: back"))
	b.WriteString("\n")

	return b.String() + "\n" + m.renderStatusBar()
}
