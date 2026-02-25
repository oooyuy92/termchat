// internal/ui/update.go
package ui

import (
	"context"
	"fmt"
	"strconv"
	"strings"

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
		return m, nil

	case tea.KeyMsg:
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
		if m.mode == modeMessageBrowse {
			return m.updateMessageBrowse(msg)
		}

		if m.streaming {
			if m.handleChatScrollKey(msg.String()) {
				return m, nil
			}
			if msg.String() == "ctrl+c" {
				if m.confirmQuit {
					return m, tea.Quit
				}
				// Cancel the in-flight HTTP request
				if m.streamCtrl != nil {
					m.streamCtrl.cancel()
				}
				m.streaming = false
				m.currentResp = ""
				m.currentThinking = ""
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
			m.handleChatScrollKey(msg.String())
			return m, nil

		case "esc":
			m.escCount++
			if m.escCount >= 2 {
				m.escCount = 0
				if len(m.history.Messages()) == 0 {
					m.statusMsg = "No messages to browse"
					return m, nil
				}
				m.browseCursor = len(m.history.Messages()) - 1
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
			input := strings.TrimSpace(m.input)
			if input == "" {
				return m, nil
			}
			m.input = ""

			if strings.HasPrefix(input, "/") {
				return m.handleCommand(input)
			}

			m.history.Add(chat.Message{Role: "user", Content: input, Images: m.pendingImages})
			m.pendingImages = nil
			m.streaming = true
			m.chatFollowBottom = true
			m.currentResp = ""
			m.currentThinking = ""
			m.err = nil

			ctx, cancel := context.WithCancel(context.Background())
			m.streamCtrl = &streamControl{cancel: cancel}

			return m, m.sendStreamCmd(ctx)

		case "backspace":
			runes := []rune(m.input)
			if len(runes) > 0 {
				m.input = string(runes[:len(runes)-1])
			}

		case "ctrl+v":
			return m, m.pasteFromClipboard()

		default:
			if msg.Type == tea.KeyRunes {
				raw := string(msg.Runes)
				// Some terminal/tmux combinations leak SGR mouse bytes as runes
				// (e.g. "<65;43;25M"). Swallow them so they don't pollute input.
				if m.handleMouseFallbackRunes(raw) {
					return m, nil
				}
				m.input += raw
				// Enter slash autocomplete mode when "/" is typed as the first character
				if m.input == "/" {
					m.mode = modeSlashComplete
					m.slashAC = slashComplete{
						matches: filterSlashCmds("/"),
						cursor:  0,
						offset:  0,
					}
				}
			}
		}
		return m, nil

	case streamChunkMsg:
		m.currentResp += msg.Content
		m.currentThinking += msg.Thinking
		return m, m.readNextChunk()

	case streamStartMsg:
		m.streamCh = msg.chunks
		m.streamErr = msg.errs
		return m, m.readNextChunk()

	case streamDoneMsg:
		m.streaming = false
		m.escCount = 0
		if m.currentResp != "" {
			m.history.Add(chat.Message{Role: "assistant", Content: m.currentResp})
		}
		m.currentResp = ""
		m.currentThinking = ""
		return m, m.autoSaveCmd()

	case streamErrMsg:
		m.streaming = false
		m.escCount = 0
		m.err = msg.Err
		m.currentResp = ""
		m.currentThinking = ""
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
		if msg.Image != nil {
			m.imageCounter++
			m.pendingImages = append(m.pendingImages, *msg.Image)
			m.input += fmt.Sprintf("[image %d]", m.imageCounter)
			m.statusMsg = fmt.Sprintf("Image %d pasted", m.imageCounter)
			return m, nil
		}
		m.input += msg.Text
		if m.input == "/" {
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

// handleMouseFallbackRunes parses mouse SGR fragments that may arrive as text
// and converts wheel events into chat scrolling.
func (m *Model) handleMouseFallbackRunes(raw string) bool {
	handled := false
	for i := 0; i < len(raw); i++ {
		if raw[i] != '<' {
			continue
		}
		j := i + 1
		for j < len(raw) {
			c := raw[j]
			if (c >= '0' && c <= '9') || c == ';' {
				j++
				continue
			}
			break
		}
		if j >= len(raw) || (raw[j] != 'M' && raw[j] != 'm') || j == i+1 {
			continue
		}

		parts := strings.Split(raw[i+1:j], ";")
		if len(parts) != 3 {
			handled = true
			i = j
			continue
		}
		btn, err := strconv.Atoi(parts[0])
		if err == nil {
			// xterm mouse encoding puts modifiers on top of base button code.
			baseBtn := btn &^ (4 | 8 | 16)
			switch baseBtn {
			case 64:
				m.scrollChatBy(-3)
			case 65:
				m.scrollChatBy(3)
			}
		}
		handled = true
		i = j
	}
	return handled
}

func (m Model) maxChatScrollTop() int {
	return m.maxChatScrollForContent(m.buildChatContent())
}

func (m *Model) scrollChatBy(delta int) bool {
	maxTop := m.maxChatScrollTop()
	if maxTop == 0 {
		m.chatScrollTop = 0
		m.chatFollowBottom = true
		return false
	}

	oldTop := m.chatScrollTop
	oldFollow := m.chatFollowBottom

	m.chatFollowBottom = false
	m.chatScrollTop += delta
	if m.chatScrollTop < 0 {
		m.chatScrollTop = 0
	}
	if m.chatScrollTop > maxTop {
		m.chatScrollTop = maxTop
	}
	if m.chatScrollTop == maxTop {
		m.chatFollowBottom = true
	}
	return oldTop != m.chatScrollTop || oldFollow != m.chatFollowBottom
}

func (m *Model) scrollChatToTop() bool {
	oldTop := m.chatScrollTop
	oldFollow := m.chatFollowBottom
	m.chatScrollTop = 0
	m.chatFollowBottom = false
	return oldTop != m.chatScrollTop || oldFollow != m.chatFollowBottom
}

func (m *Model) scrollChatToBottom() bool {
	maxTop := m.maxChatScrollTop()
	oldTop := m.chatScrollTop
	oldFollow := m.chatFollowBottom
	m.chatScrollTop = maxTop
	m.chatFollowBottom = true
	return oldTop != m.chatScrollTop || oldFollow != m.chatFollowBottom
}

func (m *Model) handleChatScrollKey(key string) bool {
	switch key {
	case "up":
		return m.scrollChatBy(-1)
	case "down":
		return m.scrollChatBy(1)
	case "pgup":
		return m.scrollChatBy(-(m.chatViewportHeight() - 1))
	case "pgdown":
		return m.scrollChatBy(m.chatViewportHeight() - 1)
	case "home":
		return m.scrollChatToTop()
	case "end":
		return m.scrollChatToBottom()
	default:
		return false
	}
}

func (m Model) sendStreamCmd(ctx context.Context) tea.Cmd {
	messages := m.history.ToAPIMessages()
	client := m.client
	temp := m.cfg.Parameters.Temperature
	maxTok := m.cfg.Parameters.MaxTokens
	reasoningEffort := m.cfg.Parameters.ReasoningEffort
	budgetTokens := m.cfg.Parameters.BudgetTokens

	return func() tea.Msg {
		chunks, errs := client.SendStreamChan(ctx, messages, temp, maxTok, reasoningEffort, budgetTokens)
		return streamStartMsg{chunks: chunks, errs: errs}
	}
}

func (m Model) readNextChunk() tea.Cmd {
	ch := m.streamCh
	errCh := m.streamErr
	return func() tea.Msg {
		select {
		case chunk, ok := <-ch:
			if !ok {
				// Channel closed, check for errors
				select {
				case err := <-errCh:
					if err != nil {
						return streamErrMsg{Err: err}
					}
				default:
				}
				return streamDoneMsg{}
			}
			return streamChunkMsg{Content: chunk.Content, Thinking: chunk.Thinking}
		case err := <-errCh:
			if err != nil {
				return streamErrMsg{Err: err}
			}
			return streamDoneMsg{}
		}
	}
}

func (m Model) autoSaveCmd() tea.Cmd {
	msgs := m.history.Messages()
	store := m.store
	name := m.autoSaveName
	return func() tea.Msg {
		err := store.Save(name, msgs)
		return autoSavedMsg{Err: err}
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
		m.history.Clear()
		m.totalTokens = 0
		m.pendingImages = nil
		m.imageCounter = 0
		m.chatScrollTop = 0
		m.chatFollowBottom = true
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
		m.client.SetModel(parts[1])
		m.cfg.API.Model = parts[1]
		m.statusMsg = "Model set to " + parts[1]
		return m, m.saveConfigCmd()

	case "/history":
		convs, err := m.store.ListWithDate()
		if err != nil {
			m.statusMsg = "Failed to load conversations: " + err.Error()
			return m, nil
		}
		m.resumePick = buildResumePicker(convs)
		m.mode = modeResume
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
