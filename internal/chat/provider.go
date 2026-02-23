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

// newHTTPClient returns an http.Client matching http.DefaultTransport
// settings but with explicit timeouts to prevent indefinite hangs.
func newHTTPClient() *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			ForceAttemptHTTP2:     true,
			MaxIdleConns:          100,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   15 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
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
