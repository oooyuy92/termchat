package ui

import (
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/termchat/termchat/internal/roles"
)

func TestRoleEditorEnterSavesPrompt(t *testing.T) {
	store := newBrowserTestStore(t)
	m := newBrowserTestModel(t, store)
	m.rolesPath = filepath.Join(t.TempDir(), "roles.yaml")
	m.mode = modeRoles
	m.roleEd = roleEditor{
		items: []roles.Role{
			{Name: "writer", Prompt: "old prompt"},
		},
		cursor:      0,
		subMode:     roleModeEditPrompt,
		editBuf:     "new prompt",
		savedName:   "writer",
		savedPrompt: "old prompt",
	}

	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(Model)

	if got := m.roleEd.items[0].Prompt; got != "new prompt" {
		t.Fatalf("prompt = %q, want new prompt", got)
	}
	if m.roleEd.subMode != roleModeList {
		t.Fatalf("subMode = %v, want roleModeList", m.roleEd.subMode)
	}
	if cmd == nil {
		t.Fatal("cmd = nil, want save cmd")
	}
	msg := cmd()
	if _, ok := msg.(rolesSavedMsg); !ok {
		t.Fatalf("cmd() returned %T, want rolesSavedMsg", msg)
	}
	saved, err := roles.Load(m.rolesPath)
	if err != nil {
		t.Fatalf("roles.Load() error = %v", err)
	}
	if got := saved[0].Prompt; got != "new prompt" {
		t.Fatalf("saved prompt = %q, want new prompt", got)
	}
}

func TestRoleEditorShiftEnterAddsNewline(t *testing.T) {
	store := newBrowserTestStore(t)
	m := newBrowserTestModel(t, store)
	m.mode = modeRoles
	m.roleEd = roleEditor{
		items: []roles.Role{
			{Name: "writer", Prompt: "old prompt"},
		},
		cursor:      0,
		subMode:     roleModeEditPrompt,
		editBuf:     "line1",
		savedName:   "writer",
		savedPrompt: "old prompt",
	}

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("\n")})
	m = next.(Model)

	if got := m.roleEd.editBuf; got != "line1\n" {
		t.Fatalf("editBuf = %q, want line1\\n", got)
	}
	if m.roleEd.subMode != roleModeEditPrompt {
		t.Fatalf("subMode = %v, want roleModeEditPrompt", m.roleEd.subMode)
	}
}
