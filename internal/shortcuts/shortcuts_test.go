package shortcuts

import (
	"path/filepath"
	"testing"
)

func TestLoadNonExistent(t *testing.T) {
	dir := t.TempDir()
	items, err := Load(filepath.Join(dir, "shortcuts.yaml"))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(items) != 0 {
		t.Errorf("Load() = %v, want empty", items)
	}
}

func TestSaveAndLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "shortcuts.yaml")
	input := []Shortcut{
		{Name: "translate", Content: "请翻译以下内容："},
		{Name: "review", Content: "请审查以下代码："},
	}
	if err := Save(path, input); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(loaded) != 2 {
		t.Fatalf("Load() len = %d, want 2", len(loaded))
	}
	if loaded[0].Name != "translate" || loaded[0].Content != "请翻译以下内容：" {
		t.Errorf("loaded[0] = %+v", loaded[0])
	}
	if loaded[1].Name != "review" || loaded[1].Content != "请审查以下代码：" {
		t.Errorf("loaded[1] = %+v", loaded[1])
	}
}

func TestSaveEmpty(t *testing.T) {
	path := filepath.Join(t.TempDir(), "shortcuts.yaml")
	if err := Save(path, []Shortcut{}); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(loaded) != 0 {
		t.Errorf("Load() = %v, want empty", loaded)
	}
}

func TestSaveNil(t *testing.T) {
	path := filepath.Join(t.TempDir(), "shortcuts.yaml")
	if err := Save(path, nil); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(loaded) != 0 {
		t.Errorf("Load() after Save(nil) = %v, want empty", loaded)
	}
}