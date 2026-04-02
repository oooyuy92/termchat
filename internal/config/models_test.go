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
				Name:    "gateway",
				BaseURL: "https://example.test/v1",
				APIKey:  "secret",
				Models: []ModelEntry{
					{Name: "flash", Model: "gemini-3-flash-preview", APIFormat: "gemini"},
					{Name: "pro", Model: "gemini-3.1-pro-preview", APIFormat: "gemini"},
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
	if got.Providers[0].Models[1].APIFormat != "gemini" {
		t.Fatalf("APIFormat = %q, want gemini", got.Providers[0].Models[1].APIFormat)
	}
}

func TestSaveAndLoadModelRegistryPreservesModelParameters(t *testing.T) {
	path := filepath.Join(t.TempDir(), "models.yaml")
	want := ModelRegistry{
		Providers: []ProviderEntry{
			{
				Name:    "gateway",
				BaseURL: "https://example.test/v1",
				APIKey:  "secret",
				Models: []ModelEntry{
					{
						Name:            "flash",
						Model:           "gemini-3-flash-preview",
						APIFormat:       "gemini",
						Temperature:     0.4,
						MaxTokens:       8192,
						ReasoningEffort: "medium",
						BudgetTokens:    2048,
					},
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
	if got.Providers[0].Models[0].Temperature != 0.4 {
		t.Fatalf("Temperature = %f, want 0.4", got.Providers[0].Models[0].Temperature)
	}
	if got.Providers[0].Models[0].MaxTokens != 8192 {
		t.Fatalf("MaxTokens = %d, want 8192", got.Providers[0].Models[0].MaxTokens)
	}
	if got.Providers[0].Models[0].ReasoningEffort != "medium" {
		t.Fatalf("ReasoningEffort = %q, want medium", got.Providers[0].Models[0].ReasoningEffort)
	}
	if got.Providers[0].Models[0].BudgetTokens != 2048 {
		t.Fatalf("BudgetTokens = %d, want 2048", got.Providers[0].Models[0].BudgetTokens)
	}
	if got.Providers[0].Models[0].APIFormat != "gemini" {
		t.Fatalf("APIFormat = %q, want gemini", got.Providers[0].Models[0].APIFormat)
	}
}
