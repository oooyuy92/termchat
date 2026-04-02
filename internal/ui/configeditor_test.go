package ui

import (
	"testing"

	"github.com/termchat/termchat/internal/config"
)

func TestBuildConfigFieldsExcludesModelAndAPIFields(t *testing.T) {
	fields := buildConfigFields(config.DefaultConfig())
	for _, field := range fields {
		switch field.Key {
		case "provider", "base_url", "api_key", "model", "temperature", "max_tokens", "reasoning_effort", "budget_tokens":
			t.Fatalf("unexpected model field %q in settings", field.Key)
		}
	}
}
