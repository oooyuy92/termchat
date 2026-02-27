// internal/chat/client_gemini.go
package chat

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"google.golang.org/genai"
)

// debugLog writes diagnostic info to ~/.config/termchat/gemini_debug.log
// when the TERMCHAT_DEBUG environment variable is set.
var geminiDebugLogger = func() *log.Logger {
	if os.Getenv("TERMCHAT_DEBUG") == "" {
		return nil
	}
	home, _ := os.UserHomeDir()
	f, err := os.OpenFile(home+"/.config/termchat/gemini_debug.log",
		os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return nil
	}
	return log.New(f, "", log.LstdFlags)
}()

// GeminiClient implements Provider for the Google Gemini API using the official SDK.
type GeminiClient struct {
	apiKey string
	model  string
	client *genai.Client
}

func NewGeminiClient(baseURL, apiKey, model string) *GeminiClient {
	g := &GeminiClient{apiKey: apiKey, model: canonicalGeminiModel(model)}
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

func (c *GeminiClient) Model() string        { return c.model }
func (c *GeminiClient) SetModel(m string)    { c.model = canonicalGeminiModel(m) }
func (c *GeminiClient) BaseURL() string      { return "https://generativelanguage.googleapis.com" }
func (c *GeminiClient) APIKey() string       { return c.apiKey }
func (c *GeminiClient) SupportsVision() bool { return true }

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
		if geminiDebugLogger != nil {
			geminiDebugLogger.Printf("ThinkingConfig: Level=%q Budget=%v IncludeThoughts=%v", tc.ThinkingLevel, tc.ThinkingBudget, tc.IncludeThoughts)
		}
	}

	gotData := false
	partIdx := 0
	for resp, err := range c.client.Models.GenerateContentStream(ctx, c.model, contents, config) {
		if err != nil {
			return fmt.Errorf("stream: %w", err)
		}
		if resp == nil || len(resp.Candidates) == 0 || resp.Candidates[0].Content == nil {
			if geminiDebugLogger != nil {
				geminiDebugLogger.Printf("chunk: empty response (resp=%v)", resp != nil)
			}
			continue
		}
		for _, part := range resp.Candidates[0].Content.Parts {
			if geminiDebugLogger != nil {
				geminiDebugLogger.Printf("part[%d]: Thought=%v Text=%q (len=%d) ThoughtSig=%d",
					partIdx, part.Thought, truncate(part.Text, 80), len(part.Text), len(part.ThoughtSignature))
				partIdx++
			}
			if part.Text == "" {
				continue
			}
			gotData = true
			if part.Thought {
				chunks <- StreamChunk{Thinking: part.Text}
			} else {
				chunks <- StreamChunk{Content: part.Text}
			}
		}
	}
	if !gotData {
		return fmt.Errorf("no response from model %s", c.model)
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
	model := strings.ToLower(c.model)
	if strings.Contains(model, "gemini-3") {
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
		if level != "" {
			tl := genai.ThinkingLevel(level)
			return &genai.ThinkingConfig{
				ThinkingLevel:  tl,
				IncludeThoughts: true,
			}
		}
		// Thinking is on by default for Gemini 3; return empty config
		// so that thought parts are included in the streaming response.
		return &genai.ThinkingConfig{IncludeThoughts: true}
	}

	// Gemini 2.5: use ThinkingBudget.
	// Map reasoningEffort to a budget when no explicit budget is set.
	if budgetTokens <= 0 && reasoningEffort != "" {
		switch strings.ToLower(reasoningEffort) {
		case "low":
			budgetTokens = 1024
		case "medium":
			budgetTokens = 8192
		case "high":
			budgetTokens = 24576
		}
	}
	if budgetTokens > 0 {
		b := int32(budgetTokens)
		return &genai.ThinkingConfig{
			ThinkingBudget: &b,
			IncludeThoughts: true,
		}
	}
	// For 2.5 thinking models, enable thinking with default budget
	// so that thought parts are included in the streaming response.
	if strings.Contains(model, "2.5") {
		return &genai.ThinkingConfig{IncludeThoughts: true}
	}
	return nil
}

func canonicalGeminiModel(model string) string {
	trimmed := strings.TrimSpace(model)
	switch strings.ToLower(trimmed) {
	case "gemini-3.1-pro-preview":
		// Marketing name is "Gemini 3.1 Pro Preview", but API model code is gemini-3-pro-preview.
		return "gemini-3-pro-preview"
	default:
		return trimmed
	}
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "..."
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
