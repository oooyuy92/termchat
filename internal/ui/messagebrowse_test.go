// internal/ui/messagebrowse_test.go
package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/termchat/termchat/internal/chat"
	"github.com/termchat/termchat/internal/config"
	"github.com/termchat/termchat/internal/storage"
)

func newBrowserTestStore(t *testing.T) *storage.Store {
	t.Helper()
	store, err := storage.New(t.TempDir())
	if err != nil {
		t.Fatalf("storage.New() error = %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func newBrowserTestModel(t *testing.T, store *storage.Store) Model {
	t.Helper()
	cfg := config.DefaultConfig()
	tab, err := newTabSession(cfg, &stubProvider{model: "test-model"}, 80)
	if err != nil {
		t.Fatalf("newTabSession() error = %v", err)
	}
	return Model{
		cfg:       cfg,
		store:     store,
		theme:     DarkTheme,
		tabs:      []TabSession{tab},
		activeTab: 0,
		width:     80,
		height:    24,
	}
}

func seedConversation(t *testing.T, store *storage.Store, name string, msgs []chat.Message) {
	t.Helper()
	for _, msg := range msgs {
		if _, err := store.AppendMessage(name, msg); err != nil {
			t.Fatalf("AppendMessage(%q) error = %v", msg.Content, err)
		}
	}
}

func mustLoadActiveTimeline(t *testing.T, store *storage.Store, name string) []chat.Message {
	t.Helper()
	msgs, err := store.LoadActiveTimeline(name)
	if err != nil {
		t.Fatalf("LoadActiveTimeline() error = %v", err)
	}
	return msgs
}

func TestIncrementalPersistence(t *testing.T) {
	store := newBrowserTestStore(t)
	model := newBrowserTestModel(t, store)

	// Seed a conversation with a user message
	seedConversation(t, store, "conv", []chat.Message{
		{Seq: 1, Role: "user", Content: "q1"},
	})

	model.tabs[0].autoSaveName = "conv"
	model.tabs[0].history.ReplaceMessages(mustLoadActiveTimeline(t, store, "conv"))

	// Verify the message was loaded
	msgs := model.tabs[0].history.Messages()
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	if msgs[0].Content != "q1" {
		t.Fatalf("expected content 'q1', got %q", msgs[0].Content)
	}
}

func newVersionedBrowseModel(t *testing.T) Model {
	t.Helper()
	store := newBrowserTestStore(t)
	m := newBrowserTestModel(t, store)
	m.messageBrowse.turns = []browseTurn{
		{
			User: chat.Message{ID: 1, Seq: 1, Role: "user", Content: "q1"},
			AssistantVersions: []chat.Message{
				{ID: 2, Seq: 1, Role: "assistant", Content: "v1", VersionGroupID: 2, VersionNumber: 1, TotalVersions: 2},
				{ID: 3, Seq: 1, Role: "assistant", Content: "v2", VersionGroupID: 2, VersionNumber: 2, TotalVersions: 2},
			},
			ActiveVersion:  0,
			PreviewVersion: 0,
		},
		{
			User: chat.Message{ID: 4, Seq: 2, Role: "user", Content: "q2"},
			AssistantVersions: []chat.Message{
				{ID: 5, Seq: 2, Role: "assistant", Content: "a2", VersionNumber: 1, TotalVersions: 1},
			},
			ActiveVersion:  0,
			PreviewVersion: 0,
		},
	}
	return m
}

func TestMessageBrowse_RightArrowChangesPreviewOnly(t *testing.T) {
	m := newVersionedBrowseModel(t)
	m.mode = modeMessageBrowse
	m.messageBrowse.mode = browseModeMessage

	before := m.messageBrowse.turns[0].ActiveVersion
	m, _ = m.updateMessageBrowse(tea.KeyMsg{Type: tea.KeyRight})

	if m.messageBrowse.turns[0].ActiveVersion != before {
		t.Fatalf("active version changed during preview navigation")
	}
	if m.messageBrowse.turns[0].PreviewVersion == before {
		t.Fatalf("preview version did not move")
	}
}

func TestMessageBrowse_EnterOnPreviewWithLaterTurnsOpensConfirmation(t *testing.T) {
	m := newVersionedBrowseModel(t)
	m.mode = modeMessageBrowse
	m.messageBrowse.mode = browseModeMessage
	m.messageBrowse.turnIdx = 0
	m.messageBrowse.turns[0].PreviewVersion = 1

	m, _ = m.updateMessageBrowse(tea.KeyMsg{Type: tea.KeyEnter})
	if m.messageBrowse.pendingConfirm.kind != confirmApplyPreview {
		t.Fatalf("pending confirm kind = %v, want confirmApplyPreview", m.messageBrowse.pendingConfirm.kind)
	}
}

func TestMessageBrowse_VEntersCompareMode(t *testing.T) {
	m := newVersionedBrowseModel(t)
	m.mode = modeMessageBrowse
	m.messageBrowse.mode = browseModeMessage

	m, _ = m.updateMessageBrowse(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("v")})
	if m.messageBrowse.mode != browseModeCompare {
		t.Fatalf("mode = %v, want browseModeCompare", m.messageBrowse.mode)
	}
}

func newEditableBrowseModel(t *testing.T) Model {
	t.Helper()
	store := newBrowserTestStore(t)
	m := newBrowserTestModel(t, store)

	// Seed conversation with messages
	name := "conv"
	userID, _ := store.AppendMessage(name, chat.Message{Seq: 1, Role: "user", Content: "q1"})
	assistantID, _ := store.AppendMessage(name, chat.Message{Seq: 1, Role: "assistant", Content: "v1", VersionNumber: 1})
	_ = store.InitVersionGroup(assistantID)
	_, _ = store.AppendAssistantVersion(name, assistantID, chat.Message{Seq: 1, Role: "assistant", Content: "v2", VersionNumber: 2})
	user2ID, _ := store.AppendMessage(name, chat.Message{Seq: 2, Role: "user", Content: "q2"})
	assistant2ID, _ := store.AppendMessage(name, chat.Message{Seq: 2, Role: "assistant", Content: "a2", VersionNumber: 1})

	m.tabs[0].autoSaveName = name

	// Build browse turns
	m.messageBrowse.turns = []browseTurn{
		{
			User: chat.Message{ID: userID, Seq: 1, Role: "user", Content: "q1"},
			AssistantVersions: []chat.Message{
				{ID: assistantID, Seq: 1, Role: "assistant", Content: "v1", VersionGroupID: assistantID, VersionNumber: 1, TotalVersions: 2},
				{ID: assistantID + 1, Seq: 1, Role: "assistant", Content: "v2", VersionGroupID: assistantID, VersionNumber: 2, TotalVersions: 2},
			},
			ActiveVersion:  0,
			PreviewVersion: 0,
		},
		{
			User: chat.Message{ID: user2ID, Seq: 2, Role: "user", Content: "q2"},
			AssistantVersions: []chat.Message{
				{ID: assistant2ID, Seq: 2, Role: "assistant", Content: "a2", VersionNumber: 1, TotalVersions: 1},
			},
			ActiveVersion:  0,
			PreviewVersion: 0,
		},
	}
	return m
}

func TestMessageBrowse_EditSaveOnlyMarksTurnWithoutTruncating(t *testing.T) {
	m := newEditableBrowseModel(t)
	m.mode = modeMessageBrowse

	m, _ = m.updateMessageBrowse(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("e")})
	m.messageBrowse.editBuffer = "edited question"
	m, _ = m.updateMessageBrowse(tea.KeyMsg{Type: tea.KeyEnter})
	m.messageBrowse.pendingConfirm.cursor = 1 // Save Only
	m, _ = m.updateMessageBrowse(tea.KeyMsg{Type: tea.KeyEnter})

	turn := m.messageBrowse.turns[0]
	if !turn.User.EditedAfterGeneration {
		t.Fatalf("expected user turn to be marked edited")
	}
	if !turn.AssistantVersions[turn.ActiveVersion].StaleAfterUserEdit {
		t.Fatalf("expected assistant turn to be marked stale")
	}
	if len(m.messageBrowse.turns) < 2 {
		t.Fatalf("later turns should be preserved")
	}
}
