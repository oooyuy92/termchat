package ui

import (
	"context"
	"regexp"
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

func TestBuildChatContentNestedQuoteWrapKeepsIndentedPrefix(t *testing.T) {
	renderer, err := buildRenderer("dark", 24)
	if err != nil {
		t.Fatalf("buildRenderer() error = %v", err)
	}

	history := chat.NewHistory()
	history.Add(chat.Message{
		Role: "assistant",
		Content: "- 设计实验流程：\n" +
			"  > " + strings.Repeat("汉", 80),
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
	indentedQuote := regexp.MustCompile(`^\s*│ `)

	quoteContentLines := 0
	for _, line := range lines {
		if !strings.Contains(line, "汉") {
			continue
		}
		quoteContentLines++
		if !indentedQuote.MatchString(line) {
			t.Fatalf("nested quote continuation lost prefix: %q", line)
		}
	}
	if quoteContentLines < 2 {
		t.Fatalf("expected wrapped nested quote to span multiple lines, got %d", quoteContentLines)
	}
}

func TestBuildChatContentLazyQuoteContinuationKeepsPrefix(t *testing.T) {
	renderer, err := buildRenderer("dark", 24)
	if err != nil {
		t.Fatalf("buildRenderer() error = %v", err)
	}

	history := chat.NewHistory()
	history.Add(chat.Message{
		Role: "assistant",
		Content: "> " + strings.Repeat("甲", 30) + "\n" +
			strings.Repeat("乙", 40),
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
		if !strings.Contains(line, "甲") && !strings.Contains(line, "乙") {
			continue
		}
		quoteContentLines++
		if !strings.HasPrefix(line, "│ ") {
			t.Fatalf("lazy quote continuation lost prefix: %q", line)
		}
	}
	if quoteContentLines < 2 {
		t.Fatalf("expected wrapped lazy quote to span multiple lines, got %d", quoteContentLines)
	}
}

func TestBuildChatContentLeavesHeadroomToAvoidTerminalSoftWrap(t *testing.T) {
	const width = 30
	renderer, err := buildRenderer("dark", width)
	if err != nil {
		t.Fatalf("buildRenderer() error = %v", err)
	}

	history := chat.NewHistory()
	history.Add(chat.Message{
		Role: "assistant",
		Content: "> [填入目标]。请帮我草拟一个标准的实验步骤流程图。需要包含：数据预处理、" +
			"基线模型选择（Baseline）、评估指标（Metrics）以及消融实验（Ablation Study）的设计建议。",
	})

	m := Model{
		theme:    DarkTheme,
		history:  history,
		renderer: renderer,
		client:   &stubProvider{model: "test-model"},
		width:    width,
	}

	out := xansi.Strip(m.buildChatContent())
	lines := strings.Split(out, "\n")
	maxWidth := 0
	for _, line := range lines {
		w := xansi.StringWidth(line)
		if w > maxWidth {
			maxWidth = w
		}
	}
	if maxWidth > width-6 {
		t.Fatalf("line width %d exceeds safe headroom width %d", maxWidth, width-6)
	}
}
