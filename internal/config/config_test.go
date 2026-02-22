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
}
