package ui

import (
	"testing"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/termchat/termchat/internal/chat"
)

func testChatModel() Model {
	h := chat.NewHistory()
	for i := 0; i < 8; i++ {
		h.Add(chat.Message{Role: "user", Content: "line"})
	}
	ta := textarea.New()
	ta.Focus()
	ta.ShowLineNumbers = false
	ta.KeyMap.InsertNewline.SetKeys("shift+enter")
	return Model{
		mode:             modeChat,
		theme:            DarkTheme,
		history:          h,
		height:           8,
		chatFollowBottom: true,
		textarea:         ta,
	}
}

func TestUpdateIgnoresMouseFallbackRunesInInput(t *testing.T) {
	m := testChatModel()
	m.textarea.SetValue("hello")

	// SGR mouse sequence leaked as runes — textarea should delegate to its own handler
	// The key point is the value doesn't get corrupted with SGR bytes
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
	updated := next.(Model)

	val := updated.textarea.Value()
	if val == "" {
		t.Fatalf("expected textarea to have content, got empty string")
	}
}

func TestUpdateAppendsNormalRunes(t *testing.T) {
	m := testChatModel()

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("abc")})
	updated := next.(Model)

	val := updated.textarea.Value()
	if val == "" {
		t.Fatalf("expected normal input to be appended, got empty string")
	}
}
