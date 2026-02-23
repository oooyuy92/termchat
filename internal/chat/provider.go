// internal/chat/provider.go
package chat

import (
	"context"
	"net"
	"net/http"
	"time"
)

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

// newHTTPClient returns an http.Client with a 30-second dial timeout.
// No overall timeout is set because streaming responses can last a long time.
func newHTTPClient() *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				Timeout: 30 * time.Second,
			}).DialContext,
			TLSHandshakeTimeout: 15 * time.Second,
		},
	}
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
