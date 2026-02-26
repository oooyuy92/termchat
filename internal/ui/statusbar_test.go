package ui

import (
	"context"
	"strings"
	"testing"

	xansi "github.com/charmbracelet/x/ansi"
	"github.com/termchat/termchat/internal/chat"
)

type statusStubProvider struct {
	model string
}

func (s *statusStubProvider) SendStreamChan(_ context.Context, _ []chat.Message, _ float64, _ int, _ string, _ int) (<-chan chat.StreamChunk, <-chan error) {
	return nil, nil
}

func (s *statusStubProvider) Model() string         { return s.model }
func (s *statusStubProvider) SetModel(model string) { s.model = model }
func (s *statusStubProvider) SetBaseURL(string)     {}
func (s *statusStubProvider) SetAPIKey(string)      {}
func (s *statusStubProvider) BaseURL() string       { return "" }
func (s *statusStubProvider) APIKey() string        { return "" }

func TestRenderStatusBarDoesNotWrapToSecondLine(t *testing.T) {
	tab := TabSession{
		history: chat.NewHistory(),
		client:  &statusStubProvider{model: "gemini-3-pro-preview-very-long"},
	}
	m := Model{
		theme:     DarkTheme,
		width:     40,
		tabs:      []TabSession{tab},
		activeTab: 0,
		statusMsg: "Resumed: 2026-02-24_134124",
	}

	bar := m.renderStatusBar()
	if strings.Count(bar, "\n") != 0 {
		t.Fatalf("status bar unexpectedly wrapped: %q", bar)
	}
	if got := xansi.StringWidth(bar); got > m.width {
		t.Fatalf("status bar width %d exceeds terminal width %d", got, m.width)
	}
}
