// internal/ui/messagebrowse.go
package ui

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type clipboardResultMsg struct{ Err error }

func copyToClipboardCmd(text string) tea.Cmd {
	return func() tea.Msg {
		return clipboardResultMsg{Err: writeToClipboard(text)}
	}
}

// writeToClipboard copies text to the system clipboard using platform-native commands.
func writeToClipboard(text string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("pbcopy")
	case "windows":
		cmd = exec.Command("clip")
	default:
		// Linux: try wl-copy (Wayland), then xclip, then xsel
		if _, err := exec.LookPath("wl-copy"); err == nil {
			cmd = exec.Command("wl-copy")
		} else if _, err := exec.LookPath("xclip"); err == nil {
			cmd = exec.Command("xclip", "-selection", "clipboard")
		} else if _, err := exec.LookPath("xsel"); err == nil {
			cmd = exec.Command("xsel", "--clipboard", "--input")
		} else {
			return fmt.Errorf("no clipboard tool found (install xclip, xsel, or wl-clipboard)")
		}
	}
	cmd.Stdin = strings.NewReader(text)
	return cmd.Run()
}

func (m Model) updateMessageBrowse(msg tea.KeyMsg) (Model, tea.Cmd) {
	// Reset confirmQuit on any key other than ctrl+c
	if msg.String() != "ctrl+c" {
		m.confirmQuit = false
	}

	tab := &m.tabs[m.activeTab]
	msgs := tab.history.Messages()

	switch msg.String() {
	case "esc":
		m.mode = modeChat
		m.escCount = 0
		return m, nil

	case "ctrl+c":
		if m.confirmQuit {
			return m, tea.Quit
		}
		m.confirmQuit = true
		m.statusMsg = "Press Ctrl+C again to quit"
		return m, nil

	case "up", "k":
		if m.browseCursor > 0 {
			m.browseCursor--
		}

	case "down", "j":
		if m.browseCursor < len(msgs)-1 {
			m.browseCursor++
		}

	case "enter":
		// Rollback: keep messages[0..cursor] inclusive
		if len(msgs) == 0 {
			return m, nil
		}
		targetMsg := msgs[m.browseCursor]
		if err := m.store.TruncateAfterSeq(tab.autoSaveName, targetMsg.Seq); err != nil {
			m.statusMsg = "Rollback failed: " + err.Error()
			return m, nil
		}
		if err := m.reloadActiveTimeline(m.activeTab); err != nil {
			m.statusMsg = "Reload failed: " + err.Error()
			return m, nil
		}
		m.statusMsg = fmt.Sprintf("Rolled back to message %d", m.browseCursor+1)
		m.mode = modeChat
		return m, nil

	case "d":
		if len(msgs) == 0 {
			return m, nil
		}
		// For now, keep the simple in-memory delete and save
		// TODO: implement proper storage-level delete in future tasks
		tab.history.DeleteAt(m.browseCursor)
		remaining := tab.history.Messages()
		if len(remaining) == 0 {
			tab.viewport.SetContent(m.buildChatContent())
			tab.viewport.GotoBottom()
			tab.chatFollowBottom = true
			m.mode = modeChat
			m.statusMsg = "All messages deleted"
			return m, m.autoSaveCmd(m.activeTab)
		}
		if m.browseCursor >= len(remaining) {
			m.browseCursor = len(remaining) - 1
		}
		return m, m.autoSaveCmd(m.activeTab)

	case "b":
		// Branch: save history[0..cursor] as a brand-new conversation
		if len(msgs) == 0 {
			return m, nil
		}
		newName := time.Now().Format("2006-01-02_150405")

		// Save truncated history to new conversation
		branchMsgs := msgs[:m.browseCursor+1]
		if err := m.store.Save(newName, branchMsgs); err != nil {
			m.statusMsg = "Branch failed: " + err.Error()
			return m, nil
		}

		tab.autoSaveName = newName
		tab.history.ReplaceMessages(branchMsgs)
		tab.viewport.SetContent(m.buildChatContent())
		tab.viewport.GotoBottom()
		tab.chatFollowBottom = true
		m.statusMsg = fmt.Sprintf("Branched at message %d: %s", m.browseCursor+1, newName)
		m.mode = modeChat
		return m, nil

	case "c":
		if m.browseCursor >= 0 && m.browseCursor < len(msgs) {
			return m, copyToClipboardCmd(msgs[m.browseCursor].Content)
		}
	}

	return m, nil
}

func (m Model) viewMessageBrowse() string {
	tabBar := (&m).renderTabBar()
	tab := m.activeTabSession()

	// If no turns, show empty state
	if len(m.messageBrowse.turns) == 0 {
		var b strings.Builder
		b.WriteString(m.theme.ConfigTitleStyle().Render("Browse Messages") + "\n\n")
		b.WriteString(m.theme.ConfigHelpStyle().Render("  No messages.") + "\n\n")
		b.WriteString(m.theme.ConfigHelpStyle().Render("  Esc: back") + "\n")
		return lipgloss.JoinVertical(lipgloss.Left, tabBar, b.String()+"\n"+m.renderStatusBar())
	}

	// Get current turn
	turnIdx := m.messageBrowse.turnIdx
	if turnIdx >= len(m.messageBrowse.turns) {
		turnIdx = len(m.messageBrowse.turns) - 1
	}
	turn := m.messageBrowse.turns[turnIdx]

	// Calculate pane width (split screen in half, minus some padding)
	paneWidth := (m.width - 3) / 2
	if paneWidth < 20 {
		paneWidth = 20
	}

	// Render left pane (User)
	left := renderBrowsePane("User", turn.User.Content, m.messageBrowse.leftScroll, paneWidth, m.height, m.theme)

	// Render right pane (Assistant with version)
	var right string
	if len(turn.AssistantVersions) > 0 {
		previewIdx := turn.PreviewVersion
		if previewIdx >= len(turn.AssistantVersions) {
			previewIdx = 0
		}
		rightMsg := turn.AssistantVersions[previewIdx]
		rightTitle := fmt.Sprintf("Assistant v%d/%d", rightMsg.VersionNumber, rightMsg.TotalVersions)

		// Render assistant content with markdown
		var content string
		rendered, err := tab.renderer.Render(rightMsg.Content)
		if err != nil {
			content = rightMsg.Content
		} else {
			content = cleanGlamourOutput(rendered)
		}
		right = renderBrowsePane(rightTitle, content, m.messageBrowse.rightScroll, paneWidth, m.height, m.theme)
	} else {
		right = renderBrowsePane("Assistant", "(no versions)", 0, paneWidth, m.height, m.theme)
	}

	// Join panes horizontally
	body := lipgloss.JoinHorizontal(lipgloss.Top, left, right)

	// Footer help
	help := m.theme.ConfigHelpStyle().Render(
		"↑↓: turn  ←→: version  e: edit  g: new version  v: compare  Enter: apply/confirm  Esc: back",
	)

	return lipgloss.JoinVertical(lipgloss.Left, tabBar, body, help, m.renderStatusBar())
}

// renderBrowsePane renders a single pane with title, content, and scroll handling
func renderBrowsePane(title, content string, scroll, width, totalHeight int, theme Theme) string {
	// Calculate available height for content
	// totalHeight - tabbar(1) - help(1) - status(1) - padding(3)
	availableHeight := totalHeight - 6
	if availableHeight < 5 {
		availableHeight = 5
	}

	// Split content into lines
	contentLines := strings.Split(content, "\n")

	// Apply scroll offset
	startLine := scroll
	if startLine >= len(contentLines) {
		startLine = len(contentLines) - 1
	}
	if startLine < 0 {
		startLine = 0
	}

	endLine := startLine + availableHeight
	if endLine > len(contentLines) {
		endLine = len(contentLines)
	}

	visibleLines := contentLines[startLine:endLine]

	// Wrap each line to fit pane width
	var wrappedLines []string
	for _, line := range visibleLines {
		if utf8.RuneCountInString(line) <= width {
			wrappedLines = append(wrappedLines, line)
		} else {
			// Simple word wrap
			runes := []rune(line)
			for len(runes) > 0 {
				if len(runes) <= width {
					wrappedLines = append(wrappedLines, string(runes))
					break
				}
				wrappedLines = append(wrappedLines, string(runes[:width]))
				runes = runes[width:]
			}
		}
	}

	// Pad lines to consistent width
	for i, line := range wrappedLines {
		lineLen := utf8.RuneCountInString(line)
		if lineLen < width {
			wrappedLines[i] = line + strings.Repeat(" ", width-lineLen)
		}
	}

	// Build pane
	var b strings.Builder

	// Title bar
	titleBar := " " + title + " "
	titleLen := utf8.RuneCountInString(titleBar)
	if titleLen < width {
		titleBar += strings.Repeat("─", width-titleLen)
	}
	b.WriteString(theme.ConfigTitleStyle().Render(titleBar) + "\n")

	// Content
	b.WriteString(strings.Join(wrappedLines, "\n"))

	// Scroll indicators
	if startLine > 0 {
		b.WriteString("\n" + theme.ConfigHelpStyle().Render("(↑ more…)"))
	}
	if endLine < len(contentLines) {
		b.WriteString("\n" + theme.ConfigHelpStyle().Render("(↓ more…)"))
	}

	return b.String()
}
