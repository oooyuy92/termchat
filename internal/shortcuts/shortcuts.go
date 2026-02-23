// internal/shortcuts/shortcuts.go
package shortcuts

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Shortcut is a named prompt template.
type Shortcut struct {
	Name    string `yaml:"name"`
	Content string `yaml:"content"`
}

type file struct {
	Shortcuts []Shortcut `yaml:"shortcuts"`
}

// Load reads shortcuts from path. Returns an empty slice (not an error)
// if the file does not exist yet.
func Load(path string) ([]Shortcut, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return []Shortcut{}, nil
		}
		return nil, fmt.Errorf("shortcuts: read %s: %w", path, err)
	}
	var f file
	if err := yaml.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("shortcuts: parse %s: %w", path, err)
	}
	if f.Shortcuts == nil {
		return []Shortcut{}, nil
	}
	return f.Shortcuts, nil
}

// Save writes shortcuts to path, creating parent directories as needed.
func Save(path string, items []Shortcut) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("shortcuts: mkdir %s: %w", filepath.Dir(path), err)
	}
	data, err := yaml.Marshal(file{Shortcuts: items})
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("shortcuts: write %s: %w", path, err)
	}
	return nil
}
