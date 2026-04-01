// internal/storage/conversation_test.go
package storage

import (
	"strings"
	"testing"

	"github.com/termchat/termchat/internal/chat"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	store, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	t.Cleanup(func() { store.Close() })
	return store
}

func TestSaveAndLoad(t *testing.T) {
	store := newTestStore(t)

	messages := []chat.Message{
		{Role: "user", Content: "hello"},
		{Role: "assistant", Content: "hi there"},
	}

	if err := store.Save("test-conv", messages); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	loaded, err := store.Load("test-conv")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if len(loaded) != 2 {
		t.Fatalf("loaded len = %d, want 2", len(loaded))
	}
	if loaded[0].Content != "hello" {
		t.Errorf("loaded[0].Content = %q, want %q", loaded[0].Content, "hello")
	}
	if loaded[0].Role != "user" {
		t.Errorf("loaded[0].Role = %q, want %q", loaded[0].Role, "user")
	}
	if loaded[1].Role != "assistant" {
		t.Errorf("loaded[1].Role = %q, want %q", loaded[1].Role, "assistant")
	}
}

func TestSaveOverwrites(t *testing.T) {
	store := newTestStore(t)

	if err := store.Save("conv", []chat.Message{{Role: "user", Content: "first"}}); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	if err := store.Save("conv", []chat.Message{{Role: "user", Content: "second"}}); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	loaded, err := store.Load("conv")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(loaded) != 1 || loaded[0].Content != "second" {
		t.Errorf("expected only second message, got %+v", loaded)
	}
}

func TestLoadNotFound(t *testing.T) {
	store := newTestStore(t)

	_, err := store.Load("nonexistent")
	if err != ErrNotFound {
		t.Errorf("Load() error = %v, want ErrNotFound", err)
	}
}

func TestList(t *testing.T) {
	store := newTestStore(t)

	if err := store.Save("conv-a", []chat.Message{{Role: "user", Content: "a"}}); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	if err := store.Save("conv-b", []chat.Message{{Role: "user", Content: "b"}}); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	names, err := store.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(names) != 2 {
		t.Fatalf("List() len = %d, want 2", len(names))
	}
	// conv-b was saved last so it should appear first (ORDER BY updated_at DESC)
	if names[0] != "conv-b" || names[1] != "conv-a" {
		t.Errorf("List() = %v, want [conv-b conv-a]", names)
	}
}

func TestListEmpty(t *testing.T) {
	store := newTestStore(t)

	names, err := store.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(names) != 0 {
		t.Errorf("List() = %v, want empty", names)
	}
}

func TestLoadAllForSearch(t *testing.T) {
	dir := t.TempDir()
	s, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	_ = s.Save("conv-a", []chat.Message{
		{Role: "user", Content: "hello world"},
		{Role: "assistant", Content: "hi there"},
	})
	_ = s.Save("conv-b", []chat.Message{
		{Role: "user", Content: "深度求索"},
	})

	items, err := s.LoadAllForSearch()
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("want 2 items, got %d", len(items))
	}
	if items[0].Name != "conv-b" {
		t.Errorf("want conv-b first, got %s", items[0].Name)
	}
	if !strings.Contains(items[1].FullText, "hello world") {
		t.Errorf("FullText missing 'hello world': %s", items[1].FullText)
	}
	if !strings.Contains(items[1].FullText, "hi there") {
		t.Errorf("FullText missing 'hi there': %s", items[1].FullText)
	}
	if !strings.Contains(items[0].FullText, "深度求索") {
		t.Errorf("FullText missing Chinese content: %s", items[0].FullText)
	}
}

func TestListWithDate(t *testing.T) {
	store := newTestStore(t)

	if err := store.Save("conv-a", []chat.Message{{Role: "user", Content: "hello from conv-a"}}); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	if err := store.Save("conv-b", []chat.Message{{Role: "assistant", Content: "bot first"}, {Role: "user", Content: "hello from conv-b"}}); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	convs, err := store.ListWithDate()
	if err != nil {
		t.Fatalf("ListWithDate() error = %v", err)
	}
	if len(convs) != 2 {
		t.Fatalf("ListWithDate() len = %d, want 2", len(convs))
	}
	// Date must be a non-empty string in YYYY-MM-DD format (10 chars).
	for _, c := range convs {
		if len(c.Date) != 10 {
			t.Errorf("conv %q: Date = %q, want 10-char YYYY-MM-DD", c.Name, c.Date)
		}
		if c.Name == "" {
			t.Error("expected non-empty Name")
		}
	}
	// conv-b was saved last — must appear first.
	if convs[0].Name != "conv-b" {
		t.Errorf("convs[0].Name = %q, want %q", convs[0].Name, "conv-b")
	}
	// Summary must be first user message content.
	if convs[0].Summary != "hello from conv-b" {
		t.Errorf("convs[0].Summary = %q, want %q", convs[0].Summary, "hello from conv-b")
	}
	if convs[1].Summary != "hello from conv-a" {
		t.Errorf("convs[1].Summary = %q, want %q", convs[1].Summary, "hello from conv-a")
	}
}

func TestConversationStore_InsertAndLoadActiveTimeline(t *testing.T) {
	store := newTestStore(t)

	name := "conv"
	userID, err := store.AppendMessage(name, chat.Message{Seq: 1, Role: "user", Content: "question"})
	if err != nil {
		t.Fatalf("AppendMessage(user) error = %v", err)
	}
	replyID, err := store.AppendMessage(name, chat.Message{
		Seq:           1,
		Role:          "assistant",
		Content:       "answer v1",
		VersionNumber: 1,
	})
	if err != nil {
		t.Fatalf("AppendMessage(assistant) error = %v", err)
	}
	if err := store.InitVersionGroup(replyID); err != nil {
		t.Fatalf("InitVersionGroup() error = %v", err)
	}
	if _, err := store.AppendAssistantVersion(name, replyID, chat.Message{
		Seq:           1,
		Role:          "assistant",
		Content:       "answer v2",
		VersionNumber: 2,
	}); err != nil {
		t.Fatalf("AppendAssistantVersion() error = %v", err)
	}

	msgs, err := store.LoadActiveTimeline(name)
	if err != nil {
		t.Fatalf("LoadActiveTimeline() error = %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("len = %d, want 2", len(msgs))
	}
	if msgs[0].ID != userID {
		t.Fatalf("user ID = %d, want %d", msgs[0].ID, userID)
	}
	if msgs[1].Content != "answer v1" {
		t.Fatalf("assistant content = %q, want answer v1", msgs[1].Content)
	}
	if msgs[1].TotalVersions != 2 {
		t.Fatalf("TotalVersions = %d, want 2", msgs[1].TotalVersions)
	}
}

func TestConversationStore_SetActiveVersionAndTruncateAfterSeq(t *testing.T) {
	store := newTestStore(t)

	name := "conv"
	_, _ = store.AppendMessage(name, chat.Message{Seq: 1, Role: "user", Content: "q1"})
	replyID, _ := store.AppendMessage(name, chat.Message{Seq: 1, Role: "assistant", Content: "v1", VersionNumber: 1})
	_, _ = store.AppendMessage(name, chat.Message{Seq: 2, Role: "user", Content: "q2"})
	_, _ = store.AppendMessage(name, chat.Message{Seq: 2, Role: "assistant", Content: "a2", VersionNumber: 1})

	if err := store.InitVersionGroup(replyID); err != nil {
		t.Fatalf("InitVersionGroup() error = %v", err)
	}
	if _, err := store.AppendAssistantVersion(name, replyID, chat.Message{Seq: 1, Role: "assistant", Content: "v2", VersionNumber: 2}); err != nil {
		t.Fatalf("AppendAssistantVersion() error = %v", err)
	}
	if err := store.SetActiveVersion(name, replyID, 2); err != nil {
		t.Fatalf("SetActiveVersion() error = %v", err)
	}
	if err := store.TruncateAfterSeq(name, 1); err != nil {
		t.Fatalf("TruncateAfterSeq() error = %v", err)
	}

	msgs, err := store.LoadActiveTimeline(name)
	if err != nil {
		t.Fatalf("LoadActiveTimeline() error = %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("len = %d, want 2", len(msgs))
	}
	if msgs[1].Content != "v2" {
		t.Fatalf("assistant content = %q, want v2", msgs[1].Content)
	}
}

func TestConversationStore_MarkTurnEditedAndClear(t *testing.T) {
	store := newTestStore(t)

	name := "conv"
	userID, _ := store.AppendMessage(name, chat.Message{Seq: 1, Role: "user", Content: "old"})
	assistantID, _ := store.AppendMessage(name, chat.Message{Seq: 1, Role: "assistant", Content: "reply", VersionNumber: 1})

	if err := store.MarkTurnEdited(userID, assistantID); err != nil {
		t.Fatalf("MarkTurnEdited() error = %v", err)
	}
	msgs, _ := store.LoadActiveTimeline(name)
	if !msgs[0].EditedAfterGeneration || !msgs[1].StaleAfterUserEdit {
		t.Fatalf("expected edited/stale flags, got %+v", msgs)
	}

	if err := store.ClearTurnEdited(userID, assistantID); err != nil {
		t.Fatalf("ClearTurnEdited() error = %v", err)
	}
	msgs, _ = store.LoadActiveTimeline(name)
	if msgs[0].EditedAfterGeneration || msgs[1].StaleAfterUserEdit {
		t.Fatalf("expected edited/stale flags to clear, got %+v", msgs)
	}
}
