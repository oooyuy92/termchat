// internal/ui/messagebrowse_view_test.go
package ui

import (
	"strings"
	"testing"

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
