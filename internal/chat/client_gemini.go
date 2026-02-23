// internal/chat/client_gemini.go
package chat

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// GeminiClient implements Provider for the Google Gemini API.
type GeminiClient struct {
	baseURL string
	apiKey  string
	model   string
	http    *http.Client
}

func NewGeminiClient(baseURL, apiKey, model string) *GeminiClient {
	if baseURL == "" {
		baseURL = "https://generativelanguage.googleapis.com"
	}
	return &GeminiClient{
		baseURL: baseURL,
		apiKey:  apiKey,
		model:   model,
		http:    &http.Client{},
	}
}

func (c *GeminiClient) Model() string       { return c.model }
func (c *GeminiClient) SetModel(m string)   { c.model = m }
func (c *GeminiClient) SetBaseURL(u string) { c.baseURL = u }
func (c *GeminiClient) SetAPIKey(k string)  { c.apiKey = k }
func (c *GeminiClient) BaseURL() string     { return c.baseURL }
func (c *GeminiClient) APIKey() string      { return c.apiKey }

type geminiContent struct {
	Role  string       `json:"role"`
	Parts []geminiPart `json:"parts"`
}

type geminiPart struct {
	Text       string        `json:"text,omitempty"`
	InlineData *geminiInline `json:"inline_data,omitempty"`
}

type geminiInline struct {
	MimeType string `json:"mime_type"`
	Data     string `json:"data"`
}

type geminiRequest struct {
	Contents          []geminiContent  `json:"contents"`
	SystemInstruction *geminiContent   `json:"systemInstruction,omitempty"`
	GenerationConfig  *geminiGenConfig `json:"generationConfig,omitempty"`
}

type geminiGenConfig struct {
	Temperature     float64            `json:"temperature,omitempty"`
	MaxOutputTokens int                `json:"maxOutputTokens,omitempty"`
	ThinkingConfig  *geminiThinkConfig `json:"thinkingConfig,omitempty"`
}

type geminiThinkConfig struct {
	ThinkingBudget int `json:"thinkingBudget"`
}

func (c *GeminiClient) SendStreamChan(ctx context.Context, messages []Message, temp float64, maxTokens int, reasoningEffort string, budgetTokens int) (<-chan StreamChunk, <-chan error) {
	chunks := make(chan StreamChunk)
	errs := make(chan error, 1)

	go func() {
		defer close(chunks)
		defer close(errs)

		if err := c.stream(ctx, messages, temp, maxTokens, budgetTokens, chunks); err != nil {
			errs <- err
		}
	}()

	return chunks, errs
}

func (c *GeminiClient) stream(ctx context.Context, messages []Message, temp float64, maxTokens int, budgetTokens int, chunks chan<- StreamChunk) error {
	var systemText string
	var contents []geminiContent
	for _, m := range messages {
		if m.Role == "system" {
			systemText = m.Content
			continue
		}
		role := m.Role
		if role == "assistant" {
			role = "model"
		}
		var parts []geminiPart
		for _, img := range m.Images {
			parts = append(parts, geminiPart{
				InlineData: &geminiInline{
					MimeType: img.MimeType,
					Data:     base64.StdEncoding.EncodeToString(img.Data),
				},
			})
		}
		if m.Content != "" {
			parts = append(parts, geminiPart{Text: m.Content})
		}
		contents = append(contents, geminiContent{Role: role, Parts: parts})
	}

	req := geminiRequest{
		Contents: contents,
		GenerationConfig: &geminiGenConfig{
			Temperature:     temp,
			MaxOutputTokens: maxTokens,
		},
	}

	if systemText != "" {
		req.SystemInstruction = &geminiContent{
			Parts: []geminiPart{{Text: systemText}},
		}
	}

	if budgetTokens > 0 {
		req.GenerationConfig.ThinkingConfig = &geminiThinkConfig{ThinkingBudget: budgetTokens}
	}

	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	url := fmt.Sprintf("%s/v1beta/models/%s:streamGenerateContent?alt=sse&key=%s",
		c.baseURL, c.model, c.apiKey)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("new request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return fmt.Errorf("http: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("gemini error %d: %s", resp.StatusCode, string(b))
	}

	return c.parseSSE(resp.Body, chunks)
}

func (c *GeminiClient) parseSSE(r io.Reader, chunks chan<- StreamChunk) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "" {
			continue
		}

		var resp struct {
			Candidates []struct {
				Content struct {
					Parts []struct {
						Text    string `json:"text"`
						Thought bool   `json:"thought"`
					} `json:"parts"`
				} `json:"content"`
			} `json:"candidates"`
		}
		if err := json.Unmarshal([]byte(data), &resp); err != nil {
			continue
		}

		for _, candidate := range resp.Candidates {
			for _, part := range candidate.Content.Parts {
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
	}
	return scanner.Err()
}
