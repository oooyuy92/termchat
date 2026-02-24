package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	content := []byte(`api:
  base_url: "https://api.example.com/v1"
  api_key: "test-key"
  model: "gpt-4o"
parameters:
  temperature: 0.7
  max_tokens: 4096
  reasoning_effort: "high"
storage:
  dir: "/tmp/termchat/conversations"
`)
	if err := os.WriteFile(configPath, content, 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.API.BaseURL != "https://api.example.com/v1" {
		t.Errorf("BaseURL = %q, want %q", cfg.API.BaseURL, "https://api.example.com/v1")
	}
	if cfg.API.APIKey != "test-key" {
		t.Errorf("APIKey = %q, want %q", cfg.API.APIKey, "test-key")
	}
	if cfg.API.Model != "gpt-4o" {
		t.Errorf("Model = %q, want %q", cfg.API.Model, "gpt-4o")
	}
	if cfg.Parameters.Temperature != 0.7 {
		t.Errorf("Temperature = %f, want %f", cfg.Parameters.Temperature, 0.7)
	}
	if cfg.Parameters.MaxTokens != 4096 {
		t.Errorf("MaxTokens = %d, want %d", cfg.Parameters.MaxTokens, 4096)
	}
	if cfg.Parameters.ReasoningEffort != "high" {
		t.Errorf("ReasoningEffort = %q, want %q", cfg.Parameters.ReasoningEffort, "high")
	}
}

func TestLoadConfigDefaults(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	content := []byte(`api:
  api_key: "test-key"
`)
	if err := os.WriteFile(configPath, content, 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.API.BaseURL != "https://api.openai.com/v1" {
		t.Errorf("default BaseURL = %q, want %q", cfg.API.BaseURL, "https://api.openai.com/v1")
	}
	if cfg.API.Model != "gpt-4o" {
		t.Errorf("default Model = %q, want %q", cfg.API.Model, "gpt-4o")
	}
	if cfg.Parameters.Temperature != 0.7 {
		t.Errorf("default Temperature = %f, want %f", cfg.Parameters.Temperature, 0.7)
	}
	if cfg.Parameters.MaxTokens != 4096 {
		t.Errorf("default MaxTokens = %d, want %d", cfg.Parameters.MaxTokens, 4096)
	}
	if cfg.Settings.AlternateScreen != "auto" {
		t.Errorf("default AlternateScreen = %q, want %q", cfg.Settings.AlternateScreen, "auto")
	}
}

func TestLoadConfigAlternateScreen(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	content := []byte(`settings:
  alternate_screen: "never"
`)
	if err := os.WriteFile(configPath, content, 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Settings.AlternateScreen != "never" {
		t.Errorf("AlternateScreen = %q, want %q", cfg.Settings.AlternateScreen, "never")
	}
}

func TestLoadOrDefaultMissing(t *testing.T) {
	cfg, missing, err := LoadOrDefault("/nonexistent/path/config.yaml")
	if err != nil {
		t.Fatalf("LoadOrDefault() error = %v", err)
	}
	if !missing {
		t.Error("LoadOrDefault() missing = false, want true")
	}
	if cfg.API.BaseURL != "https://api.openai.com/v1" {
		t.Errorf("BaseURL = %q, want default", cfg.API.BaseURL)
	}
	if cfg.Parameters.Temperature != 0.7 {
		t.Errorf("Temperature = %f, want 0.7", cfg.Parameters.Temperature)
	}
}

func TestLoadOrDefaultBadFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	// Write invalid YAML
	if err := os.WriteFile(path, []byte(":\ninvalid: [yaml\n"), 0644); err != nil {
		t.Fatal(err)
	}
	_, missing, err := LoadOrDefault(path)
	if err == nil {
		t.Error("LoadOrDefault() error = nil, want non-nil for bad YAML")
	}
	if missing {
		t.Error("LoadOrDefault() missing = true for bad YAML, want false")
	}
}

func TestLoadOrDefaultExisting(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := []byte("api:\n  api_key: \"sk-test\"\n")
	if err := os.WriteFile(path, content, 0644); err != nil {
		t.Fatal(err)
	}
	cfg, missing, err := LoadOrDefault(path)
	if err != nil {
		t.Fatalf("LoadOrDefault() error = %v", err)
	}
	if missing {
		t.Error("LoadOrDefault() missing = true for existing file, want false")
	}
	if cfg.API.APIKey != "sk-test" {
		t.Errorf("APIKey = %q, want %q", cfg.API.APIKey, "sk-test")
	}
}

func TestSaveConfig(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "subdir", "config.yaml")

	cfg := Config{
		API: APIConfig{
			BaseURL: "https://api.example.com/v1",
			APIKey:  "sk-test",
			Model:   "gpt-4o-mini",
		},
		Parameters: ParametersConfig{
			Temperature:     0.5,
			MaxTokens:       2048,
			ReasoningEffort: "medium",
		},
		Storage: StorageConfig{
			Dir: "/tmp/test",
		},
	}

	if err := Save(configPath, cfg); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	loaded, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() after Save() error = %v", err)
	}

	if loaded.API.BaseURL != cfg.API.BaseURL {
		t.Errorf("BaseURL = %q, want %q", loaded.API.BaseURL, cfg.API.BaseURL)
	}
	if loaded.API.APIKey != cfg.API.APIKey {
		t.Errorf("APIKey = %q, want %q", loaded.API.APIKey, cfg.API.APIKey)
	}
	if loaded.API.Model != cfg.API.Model {
		t.Errorf("Model = %q, want %q", loaded.API.Model, cfg.API.Model)
	}
	if loaded.Parameters.Temperature != cfg.Parameters.Temperature {
		t.Errorf("Temperature = %f, want %f", loaded.Parameters.Temperature, cfg.Parameters.Temperature)
	}
	if loaded.Parameters.MaxTokens != cfg.Parameters.MaxTokens {
		t.Errorf("MaxTokens = %d, want %d", loaded.Parameters.MaxTokens, cfg.Parameters.MaxTokens)
	}
	if loaded.Parameters.ReasoningEffort != cfg.Parameters.ReasoningEffort {
		t.Errorf("ReasoningEffort = %q, want %q", loaded.Parameters.ReasoningEffort, cfg.Parameters.ReasoningEffort)
	}
}
