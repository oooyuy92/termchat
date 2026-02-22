// internal/ui/update.go
package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/termchat/termchat/internal/chat"
)

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		if m.mode == modeConfig {
			return m.updateConfigMode(msg)
		}

		if m.streaming {
			if msg.String() == "ctrl+c" {
				return m, tea.Quit
			}
			return m, nil
		}

		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit

		case "enter":
			input := strings.TrimSpace(m.input)
			if input == "" {
				return m, nil
			}
			m.input = ""

			if strings.HasPrefix(input, "/") {
				return m.handleCommand(input)
			}

			m.history.Add(chat.Message{Role: "user", Content: input})
			m.streaming = true
			m.currentResp = ""
			m.err = nil

			return m, m.sendStreamCmd()

		case "backspace":
			runes := []rune(m.input)
			if len(runes) > 0 {
				m.input = string(runes[:len(runes)-1])
			}

		default:
			// Handle rune input for multi-byte characters
			if msg.Type == tea.KeyRunes {
				m.input += string(msg.Runes)
			}
		}
		return m, nil

	case streamChunkMsg:
		m.currentResp += msg.Content
		return m, m.readNextChunk()

	case streamStartMsg:
		m.streamCh = msg.chunks
		m.streamErr = msg.errs
		return m, m.readNextChunk()

	case streamDoneMsg:
		m.streaming = false
		if m.currentResp != "" {
			m.history.Add(chat.Message{Role: "assistant", Content: m.currentResp})
		}
		m.currentResp = ""
		return m, nil

	case streamErrMsg:
		m.streaming = false
		m.err = msg.Err
		m.currentResp = ""
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
	}

	return m, nil
}

func (m Model) sendStreamCmd() tea.Cmd {
	messages := m.history.ToAPIMessages()
	client := m.client
	temp := m.cfg.Parameters.Temperature
	maxTok := m.cfg.Parameters.MaxTokens
	reasoningEffort := m.cfg.Parameters.ReasoningEffort

	return func() tea.Msg {
		chunks, errs := client.SendStreamChan(messages, temp, maxTok, reasoningEffort)
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
			return streamChunkMsg{Content: chunk}
		case err := <-errCh:
			return streamErrMsg{Err: err}
		}
	}
}

func (m Model) handleCommand(input string) (tea.Model, tea.Cmd) {
	parts := strings.Fields(input)
	cmd := parts[0]

	switch cmd {
	case "/exit":
		return m, tea.Quit

	case "/clear":
		m.history.Clear()
		m.totalTokens = 0
		m.statusMsg = "Conversation cleared"
		return m, nil

	case "/model":
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

	case "/save":
		name := "default"
		if len(parts) >= 2 {
			name = parts[1]
		}
		err := m.store.Save(name, m.history.Messages())
		if err != nil {
			m.statusMsg = "Save failed: " + err.Error()
		} else {
			m.statusMsg = "Saved as " + name
		}
		return m, nil

	case "/load":
		name := "default"
		if len(parts) >= 2 {
			name = parts[1]
		}
		msgs, err := m.store.Load(name)
		if err != nil {
			m.statusMsg = "Load failed: " + err.Error()
		} else {
			m.history.Clear()
			for _, msg := range msgs {
				m.history.Add(msg)
			}
			m.statusMsg = "Loaded " + name
		}
		return m, nil

	case "/list":
		names, err := m.store.List()
		if err != nil {
			m.statusMsg = "List failed: " + err.Error()
		} else if len(names) == 0 {
			m.statusMsg = "No saved conversations"
		} else {
			m.statusMsg = "Saved: " + strings.Join(names, ", ")
		}
		return m, nil

	default:
		m.statusMsg = "Unknown command: " + cmd
		return m, nil
	}
}
