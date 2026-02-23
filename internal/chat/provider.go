// internal/chat/provider.go
package chat

import "context"

// Provider is the interface all LLM backends must implement.
type Provider interface {
	SendStreamChan(ctx context.Context, messages []Message, temp float64, maxTokens int, reasoningEffort string, budgetTokens int) (<-chan StreamChunk, <-chan error)
	Model() string
	SetModel(string)
	SetBaseURL(string)
	SetAPIKey(string)
	BaseURL() string
	APIKey() string
}

// NewProvider creates the appropriate Provider for the given provider string.
// provider: "anthropic" | "gemini" | "openai" | "openai-compatible"
func NewProvider(provider, baseURL, apiKey, model string) Provider {
	switch provider {
	case "anthropic":
		return NewAnthropicClient(baseURL, apiKey, model)
	case "gemini":
		return NewGeminiClient(baseURL, apiKey, model)
	default: // "openai", "openai-compatible"
		return NewOpenAIClient(baseURL, apiKey, model)
	}
}
