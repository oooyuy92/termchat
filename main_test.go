package main

import (
	"testing"

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
