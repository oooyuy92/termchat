// internal/ui/messagebrowse_view_test.go
package ui

import (
	"strings"
	"testing"

	xansi "github.com/charmbracelet/x/ansi"
	"github.com/termchat/termchat/internal/chat"
	"github.com/termchat/termchat/internal/config"
)

func newMessageBrowseViewModel(t *testing.T) Model {
	t.Helper()
	cfg := config.DefaultConfig()
	tab, err := newTabSession(cfg, &stubProvider{model: "test-model"}, 80)
	if err != nil {
		t.Fatalf("newTabSession() error = %v", err)
	}
	return Model{
		cfg:       cfg,
		theme:     DarkTheme,
		tabs:      []TabSession{tab},
		activeTab: 0,
		width:     80,
		height:    24,
	}
}

func TestViewMessageBrowse_RendersTurnPanes(t *testing.T) {
	m := newMessageBrowseViewModel(t)
	m.mode = modeMessageBrowse
	m.messageBrowse.mode = browseModeMessage
	m.messageBrowse.turns = []browseTurn{
		{
			User: chat.Message{Role: "user", Content: "left pane"},
			AssistantVersions: []chat.Message{
				{Role: "assistant", Content: "right pane", VersionNumber: 1, TotalVersions: 2},
			},
		},
	}

	out := m.viewMessageBrowse()
	if !strings.Contains(out, "User") {
		t.Fatalf("output missing User header: %q", out)
	}
	if !strings.Contains(out, "Assistant v1/2") {
		t.Fatalf("output missing Assistant version header: %q", out)
	}
}

func TestRenderBrowsePane_DoesNotLeakANSIFragments(t *testing.T) {
	out := renderBrowsePane("Assistant", "\x1b[34mabcdefghi\x1b[0m", 0, 4, 12, DarkTheme)
	plain := xansi.Strip(out)
	compacted := strings.Join(strings.Fields(plain), "")

	if strings.Contains(plain, "[34m") || strings.Contains(plain, "[0m") || strings.Contains(plain, "34m") {
		t.Fatalf("pane leaked ANSI fragments: %q", plain)
	}
	if !strings.Contains(compacted, "abcdefghi") {
		t.Fatalf("pane lost content after rendering: %q", plain)
	}
}

func TestViewMessageBrowse_HelpIncludesDeleteAndBranchWhenNarrow(t *testing.T) {
	m := newMessageBrowseViewModel(t)
	m.width = 48
	m.mode = modeMessageBrowse
	m.messageBrowse.mode = browseModeMessage
	m.messageBrowse.turns = []browseTurn{
		{
			User: chat.Message{Role: "user", Content: "left pane"},
			AssistantVersions: []chat.Message{
				{Role: "assistant", Content: "right pane", VersionNumber: 1, TotalVersions: 2},
			},
		},
	}

	out := xansi.Strip(m.viewMessageBrowse())
	if !strings.Contains(out, "d: delete") {
		t.Fatalf("output missing delete help: %q", out)
	}
	if !strings.Contains(out, "b: branch") {
		t.Fatalf("output missing branch help: %q", out)
	}
	if !strings.Contains(out, "u: undo") {
		t.Fatalf("output missing undo help: %q", out)
	}
}

func TestViewMessageBrowseShowsProviderAndModelLabels(t *testing.T) {
	m := newMessageBrowseViewModel(t)
	m.width = 140
	m.mode = modeMessageBrowse
	m.messageBrowse.mode = browseModeMessage
	m.messageBrowse.turns = []browseTurn{
		{
			User: chat.Message{Role: "user", Content: "left pane"},
			AssistantVersions: []chat.Message{
				{
					Role:             "assistant",
					Content:          "right pane",
					VersionNumber:    1,
					TotalVersions:    2,
					SnapshotProvider: "gateway",
					SnapshotModel:    "gemini-3-flash-preview",
				},
			},
		},
	}

	out := m.viewMessageBrowse()
	if !strings.Contains(out, "gateway") {
		t.Fatalf("output missing provider label: %q", out)
	}
	if !strings.Contains(out, "gemini-3-flash-preview") {
		t.Fatalf("output missing model label: %q", out)
	}
}
