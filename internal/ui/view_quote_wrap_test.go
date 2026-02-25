package ui

import (
	"context"
	"strings"
	"testing"

	xansi "github.com/charmbracelet/x/ansi"
	"github.com/termchat/termchat/internal/chat"
)

type stubProvider struct {
	model string
}

func (s *stubProvider) SendStreamChan(_ context.Context, _ []chat.Message, _ float64, _ int, _ string, _ int) (<-chan chat.StreamChunk, <-chan error) {
	return nil, nil
}

func (s *stubProvider) Model() string         { return s.model }
func (s *stubProvider) SetModel(model string) { s.model = model }
func (s *stubProvider) SetBaseURL(string)     {}
func (s *stubProvider) SetAPIKey(string)      {}
func (s *stubProvider) BaseURL() string       { return "" }
func (s *stubProvider) APIKey() string        { return "" }

func TestBuildChatContentQuoteWrapKeepsPrefix(t *testing.T) {
	renderer, err := buildRenderer("dark", 24)
	if err != nil {
		t.Fatalf("buildRenderer() error = %v", err)
	}

	history := chat.NewHistory()
	history.Add(chat.Message{
		Role:    "assistant",
		Content: "> " + strings.Repeat("汉", 80),
	})

	m := Model{
		theme:    DarkTheme,
		history:  history,
		renderer: renderer,
		client:   &stubProvider{model: "test-model"},
		width:    24,
	}

	out := xansi.Strip(m.buildChatContent())
	lines := strings.Split(out, "\n")

	quoteContentLines := 0
	for _, line := range lines {
		if !strings.Contains(line, "汉") {
			continue
		}
		quoteContentLines++
		if !strings.HasPrefix(line, "│ ") {
			t.Fatalf("quote continuation lost prefix: %q", line)
		}
	}
	if quoteContentLines < 2 {
		t.Fatalf("expected wrapped quote to span multiple lines, got %d", quoteContentLines)
	}
}
