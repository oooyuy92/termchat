package ui

import (
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/termchat/termchat/internal/shortcuts"
)

func TestShortcutEditorEnterSavesContent(t *testing.T) {
	store := newBrowserTestStore(t)
	m := newBrowserTestModel(t, store)
	m.shortcutsPath = filepath.Join(t.TempDir(), "shortcuts.yaml")
	m.mode = modeShortcuts
	m.shortcutEd = shortcutEditor{
		items: []shortcuts.Shortcut{
			{Name: "hello", Content: "old content"},
		},
		cursor:       0,
		subMode:      shortcutModeEditContent,
		editBuf:      "new content",
		savedName:    "hello",
		savedContent: "old content",
	}

	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(Model)

	if got := m.shortcutEd.items[0].Content; got != "new content" {
		t.Fatalf("content = %q, want new content", got)
	}
	if m.shortcutEd.subMode != shortcutModeList {
		t.Fatalf("subMode = %v, want shortcutModeList", m.shortcutEd.subMode)
	}
	if cmd == nil {
		t.Fatal("cmd = nil, want save cmd")
	}
	msg := cmd()
	if _, ok := msg.(shortcutsSavedMsg); !ok {
		t.Fatalf("cmd() returned %T, want shortcutsSavedMsg", msg)
	}
	saved, err := shortcuts.Load(m.shortcutsPath)
	if err != nil {
		t.Fatalf("shortcuts.Load() error = %v", err)
	}
	if got := saved[0].Content; got != "new content" {
		t.Fatalf("saved content = %q, want new content", got)
	}
}

func TestShortcutEditorShiftEnterAddsNewline(t *testing.T) {
	store := newBrowserTestStore(t)
	m := newBrowserTestModel(t, store)
	m.mode = modeShortcuts
	m.shortcutEd = shortcutEditor{
		items: []shortcuts.Shortcut{
			{Name: "hello", Content: "old content"},
		},
		cursor:       0,
		subMode:      shortcutModeEditContent,
		editBuf:      "line1",
		savedName:    "hello",
		savedContent: "old content",
	}

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("\n")})
	m = next.(Model)

	if got := m.shortcutEd.editBuf; got != "line1\n" {
		t.Fatalf("editBuf = %q, want line1\\n", got)
	}
	if m.shortcutEd.subMode != shortcutModeEditContent {
		t.Fatalf("subMode = %v, want shortcutModeEditContent", m.shortcutEd.subMode)
	}
}
