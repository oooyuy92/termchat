// internal/storage/conversation_test.go
package storage

import (
	"database/sql"
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
	if err != sql.ErrNoRows {
		t.Errorf("Load() error = %v, want sql.ErrNoRows", err)
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
