// internal/roles/roles.go
package roles

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Role is a named system prompt preset.
type Role struct {
	Name   string `yaml:"name"`
	Prompt string `yaml:"prompt"`
}

// Defaults are the built-in role presets seeded when the user first opens /roles.
var Defaults = []Role{
	{Name: "通用助手", Prompt: ""},
	{Name: "代码助手", Prompt: "You are an expert programming assistant. Be concise and precise. Prefer working code over lengthy explanations."},
}

type file struct {
	Roles []Role `yaml:"roles"`
}

// Load reads roles from path. Returns an empty slice (not an error) if the file
// does not exist yet.
func Load(path string) ([]Role, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return []Role{}, nil
		}
		return nil, fmt.Errorf("roles: read %s: %w", path, err)
	}
	var f file
	if err := yaml.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("roles: parse %s: %w", path, err)
	}
	if f.Roles == nil {
		return []Role{}, nil
	}
	return f.Roles, nil
}

// Save writes roles to path, creating parent directories as needed.
func Save(path string, items []Role) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("roles: mkdir %s: %w", filepath.Dir(path), err)
	}
	data, err := yaml.Marshal(file{Roles: items})
	if err != nil {
		return fmt.Errorf("roles: marshal: %w", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("roles: write %s: %w", path, err)
	}
	return nil
}
