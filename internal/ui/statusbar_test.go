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
func (s *statusStubProvider) SupportsVision() bool  { return false }

func TestRenderStatusBarWrapsToSecondLineWhenNarrow(t *testing.T) {
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
	lines := strings.Split(xansi.Strip(bar), "\n")
	if len(lines) != 2 {
		t.Fatalf("status bar line count = %d, want 2: %q", len(lines), bar)
	}
	for _, line := range lines {
		if got := xansi.StringWidth(line); got > m.width {
			t.Fatalf("status bar line width %d exceeds terminal width %d: %q", got, m.width, line)
		}
	}
}

func TestRenderStatusBarStaysSingleLineWhenWideEnough(t *testing.T) {
	tab := TabSession{
		history: chat.NewHistory(),
		client:  &statusStubProvider{model: "gemini-3-pro-preview"},
	}
	m := Model{
		theme:     DarkTheme,
		width:     120,
		tabs:      []TabSession{tab},
		activeTab: 0,
		statusMsg: "Ready",
	}

	bar := m.renderStatusBar()
	if strings.Count(xansi.Strip(bar), "\n") != 0 {
		t.Fatalf("status bar unexpectedly wrapped: %q", bar)
	}
}
