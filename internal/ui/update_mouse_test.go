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

func TestHandleMouseFallbackRunesScrollsAndSwallows(t *testing.T) {
	m := testChatModel()

	if !m.handleMouseFallbackRunes("1<65;43;25ML") {
		t.Fatalf("expected mouse fallback runes to be detected")
	}
	if m.chatScrollTop == 0 {
		t.Fatalf("expected wheel-down fallback to scroll chat")
	}
	if m.chatFollowBottom {
		t.Fatalf("expected manual scroll to disable follow-bottom")
	}

	topAfterDown := m.chatScrollTop
	if !m.handleMouseFallbackRunes("<64;43;25M") {
		t.Fatalf("expected wheel-up fallback to be detected")
	}
	if m.chatScrollTop >= topAfterDown {
		t.Fatalf("expected wheel-up fallback to decrease scrollTop")
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
