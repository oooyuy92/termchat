package config

import (
	"path/filepath"
	"testing"
)

func TestDefaultModelRegistry(t *testing.T) {
	reg := DefaultModelRegistry()
	if len(reg.Providers) != 0 {
		t.Fatalf("Providers len = %d, want 0", len(reg.Providers))
	}
}

func TestModelRegistryPathUsesConfigDir(t *testing.T) {
	got := ModelRegistryPath("/tmp/termchat/config.yaml")
	want := filepath.Join("/tmp/termchat", "models.yaml")
	if got != want {
		t.Fatalf("ModelRegistryPath() = %q, want %q", got, want)
	}
}

func TestLoadModelRegistryMissingReturnsDefault(t *testing.T) {
	path := filepath.Join(t.TempDir(), "models.yaml")
	reg, missing, err := LoadModelRegistryOrDefault(path)
	if err != nil {
		t.Fatalf("LoadModelRegistryOrDefault() error = %v", err)
	}
	if !missing {
		t.Fatalf("missing = false, want true")
	}
	if len(reg.Providers) != 0 {
		t.Fatalf("Providers len = %d, want 0", len(reg.Providers))
	}
}

func TestSaveAndLoadModelRegistry(t *testing.T) {
	path := filepath.Join(t.TempDir(), "models.yaml")
	want := ModelRegistry{
		Providers: []ProviderEntry{
			{
				Name:     "gateway",
				Provider: "openai-compatible",
				BaseURL:  "https://example.test/v1",
				APIKey:   "secret",
				Models: []ModelEntry{
					{Name: "flash", Model: "gemini-3-flash-preview"},
					{Name: "pro", Model: "gemini-3.1-pro-preview"},
				},
			},
		},
	}
	if err := SaveModelRegistry(path, want); err != nil {
		t.Fatalf("SaveModelRegistry() error = %v", err)
	}

	got, missing, err := LoadModelRegistryOrDefault(path)
	if err != nil {
		t.Fatalf("LoadModelRegistryOrDefault() error = %v", err)
	}
	if missing {
		t.Fatalf("missing = true, want false")
	}
	if got.Providers[0].Name != "gateway" {
		t.Fatalf("provider name = %q, want gateway", got.Providers[0].Name)
	}
	if got.Providers[0].Models[1].Model != "gemini-3.1-pro-preview" {
		t.Fatalf("model = %q, want gemini-3.1-pro-preview", got.Providers[0].Models[1].Model)
	}
}
