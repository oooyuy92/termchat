// internal/chat/client_openai.go
package chat

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// StreamChunk carries both content and thinking text from a streaming response.
type StreamChunk struct {
	Content  string
	Thinking string
}

type OpenAIClient struct {
	baseURL string
	apiKey  string
	model   string
	http    *http.Client
}

func NewOpenAIClient(baseURL, apiKey, model string) *OpenAIClient {
	return &OpenAIClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		model:   model,
		http:    newHTTPClient(),
	}
}

func (c *OpenAIClient) SetModel(model string) {
	c.model = model
}

func (c *OpenAIClient) Model() string {
	return c.model
}

func (c *OpenAIClient) SetBaseURL(url string) {
	c.baseURL = strings.TrimRight(url, "/")
}

func (c *OpenAIClient) BaseURL() string {
	return c.baseURL
}

func (c *OpenAIClient) SetAPIKey(key string) {
	c.apiKey = key
}

func (c *OpenAIClient) APIKey() string { return c.apiKey }

func (c *OpenAIClient) SupportsVision() bool {
	m := c.model
	return strings.Contains(m, "gpt-4o") ||
		strings.Contains(m, "gpt-4-turbo") ||
		strings.Contains(m, "gpt-4-vision") ||
		strings.Contains(m, "o1") ||
		strings.Contains(m, "o3") ||
		strings.Contains(m, "o4")
}

type openAIImageURL struct {
	URL string `json:"url"`
}

type openAIContentPart struct {
	Type     string          `json:"type"`
	Text     string          `json:"text,omitempty"`
	ImageURL *openAIImageURL `json:"image_url,omitempty"`
}

type openAIMessage struct {
	Role    string      `json:"role"`
	Content interface{} `json:"content"` // string or []openAIContentPart
}

func toOpenAIMessages(messages []Message) []openAIMessage {
	out := make([]openAIMessage, 0, len(messages))
	for _, m := range messages {
		if len(m.Images) == 0 {
			out = append(out, openAIMessage{Role: m.Role, Content: m.Content})
			continue
		}
		parts := make([]openAIContentPart, 0, len(m.Images)+1)
		for _, img := range m.Images {
			encoded := base64.StdEncoding.EncodeToString(img.Data)
			url := "data:" + img.MimeType + ";base64," + encoded
			parts = append(parts, openAIContentPart{
				Type:     "image_url",
				ImageURL: &openAIImageURL{URL: url},
			})
		}
		if m.Content != "" {
			parts = append(parts, openAIContentPart{Type: "text", Text: m.Content})
		}
		out = append(out, openAIMessage{Role: m.Role, Content: parts})
	}
	return out
}

type chatRequest struct {
	Model           string          `json:"model"`
	Messages        []openAIMessage `json:"messages"`
	Stream          bool      `json:"stream"`
	Temperature     float64   `json:"temperature,omitempty"`
	MaxTokens       int       `json:"max_tokens,omitempty"`
	ReasoningEffort string    `json:"reasoning_effort,omitempty"`
}

type chatChunk struct {
	Choices []struct {
		Delta struct {
			Content          string `json:"content"`
			ReasoningContent string `json:"reasoning_content"`
			Thinking         string `json:"thinking"`
		} `json:"delta"`
		FinishReason *string `json:"finish_reason"`
		Thought      *bool   `json:"thought"`
	} `json:"choices"`
	Usage *struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

func (c *OpenAIClient) SendStream(ctx context.Context, messages []Message, temperature float64, maxTokens int, reasoningEffort string, onChunk func(content, thinking string)) error {
	reqBody := chatRequest{
		Model:           c.model,
		Messages:        toOpenAIMessages(messages),
		Stream:          true,
		Temperature:     temperature,
		MaxTokens:       maxTokens,
		ReasoningEffort: reasoningEffort,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var buf bytes.Buffer
		buf.ReadFrom(resp.Body)
		return fmt.Errorf("API error (status %d): %s", resp.StatusCode, buf.String())
	}

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()

		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}

		var chunk chatChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}

		if len(chunk.Choices) == 0 {
			continue
		}

		delta := chunk.Choices[0].Delta
		thinking := delta.ReasoningContent // OpenAI / DeepSeek
		if thinking == "" {
			thinking = delta.Thinking // Anthropic proxy
		}
		content := delta.Content

		// Gemini: thought=true means content is actually thinking
		if chunk.Choices[0].Thought != nil && *chunk.Choices[0].Thought {
			thinking = content
			content = ""
		}

		if content != "" || thinking != "" {
			onChunk(content, thinking)
		}
	}

	return scanner.Err()
}

// SendStreamChan wraps SendStream and returns a channel for chunk-by-chunk consumption.
// This is designed for use with bubbletea's Cmd pattern where each chunk triggers
// a new Cmd to read the next one.
func (c *OpenAIClient) SendStreamChan(ctx context.Context, messages []Message, temperature float64, maxTokens int, reasoningEffort string, budgetTokens int) (<-chan StreamChunk, <-chan error) {
	chunks := make(chan StreamChunk, 10)
	errs := make(chan error, 1)
	go func() {
		defer close(chunks)
		err := c.SendStream(ctx, messages, temperature, maxTokens, reasoningEffort, func(content, thinking string) {
			chunks <- StreamChunk{Content: content, Thinking: thinking}
		})
		if err != nil {
			errs <- err
		}
	}()
	return chunks, errs
}
