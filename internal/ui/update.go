// internal/ui/update.go
package ui

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/termchat/termchat/internal/chat"
	"github.com/termchat/termchat/internal/roles"
	"github.com/termchat/termchat/internal/shortcuts"
)

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		if msg.Width > 0 {
			m.recreateRenderer(msg.Width)
		}
		// viewport height = total height - 1 (input line) - 1 (status bar)
		vpHeight := msg.Height - 2
		if vpHeight < 1 {
			vpHeight = 1
		}
		tab := &m.tabs[m.activeTab]
		tab.viewport.Width = msg.Width
		tab.viewport.Height = vpHeight
		tab.textarea.SetWidth(msg.Width)
		// Re-render content at new size
		tab.viewport.SetContent(m.buildChatContent())
		if tab.chatFollowBottom {
			tab.viewport.GotoBottom()
		}
		return m, nil

	case tea.KeyMsg:
		if m.mode == modeTabRename {
			return m.updateTabRenameMode(msg)
		}
		if m.mode == modeTabOverflow {
			return m.updateTabOverflow(msg)
		}

		// Tab management — global shortcuts
		switch msg.String() {
		case "ctrl+t":
			cmd := m.newTab()
			return m, cmd
		case "ctrl+w":
			m.closeTab(m.activeTab)
			return m, nil
		case "alt+1", "alt+2", "alt+3", "alt+4", "alt+5",
			"alt+6", "alt+7", "alt+8", "alt+9":
			digit := int(msg.String()[4] - '1') // "alt+1" → 0, "alt+9" → 8
			m.switchTab(digit)
			return m, nil
		case "alt+0":
			m.mode = modeTabOverflow
			return m, nil
		case "f2":
			if m.mode == modeChat {
				m.tabRename = m.tabs[m.activeTab].name
				m.mode = modeTabRename
				return m, nil
			}
		}

		if m.mode == modeConfig {
			return m.updateConfigMode(msg)
		}
		if m.mode == modeResume {
			return m.updateResumeMode(msg)
		}
		if m.mode == modeShortcuts {
			return m.updateShortcutsMode(msg)
		}
		if m.mode == modeRolePicker {
			return m.updateRolePickerMode(msg)
		}
		if m.mode == modeRoles {
			return m.updateRolesMode(msg)
		}
		if m.mode == modeOnboard {
			return m.updateOnboardMode(msg)
		}
		if m.mode == modeSlashComplete {
			return m.updateSlashComplete(msg)
		}
		if m.mode == modeModelSelector {
			return m.updateModelSelector(msg)
		}
		if m.mode == modeMessageBrowse {
			return m.updateMessageBrowse(msg)
		}

		tab := &m.tabs[m.activeTab]
		if tab.streaming {
			if isScrollKey(msg.String()) {
				var cmd tea.Cmd
				tab.viewport, cmd = tab.viewport.Update(msg)
				tab.chatFollowBottom = tab.viewport.AtBottom()
				return m, cmd
			}
			if msg.String() == "ctrl+c" {
				if m.confirmQuit {
					return m, tea.Quit
				}
				// Cancel the in-flight HTTP request
				if tab.streamCtrl != nil {
					tab.streamCtrl.cancel()
				}
				tab.streaming = false
				tab.currentResp = ""
				tab.currentThinking = ""
				tab.viewport.SetContent(m.buildChatContent())
				if tab.chatFollowBottom {
					tab.viewport.GotoBottom()
				}
				m.confirmQuit = true
				m.statusMsg = "Press Ctrl+C again to quit"
				return m, nil
			}
			return m, nil
		}

		// Reset confirmQuit on any key other than ctrl+c
		if msg.String() != "ctrl+c" {
			m.confirmQuit = false
		}
		// Reset escCount on any key other than esc and ctrl+c
		if msg.String() != "ctrl+c" && msg.String() != "esc" {
			m.escCount = 0
		}

		switch msg.String() {
		case "up", "down", "pgup", "pgdown", "home", "end":
			var cmd tea.Cmd
			tab.viewport, cmd = tab.viewport.Update(msg)
			tab.chatFollowBottom = tab.viewport.AtBottom()
			return m, cmd

		case "esc":
			m.escCount++
			if m.escCount >= 2 {
				m.escCount = 0
				if len(tab.history.Messages()) == 0 {
					m.statusMsg = "No messages to browse"
					return m, nil
				}

				// Build browser state from history
				if err := m.buildBrowserState(); err != nil {
					m.statusMsg = "Failed to build browser: " + err.Error()
					return m, nil
				}

				m.mode = modeMessageBrowse
			} else {
				m.statusMsg = "Press Esc again to browse messages"
			}
			return m, nil

		case "ctrl+c":
			if m.confirmQuit {
				return m, tea.Quit
			}
			m.confirmQuit = true
			m.statusMsg = "Press Ctrl+C again to quit"
			return m, nil

		case "enter":
			input := strings.TrimSpace(tab.textarea.Value())
			if input == "" {
				return m, nil
			}
			if strings.HasPrefix(input, "/") {
				tab.textarea.Reset()
				return m.handleCommand(input)
			}
			if _, _, err := m.resolveCurrentTabModelSelection(tab); err != nil {
				m.statusMsg = "Send failed: " + err.Error()
				return m, nil
			}
			tab.textarea.Reset()

			if len(tab.pendingImages) > 0 && !tab.client.SupportsVision() {
				tab.pendingImages = nil
				tab.imageCounter = 0
				m.statusMsg = "⚠ 当前模型不支持图片，已忽略"
			}

			// Create user message with next sequence number
			userMsg := chat.Message{
				Seq:     nextSeq(tab.history.Messages()),
				Role:    "user",
				Content: input,
				Images:  tab.pendingImages,
			}

			// Persist user message to storage
			userID, err := m.store.AppendMessage(tab.autoSaveName, userMsg)
			if err != nil {
				m.statusMsg = "Save failed: " + err.Error()
				return m, nil
			}
			userMsg.ID = userID

			// Add to history
			tab.history.Add(userMsg)
			tab.pendingImages = nil
			tab.streaming = true
			tab.chatFollowBottom = true
			tab.currentResp = ""
			tab.currentThinking = ""
			tab.err = nil

			ctx, cancel := context.WithCancel(context.Background())
			tab.streamCtrl = &streamControl{cancel: cancel}
			tab.viewport.SetContent(m.buildChatContent())
			tab.viewport.GotoBottom()

			return m, tea.Batch(m.sendStreamCmd(ctx, m.activeTab), tab.spinner.Tick)

		case "ctrl+v":
			return m, m.pasteFromClipboard()

		default:
			var cmd tea.Cmd
			tab.textarea, cmd = tab.textarea.Update(msg)
			// Enter slash autocomplete mode when "/" is typed as the first character
			if tab.textarea.Value() == "/" {
				m.mode = modeSlashComplete
				m.slashAC = slashComplete{
					matches: filterSlashCmds("/"),
					cursor:  0,
					offset:  0,
				}
			}
			return m, cmd
		}
		return m, nil

	case spinner.TickMsg:
		tab := &m.tabs[m.activeTab]
		if tab.streaming {
			var cmd tea.Cmd
			tab.spinner, cmd = tab.spinner.Update(msg)
			return m, cmd
		}
		return m, nil

	case tea.MouseMsg:
		// Tab bar click (row 0)
		if msg.Y == 0 && msg.Action == tea.MouseActionPress {
			// Compute zones on-demand to avoid stale state from View()'s value receiver
			zones := m.computeTabBarZones()
			var zone *tabHitZone
			for i := range zones {
				z := &zones[i]
				if msg.X >= z.startX && msg.X <= z.endX {
					zone = z
					break
				}
			}
			if zone != nil {
				switch zone.action {
				case tabHitSelect:
					m.switchTab(zone.tabIdx)
				case tabHitClose:
					m.closeTab(zone.tabIdx)
				case tabHitNew:
					cmd := m.newTab()
					return m, cmd
				case tabHitOverflow:
					m.mode = modeTabOverflow
				}
				return m, nil
			}
		}
		// Mouse wheel in message browse mode
		if m.mode == modeMessageBrowse && msg.Action == tea.MouseActionPress {
			if m.messageBrowse.mode == browseModeCompare {
				if msg.Button == tea.MouseButtonWheelDown {
					m.focusCompareCardAt(msg.X)
					m.scrollCompareCard(1)
					return m, nil
				} else if msg.Button == tea.MouseButtonWheelUp {
					m.focusCompareCardAt(msg.X)
					m.scrollCompareCard(-1)
					return m, nil
				}
			}
		}
		// Viewport scrolling for the chat area
		if m.mode == modeChat {
			tab := &m.tabs[m.activeTab]
			var cmd tea.Cmd
			tab.viewport, cmd = tab.viewport.Update(msg)
			tab.chatFollowBottom = tab.viewport.AtBottom()
			return m, cmd
		}
		return m, nil

	case streamChunkMsg:
		if msg.TabIdx < len(m.tabs) {
			t := &m.tabs[msg.TabIdx]
			t.currentResp += msg.Content
			t.currentThinking += msg.Thinking
			if msg.TabIdx == m.activeTab {
				t.viewport.SetContent(m.buildChatContent())
				if t.chatFollowBottom {
					t.viewport.GotoBottom()
				}
			}
		}
		return m, m.readNextChunk(msg.TabIdx)

	case streamStartMsg:
		if msg.TabIdx < len(m.tabs) {
			t := &m.tabs[msg.TabIdx]
			t.streamCh = msg.chunks
			t.streamErr = msg.errs
		}
		return m, tea.Batch(m.readNextChunk(msg.TabIdx), m.tabs[msg.TabIdx].spinner.Tick)

	case streamDoneMsg:
		if msg.TabIdx < len(m.tabs) {
			t := &m.tabs[msg.TabIdx]
			t.streaming = false
			if t.currentResp != "" {
				// Get the user message to determine the sequence number
				msgs := t.history.Messages()
				var userMsg chat.Message
				if len(msgs) > 0 {
					userMsg = msgs[len(msgs)-1]
				}

				snapshotProvider, snapshotModel, snapshotAPIFormat, snapshotRoleName, snapshotRolePrompt := m.generationSnapshotForTab(t)

				// Create assistant message
				assistantMsg := chat.Message{
					Seq:                userMsg.Seq,
					Role:               "assistant",
					Content:            t.currentResp,
					VersionNumber:      1,
					SnapshotProvider:   snapshotProvider,
					SnapshotModel:      snapshotModel,
					SnapshotAPIFormat:  snapshotAPIFormat,
					SnapshotRoleName:   snapshotRoleName,
					SnapshotRolePrompt: snapshotRolePrompt,
				}

				// Persist assistant message to storage
				assistantID, err := m.store.AppendMessage(t.autoSaveName, assistantMsg)
				if err == nil {
					assistantMsg.ID = assistantID
				}

				t.history.Add(assistantMsg)
			}
			t.currentResp = ""
			t.currentThinking = ""
			if msg.TabIdx == m.activeTab {
				t.viewport.SetContent(m.buildChatContent())
				if t.chatFollowBottom {
					t.viewport.GotoBottom()
				}
			}
		}
		if msg.TabIdx == m.activeTab {
			m.escCount = 0
		}
		return m, nil

	case streamErrMsg:
		if msg.TabIdx < len(m.tabs) {
			t := &m.tabs[msg.TabIdx]
			t.streaming = false
			errorText := "Error: " + msg.Err.Error()
			snapshotProvider, snapshotModel, snapshotAPIFormat, snapshotRoleName, snapshotRolePrompt := m.generationSnapshotForTab(t)
			assistantMsg := chat.Message{
				Seq:                nextSeq(t.history.Messages()),
				Role:               "assistant",
				Content:            errorText,
				VersionNumber:      1,
				SnapshotProvider:   snapshotProvider,
				SnapshotModel:      snapshotModel,
				SnapshotAPIFormat:  snapshotAPIFormat,
				SnapshotRoleName:   snapshotRoleName,
				SnapshotRolePrompt: snapshotRolePrompt,
			}
			msgs := t.history.Messages()
			if len(msgs) > 0 && msgs[len(msgs)-1].Role == "user" {
				assistantMsg.Seq = msgs[len(msgs)-1].Seq
			}
			if assistantID, err := m.store.AppendMessage(t.autoSaveName, assistantMsg); err == nil {
				assistantMsg.ID = assistantID
			} else {
				m.statusMsg = "Save failed: " + err.Error()
			}
			t.history.Add(assistantMsg)
			t.err = nil
			t.currentResp = ""
			t.currentThinking = ""
			if msg.TabIdx == m.activeTab {
				t.viewport.SetContent(m.buildChatContent())
				if t.chatFollowBottom {
					t.viewport.GotoBottom()
				}
			}
		}
		return m, nil

	case commandResultMsg:
		m.statusMsg = msg.Text
		return m, nil

	case configSavedMsg:
		if msg.Err != nil {
			m.statusMsg = "Config save failed: " + msg.Err.Error()
		} else {
			m.statusMsg = "Config saved"
		}
		return m, nil

	case autoSavedMsg:
		if msg.Err != nil {
			m.statusMsg = "Auto-save failed: " + msg.Err.Error()
		}
		return m, nil

	case shortcutsSavedMsg:
		if msg.Err != nil {
			m.statusMsg = "Shortcuts save failed: " + msg.Err.Error()
		} else {
			m.statusMsg = "Shortcuts saved"
		}
		return m, nil

	case rolesSavedMsg:
		if msg.Err != nil {
			m.statusMsg = "Roles save failed: " + msg.Err.Error()
		} else {
			m.statusMsg = "Roles saved"
		}
		return m, nil

	case clipboardResultMsg:
		if msg.Err != nil {
			m.statusMsg = "Copy failed: " + msg.Err.Error()
		} else {
			m.statusMsg = "Copied"
		}
		return m, nil

	case clipboardImageMsg:
		if msg.Err != nil {
			m.statusMsg = "Paste failed: " + msg.Err.Error()
			return m, nil
		}
		tab := &m.tabs[m.activeTab]
		if msg.Image != nil {
			tab.imageCounter++
			tab.pendingImages = append(tab.pendingImages, *msg.Image)
			tab.textarea.InsertString(fmt.Sprintf("[image %d]", tab.imageCounter))
			m.statusMsg = fmt.Sprintf("Image %d pasted", tab.imageCounter)
			return m, nil
		}
		tab.textarea.InsertString(msg.Text)
		if tab.textarea.Value() == "/" {
			m.mode = modeSlashComplete
			m.slashAC = slashComplete{
				matches: filterSlashCmds("/"),
				cursor:  0,
				offset:  0,
			}
		}
		return m, nil
	}

	return m, nil
}

func (m Model) updateTabRenameMode(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		name := strings.TrimSpace(m.tabRename)
		if name == "" {
			name = m.tabs[m.activeTab].client.Model()
		}
		m.tabs[m.activeTab].name = name
		m.tabRename = ""
		m.mode = modeChat
	case "esc":
		m.tabRename = ""
		m.mode = modeChat
	case "backspace":
		runes := []rune(m.tabRename)
		if len(runes) > 0 {
			m.tabRename = string(runes[:len(runes)-1])
		}
	default:
		if msg.Type == tea.KeyRunes {
			m.tabRename += string(msg.Runes)
		}
	}
	return m, nil
}

func (m *Model) newTab() tea.Cmd {
	current := m.tabs[m.activeTab]
	tab, err := newTabSession(m.cfg, m.tabs[m.activeTab].client, m.width)
	if err != nil {
		m.statusMsg = "Failed to create tab: " + err.Error()
		return nil
	}
	tab.providerConfigName = current.providerConfigName
	tab.apiFormat = current.apiFormat
	tab.modelConfigName = current.modelConfigName
	m.tabs = append(m.tabs, tab)
	m.activeTab = len(m.tabs) - 1
	m.syncActiveTabWindowSize()
	m.statusMsg = ""
	return nil
}

func (m *Model) closeTab(idx int) {
	if len(m.tabs) <= 1 {
		m.statusMsg = "Cannot close last tab"
		return
	}
	// Cancel in-flight stream
	if m.tabs[idx].streamCtrl != nil {
		m.tabs[idx].streamCtrl.cancel()
	}
	m.tabs = append(m.tabs[:idx], m.tabs[idx+1:]...)
	if m.activeTab >= len(m.tabs) {
		m.activeTab = len(m.tabs) - 1
	} else if m.activeTab > idx {
		m.activeTab--
	}
	// Refresh viewport content for newly active tab
	m.tabs[m.activeTab].viewport.SetContent(m.buildChatContent())
	if m.tabs[m.activeTab].chatFollowBottom {
		m.tabs[m.activeTab].viewport.GotoBottom()
	}
}

func (m *Model) switchTab(idx int) {
	if idx < 0 || idx >= len(m.tabs) {
		return
	}
	if idx == m.activeTab {
		return
	}
	m.activeTab = idx
	m.syncActiveTabWindowSize()
	m.tabs[idx].viewport.SetContent(m.buildChatContent())
	if m.tabs[idx].chatFollowBottom {
		m.tabs[idx].viewport.GotoBottom()
	}
}

func (m *Model) syncActiveTabWindowSize() {
	if len(m.tabs) == 0 {
		return
	}
	tab := &m.tabs[m.activeTab]
	if m.width > 0 {
		tab.viewport.Width = m.width
		tab.textarea.SetWidth(m.width)
	}
	vpHeight := m.height - 2
	if vpHeight < 1 {
		vpHeight = 1
	}
	tab.viewport.Height = vpHeight
}

// looksLikeSGRMouse reports whether s contains an SGR mouse sequence
// (e.g. "<65;43;25M") that leaked through as key runes.
func looksLikeSGRMouse(s string) bool {
	return len(s) > 3 && s[0] == '<'
}

func isScrollKey(key string) bool {
	switch key {
	case "up", "down", "pgup", "pgdown", "home", "end":
		return true
	}
	return false
}

func (m Model) sendStreamCmd(ctx context.Context, tabIdx int) tea.Cmd {
	tab := m.tabs[tabIdx]
	messages := tab.history.ToAPIMessages()
	client := tab.client
	temp, maxTok, reasoningEffort, budgetTokens := m.generationParametersForTab(&tab)

	return func() tea.Msg {
		chunks, errs := client.SendStreamChan(ctx, messages, temp, maxTok, reasoningEffort, budgetTokens)
		return streamStartMsg{TabIdx: tabIdx, chunks: chunks, errs: errs}
	}
}

func (m Model) readNextChunk(tabIdx int) tea.Cmd {
	ch := m.tabs[tabIdx].streamCh
	errCh := m.tabs[tabIdx].streamErr
	return func() tea.Msg {
		select {
		case chunk, ok := <-ch:
			if !ok {
				// Channel closed, check for errors
				select {
				case err := <-errCh:
					if err != nil {
						return streamErrMsg{TabIdx: tabIdx, Err: err}
					}
				default:
				}
				return streamDoneMsg{TabIdx: tabIdx}
			}
			return streamChunkMsg{TabIdx: tabIdx, Content: chunk.Content, Thinking: chunk.Thinking}
		case err := <-errCh:
			if err != nil {
				return streamErrMsg{TabIdx: tabIdx, Err: err}
			}
			return streamDoneMsg{TabIdx: tabIdx}
		}
	}
}

func (m Model) autoSaveCmd(tabIdx int) tea.Cmd {
	msgs := m.tabs[tabIdx].history.Messages()
	store := m.store
	name := m.tabs[tabIdx].autoSaveName
	return func() tea.Msg {
		err := store.Save(name, msgs)
		return autoSavedMsg{TabIdx: tabIdx, Err: err}
	}
}

func (m Model) pasteFromClipboard() tea.Cmd {
	return func() tea.Msg {
		img, err := readImageFromClipboard()
		if err != nil {
			return clipboardImageMsg{Err: err}
		}
		if img != nil {
			return clipboardImageMsg{Image: img}
		}
		text, err := readTextFromClipboard()
		return clipboardImageMsg{Text: text, Err: err}
	}
}

func (m Model) handleCommand(input string) (tea.Model, tea.Cmd) {
	m.escCount = 0
	parts := strings.Fields(input)
	cmd := parts[0]

	switch cmd {
	case "/exit":
		return m, tea.Quit

	case "/clear":
		tab := &m.tabs[m.activeTab]
		tab.history.Clear()
		tab.totalTokens = 0
		tab.pendingImages = nil
		tab.imageCounter = 0
		tab.chatFollowBottom = true
		tab.viewport.SetContent("")
		tab.viewport.GotoBottom()
		m.statusMsg = "Conversation cleared"
		return m, nil

	case "/settings":
		if len(parts) < 2 {
			// No argument: enter config editor mode
			m.mode = modeConfig
			m.configEd = configEditor{
				fields: buildConfigFields(m.cfg),
			}
			return m, nil
		}
		m.activeTabSession().client.SetModel(parts[1])
		m.cfg.API.Model = parts[1]
		m.activeTabSession().name = parts[1]
		m.statusMsg = "Model set to " + parts[1]
		return m, m.saveConfigCmd()

	case "/history":
		convs, err := m.store.ListWithDate()
		if err != nil {
			m.statusMsg = "Failed to load conversations: " + err.Error()
			return m, nil
		}
		allItems, err := m.store.LoadAllForSearch()
		if err != nil {
			m.statusMsg = "Failed to load search index: " + err.Error()
			return m, nil
		}
		pick := buildResumePicker(convs)
		pick.allItems = allItems
		m.resumePick = pick
		m.mode = modeResume
		return m, nil

	case "/model":
		m.modelSel = newModelSelectorState(m.modelRegistry, modelSelectorManage)
		m.mode = modeModelSelector
		return m, nil

	case "/shortcuts":
		items, err := shortcuts.Load(m.shortcutsPath)
		if err != nil {
			m.statusMsg = "Failed to load shortcuts: " + err.Error()
			return m, nil
		}
		m.shortcutEd = shortcutEditor{items: items}
		m.mode = modeShortcuts
		return m, nil

	case "/roles":
		items, err := roles.Load(m.rolesPath)
		if err != nil {
			m.statusMsg = "Failed to load roles: " + err.Error()
			return m, nil
		}
		if len(items) == 0 {
			items = append([]roles.Role{}, roles.Defaults...)
		}
		m.roleEd = roleEditor{items: items}
		m.mode = modeRoles
		return m, nil

	default:
		m.statusMsg = "Unknown command: " + cmd
		return m, nil
	}
}

// nextSeq returns the next sequence number for a new message.
func nextSeq(msgs []chat.Message) int {
	if len(msgs) == 0 {
		return 1
	}
	return msgs[len(msgs)-1].Seq + 1
}

// reloadActiveTimeline reloads the active timeline from storage for the given tab.
func (m *Model) reloadActiveTimeline(tabIdx int) error {
	name := m.tabs[tabIdx].autoSaveName
	msgs, err := m.store.LoadActiveTimeline(name)
	if err != nil {
		return err
	}
	m.tabs[tabIdx].history.ReplaceMessages(msgs)
	m.tabs[tabIdx].viewport.SetContent(m.buildChatContent())
	m.tabs[tabIdx].viewport.GotoBottom()
	m.tabs[tabIdx].chatFollowBottom = true
	return nil
}
