// internal/ui/messagebrowse_test.go
package ui

import (
	"testing"

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
