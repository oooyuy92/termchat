package config

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type ModelRegistry struct {
	Providers []ProviderEntry `yaml:"providers"`
}

type ProviderEntry struct {
	Name     string       `yaml:"name"`
	Provider string       `yaml:"provider"`
	BaseURL  string       `yaml:"base_url"`
	APIKey   string       `yaml:"api_key"`
	Models   []ModelEntry `yaml:"models"`
}

type ModelEntry struct {
	Name  string `yaml:"name"`
	Model string `yaml:"model"`
}

func DefaultModelRegistry() ModelRegistry {
	return ModelRegistry{}
}

func ModelRegistryPath(cfgPath string) string {
	return filepath.Join(filepath.Dir(cfgPath), "models.yaml")
}

func LoadModelRegistry(path string) (ModelRegistry, error) {
	reg := DefaultModelRegistry()
	data, err := os.ReadFile(path)
	if err != nil {
		return reg, err
	}
	if err := yaml.Unmarshal(data, &reg); err != nil {
		return reg, err
	}
	return reg, nil
}

func LoadModelRegistryOrDefault(path string) (reg ModelRegistry, missing bool, err error) {
	reg, err = LoadModelRegistry(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return DefaultModelRegistry(), true, nil
		}
		return reg, false, err
	}
	return reg, false, nil
}

func SaveModelRegistry(path string, reg ModelRegistry) error {
	data, err := yaml.Marshal(reg)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
