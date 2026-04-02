// internal/ui/messagebrowse.go
package ui

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	xansi "github.com/charmbracelet/x/ansi"
	"github.com/muesli/reflow/wrap"
	"github.com/termchat/termchat/internal/chat"
	"github.com/termchat/termchat/internal/config"
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

// buildBrowserState constructs browser turns from persisted rows, including
// soft-deleted user/assistant rows so half-empty turns can still render.
func (m *Model) buildBrowserState() error {
	tab := &m.tabs[m.activeTab]
	msgs, err := m.store.LoadBrowseMessages(tab.autoSaveName)
	if err != nil {
		return err
	}
	if len(msgs) == 0 {
		return fmt.Errorf("no messages to browse")
	}

	m.messageBrowse.turns = nil
	m.messageBrowse.compareCardScrolls = make(map[int]int)

	appendTurn := func(turn browseTurn) {
		if turn.User.ID == 0 && len(turn.AssistantVersions) == 0 {
			return
		}
		if turn.User.Deleted && len(turn.AssistantVersions) == 0 {
			return
		}

		activeIdx := 0
		for idx := range turn.AssistantVersions {
			turn.AssistantVersions[idx].TotalVersions = len(turn.AssistantVersions)
			if turn.AssistantVersions[idx].IsActiveVersion {
				activeIdx = idx
			}
		}
		if activeIdx >= len(turn.AssistantVersions) {
			activeIdx = 0
		}
		turn.ActiveVersion = activeIdx
		turn.PreviewVersion = activeIdx
		m.messageBrowse.turns = append(m.messageBrowse.turns, turn)
	}

	var current browseTurn
	var hasCurrent bool
	for _, msg := range msgs {
		switch msg.Role {
		case "user":
			if hasCurrent {
				appendTurn(current)
			}
			current = browseTurn{
				User:    msg,
				TurnSeq: msg.Seq,
			}
			hasCurrent = true
		case "assistant":
			if !hasCurrent {
				current = browseTurn{TurnSeq: msg.Seq}
				hasCurrent = true
			}
			if !msg.Deleted {
				current.AssistantVersions = append(current.AssistantVersions, msg)
			}
		}
	}
	if hasCurrent {
		appendTurn(current)
	}

	if len(m.messageBrowse.turns) == 0 {
		m.messageBrowse.turnIdx = 0
		m.messageBrowse.mode = browseModeMessage
		m.messageBrowse.leftScroll = 0
		m.messageBrowse.rightScroll = 0
		m.messageBrowse.compareCardIdx = 0
		return nil
	}

	m.messageBrowse.turnIdx = len(m.messageBrowse.turns) - 1
	m.messageBrowse.mode = browseModeMessage
	m.messageBrowse.leftScroll = 0
	m.messageBrowse.rightScroll = 0
	m.messageBrowse.compareCardIdx = m.currentBrowseTurn().PreviewVersion
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

func (m *Model) moveBrowseTurn(delta int) {
	next := m.messageBrowse.turnIdx + delta
	if next < 0 || next >= len(m.messageBrowse.turns) {
		return
	}
	m.messageBrowse.turnIdx = next
	m.messageBrowse.leftScroll = 0
	m.messageBrowse.rightScroll = 0
	m.messageBrowse.compareCardIdx = m.currentBrowseTurn().PreviewVersion
}

func (m *Model) scrollCompareCard(delta int) {
	if m.messageBrowse.compareCardScrolls == nil {
		m.messageBrowse.compareCardScrolls = make(map[int]int)
	}
	idx := m.messageBrowse.compareCardIdx
	next := m.messageBrowse.compareCardScrolls[idx] + delta
	if next < 0 {
		next = 0
	}
	m.messageBrowse.compareCardScrolls[idx] = next
}

func (m *Model) focusCompareCardAt(x int) {
	turn := m.currentBrowseTurn()
	if len(turn.AssistantVersions) == 0 {
		return
	}

	cardWidth := (m.width - 4) / len(turn.AssistantVersions)
	if cardWidth < 20 {
		cardWidth = 20
	}

	idx := x / cardWidth
	if idx < 0 {
		idx = 0
	}
	if idx >= len(turn.AssistantVersions) {
		idx = len(turn.AssistantVersions) - 1
	}

	m.messageBrowse.compareCardIdx = idx
	turn.PreviewVersion = idx
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
		return m.applyPreviewWithoutTruncate()
	}

	// Open confirmation chooser
	m.messageBrowse.pendingConfirm = browseConfirmState{
		kind:   confirmApplyPreview,
		cursor: 0,
	}
	return m, nil
}

func (m *Model) rebuildBrowserStateAtSeq(turnSeq int) error {
	if err := m.reloadActiveTimeline(m.activeTab); err != nil && err.Error() != "conversation not found" {
		return err
	}
	if err := m.buildBrowserState(); err != nil && err.Error() != "no messages to browse" {
		return err
	}
	if len(m.messageBrowse.turns) == 0 {
		m.messageBrowse.turnIdx = 0
		m.messageBrowse.compareCardIdx = 0
		return nil
	}

	targetIdx := len(m.messageBrowse.turns) - 1
	for idx, turn := range m.messageBrowse.turns {
		if turn.TurnSeq >= turnSeq {
			targetIdx = idx
			if turn.TurnSeq == turnSeq {
				break
			}
		}
	}
	m.messageBrowse.turnIdx = targetIdx
	m.messageBrowse.compareCardIdx = m.messageBrowse.turns[targetIdx].PreviewVersion
	return nil
}

func (m Model) applyPreviewWithoutTruncate() (Model, tea.Cmd) {
	turn := m.currentBrowseTurn()
	previewMsg := turn.AssistantVersions[turn.PreviewVersion]

	if err := m.store.SetActiveVersion(m.tabs[m.activeTab].autoSaveName, previewMsg.VersionGroupID, previewMsg.VersionNumber); err != nil {
		m.statusMsg = "Failed to apply: " + err.Error()
		return m, nil
	}
	if err := m.rebuildBrowserStateAtSeq(turn.TurnSeq); err != nil {
		m.statusMsg = "Failed to reload: " + err.Error()
		return m, nil
	}

	m.statusMsg = "Version applied"
	return m, nil
}

func (m Model) applyBrowseConfirmation() (Model, tea.Cmd) {
	switch m.messageBrowse.pendingConfirm.kind {
	case confirmApplyPreview:
		turn := m.currentBrowseTurn()
		previewMsg := turn.AssistantVersions[turn.PreviewVersion]
		if err := m.store.SetActiveVersion(m.tabs[m.activeTab].autoSaveName, previewMsg.VersionGroupID, previewMsg.VersionNumber); err != nil {
			m.statusMsg = "Failed to apply: " + err.Error()
			m.messageBrowse.pendingConfirm = browseConfirmState{kind: confirmNone}
			return m, nil
		}
		if m.messageBrowse.pendingConfirm.cursor == 1 {
			if err := m.store.TruncateAfterSeq(m.tabs[m.activeTab].autoSaveName, turn.User.Seq); err != nil {
				m.statusMsg = "Failed to truncate: " + err.Error()
				m.messageBrowse.pendingConfirm = browseConfirmState{kind: confirmNone}
				return m, nil
			}
		}
		if err := m.rebuildBrowserStateAtSeq(turn.TurnSeq); err != nil {
			m.statusMsg = "Failed to reload: " + err.Error()
			m.messageBrowse.pendingConfirm = browseConfirmState{kind: confirmNone}
			return m, nil
		}
		m.messageBrowse.pendingConfirm = browseConfirmState{kind: confirmNone}
		m.statusMsg = "Version applied"
		return m, nil
	case confirmEditRegenerate:
		selected := m.messageBrowse.pendingConfirm.cursor
		turn := m.currentBrowseTurn()

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
			for _, assistant := range turn.AssistantVersions {
				if err := m.store.MarkTurnEdited(turn.User.ID, assistant.ID); err != nil {
					m.statusMsg = "Mark stale failed: " + err.Error()
					m.messageBrowse.pendingConfirm = browseConfirmState{kind: confirmNone}
					m.messageBrowse.editMode = false
					return m, nil
				}
			}
			return m.reloadBrowseTurnState()
		}
	case confirmDeleteSelection:
		return m.applyDeleteSelection()
	}
	return m, nil
}

func (m Model) reloadBrowseTurnState() (Model, tea.Cmd) {
	turnSeq := m.currentBrowseTurn().TurnSeq
	if err := m.rebuildBrowserStateAtSeq(turnSeq); err != nil {
		m.statusMsg = "Reload failed: " + err.Error()
		return m, nil
	}

	m.messageBrowse.pendingConfirm = browseConfirmState{kind: confirmNone}
	m.messageBrowse.editMode = false
	m.statusMsg = "Turn updated"
	return m, nil
}

func (m Model) buildTurnRegenerationMessages(turn browseTurn, userContent string, rolePrompt string) []chat.Message {
	tab := &m.tabs[m.activeTab]
	apiMessages := make([]chat.Message, 0, turn.User.Seq+1)
	if rolePrompt != "" {
		apiMessages = append(apiMessages, chat.Message{Role: "system", Content: rolePrompt})
	}

	foundCurrentUser := false
	for _, msg := range tab.history.Messages() {
		if msg.Seq < turn.User.Seq {
			apiMessages = append(apiMessages, chat.Message{
				Role:    msg.Role,
				Content: msg.Content,
				Images:  msg.Images,
			})
			continue
		}
		if msg.Seq == turn.User.Seq && msg.Role == "user" {
			apiMessages = append(apiMessages, chat.Message{
				Role:    "user",
				Content: userContent,
				Images:  msg.Images,
			})
			foundCurrentUser = true
		}
		break
	}

	if !foundCurrentUser {
		apiMessages = append(apiMessages, chat.Message{Role: "user", Content: userContent})
	}

	return apiMessages
}

func collectAssistantResponseOrError(chunks <-chan chat.StreamChunk, errs <-chan error) string {
	var resp strings.Builder
	for chunks != nil || errs != nil {
		select {
		case chunk, ok := <-chunks:
			if !ok {
				chunks = nil
				continue
			}
			resp.WriteString(chunk.Content)
		case err, ok := <-errs:
			if !ok {
				errs = nil
				continue
			}
			if err != nil {
				return "Error: " + err.Error()
			}
		}
	}
	return resp.String()
}

func (m *Model) lookupProviderConfig(name string) (config.ProviderEntry, bool) {
	for _, entry := range m.modelRegistry.Providers {
		if entry.Name == name {
			return entry, true
		}
	}
	return config.ProviderEntry{}, false
}

func (m *Model) providerFromAssistantSnapshot(msg chat.Message) (chat.Provider, error) {
	if !msg.HasGenerationSnapshot() {
		return nil, fmt.Errorf("missing generation snapshot")
	}
	entry, ok := m.lookupProviderConfig(msg.SnapshotProvider)
	if !ok {
		return nil, fmt.Errorf("snapshot provider %q not found", msg.SnapshotProvider)
	}
	if strings.TrimSpace(msg.SnapshotAPIFormat) == "" {
		return nil, fmt.Errorf("snapshot api_format is missing")
	}
	return m.newProviderClient(msg.SnapshotAPIFormat, entry.BaseURL, entry.APIKey, msg.SnapshotModel), nil
}

func (m *Model) currentTabGenerationClient() (chat.Provider, string, string, string, string, string, error) {
	tab := m.activeTabSession()
	if tab == nil {
		return nil, "", "", "", "", "", fmt.Errorf("no active tab")
	}
	provider, model, err := m.resolveCurrentTabModelSelection(tab)
	if err != nil {
		return nil, "", "", "", "", "", err
	}
	roleName := m.activeRole
	rolePrompt := tab.history.SystemPrompt()
	return m.newProviderClient(model.APIFormat, provider.BaseURL, provider.APIKey, model.Model), provider.Name, model.Model, model.APIFormat, roleName, rolePrompt, nil
}

func (m Model) regenerateAssistantVersionContent(turn browseTurn, target chat.Message, editedContent string) (string, error) {
	client, err := m.providerFromAssistantSnapshot(target)
	if err != nil {
		return "", err
	}
	apiMessages := m.buildTurnRegenerationMessages(turn, editedContent, target.SnapshotRolePrompt)
	chunks, errs := client.SendStreamChan(
		context.Background(),
		apiMessages,
		m.cfg.Parameters.Temperature,
		m.cfg.Parameters.MaxTokens,
		m.cfg.Parameters.ReasoningEffort,
		m.cfg.Parameters.BudgetTokens,
	)
	if chunks == nil || errs == nil {
		return "", fmt.Errorf("provider returned no stream")
	}
	return collectAssistantResponseOrError(chunks, errs), nil
}

func (m Model) regeneratePreviewVersion() (Model, tea.Cmd) {
	turn := m.currentBrowseTurn()
	if len(turn.AssistantVersions) == 0 {
		return m, nil
	}
	target := turn.AssistantVersions[turn.PreviewVersion]

	client, snapshotProvider, snapshotModel, snapshotAPIFormat, snapshotRoleName, snapshotRolePrompt, err := m.currentTabGenerationClient()
	if err != nil {
		m.statusMsg = "Regenerate failed: " + err.Error()
		return m, nil
	}
	temp, maxTokens, reasoningEffort, budgetTokens := m.generationParametersForTab(m.activeTabSession())

	apiMessages := m.buildTurnRegenerationMessages(*turn, turn.User.Content, snapshotRolePrompt)
	chunks, errs := client.SendStreamChan(
		context.Background(),
		apiMessages,
		temp,
		maxTokens,
		reasoningEffort,
		budgetTokens,
	)
	if chunks == nil || errs == nil {
		m.statusMsg = "Regenerate failed: provider returned no stream"
		return m, nil
	}

	content := collectAssistantResponseOrError(chunks, errs)
	if err := m.store.UpdateAssistantMessage(target.ID, chat.Message{
		Role:               "assistant",
		Content:            content,
		SnapshotProvider:   snapshotProvider,
		SnapshotModel:      snapshotModel,
		SnapshotAPIFormat:  snapshotAPIFormat,
		SnapshotRoleName:   snapshotRoleName,
		SnapshotRolePrompt: snapshotRolePrompt,
	}); err != nil {
		m.statusMsg = "Update failed: " + err.Error()
		return m, nil
	}
	if err := m.rebuildBrowserStateAtSeq(turn.TurnSeq); err != nil {
		m.statusMsg = "Reload failed: " + err.Error()
		return m, nil
	}
	m.statusMsg = "Version regenerated"
	return m, nil
}

func (m Model) appendAssistantVersionFromSelection(provider config.ProviderEntry, model config.ModelEntry) (Model, tea.Cmd) {
	turn := m.currentBrowseTurn()
	if err := validateGenerationModelEntry(model); err != nil {
		m.statusMsg = "New version failed: " + err.Error()
		m.mode = modeMessageBrowse
		return m, nil
	}
	client := m.newProviderClient(model.APIFormat, provider.BaseURL, provider.APIKey, model.Model)
	rolePrompt := m.tabs[m.activeTab].history.SystemPrompt()
	roleName := m.activeRole

	apiMessages := m.buildTurnRegenerationMessages(*turn, turn.User.Content, rolePrompt)
	temp, maxTokens, reasoningEffort, budgetTokens := m.parametersForModelEntry(model)
	chunks, errs := client.SendStreamChan(
		context.Background(),
		apiMessages,
		temp,
		maxTokens,
		reasoningEffort,
		budgetTokens,
	)
	if chunks == nil || errs == nil {
		m.statusMsg = "New version failed: provider returned no stream"
		m.mode = modeMessageBrowse
		return m, nil
	}

	newMsg := chat.Message{
		Seq:                turn.User.Seq,
		Role:               "assistant",
		Content:            collectAssistantResponseOrError(chunks, errs),
		VersionNumber:      len(turn.AssistantVersions) + 1,
		SnapshotProvider:   provider.Name,
		SnapshotModel:      model.Model,
		SnapshotAPIFormat:  model.APIFormat,
		SnapshotRoleName:   roleName,
		SnapshotRolePrompt: rolePrompt,
	}

	if len(turn.AssistantVersions) == 0 {
		msgID, err := m.store.AppendMessage(m.tabs[m.activeTab].autoSaveName, newMsg)
		if err != nil {
			m.statusMsg = "Append version failed: " + err.Error()
			m.mode = modeMessageBrowse
			return m, nil
		}
		if err := m.store.InitVersionGroup(msgID); err != nil {
			m.statusMsg = "Init version group failed: " + err.Error()
			m.mode = modeMessageBrowse
			return m, nil
		}
	} else {
		anchorID := turn.AssistantVersions[0].ID
		if _, err := m.store.AppendAssistantVersion(m.tabs[m.activeTab].autoSaveName, anchorID, newMsg); err != nil {
			m.statusMsg = "Append version failed: " + err.Error()
			m.mode = modeMessageBrowse
			return m, nil
		}
	}

	if err := m.rebuildBrowserStateAtSeq(turn.TurnSeq); err != nil {
		m.statusMsg = "Reload failed: " + err.Error()
		m.mode = modeMessageBrowse
		return m, nil
	}
	m.mode = modeMessageBrowse
	m.statusMsg = "New version added"
	return m, nil
}

func assistantVersionTitle(prefix string, msg chat.Message) string {
	title := fmt.Sprintf("v%d/%d", msg.VersionNumber, msg.TotalVersions)
	if prefix != "" {
		title = prefix + " " + title
	}
	if msg.StaleAfterUserEdit {
		title += " [stale]"
	}
	if msg.SnapshotProvider != "" || msg.SnapshotModel != "" {
		title += fmt.Sprintf(" %s / %s", msg.SnapshotProvider, msg.SnapshotModel)
	}
	return title
}

func browseUserTitle(turn browseTurn, editMode bool) string {
	title := "User"
	if editMode {
		return title + " [editing]"
	}
	if turn.User.EditedAfterGeneration {
		title += " [edited]"
	}
	return title
}

func (m *Model) movePendingConfirm(delta int) {
	limit := 0
	switch m.messageBrowse.pendingConfirm.kind {
	case confirmApplyPreview, confirmEditRegenerate:
		limit = 2
	case confirmDeleteSelection:
		limit = 4
	}
	if limit == 0 {
		return
	}

	next := m.messageBrowse.pendingConfirm.cursor + delta
	if next < 0 {
		next = limit - 1
	}
	if next >= limit {
		next = 0
	}
	m.messageBrowse.pendingConfirm.cursor = next
}

func (m Model) restoreLastDeletedBatch() (Model, tea.Cmd) {
	if m.messageBrowse.lastDeletedBatchID == 0 {
		m.statusMsg = "Nothing to undo"
		return m, nil
	}

	turnSeq := m.messageBrowse.lastDeletedTurnSeq
	if err := m.store.RestoreDeletedBatch(m.messageBrowse.lastDeletedBatchID); err != nil {
		m.statusMsg = "Undo failed: " + err.Error()
		return m, nil
	}
	m.messageBrowse.lastDeletedBatchID = 0
	m.messageBrowse.lastDeletedTurnSeq = 0
	if err := m.rebuildBrowserStateAtSeq(turnSeq); err != nil {
		m.statusMsg = "Undo reload failed: " + err.Error()
		return m, nil
	}
	m.statusMsg = "Deletion undone"
	return m, nil
}

func (m Model) openDeleteSelection() (Model, tea.Cmd) {
	if len(m.messageBrowse.turns) == 0 {
		return m, nil
	}
	m.messageBrowse.pendingConfirm = browseConfirmState{
		kind:   confirmDeleteSelection,
		cursor: 0,
	}
	return m, nil
}

func (m Model) applyDeleteSelection() (Model, tea.Cmd) {
	if len(m.messageBrowse.turns) == 0 {
		m.messageBrowse.pendingConfirm = browseConfirmState{kind: confirmNone}
		return m, nil
	}

	turn := m.currentBrowseTurn()
	var ids []int64

	switch m.messageBrowse.pendingConfirm.cursor {
	case deleteUserOnly:
		if !turn.User.Deleted && turn.User.ID != 0 {
			ids = append(ids, turn.User.ID)
		}
	case deleteAssistantOnly:
		if len(turn.AssistantVersions) > 0 {
			ids = append(ids, turn.AssistantVersions[turn.PreviewVersion].ID)
		}
	case deleteBothSides:
		if !turn.User.Deleted && turn.User.ID != 0 {
			ids = append(ids, turn.User.ID)
		}
		if len(turn.AssistantVersions) > 0 {
			ids = append(ids, turn.AssistantVersions[turn.PreviewVersion].ID)
		}
	case deleteCancel:
		m.messageBrowse.pendingConfirm = browseConfirmState{kind: confirmNone}
		return m, nil
	}

	if len(ids) == 0 {
		m.messageBrowse.pendingConfirm = browseConfirmState{kind: confirmNone}
		m.statusMsg = "Nothing to delete"
		return m, nil
	}

	batchID, err := m.store.SoftDeleteMessages(ids...)
	if err != nil {
		m.statusMsg = "Delete failed: " + err.Error()
		m.messageBrowse.pendingConfirm = browseConfirmState{kind: confirmNone}
		return m, nil
	}

	m.messageBrowse.lastDeletedBatchID = batchID
	m.messageBrowse.lastDeletedTurnSeq = turn.TurnSeq
	m.messageBrowse.pendingConfirm = browseConfirmState{kind: confirmNone}
	if err := m.rebuildBrowserStateAtSeq(turn.TurnSeq); err != nil {
		m.statusMsg = "Reload failed: " + err.Error()
		return m, nil
	}
	m.statusMsg = "Marked deleted"
	return m, nil
}

func (m Model) confirmOrRegenerateEditedTurn() (Model, tea.Cmd) {
	turn := m.currentBrowseTurn()
	if len(turn.AssistantVersions) == 0 {
		m.statusMsg = "Regenerate failed: no assistant reply to replace"
		m.messageBrowse.pendingConfirm = browseConfirmState{kind: confirmNone}
		m.messageBrowse.editMode = false
		return m, nil
	}

	editedContent := m.messageBrowse.editBuffer
	if err := m.store.UpdateMessageContent(turn.User.ID, editedContent); err != nil {
		m.statusMsg = "Update failed: " + err.Error()
		m.messageBrowse.pendingConfirm = browseConfirmState{kind: confirmNone}
		m.messageBrowse.editMode = false
		return m, nil
	}

	for _, assistant := range turn.AssistantVersions {
		content, err := m.regenerateAssistantVersionContent(*turn, assistant, editedContent)
		if err != nil {
			m.statusMsg = "Regenerate failed: " + err.Error()
			m.messageBrowse.pendingConfirm = browseConfirmState{kind: confirmNone}
			m.messageBrowse.editMode = false
			return m, nil
		}
		if err := m.store.UpdateMessageContent(assistant.ID, content); err != nil {
			m.statusMsg = "Update failed: " + err.Error()
			m.messageBrowse.pendingConfirm = browseConfirmState{kind: confirmNone}
			m.messageBrowse.editMode = false
			return m, nil
		}
		if err := m.store.ClearTurnEdited(turn.User.ID, assistant.ID); err != nil {
			m.statusMsg = "Clear flags failed: " + err.Error()
			m.messageBrowse.pendingConfirm = browseConfirmState{kind: confirmNone}
			m.messageBrowse.editMode = false
			return m, nil
		}
	}
	if err := m.rebuildBrowserStateAtSeq(turn.TurnSeq); err != nil {
		m.statusMsg = "Reload failed: " + err.Error()
		m.messageBrowse.pendingConfirm = browseConfirmState{kind: confirmNone}
		m.messageBrowse.editMode = false
		return m, nil
	}

	m.messageBrowse.pendingConfirm = browseConfirmState{kind: confirmNone}
	m.messageBrowse.editMode = false
	m.statusMsg = "Turn regenerated"
	return m, nil
}

func (m Model) updateMessageBrowse(msg tea.KeyMsg) (Model, tea.Cmd) {
	// Reset confirmQuit on any key other than ctrl+c
	if msg.String() != "ctrl+c" {
		m.confirmQuit = false
	}

	tab := &m.tabs[m.activeTab]
	msgs := tab.history.Messages()

	if m.messageBrowse.pendingConfirm.kind != confirmNone {
		switch msg.String() {
		case "esc":
			m.messageBrowse.pendingConfirm = browseConfirmState{kind: confirmNone}
			return m, nil
		case "up", "k", "left", "h":
			m.movePendingConfirm(-1)
			return m, nil
		case "down", "j", "right", "l":
			m.movePendingConfirm(1)
			return m, nil
		case "enter":
			return m.applyBrowseConfirmation()
		}
	}

	if m.messageBrowse.editMode {
		switch msg.String() {
		case "esc":
			m.messageBrowse.editMode = false
			m.messageBrowse.editBuffer = ""
			m.messageBrowse.editDirty = false
			return m, nil
		case "backspace", "ctrl+h":
			if m.messageBrowse.editBuffer != "" {
				_, size := utf8.DecodeLastRuneInString(m.messageBrowse.editBuffer)
				if size > 0 {
					m.messageBrowse.editBuffer = m.messageBrowse.editBuffer[:len(m.messageBrowse.editBuffer)-size]
				}
			}
			m.messageBrowse.editDirty = m.messageBrowse.editBuffer != m.currentBrowseTurn().User.Content
			return m, nil
		case "enter":
			changed := m.messageBrowse.editBuffer != m.currentBrowseTurn().User.Content
			if !changed {
				m.messageBrowse.editMode = false
				m.messageBrowse.editBuffer = ""
				m.messageBrowse.editDirty = false
				return m, nil
			}
			m.messageBrowse.editDirty = true
			m.messageBrowse.pendingConfirm = browseConfirmState{
				kind:   confirmEditRegenerate,
				cursor: 0,
			}
			return m, nil
		default:
			if len(msg.Runes) > 0 {
				m.messageBrowse.editBuffer += string(msg.Runes)
				m.messageBrowse.editDirty = m.messageBrowse.editBuffer != m.currentBrowseTurn().User.Content
				return m, nil
			}
		}
	}

	switch msg.String() {
	case "esc":
		if m.messageBrowse.mode == browseModeCompare {
			m.messageBrowse.mode = browseModeMessage
			return m, nil
		}
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
		if m.messageBrowse.mode == browseModeCompare {
			m.scrollCompareCard(-1)
		} else {
			m.moveBrowseTurn(-1)
		}

	case "down", "j":
		if m.messageBrowse.mode == browseModeCompare {
			m.scrollCompareCard(1)
		} else {
			m.moveBrowseTurn(1)
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
		if m.messageBrowse.mode == browseModeCompare {
			m.messageBrowse.mode = browseModeMessage
			return m, nil
		}
		if len(m.messageBrowse.turns) > 0 && m.currentBrowseTurn().VersionCount() > 1 {
			m.messageBrowse.mode = browseModeCompare
			m.messageBrowse.compareCardIdx = m.currentBrowseTurn().PreviewVersion
		}

	case "e":
		if len(m.messageBrowse.turns) > 0 && !m.currentBrowseTurn().User.Deleted {
			turn := m.currentBrowseTurn()
			m.messageBrowse.editMode = true
			m.messageBrowse.editBuffer = turn.User.Content
			m.messageBrowse.editDirty = false
		}

	case "g":
		if len(m.messageBrowse.turns) > 0 {
			m.modelSel = newModelSelectorState(m.modelRegistry, modelSelectorPickForNewVersion)
			m.mode = modeModelSelector
			return m, nil
		}

	case "r":
		if len(m.messageBrowse.turns) > 0 {
			return m.regeneratePreviewVersion()
		}

	case "enter":
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
		return m.openDeleteSelection()

	case "u":
		return m.restoreLastDeletedBatch()

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
	help := renderBrowseHelp(m.messageBrowse.mode, m.messageBrowse.editMode, m.width, m.theme)
	confirm := m.renderBrowseConfirm()
	statusBar := m.renderStatusBar()

	// If no turns, show empty state
	if len(m.messageBrowse.turns) == 0 {
		var b strings.Builder
		b.WriteString(m.theme.ConfigTitleStyle().Render("Browse Messages") + "\n\n")
		b.WriteString(m.theme.ConfigHelpStyle().Render("  No messages.") + "\n\n")
		b.WriteString(m.theme.ConfigHelpStyle().Render("  Esc: back") + "\n")
		return lipgloss.JoinVertical(lipgloss.Left, tabBar, b.String()+"\n"+statusBar)
	}

	// Get current turn
	turnIdx := m.messageBrowse.turnIdx
	if turnIdx >= len(m.messageBrowse.turns) {
		turnIdx = len(m.messageBrowse.turns) - 1
	}
	turn := m.messageBrowse.turns[turnIdx]
	bodyHeight := m.height - lipgloss.Height(tabBar) - lipgloss.Height(help) - lipgloss.Height(confirm) - lipgloss.Height(statusBar)
	if bodyHeight < 4 {
		bodyHeight = 4
	}

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
				content = xansi.Strip(cleanGlamourOutput(rendered))
			}

			title := assistantVersionTitle("", msg)
			if isSelected {
				title = "► " + title
			}
			card := renderBrowsePane(title, content, scroll, cardWidth, bodyHeight, m.theme)
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
		leftContent := turn.User.Content
		if turn.User.Deleted {
			leftContent = ""
		}
		if m.messageBrowse.editMode {
			leftContent = m.messageBrowse.editBuffer + "█"
		}
		left := renderBrowsePane(browseUserTitle(turn, m.messageBrowse.editMode), leftContent, m.messageBrowse.leftScroll, paneWidth, bodyHeight, m.theme)

		// Render right pane (Assistant with version)
		var right string
		if len(turn.AssistantVersions) > 0 {
			previewIdx := turn.PreviewVersion
			if previewIdx >= len(turn.AssistantVersions) {
				previewIdx = 0
			}
			rightMsg := turn.AssistantVersions[previewIdx]
			rightTitle := assistantVersionTitle("Assistant", rightMsg)

			// Render assistant content with markdown
			var content string
			rendered, err := tab.renderer.Render(rightMsg.Content)
			if err != nil {
				content = rightMsg.Content
			} else {
				content = xansi.Strip(cleanGlamourOutput(rendered))
			}
			right = renderBrowsePane(rightTitle, content, m.messageBrowse.rightScroll, paneWidth, bodyHeight, m.theme)
		} else {
			right = renderBrowsePane("Assistant", "", 0, paneWidth, bodyHeight, m.theme)
		}

		// Join panes horizontally
		body = lipgloss.JoinHorizontal(lipgloss.Top, left, right)
	}

	parts := []string{tabBar, body}
	if confirm != "" {
		parts = append(parts, confirm)
	}
	parts = append(parts, help, statusBar)
	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}

// renderBrowsePane renders a single pane with title, content, and scroll handling
func renderBrowsePane(title, content string, scroll, width, height int, theme Theme) string {
	if width < 1 {
		width = 1
	}
	if height < 3 {
		height = 3
	}

	titleBar := " " + title + " "
	if xansi.StringWidth(titleBar) > width {
		titleBar = xansi.TruncateWc(titleBar, width, "")
	}
	if fill := width - xansi.StringWidth(titleBar); fill > 0 {
		titleBar += strings.Repeat("─", fill)
	}

	content = xansi.Strip(content)
	contentLines := strings.Split(content, "\n")
	var wrappedLines []string
	for _, line := range contentLines {
		wrapped := wrap.String(line, width)
		parts := strings.Split(wrapped, "\n")
		wrappedLines = append(wrappedLines, parts...)
	}
	if len(wrappedLines) == 0 {
		wrappedLines = []string{""}
	}

	startLine := scroll
	if startLine >= len(wrappedLines) {
		startLine = len(wrappedLines) - 1
	}
	if startLine < 0 {
		startLine = 0
	}

	availableContentHeight := height - 3 // title + up/down indicators
	if availableContentHeight < 1 {
		availableContentHeight = 1
	}

	endLine := startLine + availableContentHeight
	if endLine > len(wrappedLines) {
		endLine = len(wrappedLines)
	}

	var b strings.Builder
	lineStyle := lipgloss.NewStyle().Width(width)
	b.WriteString(theme.ConfigTitleStyle().Render(titleBar) + "\n")
	for _, line := range wrappedLines[startLine:endLine] {
		b.WriteString(lineStyle.Render(line) + "\n")
	}
	if startLine > 0 {
		b.WriteString(theme.ConfigHelpStyle().Render(lineStyle.Render("(↑ more…)")) + "\n")
	}
	if endLine < len(wrappedLines) {
		b.WriteString(theme.ConfigHelpStyle().Render(lineStyle.Render("(↓ more…)")))
	}

	return strings.TrimRight(b.String(), "\n")
}

func renderBrowseHelp(mode browseMode, editMode bool, width int, theme Theme) string {
	var items []string
	if editMode {
		items = []string{
			"type: edit user",
			"Enter: confirm",
			"Esc: cancel edit",
		}
	} else if mode == browseModeCompare {
		items = []string{
			"↑↓: scroll",
			"←→: card",
			"Enter: apply",
			"g: new version",
			"r: regenerate",
			"d: delete",
			"u: undo",
			"b: branch",
			"c: copy",
			"v: message",
			"Esc: back",
		}
	} else {
		items = []string{
			"↑↓: turn",
			"←→: version",
			"Enter: apply",
			"g: new version",
			"e: edit",
			"r: regenerate",
			"d: delete",
			"u: undo",
			"b: branch",
			"c: copy",
			"v: compare",
			"Esc: back",
		}
	}

	lines := packPlainLines(items, width, 3)
	for i, line := range lines {
		lines[i] = theme.ConfigHelpStyle().Render(line)
	}
	return strings.Join(lines, "\n")
}

func (m Model) renderBrowseConfirm() string {
	if m.messageBrowse.pendingConfirm.kind == confirmNone {
		return ""
	}

	var title string
	var options []string
	switch m.messageBrowse.pendingConfirm.kind {
	case confirmDeleteSelection:
		title = "Delete"
		options = []string{"user", "assistant", "both", "cancel"}
	case confirmApplyPreview:
		title = "Apply"
		options = []string{"keep later turns", "truncate later turns"}
	case confirmEditRegenerate:
		title = "Edited User Message"
		options = []string{"regenerate", "save only"}
	}

	var rendered []string
	for idx, option := range options {
		label := option
		if idx == m.messageBrowse.pendingConfirm.cursor {
			label = "[" + option + "]"
			rendered = append(rendered, m.theme.ConfigTitleStyle().Render(label))
			continue
		}
		rendered = append(rendered, m.theme.ConfigHelpStyle().Render(label))
	}

	return m.theme.ConfigHelpStyle().Render(title+": ") + strings.Join(rendered, m.theme.ConfigHelpStyle().Render("  "))
}

func packPlainLines(items []string, width, maxLines int) []string {
	if len(items) == 0 {
		return nil
	}

	if width <= 0 {
		return []string{strings.Join(items, "  ")}
	}

	var lines []string
	current := ""
	for _, item := range items {
		candidate := item
		if current != "" {
			candidate = current + "  " + item
		}
		if current == "" && xansi.StringWidth(item) > width {
			lines = append(lines, xansi.TruncateWc(item, width, ""))
			continue
		}
		if xansi.StringWidth(candidate) <= width {
			current = candidate
			continue
		}
		if current != "" {
			lines = append(lines, current)
		}
		current = item
	}
	if current != "" {
		lines = append(lines, current)
	}
	if maxLines > 0 && len(lines) > maxLines {
		last := strings.Join(lines[maxLines-1:], "  ")
		lines = append(lines[:maxLines-1], xansi.TruncateWc(last, width, ""))
	}
	return lines
}

func (m Model) deleteCurrentBrowseSelection() (Model, tea.Cmd) {
	return m.openDeleteSelection()
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
		if !t.User.Deleted {
			branchMsgs = append(branchMsgs, t.User)
		}

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
	reloaded, err := m.store.LoadActiveTimeline(newName)
	if err != nil {
		m.statusMsg = "Branch reload failed: " + err.Error()
		return m, nil
	}
	tab.history.ReplaceMessages(reloaded)
	tab.viewport.SetContent(m.buildChatContent())
	tab.viewport.GotoBottom()
	tab.chatFollowBottom = true

	m.statusMsg = fmt.Sprintf("Branched to: %s", newName)
	m.mode = modeChat
	return m, nil
}
