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
	"github.com/termchat/termchat/internal/chat"
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

func (m *Model) currentBrowseTurn() *browseTurn {
	return &m.messageBrowse.turns[m.messageBrowse.turnIdx]
}

// buildBrowserState constructs messageBrowse.turns from active timeline.
// Groups messages into user-assistant pairs, loads all versions for each assistant message.
func (m *Model) buildBrowserState() error {
	tab := &m.tabs[m.activeTab]
	msgs := tab.history.Messages()

	if len(msgs) == 0 {
		return fmt.Errorf("no messages to browse")
	}

	m.messageBrowse.turns = nil
	m.messageBrowse.compareCardScrolls = make(map[int]int)

	for i := 0; i < len(msgs); i++ {
		if msgs[i].Role != "user" {
			continue
		}

		userMsg := msgs[i]

		// Find assistant message(s) after this user message
		var assistantVersions []chat.Message
		if i+1 < len(msgs) && msgs[i+1].Role == "assistant" {
			anchorID := msgs[i+1].ID
			versions, err := m.store.ListVersions(tab.autoSaveName, anchorID)
			if err != nil {
				return err
			}
			assistantVersions = versions
		}

		// Find which version is active
		activeIdx := 0
		if len(assistantVersions) > 0 {
			for idx, v := range assistantVersions {
				// The active timeline message ID matches one of the versions
				if i+1 < len(msgs) && v.ID == msgs[i+1].ID {
					activeIdx = idx
					break
				}
			}
		}

		m.messageBrowse.turns = append(m.messageBrowse.turns, browseTurn{
			User:              userMsg,
			AssistantVersions: assistantVersions,
			ActiveVersion:     activeIdx,
			PreviewVersion:    activeIdx,
		})

		// Skip the assistant message we just processed
		if len(assistantVersions) > 0 {
			i++
		}
	}

	// Start at last turn
	m.messageBrowse.turnIdx = len(m.messageBrowse.turns) - 1
	m.messageBrowse.mode = browseModeMessage
	m.messageBrowse.leftScroll = 0
	m.messageBrowse.rightScroll = 0
	m.messageBrowse.compareCardIdx = 0

	return nil
}

func (m *Model) movePreviewVersion(delta int) {
	turn := m.currentBrowseTurn()
	next := turn.PreviewVersion + delta
	if next < 0 || next >= len(turn.AssistantVersions) {
		return
	}
	turn.PreviewVersion = next
}

func (m *Model) moveCompareCard(delta int) {
	next := m.messageBrowse.compareCardIdx + delta
	if next < 0 || next >= len(m.currentBrowseTurn().AssistantVersions) {
		return
	}
	m.messageBrowse.compareCardIdx = next
	m.currentBrowseTurn().PreviewVersion = next
}

func (m Model) confirmOrApplyPreview() (Model, tea.Cmd) {
	turn := m.currentBrowseTurn()

	// No-op if preview == active
	if turn.PreviewVersion == turn.ActiveVersion {
		return m, nil
	}

	// Check if there are later turns
	hasLaterTurns := m.messageBrowse.turnIdx < len(m.messageBrowse.turns)-1

	if !hasLaterTurns {
		// Apply immediately if no later turns
		turn.ActiveVersion = turn.PreviewVersion
		m.statusMsg = "Version applied"
		return m, nil
	}

	// Open confirmation chooser
	m.messageBrowse.pendingConfirm = browseConfirmState{
		kind:   confirmApplyPreview,
		cursor: 0,
	}
	return m, nil
}

func (m Model) applyBrowseConfirmation() (Model, tea.Cmd) {
	switch m.messageBrowse.pendingConfirm.kind {
	case confirmApplyPreview:
		// TODO: implement apply/branch logic based on cursor
		// For now, just apply
		turn := m.currentBrowseTurn()
		turn.ActiveVersion = turn.PreviewVersion
		m.messageBrowse.pendingConfirm = browseConfirmState{kind: confirmNone}
		m.statusMsg = "Version applied"
		return m, nil
	case confirmEditRegenerate:
		selected := m.messageBrowse.pendingConfirm.cursor
		turn := m.currentBrowseTurn()
		activeAssistant := turn.AssistantVersions[turn.ActiveVersion]

		switch selected {
		case 0: // Regenerate
			return m.confirmOrRegenerateEditedTurn()
		case 1: // Save Only
			if err := m.store.UpdateMessageContent(turn.User.ID, m.messageBrowse.editBuffer); err != nil {
				m.statusMsg = "Update failed: " + err.Error()
				m.messageBrowse.pendingConfirm = browseConfirmState{kind: confirmNone}
				m.messageBrowse.editMode = false
				return m, nil
			}
			if err := m.store.MarkTurnEdited(turn.User.ID, activeAssistant.ID); err != nil {
				m.statusMsg = "Mark stale failed: " + err.Error()
				m.messageBrowse.pendingConfirm = browseConfirmState{kind: confirmNone}
				m.messageBrowse.editMode = false
				return m, nil
			}
			return m.reloadBrowseTurnState()
		}
	}
	return m, nil
}

func (m Model) reloadBrowseTurnState() (Model, tea.Cmd) {
	turn := m.currentBrowseTurn()

	// Reload the user message from storage
	tab := &m.tabs[m.activeTab]
	msgs, err := m.store.LoadActiveTimeline(tab.autoSaveName)
	if err != nil {
		m.statusMsg = "Reload failed: " + err.Error()
		return m, nil
	}

	// Find and update the current turn's user message
	for _, msg := range msgs {
		if msg.ID == turn.User.ID {
			turn.User = msg
		}
		// Update assistant versions
		for i, av := range turn.AssistantVersions {
			if msg.ID == av.ID {
				turn.AssistantVersions[i] = msg
			}
		}
	}

	m.messageBrowse.pendingConfirm = browseConfirmState{kind: confirmNone}
	m.messageBrowse.editMode = false
	m.statusMsg = "Turn updated"
	return m, nil
}

func (m Model) confirmOrRegenerateEditedTurn() (Model, tea.Cmd) {
	// TODO: implement regenerate logic
	// For now, just clear edit mode
	m.messageBrowse.pendingConfirm = browseConfirmState{kind: confirmNone}
	m.messageBrowse.editMode = false
	m.statusMsg = "Regenerate not yet implemented"
	return m, nil
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

	case "left", "h":
		if m.messageBrowse.mode == browseModeCompare {
			m.moveCompareCard(-1)
		} else {
			m.movePreviewVersion(-1)
		}

	case "right", "l":
		if m.messageBrowse.mode == browseModeCompare {
			m.moveCompareCard(1)
		} else {
			m.movePreviewVersion(1)
		}

	case "v":
		if len(m.messageBrowse.turns) > 0 && m.currentBrowseTurn().VersionCount() > 1 {
			m.messageBrowse.mode = browseModeCompare
			m.messageBrowse.compareCardIdx = m.currentBrowseTurn().PreviewVersion
		}

	case "e":
		if len(m.messageBrowse.turns) > 0 {
			turn := m.currentBrowseTurn()
			m.messageBrowse.editMode = true
			m.messageBrowse.editBuffer = turn.User.Content
			m.messageBrowse.editDirty = false
		}

	case "enter":
		if m.messageBrowse.pendingConfirm.kind != confirmNone {
			return m.applyBrowseConfirmation()
		}
		// If in edit mode, open confirmation dialog
		if m.messageBrowse.editMode {
			m.messageBrowse.pendingConfirm = browseConfirmState{
				kind:   confirmEditRegenerate,
				cursor: 0,
			}
			return m, nil
		}
		// Old rollback logic - only if not in browse mode with turns
		if len(m.messageBrowse.turns) > 0 {
			return m.confirmOrApplyPreview()
		}
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
		return m.deleteCurrentBrowseSelection()

	case "b":
		return m.branchFromCurrentBrowseTurn()

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

	var body string

	// Compare mode: render multiple version cards
	if m.messageBrowse.mode == browseModeCompare {
		if m.messageBrowse.compareCardScrolls == nil {
			m.messageBrowse.compareCardScrolls = make(map[int]int)
		}

		cardWidth := (m.width - 4) / len(turn.AssistantVersions)
		if cardWidth < 20 {
			cardWidth = 20
		}

		var cards []string
		for idx, msg := range turn.AssistantVersions {
			scroll := m.messageBrowse.compareCardScrolls[idx]
			isSelected := idx == m.messageBrowse.compareCardIdx

			// Render content with markdown
			var content string
			rendered, err := tab.renderer.Render(msg.Content)
			if err != nil {
				content = msg.Content
			} else {
				content = cleanGlamourOutput(rendered)
			}

			title := fmt.Sprintf("v%d/%d", msg.VersionNumber, msg.TotalVersions)
			if isSelected {
				title = "► " + title
			}
			card := renderBrowsePane(title, content, scroll, cardWidth, m.height, m.theme)
			cards = append(cards, card)
		}
		body = lipgloss.JoinHorizontal(lipgloss.Top, cards...)
	} else {
		// Message mode: render left/right panes
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
		body = lipgloss.JoinHorizontal(lipgloss.Top, left, right)
	}

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

func (m Model) deleteCurrentBrowseSelection() (Model, tea.Cmd) {
	if len(m.messageBrowse.turns) == 0 {
		return m, nil
	}

	turn := m.currentBrowseTurn()
	if len(turn.AssistantVersions) == 0 {
		return m, nil
	}

	// Delete the active version (in-memory only for now)
	activeIdx := turn.ActiveVersion

	// Remove from in-memory list
	turn.AssistantVersions = append(
		turn.AssistantVersions[:activeIdx],
		turn.AssistantVersions[activeIdx+1:]...,
	)

	// If no versions remain, remove the entire turn
	if len(turn.AssistantVersions) == 0 {
		m.messageBrowse.turns = append(
			m.messageBrowse.turns[:m.messageBrowse.turnIdx],
			m.messageBrowse.turns[m.messageBrowse.turnIdx+1:]...,
		)
		if m.messageBrowse.turnIdx >= len(m.messageBrowse.turns) && m.messageBrowse.turnIdx > 0 {
			m.messageBrowse.turnIdx--
		}
		m.statusMsg = "Turn deleted"
		return m, nil
	}

	// Promote nearest remaining version
	if activeIdx >= len(turn.AssistantVersions) {
		activeIdx = len(turn.AssistantVersions) - 1
	}
	turn.ActiveVersion = activeIdx
	turn.PreviewVersion = activeIdx

	m.statusMsg = "Version deleted"
	return m, nil
}

func (m Model) branchFromCurrentBrowseTurn() (Model, tea.Cmd) {
	if len(m.messageBrowse.turns) == 0 {
		return m, nil
	}

	tab := &m.tabs[m.activeTab]

	// Build branch messages up to current turn
	var branchMsgs []chat.Message

	for i := 0; i <= m.messageBrowse.turnIdx; i++ {
		t := &m.messageBrowse.turns[i]
		branchMsgs = append(branchMsgs, t.User)

		// Use preview version if it differs from active
		versionIdx := t.ActiveVersion
		if i == m.messageBrowse.turnIdx && t.PreviewVersion != t.ActiveVersion {
			versionIdx = t.PreviewVersion
		}

		if versionIdx < len(t.AssistantVersions) {
			branchMsgs = append(branchMsgs, t.AssistantVersions[versionIdx])
		}
	}

	// Create new conversation
	newName := time.Now().Format("2006-01-02_150405")
	if err := m.store.Save(newName, branchMsgs); err != nil {
		m.statusMsg = "Branch failed: " + err.Error()
		return m, nil
	}

	// Switch to new conversation
	tab.autoSaveName = newName
	tab.history.ReplaceMessages(branchMsgs)
	tab.viewport.SetContent(m.buildChatContent())
	tab.viewport.GotoBottom()
	tab.chatFollowBottom = true

	m.statusMsg = fmt.Sprintf("Branched to: %s", newName)
	m.mode = modeChat
	return m, nil
}
