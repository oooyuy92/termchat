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
func (s *stubProvider) SupportsVision() bool  { return false }

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

	tab := TabSession{
		history:  history,
		renderer: renderer,
		client:   &stubProvider{model: "test-model"},
	}
	m := Model{
		theme:     DarkTheme,
		tabs:      []TabSession{tab},
		activeTab: 0,
		width:     24,
	}

	out := xansi.Strip(m.buildChatContent())
	lines := strings.Split(out, "\n")

	quoteContentLines := 0
	for _, line := range lines {
		if !strings.Contains(line, "汉") {
			continue
		}
		quoteContentLines++
		if !strings.HasPrefix(strings.TrimLeft(line, " "), "│ ") {
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

	tab := TabSession{
		history:  history,
		renderer: renderer,
		client:   &stubProvider{model: "test-model"},
	}
	m := Model{
		theme:     DarkTheme,
		tabs:      []TabSession{tab},
		activeTab: 0,
		width:     24,
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

	tab := TabSession{
		history:  history,
		renderer: renderer,
		client:   &stubProvider{model: "test-model"},
	}
	m := Model{
		theme:     DarkTheme,
		tabs:      []TabSession{tab},
		activeTab: 0,
		width:     24,
	}

	out := xansi.Strip(m.buildChatContent())
	lines := strings.Split(out, "\n")

	quoteContentLines := 0
	for _, line := range lines {
		if !strings.Contains(line, "甲") && !strings.Contains(line, "乙") {
			continue
		}
		quoteContentLines++
		if !strings.HasPrefix(strings.TrimLeft(line, " "), "│ ") {
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

	tab := TabSession{
		history:  history,
		renderer: renderer,
		client:   &stubProvider{model: "test-model"},
	}
	m := Model{
		theme:     DarkTheme,
		tabs:      []TabSession{tab},
		activeTab: 0,
		width:     width,
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
	if maxWidth > width-5 {
		t.Fatalf("line width %d exceeds safe headroom width %d", maxWidth, width-5)
	}
}

func TestBuildChatContentKeepsDecimalTokenIntact(t *testing.T) {
	renderer, err := buildRenderer("dark", 24)
	if err != nil {
		t.Fatalf("buildRenderer() error = %v", err)
	}

	history := chat.NewHistory()
	history.Add(chat.Message{
		Role:    "assistant",
		Content: "它位于中国和尼泊尔边境线上，它的海拔高度为 8848.86 米。",
	})

	tab := TabSession{
		history:  history,
		renderer: renderer,
		client:   &stubProvider{model: "test-model"},
	}
	m := Model{
		theme:     DarkTheme,
		tabs:      []TabSession{tab},
		activeTab: 0,
		width:     24,
	}

	out := xansi.Strip(m.buildChatContent())
	if strings.Contains(out, "8848.\n86") {
		t.Fatalf("decimal token was split across lines: %q", out)
	}
	if !strings.Contains(out, "8848.86") {
		t.Fatalf("rendered output missing decimal token: %q", out)
	}
}

func TestBuildChatContentAddsHorizontalInsetToMessages(t *testing.T) {
	renderer, err := buildRenderer("dark", 40)
	if err != nil {
		t.Fatalf("buildRenderer() error = %v", err)
	}

	history := chat.NewHistory()
	history.Add(chat.Message{
		Role:    "assistant",
		Content: "你好",
	})

	tab := TabSession{
		history:  history,
		renderer: renderer,
		client:   &stubProvider{model: "test-model"},
	}
	m := Model{
		theme:     DarkTheme,
		tabs:      []TabSession{tab},
		activeTab: 0,
		width:     40,
	}

	out := xansi.Strip(m.buildChatContent())
	lines := strings.Split(out, "\n")
	for _, line := range lines {
		if strings.Contains(line, "test-model:") || strings.Contains(line, "你好") {
			if !strings.HasPrefix(line, " ") {
				t.Fatalf("message line missing horizontal inset: %q", line)
			}
		}
	}
}

func TestWrapRenderedLineKeepsDecimalTokenIntact(t *testing.T) {
	line := "海拔高度为 8848.86 米。"
	got := wrapRenderedLine(line, 16)
	if strings.Contains(got, "8848.\n86") {
		t.Fatalf("decimal token was split across lines: %q", got)
	}
}

func TestBuildChatContentAvoidsAwkwardMidSentenceBreaks(t *testing.T) {
	renderer, err := buildRenderer("dark", 24)
	if err != nil {
		t.Fatalf("buildRenderer() error = %v", err)
	}

	history := chat.NewHistory()
	history.Add(chat.Message{
		Role: "assistant",
		Content: "它位于喜马拉雅山脉，也处中国和尼泊尔的边界线上。" +
			"根据中国和尼泊尔在2020年共同宣布的最新测量数据，它的最新高度为 8848.86 米。",
	})

	tab := TabSession{
		history:  history,
		renderer: renderer,
		client:   &stubProvider{model: "test-model"},
	}
	m := Model{
		theme:     DarkTheme,
		tabs:      []TabSession{tab},
		activeTab: 0,
		width:     24,
	}

	out := xansi.Strip(m.buildChatContent())
	if strings.Contains(out, "边界线上。\n根据") {
		t.Fatalf("rendered output contains renderer pre-wrap artifact: %q", out)
	}
}
