# Multi-Provider Support Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add native Anthropic and Gemini API support alongside the existing OpenAI-compatible client, selectable via a `provider` config field (`openai | openai-compatible | anthropic | gemini`).

**Architecture:** Extract a `Provider` interface in the chat package; rename the current `Client` to `OpenAIClient`; add `AnthropicClient` and `GeminiClient` implementations; wire via a `NewProvider` factory. Config gains `provider` (enum) and `budget_tokens` (int) fields. The settings editor auto-fills the default base_url when provider changes and swaps the live client.

**Tech Stack:** Go 1.24, net/http (SSE), BubbleTea TUI, gopkg.in/yaml.v3, httptest (tests)

**Build command:** `GOPATH=/home/cc/gopath GOROOT=/home/cc/go /home/cc/go/bin/go`

---

### Task 1: Add `Provider` and `BudgetTokens` to config

**Files:**
- Modify: `internal/config/config.go`

**Step 1: Add fields and helper**

In `internal/config/config.go`:

1. Add `Provider string` to `APIConfig`:
```go
type APIConfig struct {
    Provider string `yaml:"provider"`
    BaseURL  string `yaml:"base_url"`
    APIKey   string `yaml:"api_key"`
    Model    string `yaml:"model"`
}
```

2. Add `BudgetTokens int` to `ParametersConfig`:
```go
type ParametersConfig struct {
    Temperature     float64 `yaml:"temperature"`
    MaxTokens       int     `yaml:"max_tokens"`
    ReasoningEffort string  `yaml:"reasoning_effort,omitempty"`
    BudgetTokens    int     `yaml:"budget_tokens,omitempty"`
}
```

3. Update `DefaultConfig()` to set `Provider: "openai-compatible"`.

4. Add the helper function (before `DefaultConfig`):
```go
// ProviderDefaultBaseURL returns the canonical base URL for a provider.
// Returns "" for openai-compatible (user must supply).
func ProviderDefaultBaseURL(provider string) string {
    switch provider {
    case "openai":
        return "https://api.openai.com/v1"
    case "anthropic":
        return "https://api.anthropic.com"
    case "gemini":
        return "https://generativelanguage.googleapis.com"
    default: // openai-compatible
        return ""
    }
}
```

**Step 2: Build to verify**

```bash
GOPATH=/home/cc/gopath GOROOT=/home/cc/go /home/cc/go/bin/go build ./...
```

Expected: no errors.

**Step 3: Commit**

```bash
git add internal/config/config.go
git commit -m "feat: add Provider and BudgetTokens to config"
```

---

### Task 2: Provider interface + rename OpenAIClient

**Files:**
- Create: `internal/chat/provider.go`
- Rename/modify: `internal/chat/client.go` → `internal/chat/client_openai.go`
- Modify: `internal/chat/client_test.go`

**Step 1: Create `internal/chat/provider.go`**

```go
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
```

**Step 2: Rename `client.go` to `client_openai.go` and update it**

Copy `internal/chat/client.go` content to `internal/chat/client_openai.go`, then:

1. Delete `internal/chat/client.go`
2. Rename struct `Client` → `OpenAIClient` throughout the file
3. Rename constructor `NewClient` → `NewOpenAIClient`
4. Update `SendStreamChan` signature to add `budgetTokens int` parameter (unused in OpenAI client — just accept it):
   ```go
   func (c *OpenAIClient) SendStreamChan(ctx context.Context, messages []Message, temp float64, maxTokens int, reasoningEffort string, budgetTokens int) (<-chan StreamChunk, <-chan error) {
   ```
5. Add getter methods to satisfy the interface:
   ```go
   func (c *OpenAIClient) BaseURL() string { return c.baseURL }
   func (c *OpenAIClient) APIKey() string  { return c.apiKey }
   ```
   (Check whether these already exist; if so, just ensure they're present.)

**Step 3: Update `internal/chat/client_test.go`**

Replace every occurrence of `NewClient(` with `NewOpenAIClient(`.

Update every `SendStreamChan` call to add the `budgetTokens` argument (pass `0`):
```go
chunks, errs := client.SendStreamChan(ctx, messages, 0.7, 1024, "", 0)
```

**Step 4: Run existing tests**

```bash
GOPATH=/home/cc/gopath GOROOT=/home/cc/go /home/cc/go/bin/go test ./internal/chat/... -v
```

Expected: all existing tests PASS.

**Step 5: Commit**

```bash
git add internal/chat/provider.go internal/chat/client_openai.go internal/chat/client_test.go
git rm internal/chat/client.go 2>/dev/null || true
git commit -m "feat: extract Provider interface, rename Client to OpenAIClient"
```

---

### Task 3: Implement `AnthropicClient`

**Files:**
- Create: `internal/chat/client_anthropic.go`
- Modify: `internal/chat/client_test.go` (add tests)

**Step 1: Write the failing tests**

Add to `internal/chat/client_test.go`:

```go
func TestAnthropicClient_SendStream(t *testing.T) {
    // Simulate Anthropic SSE stream
    sseBody := "event: message_start\ndata: {\"type\":\"message_start\"}\n\n" +
        "event: content_block_start\ndata: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"text\",\"text\":\"\"}}\n\n" +
        "event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"Hello\"}}\n\n" +
        "event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\" world\"}}\n\n" +
        "event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n"

    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Verify headers
        if r.Header.Get("x-api-key") == "" {
            t.Error("missing x-api-key header")
        }
        if r.Header.Get("anthropic-version") == "" {
            t.Error("missing anthropic-version header")
        }
        if r.URL.Path != "/v1/messages" {
            t.Errorf("unexpected path: %s", r.URL.Path)
        }
        w.Header().Set("Content-Type", "text/event-stream")
        fmt.Fprint(w, sseBody)
    }))
    defer server.Close()

    client := NewAnthropicClient(server.URL, "test-key", "claude-opus-4-5")
    messages := []Message{
        {Role: "user", Content: "hi"},
    }
    ctx := context.Background()
    chunks, errs := client.SendStreamChan(ctx, messages, 0.7, 1024, "", 0)

    var result string
    for chunk := range chunks {
        result += chunk.Content
    }
    if err := <-errs; err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if result != "Hello world" {
        t.Errorf("got %q, want %q", result, "Hello world")
    }
}

func TestAnthropicClient_SendStreamThinking(t *testing.T) {
    // Simulate Anthropic SSE stream with thinking block
    sseBody := "event: content_block_start\ndata: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"thinking\",\"thinking\":\"\"}}\n\n" +
        "event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"thinking_delta\",\"thinking\":\"Let me think\"}}\n\n" +
        "event: content_block_start\ndata: {\"type\":\"content_block_start\",\"index\":1,\"content_block\":{\"type\":\"text\",\"text\":\"\"}}\n\n" +
        "event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":1,\"delta\":{\"type\":\"text_delta\",\"text\":\"Answer\"}}\n\n" +
        "event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n"

    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Verify thinking is requested
        body, _ := io.ReadAll(r.Body)
        if !strings.Contains(string(body), "budget_tokens") {
            t.Error("expected budget_tokens in request when budgetTokens > 0")
        }
        w.Header().Set("Content-Type", "text/event-stream")
        fmt.Fprint(w, sseBody)
    }))
    defer server.Close()

    client := NewAnthropicClient(server.URL, "test-key", "claude-opus-4-5")
    messages := []Message{{Role: "user", Content: "think"}}
    ctx := context.Background()
    chunks, errs := client.SendStreamChan(ctx, messages, 1.0, 16000, "", 10000)

    var text, thinking string
    for chunk := range chunks {
        text += chunk.Content
        thinking += chunk.Thinking
    }
    if err := <-errs; err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if text != "Answer" {
        t.Errorf("text: got %q, want %q", text, "Answer")
    }
    if thinking != "Let me think" {
        t.Errorf("thinking: got %q, want %q", thinking, "Let me think")
    }
}

func TestAnthropicClient_SystemPrompt(t *testing.T) {
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        body, _ := io.ReadAll(r.Body)
        bodyStr := string(body)
        // system message should be extracted to top-level "system" field
        if !strings.Contains(bodyStr, `"system"`) {
            t.Error("expected top-level system field")
        }
        // system message should NOT appear in messages array
        var req map[string]interface{}
        json.Unmarshal(body, &req)
        msgs, _ := req["messages"].([]interface{})
        for _, m := range msgs {
            msg := m.(map[string]interface{})
            if msg["role"] == "system" {
                t.Error("system message should not appear in messages array")
            }
        }
        w.Header().Set("Content-Type", "text/event-stream")
        fmt.Fprint(w, "event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n")
    }))
    defer server.Close()

    client := NewAnthropicClient(server.URL, "test-key", "claude-opus-4-5")
    messages := []Message{
        {Role: "system", Content: "You are helpful"},
        {Role: "user", Content: "hi"},
    }
    ctx := context.Background()
    chunks, errs := client.SendStreamChan(ctx, messages, 0.7, 1024, "", 0)
    for range chunks {}
    <-errs
}
```

Add imports to the test file if not already present: `"io"`, `"encoding/json"`, `"strings"`.

**Step 2: Run tests to verify they fail**

```bash
GOPATH=/home/cc/gopath GOROOT=/home/cc/go /home/cc/go/bin/go test ./internal/chat/... -run "TestAnthropicClient" -v
```

Expected: FAIL with "undefined: NewAnthropicClient".

**Step 3: Implement `internal/chat/client_anthropic.go`**

```go
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
        http:    &http.Client{},
    }
}

func (c *AnthropicClient) Model() string      { return c.model }
func (c *AnthropicClient) SetModel(m string)  { c.model = m }
func (c *AnthropicClient) SetBaseURL(u string) { c.baseURL = u }
func (c *AnthropicClient) SetAPIKey(k string) { c.apiKey = k }
func (c *AnthropicClient) BaseURL() string    { return c.baseURL }
func (c *AnthropicClient) APIKey() string     { return c.apiKey }

// anthropicMessage is a message in the Anthropic API format.
type anthropicMessage struct {
    Role    string `json:"role"`
    Content string `json:"content"`
}

// anthropicRequest is the request body for the Anthropic Messages API.
type anthropicRequest struct {
    Model     string             `json:"model"`
    MaxTokens int                `json:"max_tokens"`
    Stream    bool               `json:"stream"`
    System    string             `json:"system,omitempty"`
    Messages  []anthropicMessage `json:"messages"`
    Thinking  *anthropicThinking `json:"thinking,omitempty"`
    // Temperature omitted when thinking is enabled (Anthropic requires temp=1 for thinking)
    Temperature *float64 `json:"temperature,omitempty"`
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
        defer close(errs)

        if err := c.stream(ctx, messages, temp, maxTokens, budgetTokens, chunks); err != nil {
            errs <- err
        }
    }()

    return chunks, errs
}

func (c *AnthropicClient) stream(ctx context.Context, messages []Message, temp float64, maxTokens int, budgetTokens int, chunks chan<- StreamChunk) error {
    // Extract system prompt
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
        // Anthropic requires temperature=1 when thinking is enabled
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
```

**Step 4: Run tests**

```bash
GOPATH=/home/cc/gopath GOROOT=/home/cc/go /home/cc/go/bin/go test ./internal/chat/... -run "TestAnthropicClient" -v
```

Expected: all TestAnthropicClient tests PASS.

**Step 5: Run all chat tests**

```bash
GOPATH=/home/cc/gopath GOROOT=/home/cc/go /home/cc/go/bin/go test ./internal/chat/... -v
```

Expected: all tests PASS.

**Step 6: Commit**

```bash
git add internal/chat/client_anthropic.go internal/chat/client_test.go
git commit -m "feat: add AnthropicClient with SSE streaming and thinking support"
```

---

### Task 4: Implement `GeminiClient`

**Files:**
- Create: `internal/chat/client_gemini.go`
- Modify: `internal/chat/client_test.go` (add tests)

**Step 1: Write the failing tests**

Add to `internal/chat/client_test.go`:

```go
func TestGeminiClient_SendStream(t *testing.T) {
    // Gemini SSE: each data line is a JSON candidate
    sseBody := "data: {\"candidates\":[{\"content\":{\"parts\":[{\"text\":\"Hello\"}]}}]}\n\n" +
        "data: {\"candidates\":[{\"content\":{\"parts\":[{\"text\":\" world\"}]}}]}\n\n"

    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // No Authorization header for Gemini
        if r.Header.Get("Authorization") != "" {
            t.Error("Gemini should not send Authorization header")
        }
        // API key in query param
        if r.URL.Query().Get("key") == "" {
            t.Error("expected key query param")
        }
        // alt=sse in query param
        if r.URL.Query().Get("alt") != "sse" {
            t.Errorf("expected alt=sse, got %q", r.URL.Query().Get("alt"))
        }
        w.Header().Set("Content-Type", "text/event-stream")
        fmt.Fprint(w, sseBody)
    }))
    defer server.Close()

    client := NewGeminiClient(server.URL, "test-key", "gemini-2.0-flash")
    messages := []Message{{Role: "user", Content: "hi"}}
    ctx := context.Background()
    chunks, errs := client.SendStreamChan(ctx, messages, 0.7, 1024, "", 0)

    var result string
    for chunk := range chunks {
        result += chunk.Content
    }
    if err := <-errs; err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if result != "Hello world" {
        t.Errorf("got %q, want %q", result, "Hello world")
    }
}

func TestGeminiClient_SendStreamThinking(t *testing.T) {
    sseBody := "data: {\"candidates\":[{\"content\":{\"parts\":[{\"text\":\"I think\",\"thought\":true}]}}]}\n\n" +
        "data: {\"candidates\":[{\"content\":{\"parts\":[{\"text\":\"Answer\"}]}}]}\n\n"

    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        body, _ := io.ReadAll(r.Body)
        if !strings.Contains(string(body), "thinkingBudget") {
            t.Error("expected thinkingBudget in request when budgetTokens > 0")
        }
        w.Header().Set("Content-Type", "text/event-stream")
        fmt.Fprint(w, sseBody)
    }))
    defer server.Close()

    client := NewGeminiClient(server.URL, "test-key", "gemini-2.5-pro")
    messages := []Message{{Role: "user", Content: "think"}}
    ctx := context.Background()
    chunks, errs := client.SendStreamChan(ctx, messages, 0.7, 8192, "", 5000)

    var text, thinking string
    for chunk := range chunks {
        text += chunk.Content
        thinking += chunk.Thinking
    }
    if err := <-errs; err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if text != "Answer" {
        t.Errorf("text: got %q, want %q", text, "Answer")
    }
    if thinking != "I think" {
        t.Errorf("thinking: got %q, want %q", thinking, "I think")
    }
}

func TestGeminiClient_SystemPrompt(t *testing.T) {
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        body, _ := io.ReadAll(r.Body)
        var req map[string]interface{}
        json.Unmarshal(body, &req)
        // System message should appear in systemInstruction
        if _, ok := req["systemInstruction"]; !ok {
            t.Error("expected systemInstruction field")
        }
        // contents should not include system role messages
        contents, _ := req["contents"].([]interface{})
        for _, c := range contents {
            content := c.(map[string]interface{})
            if content["role"] == "system" {
                t.Error("system role should not appear in contents")
            }
        }
        w.Header().Set("Content-Type", "text/event-stream")
        fmt.Fprint(w, "")
    }))
    defer server.Close()

    client := NewGeminiClient(server.URL, "test-key", "gemini-2.0-flash")
    messages := []Message{
        {Role: "system", Content: "Be helpful"},
        {Role: "user", Content: "hi"},
    }
    ctx := context.Background()
    chunks, errs := client.SendStreamChan(ctx, messages, 0.7, 1024, "", 0)
    for range chunks {}
    <-errs
}

func TestGeminiClient_RoleMapping(t *testing.T) {
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        body, _ := io.ReadAll(r.Body)
        // "assistant" should be mapped to "model"
        if strings.Contains(string(body), `"role":"assistant"`) {
            t.Error("assistant role should be mapped to model")
        }
        if !strings.Contains(string(body), `"model"`) {
            t.Error("expected model role in contents")
        }
        w.Header().Set("Content-Type", "text/event-stream")
        fmt.Fprint(w, "")
    }))
    defer server.Close()

    client := NewGeminiClient(server.URL, "test-key", "gemini-2.0-flash")
    messages := []Message{
        {Role: "user", Content: "hello"},
        {Role: "assistant", Content: "hi"},
        {Role: "user", Content: "bye"},
    }
    ctx := context.Background()
    chunks, errs := client.SendStreamChan(ctx, messages, 0.7, 1024, "", 0)
    for range chunks {}
    <-errs
}
```

**Step 2: Run tests to verify they fail**

```bash
GOPATH=/home/cc/gopath GOROOT=/home/cc/go /home/cc/go/bin/go test ./internal/chat/... -run "TestGeminiClient" -v
```

Expected: FAIL with "undefined: NewGeminiClient".

**Step 3: Implement `internal/chat/client_gemini.go`**

```go
// internal/chat/client_gemini.go
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
    Text string `json:"text"`
}

type geminiRequest struct {
    Contents          []geminiContent    `json:"contents"`
    SystemInstruction *geminiContent     `json:"systemInstruction,omitempty"`
    GenerationConfig  *geminiGenConfig   `json:"generationConfig,omitempty"`
}

type geminiGenConfig struct {
    Temperature     float64              `json:"temperature,omitempty"`
    MaxOutputTokens int                  `json:"maxOutputTokens,omitempty"`
    ThinkingConfig  *geminiThinkConfig   `json:"thinkingConfig,omitempty"`
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
    // Extract system prompt and build contents
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
        contents = append(contents, geminiContent{
            Role:  role,
            Parts: []geminiPart{{Text: m.Content}},
        })
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
    // Increase scanner buffer for large JSON chunks
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
            continue // skip malformed lines
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
```

**Step 4: Run tests**

```bash
GOPATH=/home/cc/gopath GOROOT=/home/cc/go /home/cc/go/bin/go test ./internal/chat/... -v
```

Expected: all tests PASS.

**Step 5: Commit**

```bash
git add internal/chat/client_gemini.go internal/chat/client_test.go
git commit -m "feat: add GeminiClient with SSE streaming and thinking support"
```

---

### Task 5: Wire `Provider` into `main.go`, `model.go`, `update.go`

**Files:**
- Modify: `main.go`
- Modify: `internal/ui/model.go`
- Modify: `internal/ui/update.go`

This task has no new tests (integration wiring). Build verification is the check.

**Step 1: Update `internal/ui/model.go`**

Find the `Model` struct field: `client *chat.Client`
Replace with: `client chat.Provider`

Find the `NewModel` function signature. It likely currently creates a client internally or receives `*chat.Client`. Change so it receives `chat.Provider`:

```go
func NewModel(cfg config.Config, configPath string, onboarding bool, client chat.Provider) Model {
```

(If the signature is different, adjust accordingly but the key change is `*chat.Client` → `chat.Provider`.)

**Step 2: Update `main.go`**

Replace client creation with `NewProvider`:

```go
// Before:
client := chat.NewClient(cfg.API.BaseURL, cfg.API.APIKey, cfg.API.Model)

// After:
client := chat.NewProvider(cfg.API.Provider, cfg.API.BaseURL, cfg.API.APIKey, cfg.API.Model)
```

**Step 3: Update `internal/ui/update.go` — `sendStreamCmd`**

In `sendStreamCmd`, add `budgetTokens` and pass it:

```go
func (m Model) sendStreamCmd(ctx context.Context) tea.Cmd {
    messages := m.history.ToAPIMessages()
    client := m.client
    temp := m.cfg.Parameters.Temperature
    maxTok := m.cfg.Parameters.MaxTokens
    reasoningEffort := m.cfg.Parameters.ReasoningEffort
    budgetTokens := m.cfg.Parameters.BudgetTokens

    return func() tea.Msg {
        chunks, errs := client.SendStreamChan(ctx, messages, temp, maxTok, reasoningEffort, budgetTokens)
        return streamStartMsg{chunks: chunks, errs: errs}
    }
}
```

**Step 4: Build**

```bash
GOPATH=/home/cc/gopath GOROOT=/home/cc/go /home/cc/go/bin/go build ./...
```

Expected: no errors.

**Step 5: Run all tests**

```bash
GOPATH=/home/cc/gopath GOROOT=/home/cc/go /home/cc/go/bin/go test ./...
```

Expected: all tests PASS.

**Step 6: Commit**

```bash
git add main.go internal/ui/model.go internal/ui/update.go
git commit -m "feat: wire Provider interface into main, model, update"
```

---

### Task 6: Settings editor — provider enum, budget_tokens, client hot-swap

**Files:**
- Modify: `internal/ui/configeditor.go`

**Context:** `configeditor.go` has:
- `configField` struct with `Label`, `Key`, `Value`, `Options []string`, `editing bool`
- `buildConfigFields(cfg)` — returns `[]configField`
- `applyFieldToConfig(key, value, cfg)` — writes field back to config
- `validateField(key, value)` — optional validation, returns `""`/error string
- `updateConfigMode` — handles key input; for enum fields (Options not nil), Enter cycles the option; for free-text fields, Enter toggles edit mode

**Step 1: Update `buildConfigFields`**

Add `Provider` as the first API field, and `BudgetTokens` in the Parameters section:

```go
// In buildConfigFields(cfg config.Config) []configField:

// API section — add at the start of API fields:
{Label: "Provider", Key: "provider", Value: cfg.API.Provider,
 Options: []string{"openai", "openai-compatible", "anthropic", "gemini"}},

// Parameters section — add after ReasoningEffort:
{Label: "Budget Tokens", Key: "budget_tokens", Value: strconv.Itoa(cfg.Parameters.BudgetTokens)},
```

Add `"strconv"` to imports if not already present.

**Step 2: Update `applyFieldToConfig`**

Add cases for `"provider"` and `"budget_tokens"`:

```go
case "provider":
    cfg.API.Provider = value
    // Auto-fill base_url if it matches any known default (including empty)
    defaultURL := config.ProviderDefaultBaseURL(value)
    if defaultURL != "" {
        cfg.API.BaseURL = defaultURL
    }
case "budget_tokens":
    n, err := strconv.Atoi(value)
    if err == nil && n >= 0 {
        cfg.Parameters.BudgetTokens = n
    }
```

**Step 3: Update `validateField`**

Add validation for `"budget_tokens"`:

```go
case "budget_tokens":
    n, err := strconv.Atoi(value)
    if err != nil || n < 0 {
        return "must be a non-negative integer"
    }
    return ""
```

**Step 4: Hot-swap client on provider change**

In `updateConfigMode`, find the block that handles enum cycling (when `Options != nil` and user presses Enter or left/right). After applying the field to config, when `field.Key == "provider"`, also update the base_url field in the editor and swap the live client:

```go
// After applyFieldToConfig for enum fields, add:
if field.Key == "provider" {
    newURL := config.ProviderDefaultBaseURL(newValue)
    if newURL != "" {
        // Update base_url field in the editor
        for i, f := range m.configEd.fields {
            if f.Key == "base_url" {
                m.configEd.fields[i].Value = newURL
                break
            }
        }
        m.cfg.API.BaseURL = newURL
    }
    // Replace the live client
    m.client = chat.NewProvider(m.cfg.API.Provider, m.cfg.API.BaseURL, m.cfg.API.APIKey, m.cfg.API.Model)
}
```

**Step 5: Build**

```bash
GOPATH=/home/cc/gopath GOROOT=/home/cc/go /home/cc/go/bin/go build ./...
```

Expected: no errors.

**Step 6: Run all tests**

```bash
GOPATH=/home/cc/gopath GOROOT=/home/cc/go /home/cc/go/bin/go test ./...
```

Expected: all tests PASS.

**Step 7: Commit**

```bash
git add internal/ui/configeditor.go
git commit -m "feat: add provider enum and budget_tokens to settings editor with client hot-swap"
```

---

### Task 7: Onboarding — add provider selection

**Files:**
- Modify: `internal/ui/onboard.go`

**Context:** The onboarding wizard in `onboard.go` builds fields with `buildOnboardFields(cfg)`. It shows each field one at a time. Enum fields (with `Options`) show a selection UI; free-text fields show a text input. The existing fields are: API Key, Base URL, Model.

**Step 1: Add provider field to `buildOnboardFields`**

Insert the provider enum field as the **first** field (before the API key field):

```go
func buildOnboardFields(cfg config.Config) []onboardField {
    return []onboardField{
        {
            Label:   "Provider",
            Key:     "provider",
            Value:   cfg.API.Provider,
            Options: []string{"openai", "openai-compatible", "anthropic", "gemini"},
        },
        {
            Label:  "API Key",
            Key:    "api_key",
            Value:  cfg.API.APIKey,
        },
        // ... existing base_url, model fields ...
    }
}
```

**Step 2: Apply provider to config in `applyOnboardField`**

Add case `"provider"` in whatever function applies onboard fields to the config:

```go
case "provider":
    cfg.API.Provider = value
    defaultURL := config.ProviderDefaultBaseURL(value)
    if defaultURL != "" && cfg.API.BaseURL == "" {
        cfg.API.BaseURL = defaultURL
    }
```

**Step 3: Build**

```bash
GOPATH=/home/cc/gopath GOROOT=/home/cc/go /home/cc/go/bin/go build ./...
```

Expected: no errors.

**Step 4: Run all tests**

```bash
GOPATH=/home/cc/gopath GOROOT=/home/cc/go /home/cc/go/bin/go test ./...
```

Expected: all tests PASS.

**Step 5: Commit**

```bash
git add internal/ui/onboard.go
git commit -m "feat: add provider selection to onboarding"
```
