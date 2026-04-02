// internal/ui/view.go
package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	xansi "github.com/charmbracelet/x/ansi"
	"github.com/muesli/reflow/wrap"
	"github.com/muesli/reflow/wordwrap"
)

const chatHorizontalInset = 1

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
		return wrapMixedLine(line, width)
	}

	const quotePrefixWidth = 2 // "│ "
	if quotePrefixWidth >= width {
		return wrap.String(line, width)
	}

	prefix := xansi.Cut(line, 0, quotePrefixWidth)
	content := xansi.Cut(line, quotePrefixWidth, xansi.StringWidth(line))
	parts := wrapMixedParts(content, width-quotePrefixWidth)
	for i, part := range parts {
		parts[i] = prefix + part
	}
	return strings.Join(parts, "\n")
}

func wrapMixedLine(line string, width int) string {
	return strings.Join(wrapMixedParts(line, width), "\n")
}

func wrapMixedParts(line string, width int) []string {
	if width <= 0 || xansi.StringWidth(line) <= width {
		return []string{line}
	}

	wordWrapped := wordwrap.String(line, width)
	parts := strings.Split(wordWrapped, "\n")
	var out []string
	for _, part := range parts {
		if xansi.StringWidth(part) <= width {
			out = append(out, part)
			continue
		}
		out = append(out, strings.Split(wrap.String(part, width), "\n")...)
	}
	return out
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
	contentWidth := m.width - chatHorizontalInset*2
	if contentWidth < 1 {
		contentWidth = 1
	}
	assistantWrapWidth := contentWidth - chatHorizontalInset - 1
	if assistantWrapWidth < 1 {
		assistantWrapWidth = 1
	}
	inset := strings.Repeat(" ", chatHorizontalInset)

	// Render conversation history.
	for _, msg := range tab.history.Messages() {
		switch msg.Role {
		case "user":
			maxBubbleWidth := contentWidth * 2 / 3
			if maxBubbleWidth < 20 {
				maxBubbleWidth = 20
			}
			wrapped := wrap.String(msg.Content, maxBubbleWidth-2)
			block := m.theme.UserLabelStyle().Render("You:") + "\n" + wrapped
			bubble := m.theme.UserMsgStyle().Render(block)
			b.WriteString(inset + lipgloss.PlaceHorizontal(contentWidth, lipgloss.Right, bubble) + "\n\n")
		case "assistant":
			label := tab.client.Model() + ":"
			if msg.TotalVersions > 1 {
				label = fmt.Sprintf("%s [%d versions]", label, msg.TotalVersions)
			}
			b.WriteString(inset + m.theme.AssistantLabelStyle().Render(label) + "\n")
			rendered, err := tab.renderer.Render(msg.Content)
			if err != nil {
				b.WriteString(inset + msg.Content + "\n\n")
			} else {
				cleaned := cleanGlamourOutput(rendered)
				// Hard-wrap long lines (e.g. Chinese text with no spaces) while
				// preserving blockquote prefixes on continuation lines.
				cleaned = hardWrapRenderedMarkdown(cleaned, assistantWrapWidth)
				b.WriteString(indentBlock(cleaned, inset) + "\n\n")
			}
		}
	}

	// Render current streaming response.
	if tab.streaming {
		b.WriteString(inset + m.theme.AssistantLabelStyle().Render(tab.client.Model()+":") + "\n")
		if tab.currentThinking != "" {
			thinking := strings.ReplaceAll(strings.TrimSpace(tab.currentThinking), "\n\n", "\n")
			b.WriteString(indentBlock(m.theme.ThinkingStyle().Render("\U0001f4ad "+thinking), inset) + "\n")
		}
		if tab.currentResp != "" {
			rendered, err := tab.renderer.Render(tab.currentResp)
			if err != nil {
				b.WriteString(indentBlock(tab.currentResp, inset))
			} else {
				cleaned := cleanGlamourOutput(rendered)
				cleaned = hardWrapRenderedMarkdown(cleaned, assistantWrapWidth)
				b.WriteString(indentBlock(cleaned, inset) + "\n")
			}
		}
	}

	// Render error.
	if tab.err != nil {
		b.WriteString(inset + m.theme.ErrStyle().Render(fmt.Sprintf("Error: %v", tab.err)) + "\n")
	}

	return b.String()
}

func indentBlock(s, prefix string) string {
	if prefix == "" || s == "" {
		return s
	}
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = prefix + line
	}
	return strings.Join(lines, "\n")
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
	if m.mode == modeModelSelector {
		return m.viewModelSelector()
	}
	if m.mode == modeMessageBrowse {
		return m.viewMessageBrowse()
	}
	if m.mode == modeTabOverflow {
		return m.viewTabOverflow()
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

	// Dynamic viewport height calculation
	vpHeight := m.height -
		lipgloss.Height(tabBar) -
		lipgloss.Height(statusBar) -
		lipgloss.Height(inputArea)
	if spinnerLine != "" {
		vpHeight -= lipgloss.Height(spinnerLine)
	}
	// Ensure minimum height
	if vpHeight < 1 {
		vpHeight = 1
	}
	// Update viewport dimensions and content if height changed
	if tab2.viewport.Height != vpHeight {
		tab2.viewport.Height = vpHeight
		// Re-render content to recalculate scroll boundaries
		tab2.viewport.SetContent(m.buildChatContent())
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
