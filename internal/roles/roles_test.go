// internal/roles/roles_test.go
package roles

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadNonExistent(t *testing.T) {
	loaded, err := Load(filepath.Join(t.TempDir(), "roles.yaml"))
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}
	if loaded == nil {
		t.Error("Load() returned nil, want non-nil slice")
	}
}

func TestSaveAndLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "roles.yaml")
	items := []Role{
		{Name: "A", Prompt: "prompt A"},
		{Name: "B", Prompt: "prompt B"},
	}
	if err := Save(path, items); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(loaded) != 2 {
		t.Fatalf("len = %d, want 2", len(loaded))
	}
	if loaded[0].Name != "A" || loaded[0].Prompt != "prompt A" {
		t.Errorf("loaded[0] = %+v, want {A prompt A}", loaded[0])
	}
	if loaded[1].Name != "B" || loaded[1].Prompt != "prompt B" {
		t.Errorf("loaded[1] = %+v, want {B prompt B}", loaded[1])
	}
}

func TestSaveEmpty(t *testing.T) {
	path := filepath.Join(t.TempDir(), "roles.yaml")
	if err := Save(path, []Role{}); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(loaded) != 0 {
		t.Errorf("len = %d, want 0", len(loaded))
	}
}

func TestSaveNil(t *testing.T) {
	path := filepath.Join(t.TempDir(), "roles.yaml")
	if err := Save(path, nil); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if loaded == nil {
		t.Error("Load() returned nil, want non-nil slice")
	}
}

func TestLoadCreatesParentDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "dir")
	path := filepath.Join(dir, "roles.yaml")
	items := []Role{{Name: "X", Prompt: "y"}}
	if err := Save(path, items); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("file not created: %v", err)
	}
}
