// internal/chat/client.go
package chat

import (
	"bufio"
	"bytes"
	"context"
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

type Client struct {
	baseURL string
	apiKey  string
	model   string
	http    *http.Client
}

func NewClient(baseURL, apiKey, model string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		model:   model,
		http:    &http.Client{},
	}
}

func (c *Client) SetModel(model string) {
	c.model = model
}

func (c *Client) Model() string {
	return c.model
}

func (c *Client) SetBaseURL(url string) {
	c.baseURL = strings.TrimRight(url, "/")
}

func (c *Client) BaseURL() string {
	return c.baseURL
}

func (c *Client) SetAPIKey(key string) {
	c.apiKey = key
}

func (c *Client) APIKey() string {
	return c.apiKey
}

type chatRequest struct {
	Model           string    `json:"model"`
	Messages        []Message `json:"messages"`
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

func (c *Client) SendStream(ctx context.Context, messages []Message, temperature float64, maxTokens int, reasoningEffort string, onChunk func(content, thinking string)) error {
	reqBody := chatRequest{
		Model:           c.model,
		Messages:        messages,
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
func (c *Client) SendStreamChan(ctx context.Context, messages []Message, temperature float64, maxTokens int, reasoningEffort string) (<-chan StreamChunk, <-chan error) {
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
