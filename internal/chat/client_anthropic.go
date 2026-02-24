// internal/chat/client_anthropic.go
package chat

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// AnthropicClient implements Provider for the Anthropic Messages API.
type AnthropicClient struct {
	baseURL string
	apiKey  string
	model   string
	http    *http.Client
}

func NewAnthropicClient(baseURL, apiKey, model string) *AnthropicClient {
	if baseURL == "" {
		baseURL = "https://api.anthropic.com"
	}
	return &AnthropicClient{
		baseURL: baseURL,
		apiKey:  apiKey,
		model:   model,
		http:    newHTTPClient(),
	}
}

func (c *AnthropicClient) Model() string       { return c.model }
func (c *AnthropicClient) SetModel(m string)   { c.model = m }
func (c *AnthropicClient) SetBaseURL(u string) { c.baseURL = u }
func (c *AnthropicClient) SetAPIKey(k string)  { c.apiKey = k }
func (c *AnthropicClient) BaseURL() string     { return c.baseURL }
func (c *AnthropicClient) APIKey() string      { return c.apiKey }

type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type anthropicRequest struct {
	Model       string             `json:"model"`
	MaxTokens   int                `json:"max_tokens"`
	Stream      bool               `json:"stream"`
	System      string             `json:"system,omitempty"`
	Messages    []anthropicMessage `json:"messages"`
	Thinking    *anthropicThinking `json:"thinking,omitempty"`
	Temperature *float64           `json:"temperature,omitempty"`
}

type anthropicThinking struct {
	Type         string `json:"type"`
	BudgetTokens int    `json:"budget_tokens"`
}

func (c *AnthropicClient) SendStreamChan(ctx context.Context, messages []Message, temp float64, maxTokens int, reasoningEffort string, budgetTokens int) (<-chan StreamChunk, <-chan error) {
	chunks := make(chan StreamChunk)
	errs := make(chan error, 1)

	go func() {
		defer close(chunks)

		if err := c.stream(ctx, messages, temp, maxTokens, budgetTokens, chunks); err != nil {
			errs <- err
		}
	}()

	return chunks, errs
}

func (c *AnthropicClient) stream(ctx context.Context, messages []Message, temp float64, maxTokens int, budgetTokens int, chunks chan<- StreamChunk) error {
	var system string
	var apiMsgs []anthropicMessage
	for _, m := range messages {
		if m.Role == "system" {
			system = m.Content
		} else {
			apiMsgs = append(apiMsgs, anthropicMessage{Role: m.Role, Content: m.Content})
		}
	}

	req := anthropicRequest{
		Model:     c.model,
		MaxTokens: maxTokens,
		Stream:    true,
		System:    system,
		Messages:  apiMsgs,
	}

	if budgetTokens > 0 {
		req.Thinking = &anthropicThinking{Type: "enabled", BudgetTokens: budgetTokens}
		t := 1.0
		req.Temperature = &t
	} else {
		req.Temperature = &temp
	}

	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/messages", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("new request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", c.apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return fmt.Errorf("http: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("anthropic error %d: %s", resp.StatusCode, string(b))
	}

	return c.parseSSE(resp.Body, chunks)
}

func (c *AnthropicClient) parseSSE(r io.Reader, chunks chan<- StreamChunk) error {
	scanner := bufio.NewScanner(r)
	var eventType string
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "event: ") {
			eventType = strings.TrimPrefix(line, "event: ")
			continue
		}
		if strings.HasPrefix(line, "data: ") {
			data := strings.TrimPrefix(line, "data: ")
			if eventType == "message_stop" {
				return nil
			}
			if eventType == "content_block_delta" {
				var ev struct {
					Delta struct {
						Type     string `json:"type"`
						Text     string `json:"text"`
						Thinking string `json:"thinking"`
					} `json:"delta"`
				}
				if err := json.Unmarshal([]byte(data), &ev); err == nil {
					switch ev.Delta.Type {
					case "text_delta":
						if ev.Delta.Text != "" {
							chunks <- StreamChunk{Content: ev.Delta.Text}
						}
					case "thinking_delta":
						if ev.Delta.Thinking != "" {
							chunks <- StreamChunk{Thinking: ev.Delta.Thinking}
						}
					}
				}
			}
			eventType = ""
		}
	}
	return scanner.Err()
}
