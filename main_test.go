package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/termchat/termchat/internal/chat"
	"github.com/termchat/termchat/internal/config"
)

func TestParseCLIArgsDefaults(t *testing.T) {
	cfg, noAlt, err := parseCLIArgs([]string{"termchat"})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if cfg != config.DefaultConfigPath() {
		t.Fatalf("expected default config path, got %q", cfg)
	}
	if noAlt {
		t.Fatal("expected no-alt-screen disabled by default")
	}
}

func TestParseCLIArgsNoAltScreen(t *testing.T) {
	cfg, noAlt, err := parseCLIArgs([]string{"termchat", "--no-alt-screen"})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if cfg != config.DefaultConfigPath() {
		t.Fatalf("expected default config path, got %q", cfg)
	}
	if !noAlt {
		t.Fatal("expected no-alt-screen enabled with --no-alt-screen")
	}
}

func TestParseCLIArgsConfig(t *testing.T) {
	cfg, noAlt, err := parseCLIArgs([]string{"termchat", "--config", "/tmp/config.yaml", "--no-alt-screen"})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if cfg != "/tmp/config.yaml" {
		t.Fatalf("expected custom config path, got %q", cfg)
	}
	if !noAlt {
		t.Fatal("expected no-alt-screen enabled")
	}
}

func TestParseCLIArgsConfigMissingValue(t *testing.T) {
	_, _, err := parseCLIArgs([]string{"termchat", "--config"})
	if err == nil {
		t.Fatal("expected error for missing --config value")
	}
}

func TestResolveAltScreenModeHonorsNoAltFlag(t *testing.T) {
	t.Setenv("ZELLIJ", "")
	if resolveAltScreenMode(true, "always") {
		t.Fatal("expected no-alt-screen flag to force inline mode")
	}
}

func TestResolveAltScreenModeAlways(t *testing.T) {
	t.Setenv("ZELLIJ", "1")
	if !resolveAltScreenMode(false, "always") {
		t.Fatal("expected always mode to enable alt screen")
	}
}

func TestResolveAltScreenModeNever(t *testing.T) {
	t.Setenv("ZELLIJ", "")
	if resolveAltScreenMode(false, "never") {
		t.Fatal("expected never mode to disable alt screen")
	}
}

func TestResolveAltScreenModeAuto(t *testing.T) {
	t.Setenv("ZELLIJ", "")
	if !resolveAltScreenMode(false, "auto") {
		t.Fatal("expected auto mode to enable alt screen outside zellij")
	}

	t.Setenv("ZELLIJ", "1")
	if resolveAltScreenMode(false, "auto") {
		t.Fatal("expected auto mode to disable alt screen in zellij")
	}
}

func TestResolveAltScreenModeInvalidFallsBackToAuto(t *testing.T) {
	t.Setenv("ZELLIJ", "1")
	if resolveAltScreenMode(false, "invalid-value") {
		t.Fatal("expected invalid mode to behave like auto in zellij")
	}
}

func TestConfigValuesFlowIntoGeminiProvider(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	content := []byte(`api:
  provider: gemini
  base_url: https://example.test/gemini
  api_key: test-key
  model: gemini-3.1-pro-preview
parameters:
  temperature: 1
  max_tokens: 40000
storage:
  dir: ~/.local/share/termchat/conversations
settings:
  theme: light
  alternate_screen: auto
`)
	if err := os.WriteFile(configPath, content, 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	cfg, missing, err := config.LoadOrDefault(configPath)
	if err != nil {
		t.Fatalf("LoadOrDefault() error = %v", err)
	}
	if missing {
		t.Fatal("LoadOrDefault() reported missing config for an existing file")
	}

	client := chat.NewProvider(cfg.API.Provider, cfg.API.BaseURL, cfg.API.APIKey, cfg.API.Model)
	if got := client.Model(); got != "gemini-3.1-pro-preview" {
		t.Fatalf("client.Model() = %q, want gemini-3.1-pro-preview", got)
	}
	if got := client.BaseURL(); got != "https://example.test/gemini" {
		t.Fatalf("client.BaseURL() = %q, want https://example.test/gemini", got)
	}
	if got := client.APIKey(); got != "test-key" {
		t.Fatalf("client.APIKey() = %q, want test-key", got)
	}
}
