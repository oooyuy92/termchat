package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/termchat/termchat/internal/chat"
)

func testChatModel() Model {
	h := chat.NewHistory()
	for i := 0; i < 8; i++ {
		h.Add(chat.Message{Role: "user", Content: "line"})
	}
	return Model{
		mode:             modeChat,
		theme:            DarkTheme,
		history:          h,
		height:           8,
		chatFollowBottom: true,
	}
}



func TestUpdateIgnoresMouseFallbackRunesInInput(t *testing.T) {
	m := testChatModel()
	m.input = "hello"

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("1<65;43;25ML")})
	updated := next.(Model)

	if updated.input != "hello" {
		t.Fatalf("expected input to stay unchanged, got %q", updated.input)
	}
}

func TestUpdateAppendsNormalRunes(t *testing.T) {
	m := testChatModel()

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("abc")})
	updated := next.(Model)

	if updated.input != "abc" {
		t.Fatalf("expected normal input to be appended, got %q", updated.input)
	}
}
