package main

import (
	"testing"

	"github.com/termchat/termchat/internal/config"
)

func TestParseCLIArgsDefaults(t *testing.T) {
	cfg, alt, err := parseCLIArgs([]string{"termchat"})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if cfg != config.DefaultConfigPath() {
		t.Fatalf("expected default config path, got %q", cfg)
	}
	if !alt {
		t.Fatal("expected alt screen enabled by default")
	}
}

func TestParseCLIArgsNoAltScreen(t *testing.T) {
	cfg, alt, err := parseCLIArgs([]string{"termchat", "--no-alt-screen"})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if cfg != config.DefaultConfigPath() {
		t.Fatalf("expected default config path, got %q", cfg)
	}
	if alt {
		t.Fatal("expected alt screen disabled with --no-alt-screen")
	}
}

func TestParseCLIArgsConfig(t *testing.T) {
	cfg, alt, err := parseCLIArgs([]string{"termchat", "--config", "/tmp/config.yaml", "--no-alt-screen"})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if cfg != "/tmp/config.yaml" {
		t.Fatalf("expected custom config path, got %q", cfg)
	}
	if alt {
		t.Fatal("expected alt screen disabled")
	}
}

func TestParseCLIArgsConfigMissingValue(t *testing.T) {
	_, _, err := parseCLIArgs([]string{"termchat", "--config"})
	if err == nil {
		t.Fatal("expected error for missing --config value")
	}
}
