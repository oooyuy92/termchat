// internal/chat/client_gemini.go
package chat

import (
	"context"
	"fmt"
	"strings"

	"google.golang.org/genai"
)

// GeminiClient implements Provider for the Google Gemini API using the official SDK.
type GeminiClient struct {
	apiKey string
	model  string
	client *genai.Client
}

func NewGeminiClient(baseURL, apiKey, model string) *GeminiClient {
	g := &GeminiClient{apiKey: apiKey, model: model}
	g.initClient()
	return g
}

func (c *GeminiClient) initClient() {
	// Errors from NewClient are deferred to the first API call.
	client, _ := genai.NewClient(context.Background(), &genai.ClientConfig{
		APIKey:     c.apiKey,
		Backend:    genai.BackendGeminiAPI,
		HTTPClient: newHTTPClient(),
	})
	c.client = client
}

func (c *GeminiClient) Model() string     { return c.model }
func (c *GeminiClient) SetModel(m string) { c.model = m }
func (c *GeminiClient) BaseURL() string   { return "https://generativelanguage.googleapis.com" }
func (c *GeminiClient) APIKey() string    { return c.apiKey }

func (c *GeminiClient) SetBaseURL(_ string) {
	// The official SDK manages the endpoint internally.
}

func (c *GeminiClient) SetAPIKey(k string) {
	c.apiKey = k
	c.initClient()
}

func (c *GeminiClient) SendStreamChan(ctx context.Context, messages []Message, temp float64, maxTokens int, reasoningEffort string, budgetTokens int) (<-chan StreamChunk, <-chan error) {
	chunks := make(chan StreamChunk)
	errs := make(chan error, 1)

	go func() {
		defer close(chunks)
		defer close(errs)

		if err := c.stream(ctx, messages, temp, maxTokens, reasoningEffort, budgetTokens, chunks); err != nil {
			errs <- err
		}
	}()

	return chunks, errs
}

func (c *GeminiClient) stream(ctx context.Context, messages []Message, temp float64, maxTokens int, reasoningEffort string, budgetTokens int, chunks chan<- StreamChunk) error {
	if c.client == nil {
		return fmt.Errorf("gemini client not initialized (check API key)")
	}

	contents, systemInstruction := c.buildContents(messages)

	t32 := float32(temp)
	config := &genai.GenerateContentConfig{
		Temperature:     &t32,
		MaxOutputTokens: int32(maxTokens),
	}

	if systemInstruction != nil {
		config.SystemInstruction = systemInstruction
	}

	if tc := c.buildThinkingConfig(reasoningEffort, budgetTokens); tc != nil {
		config.ThinkingConfig = tc
	}

	for resp, err := range c.client.Models.GenerateContentStream(ctx, c.model, contents, config) {
		if err != nil {
			return fmt.Errorf("stream: %w", err)
		}
		if resp == nil || len(resp.Candidates) == 0 || resp.Candidates[0].Content == nil {
			continue
		}
		for _, part := range resp.Candidates[0].Content.Parts {
			if part.Text == "" {
				continue
			}
			if part.Thought {
				chunks <- StreamChunk{Thinking: part.Text}
			} else {
				chunks <- StreamChunk{Content: part.Text}
			}
		}
	}
	return nil
}

func (c *GeminiClient) buildContents(messages []Message) ([]*genai.Content, *genai.Content) {
	var contents []*genai.Content
	var systemInstruction *genai.Content

	for _, m := range messages {
		if m.Role == "system" {
			systemInstruction = &genai.Content{
				Parts: []*genai.Part{{Text: m.Content}},
			}
			continue
		}
		role := m.Role
		if role == "assistant" {
			role = "model"
		}
		var parts []*genai.Part
		for _, img := range m.Images {
			parts = append(parts, &genai.Part{
				InlineData: &genai.Blob{
					MIMEType: img.MimeType,
					Data:     img.Data,
				},
			})
		}
		if m.Content != "" {
			parts = append(parts, &genai.Part{Text: m.Content})
		}
		contents = append(contents, &genai.Content{Role: role, Parts: parts})
	}
	return contents, systemInstruction
}

// buildThinkingConfig returns the appropriate thinking config for the model.
// Gemini 3.x uses ThinkingLevel; Gemini 2.5 uses ThinkingBudget.
func (c *GeminiClient) buildThinkingConfig(reasoningEffort string, budgetTokens int) *genai.ThinkingConfig {
	if strings.Contains(c.model, "gemini-3") {
		level := mapThinkingLevel(reasoningEffort)
		if level == "" && budgetTokens > 0 {
			switch {
			case budgetTokens <= 1024:
				level = string(genai.ThinkingLevelLow)
			case budgetTokens <= 8192:
				level = string(genai.ThinkingLevelMedium)
			default:
				level = string(genai.ThinkingLevelHigh)
			}
		}
		if level == "" {
			return nil
		}
		tl := genai.ThinkingLevel(level)
		return &genai.ThinkingConfig{ThinkingLevel: tl}
	}

	// Gemini 2.5: use ThinkingBudget
	if budgetTokens > 0 {
		b := int32(budgetTokens)
		return &genai.ThinkingConfig{ThinkingBudget: &b}
	}
	return nil
}

func mapThinkingLevel(effort string) string {
	switch strings.ToLower(effort) {
	case "low":
		return string(genai.ThinkingLevelLow)
	case "medium":
		return string(genai.ThinkingLevelMedium)
	case "high":
		return string(genai.ThinkingLevelHigh)
	case "minimal":
		return string(genai.ThinkingLevelMinimal)
	default:
		return ""
	}
}
