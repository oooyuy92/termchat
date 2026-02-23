# Multi-Provider Support Design

## Goal

Add native Anthropic and Gemini API support alongside the existing OpenAI-compatible client, selectable via a `provider` config field.

## Provider Field

`config.yaml` gains a `provider` field under `api`:

```yaml
api:
  provider: anthropic          # anthropic | gemini | openai | openai-compatible
  base_url: https://api.anthropic.com
  api_key: sk-ant-...
  model: claude-opus-4-5
```

Default base URLs per provider:

| provider | default base_url |
|----------|-----------------|
| `openai` | `https://api.openai.com/v1` |
| `openai-compatible` | (user-supplied) |
| `anthropic` | `https://api.anthropic.com` |
| `gemini` | `https://generativelanguage.googleapis.com` |

Selecting a provider auto-fills the default base_url; user can still override it.

## Budget Tokens

`ParametersConfig` gains a `budget_tokens int` field (default 0 = disabled).
- For Anthropic: if > 0, sends `"thinking": {"type": "enabled", "budget_tokens": N}`
- For Gemini: if > 0, sends `generationConfig.thinkingConfig.thinkingBudget`
- `reasoningEffort` remains for OpenAI-compatible models (o1, DeepSeek-R1, etc.)

## Architecture: Provider Interface

```go
// internal/chat/provider.go
type Provider interface {
    SendStreamChan(ctx context.Context, messages []Message, temp float64, maxTokens int, reasoningEffort string) (<-chan StreamChunk, <-chan error)
    Model() string
    SetModel(string)
    SetBaseURL(string)
    SetAPIKey(string)
    BaseURL() string
    APIKey() string
}

func NewProvider(provider, baseURL, apiKey, model string) Provider
```

Three implementations:
- `internal/chat/client_openai.go` — current `Client` renamed `OpenAIClient`
- `internal/chat/client_anthropic.go` — `AnthropicClient`
- `internal/chat/client_gemini.go` — `GeminiClient`

## Anthropic Protocol

- Endpoint: `POST <baseURL>/v1/messages`
- Headers: `x-api-key: <key>`, `anthropic-version: 2023-06-01`
- System prompt extracted from messages and sent as top-level `system` field
- Thinking: send `"thinking": {"type": "enabled", "budget_tokens": N}` when `budget_tokens > 0`
- SSE: parse `content_block_delta` events; `text_delta` → Content, `thinking_delta` → Thinking
- Stop: `event: message_stop`

## Gemini Protocol

- Endpoint: `POST <baseURL>/v1beta/models/{model}:streamGenerateContent?key={apiKey}`
- No Authorization header; API key in URL query param
- Message role mapping: `assistant` → `model`
- System prompt → `systemInstruction.parts[0].text`
- Thinking: `generationConfig.thinkingConfig.thinkingBudget` when `budget_tokens > 0`
- SSE: JSON stream; `candidates[0].content.parts[]`, thinking parts have `thought: true`

## Wiring

- `main.go`: `chat.NewProvider(cfg.API.Provider, ...)` → pass to `ui.NewModel`
- `internal/ui/model.go`: `client *chat.Client` → `client chat.Provider`
- `internal/ui/configeditor.go`: provider enum field with auto base_url fill on switch; BudgetTokens int field
- `internal/ui/onboard.go`: add provider selection before API key field

## Files Changed

| File | Change |
|------|--------|
| `internal/chat/provider.go` | New: Provider interface + NewProvider factory |
| `internal/chat/client_openai.go` | Renamed/refactored from client.go |
| `internal/chat/client_anthropic.go` | New |
| `internal/chat/client_gemini.go` | New |
| `internal/config/config.go` | Add Provider, BudgetTokens fields |
| `internal/ui/model.go` | client type → chat.Provider |
| `internal/ui/configeditor.go` | Provider enum, base_url auto-fill, BudgetTokens field |
| `internal/ui/onboard.go` | Provider selection |
| `main.go` | Use NewProvider |
