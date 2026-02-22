package config

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Config struct {
	API        APIConfig        `yaml:"api"`
	Parameters ParametersConfig `yaml:"parameters"`
	Storage    StorageConfig    `yaml:"storage"`
}

type APIConfig struct {
	BaseURL string `yaml:"base_url"`
	APIKey  string `yaml:"api_key"`
	Model   string `yaml:"model"`
}

type ParametersConfig struct {
	Temperature     float64 `yaml:"temperature"`
	MaxTokens       int     `yaml:"max_tokens"`
	ReasoningEffort string  `yaml:"reasoning_effort,omitempty"`
}

type StorageConfig struct {
	Dir string `yaml:"dir"`
}

func DefaultConfig() Config {
	return Config{
		API: APIConfig{
			BaseURL: "https://api.openai.com/v1",
			Model:   "gpt-4o",
		},
		Parameters: ParametersConfig{
			Temperature: 0.7,
			MaxTokens:   4096,
		},
		Storage: StorageConfig{
			Dir: "~/.local/share/termchat/conversations",
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
