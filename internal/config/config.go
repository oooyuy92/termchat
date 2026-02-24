package config

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Config struct {
	API        APIConfig        `yaml:"api"`
	Parameters ParametersConfig `yaml:"parameters"`
	Storage    StorageConfig    `yaml:"storage"`
	Settings   SettingsConfig   `yaml:"settings"`
}

type SettingsConfig struct {
	Theme           string `yaml:"theme"`
	AlternateScreen string `yaml:"alternate_screen"`
}

type APIConfig struct {
	Provider string `yaml:"provider"`
	BaseURL  string `yaml:"base_url"`
	APIKey   string `yaml:"api_key"`
	Model    string `yaml:"model"`
}

type ParametersConfig struct {
	Temperature     float64 `yaml:"temperature"`
	MaxTokens       int     `yaml:"max_tokens"`
	ReasoningEffort string  `yaml:"reasoning_effort,omitempty"`
	BudgetTokens    int     `yaml:"budget_tokens,omitempty"`
}

type StorageConfig struct {
	Dir string `yaml:"dir"`
}

// ProviderDefaultBaseURL returns the canonical base URL for a provider.
// Returns "" for openai-compatible (user must supply).
func ProviderDefaultBaseURL(provider string) string {
	switch provider {
	case "openai":
		return "https://api.openai.com/v1"
	case "anthropic":
		return "https://api.anthropic.com"
	case "gemini":
		return "https://generativelanguage.googleapis.com"
	default: // openai-compatible
		return ""
	}
}

func DefaultConfig() Config {
	return Config{
		API: APIConfig{
			Provider: "openai-compatible",
			BaseURL:  "https://api.openai.com/v1",
			Model:    "gpt-4o",
		},
		Parameters: ParametersConfig{
			Temperature: 0.7,
			MaxTokens:   4096,
		},
		Storage: StorageConfig{
			Dir: "~/.local/share/termchat/conversations",
		},
		Settings: SettingsConfig{
			Theme:           "dark",
			AlternateScreen: "auto",
		},
	}
}

func Load(path string) (Config, error) {
	cfg := DefaultConfig()

	data, err := os.ReadFile(path)
	if err != nil {
		return cfg, err
	}

	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return cfg, err
	}

	return cfg, nil
}

// LoadOrDefault loads config from path. If the file does not exist, it returns
// DefaultConfig() and missing=true. For any other error it returns the error.
func LoadOrDefault(path string) (cfg Config, missing bool, err error) {
	cfg, err = Load(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return DefaultConfig(), true, nil
		}
		return cfg, false, err
	}
	return cfg, false, nil
}

func ConfigDir() string {
	home, _ := os.UserHomeDir()
	return home + "/.config/termchat"
}

func DefaultConfigPath() string {
	return ConfigDir() + "/config.yaml"
}

func Save(path string, cfg Config) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
