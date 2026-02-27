// internal/ui/resumepicker.go
package ui

import (
	"strings"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/termchat/termchat/internal/chat"
	"github.com/termchat/termchat/internal/export"
	"github.com/termchat/termchat/internal/storage"
)

type dateGroup struct {
	date  string             // "YYYY-MM-DD"
	convs []storage.ConvInfo // conversations in this group (most recent first)
}

type resumePicker struct {
	groups    []dateGroup
	dateIdx   int
	convIdx   int
	exporting bool
	exportFmt int
	// Search state
	query         string
	allItems      []storage.ConvSearchItem
	searchResults []storage.ConvInfo
	searchCursor  int
}

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

// applySearch filters allItems by the current query and updates searchResults.
func (p *resumePicker) applySearch() {
	p.searchResults = nil
	for _, item := range p.allItems {
		if fuzzyMatch(p.query, item.FullText) {
			p.searchResults = append(p.searchResults, storage.ConvInfo{
				Name: item.Name,
				Date: item.Date,
			})
		}
	}
	p.searchCursor = 0
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

	if p.exporting {
		return m.updateExportPick(msg)
	}

	switch msg.String() {
	case "esc":
		if p.query != "" {
			p.query = ""
			p.searchResults = nil
			p.searchCursor = 0
			return m, nil
		}
		m.mode = modeChat
		m.statusMsg = "Cancelled"
		return m, nil

	case "backspace":
		if p.query != "" {
			runes := []rune(p.query)
			p.query = string(runes[:len(runes)-1])
			p.applySearch()
			return m, nil
		}

	case "up", "k":
		if p.query != "" {
			if p.searchCursor > 0 {
				p.searchCursor--
			}
		} else {
			if p.convIdx > 0 {
				p.convIdx--
			}
		}
		return m, nil

	case "down", "j":
		if p.query != "" {
			if p.searchCursor < len(p.searchResults)-1 {
				p.searchCursor++
			}
		} else {
			if len(p.groups) > 0 && p.convIdx < len(p.groups[p.dateIdx].convs)-1 {
				p.convIdx++
			}
		}
		return m, nil

	case "left", "h":
		if p.query == "" {
			if p.dateIdx > 0 {
				p.dateIdx--
				p.convIdx = 0
			}
		}
		return m, nil

	case "right", "l":
		if p.query == "" {
			if p.dateIdx < len(p.groups)-1 {
				p.dateIdx++
				p.convIdx = 0
			}
		}
		return m, nil

	case "e":
		if p.query == "" && len(p.groups) > 0 {
			p.exporting = true
		}
		return m, nil

	case "enter":
		var convName string
		if p.query != "" {
			if len(p.searchResults) == 0 || p.searchCursor >= len(p.searchResults) {
				return m, nil
			}
			convName = p.searchResults[p.searchCursor].Name
		} else {
			if len(p.groups) == 0 {
				m.mode = modeChat
				m.statusMsg = "No conversations"
				return m, nil
			}
			convName = p.groups[p.dateIdx].convs[p.convIdx].Name
		}
		msgs, err := m.store.Load(convName)
		if err != nil {
			m.statusMsg = "Load failed: " + err.Error()
			m.mode = modeChat
			return m, nil
		}
		m.activeTabSession().autoSaveName = convName
		m.activeTabSession().history.Clear()
		m.activeTabSession().history.SetSystemPrompt("")
		m.activeRole = ""
		for _, msg := range msgs {
			m.activeTabSession().history.Add(msg)
		}
		m.activeTabSession().viewport.SetContent(m.buildChatContent())
		m.activeTabSession().viewport.GotoBottom()
		m.activeTabSession().chatFollowBottom = true
		p.query = ""
		p.searchResults = nil
		m.mode = modeChat
		m.statusMsg = "Resumed: " + convName
		return m, nil

	case "ctrl+c":
		if m.confirmQuit {
			return m, tea.Quit
		}
		m.confirmQuit = true
		m.statusMsg = "Press Ctrl+C again to quit"
		return m, nil
	}

	// Printable rune input → append to query
	if msg.Type == tea.KeyRunes {
		p.query += string(msg.Runes)
		p.applySearch()
	}

	return m, nil
}

func (m Model) updateExportPick(msg tea.KeyMsg) (Model, tea.Cmd) {
	p := &m.resumePick
	switch msg.String() {
	case "left", "h":
		if p.exportFmt > 0 {
			p.exportFmt--
		}
	case "right", "l":
		if p.exportFmt < 2 {
			p.exportFmt++
		}
	case "esc":
		p.exporting = false
	case "enter":
		p.exporting = false
		conv := p.groups[p.dateIdx].convs[p.convIdx]
		// If exporting the currently active conversation, use in-memory history
		// to avoid missing the last message (autoSave is async).
		var msgs []chat.Message
		if conv.Name == m.activeTabSession().autoSaveName {
			msgs = m.activeTabSession().history.Messages()
		} else {
			var err error
			msgs, err = m.store.Load(conv.Name)
			if err != nil {
				m.statusMsg = "Export failed: " + err.Error()
				return m, nil
			}
		}
		exts := []string{"txt", "md", "pdf"}
		ext := exts[p.exportFmt]
		path, err := export.ResolvePath(m.cfg.Settings.ExportDir, conv.Name, ext)
		if err != nil {
			if err == export.ErrNoDownloadsDir {
				m.statusMsg = "Downloads folder not found — set Export Dir in /settings"
			} else {
				m.statusMsg = "Export failed: " + err.Error()
			}
			return m, nil
		}
		switch p.exportFmt {
		case 0:
			err = export.ExportTxt(path, msgs)
		case 1:
			err = export.ExportMd(path, msgs)
		case 2:
			err = export.ExportPdf(path, msgs)
		}
		if err != nil {
			m.statusMsg = "Export failed: " + err.Error()
		} else {
			m.statusMsg = "Saved to " + path
		}
	}
	return m, nil
}

func (m Model) viewResumePicker() string {
	tabBar := (&m).renderTabBar()
	var b strings.Builder
	p := m.resumePick

	b.WriteString(m.theme.ConfigTitleStyle().Render("Conversation History"))
	b.WriteString("\n\n")

	// Search input line (always shown)
	searchPrompt := m.theme.ConfigHelpStyle().Render("Search: ")
	var searchVal string
	if p.query == "" {
		searchVal = m.theme.ConfigHelpStyle().Render("(type to search)")
	} else {
		searchVal = m.theme.ConfigValueStyle().Render(p.query) + "█"
	}
	b.WriteString("  " + searchPrompt + searchVal)
	b.WriteString("\n\n")

	if p.query != "" {
		// Search results view (flat list)
		if len(p.searchResults) == 0 {
			b.WriteString(m.theme.ConfigHelpStyle().Render("  No matches."))
			b.WriteString("\n")
		} else {
			for i, conv := range p.searchResults {
				cursor := "  "
				if i == p.searchCursor {
					cursor = m.theme.ConfigCursorStyle().Render("> ")
				}
				name := m.theme.ConfigValueStyle().Render(conv.Name)
				date := m.theme.ConfigHelpStyle().Render("  " + conv.Date)
				b.WriteString(cursor + name + date + "\n")
			}
		}
		b.WriteString("\n")
		b.WriteString(m.theme.ConfigHelpStyle().Render("  ↑↓: select  |  Enter: resume  |  Esc: clear search"))
		b.WriteString("\n")
		return lipgloss.JoinVertical(lipgloss.Left, tabBar, b.String()+"\n"+m.renderStatusBar())
	}

	// Normal date-grouped view (unchanged)
	if len(p.groups) == 0 {
		b.WriteString(m.theme.ConfigHelpStyle().Render("  No saved conversations."))
		b.WriteString("\n\n")
		b.WriteString(m.theme.ConfigHelpStyle().Render("  Esc: back to chat"))
		b.WriteString("\n")
		return lipgloss.JoinVertical(lipgloss.Left, tabBar, b.String()+"\n"+m.renderStatusBar())
	}

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
	if p.exporting {
		fmts := []string{"txt", "md", "pdf"}
		var parts []string
		for i, f := range fmts {
			if i == p.exportFmt {
				parts = append(parts, m.theme.ConfigCursorStyle().Render("["+f+"]"))
			} else {
				parts = append(parts, m.theme.ConfigHelpStyle().Render(f))
			}
		}
		b.WriteString(m.theme.ConfigHelpStyle().Render("  Export: ") +
			strings.Join(parts, m.theme.ConfigHelpStyle().Render(" | ")))
		b.WriteString("\n")
		b.WriteString(m.theme.ConfigHelpStyle().Render("  ←/→: select format  Enter: export  Esc: cancel"))
		b.WriteString("\n")
	} else {
		b.WriteString(m.theme.ConfigHelpStyle().Render("  ↑↓: select  |  ←→: change date  |  Enter: resume  |  e: export  |  Esc: back"))
		b.WriteString("\n")
	}

	return lipgloss.JoinVertical(lipgloss.Left, tabBar, b.String()+"\n"+m.renderStatusBar())
}
