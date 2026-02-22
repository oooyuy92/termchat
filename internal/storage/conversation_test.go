// internal/storage/conversation_test.go
package storage

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/termchat/termchat/internal/chat"
)

func TestSaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	store := New(dir)

	messages := []chat.Message{
		{Role: "user", Content: "hello"},
		{Role: "assistant", Content: "hi there"},
	}

	err := store.Save("test-conv", messages)
	if err != nil {
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

	path := filepath.Join(dir, "test-conv.json")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Errorf("expected file %s to exist", path)
	}
}

func TestList(t *testing.T) {
	dir := t.TempDir()
	store := New(dir)

	store.Save("conv-a", []chat.Message{{Role: "user", Content: "a"}})
	store.Save("conv-b", []chat.Message{{Role: "user", Content: "b"}})

	names, err := store.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	if len(names) != 2 {
		t.Fatalf("List() len = %d, want 2", len(names))
	}
}
