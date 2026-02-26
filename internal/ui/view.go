// internal/ui/view.go
package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	xansi "github.com/charmbracelet/x/ansi"
	"github.com/muesli/reflow/wrap"
)

// cleanGlamourOutput strips glamour's leading/trailing blank lines and
// collapses inter-paragraph blank lines.
// The glamour paragraph renderer hardcodes "\n" before each non-first paragraph
// and "\n" after each paragraph, producing "\n\n" (a blank line) between them.
// Code block blank lines have ANSI codes between newlines and are unaffected.
// The MarginWriter pads every line to terminal width with spaces; stripping
// trailing spaces first converts those padded "blank" lines into empty strings
// so the subsequent \n\n collapse catches them.
func cleanGlamourOutput(s string) string {
	s = strings.TrimLeft(s, "\n")
	// Strip trailing spaces from each line to remove padding-writer artifacts.
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimRight(line, " ")
	}
	s = strings.Join(lines, "\n")
	s = strings.ReplaceAll(s, "\n\n", "\n")
	s = strings.TrimRight(s, " \n")
	return s
}

func wrapRenderedLine(line string, width int) string {
	if width <= 0 || xansi.StringWidth(line) <= width {
		return line
	}

	plain := xansi.Strip(line)
	if !strings.HasPrefix(plain, "│ ") {
		return wrap.String(line, width)
	}

	const quotePrefixWidth = 2 // "│ "
	if quotePrefixWidth >= width {
		return wrap.String(line, width)
	}

	prefix := xansi.Cut(line, 0, quotePrefixWidth)
	content := xansi.Cut(line, quotePrefixWidth, xansi.StringWidth(line))

	wrappedContent := wrap.String(content, width-quotePrefixWidth)
	parts := strings.Split(wrappedContent, "\n")
	for i, part := range parts {
		parts[i] = prefix + part
	}
	return strings.Join(parts, "\n")
}

func hardWrapRenderedMarkdown(s string, width int) string {
	wrapWidth := markdownWrapWidth(width)
	if wrapWidth <= 0 {
		return s
	}
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = wrapRenderedLine(line, wrapWidth)
	}
	return strings.Join(lines, "\n")
}

func (m Model) buildChatContent() string {
	var b strings.Builder

	tab := m.activeTabSession()

	// Render conversation history.
	for _, msg := range tab.history.Messages() {
		switch msg.Role {
		case "user":
			block := m.theme.UserLabelStyle().Render("You:") + "\n" + msg.Content
			b.WriteString(m.theme.UserMsgStyle(m.width).Render(block) + "\n\n")
		case "assistant":
			b.WriteString(m.theme.AssistantLabelStyle().Render(tab.client.Model()+":") + "\n")
			rendered, err := tab.renderer.Render(msg.Content)
			if err != nil {
				b.WriteString(msg.Content + "\n\n")
			} else {
				cleaned := cleanGlamourOutput(rendered)
				// Hard-wrap long lines (e.g. Chinese text with no spaces) while
				// preserving blockquote prefixes on continuation lines.
				cleaned = hardWrapRenderedMarkdown(cleaned, m.width)
				b.WriteString(cleaned + "\n\n")
			}
		}
	}

	// Render current streaming response.
	if tab.streaming {
		b.WriteString(m.theme.AssistantLabelStyle().Render(tab.client.Model()+":") + "\n")
		if tab.currentThinking != "" {
			thinking := strings.ReplaceAll(strings.TrimSpace(tab.currentThinking), "\n\n", "\n")
			b.WriteString(m.theme.ThinkingStyle().Render("\U0001f4ad "+thinking) + "\n")
		}
		if tab.currentResp != "" {
			rendered, err := tab.renderer.Render(tab.currentResp)
			if err != nil {
				b.WriteString(tab.currentResp)
			} else {
				cleaned := cleanGlamourOutput(rendered)
				cleaned = hardWrapRenderedMarkdown(cleaned, m.width)
				b.WriteString(cleaned + "\n")
			}
		}
	}

	// Render error.
	if tab.err != nil {
		b.WriteString(m.theme.ErrStyle().Render(fmt.Sprintf("Error: %v", tab.err)) + "\n")
	}

	return b.String()
}

func (m Model) View() string {
	if m.mode == modeConfig {
		return m.viewConfigEditor()
	}
	if m.mode == modeResume {
		return m.viewResumePicker()
	}
	if m.mode == modeShortcuts {
		return m.viewShortcutsEditor()
	}
	if m.mode == modeRolePicker {
		return m.viewRolePicker()
	}
	if m.mode == modeRoles {
		return m.viewRolesEditor()
	}
	if m.mode == modeOnboard {
		return m.viewOnboard()
	}
	if m.mode == modeSlashComplete {
		return m.viewSlashComplete()
	}
	if m.mode == modeMessageBrowse {
		return m.viewMessageBrowse()
	}

	tabBar := (&m).renderTabBar()
	statusBar := m.renderStatusBar()
	tab2 := m.activeTabSession()
	inputArea := tab2.textarea.View()

	// Spinner line: shown only during streaming
	spinnerLine := ""
	if tab2.streaming {
		spinnerLine = m.theme.SpinnerStyle().Render(tab2.spinner.View() + " 生成中...")
	}

	// Dynamic viewport height
	vpHeight := m.height -
		lipgloss.Height(tabBar) -
		lipgloss.Height(statusBar) -
		lipgloss.Height(inputArea)
	if spinnerLine != "" {
		vpHeight -= lipgloss.Height(spinnerLine)
	}
	if vpHeight < 1 {
		vpHeight = 1
	}
	if tab2.viewport.Height != vpHeight {
		tab2.viewport.Height = vpHeight
		if tab2.chatFollowBottom {
			tab2.viewport.GotoBottom()
		}
	}

	parts := []string{tabBar, tab2.viewport.View()}
	if spinnerLine != "" {
		parts = append(parts, spinnerLine)
	}
	parts = append(parts, inputArea, statusBar)
	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}
