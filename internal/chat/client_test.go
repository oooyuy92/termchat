// internal/chat/client_test.go
package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestClient_SendStream(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("Authorization = %q, want Bearer test-key", r.Header.Get("Authorization"))
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Content-Type = %q, want application/json", r.Header.Get("Content-Type"))
		}

		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatal("expected http.Flusher")
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)

		chunks := []string{"Hello", " world", "!"}
		for i, chunk := range chunks {
			fmt.Fprintf(w, "data: {\"id\":\"1\",\"object\":\"chat.completion.chunk\",\"choices\":[{\"index\":0,\"delta\":{\"content\":%q},\"finish_reason\":null}]}\n\n", chunk)
			if i == len(chunks)-1 {
				fmt.Fprintf(w, "data: {\"id\":\"1\",\"object\":\"chat.completion.chunk\",\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"stop\"}]}\n\n")
			}
			flusher.Flush()
		}
		fmt.Fprint(w, "data: [DONE]\n\n")
		flusher.Flush()
	}))
	defer server.Close()

	client := NewOpenAIClient(server.URL, "test-key", "test-model")

	messages := []Message{{Role: "user", Content: "hi"}}
	var collected strings.Builder

	err := client.SendStream(context.Background(), messages, 0.7, 100, "", func(content, thinking string) {
		collected.WriteString(content)
	})
	if err != nil {
		t.Fatalf("SendStream() error = %v", err)
	}

	if collected.String() != "Hello world!" {
		t.Errorf("collected = %q, want %q", collected.String(), "Hello world!")
	}
}

func TestClient_SendStreamThinking(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatal("expected http.Flusher")
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)

		// Send reasoning_content (OpenAI/DeepSeek format)
		fmt.Fprintf(w, "data: {\"id\":\"1\",\"object\":\"chat.completion.chunk\",\"choices\":[{\"index\":0,\"delta\":{\"reasoning_content\":\"thinking...\"},\"finish_reason\":null}]}\n\n")
		flusher.Flush()

		// Send regular content
		fmt.Fprintf(w, "data: {\"id\":\"1\",\"object\":\"chat.completion.chunk\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"answer\"},\"finish_reason\":null}]}\n\n")
		flusher.Flush()

		fmt.Fprintf(w, "data: {\"id\":\"1\",\"object\":\"chat.completion.chunk\",\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"stop\"}]}\n\n")
		flusher.Flush()
		fmt.Fprint(w, "data: [DONE]\n\n")
		flusher.Flush()
	}))
	defer server.Close()

	client := NewOpenAIClient(server.URL, "test-key", "test-model")
	messages := []Message{{Role: "user", Content: "hi"}}

	var contentBuf, thinkingBuf strings.Builder
	err := client.SendStream(context.Background(), messages, 0.7, 100, "", func(content, thinking string) {
		contentBuf.WriteString(content)
		thinkingBuf.WriteString(thinking)
	})
	if err != nil {
		t.Fatalf("SendStream() error = %v", err)
	}

	if contentBuf.String() != "answer" {
		t.Errorf("content = %q, want %q", contentBuf.String(), "answer")
	}
	if thinkingBuf.String() != "thinking..." {
		t.Errorf("thinking = %q, want %q", thinkingBuf.String(), "thinking...")
	}
}

func TestClient_SendStreamGeminiThought(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatal("expected http.Flusher")
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)

		// Gemini format: thought=true with content as thinking
		fmt.Fprintf(w, "data: {\"id\":\"1\",\"object\":\"chat.completion.chunk\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"gemini thinking\"},\"thought\":true,\"finish_reason\":null}]}\n\n")
		flusher.Flush()

		// Regular content
		fmt.Fprintf(w, "data: {\"id\":\"1\",\"object\":\"chat.completion.chunk\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"result\"},\"finish_reason\":null}]}\n\n")
		flusher.Flush()

		fmt.Fprintf(w, "data: {\"id\":\"1\",\"object\":\"chat.completion.chunk\",\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"stop\"}]}\n\n")
		flusher.Flush()
		fmt.Fprint(w, "data: [DONE]\n\n")
		flusher.Flush()
	}))
	defer server.Close()

	client := NewOpenAIClient(server.URL, "test-key", "test-model")
	messages := []Message{{Role: "user", Content: "hi"}}

	var contentBuf, thinkingBuf strings.Builder
	err := client.SendStream(context.Background(), messages, 0.7, 100, "", func(content, thinking string) {
		contentBuf.WriteString(content)
		thinkingBuf.WriteString(thinking)
	})
	if err != nil {
		t.Fatalf("SendStream() error = %v", err)
	}

	if contentBuf.String() != "result" {
		t.Errorf("content = %q, want %q", contentBuf.String(), "result")
	}
	if thinkingBuf.String() != "gemini thinking" {
		t.Errorf("thinking = %q, want %q", thinkingBuf.String(), "gemini thinking")
	}
}

func TestAnthropicClient_SendStream(t *testing.T) {
	// Simulate Anthropic SSE stream
	sseBody := "event: message_start\ndata: {\"type\":\"message_start\"}\n\n" +
		"event: content_block_start\ndata: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"text\",\"text\":\"\"}}\n\n" +
		"event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"Hello\"}}\n\n" +
		"event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\" world\"}}\n\n" +
		"event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	sseBody := "event: content_block_start\ndata: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"thinking\",\"thinking\":\"\"}}\n\n" +
		"event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"thinking_delta\",\"thinking\":\"Let me think\"}}\n\n" +
		"event: content_block_start\ndata: {\"type\":\"content_block_start\",\"index\":1,\"content_block\":{\"type\":\"text\",\"text\":\"\"}}\n\n" +
		"event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":1,\"delta\":{\"type\":\"text_delta\",\"text\":\"Answer\"}}\n\n" +
		"event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
		if !strings.Contains(bodyStr, `"system"`) {
			t.Error("expected top-level system field")
		}
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
	if err := <-errs; err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGeminiClient_SendStreamWithImage(t *testing.T) {
	sseBody := "data: {\"candidates\":[{\"content\":{\"parts\":[{\"text\":\"I see a cat\"}]}}]}\n\n"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		bodyStr := string(body)
		if !strings.Contains(bodyStr, "inline_data") {
			t.Error("expected inline_data in request body")
		}
		if !strings.Contains(bodyStr, "image/png") {
			t.Error("expected image/png mime type")
		}
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, sseBody)
	}))
	defer server.Close()

	client := NewGeminiClient(server.URL, "test-key", "gemini-2.0-flash")
	img := ImageData{MimeType: "image/png", Data: []byte("fake-png-data")}
	messages := []Message{{Role: "user", Content: "what is this?", Images: []ImageData{img}}}
	ctx := context.Background()
	chunks, errs := client.SendStreamChan(ctx, messages, 0.7, 1024, "", 0)

	var result string
	for chunk := range chunks {
		result += chunk.Content
	}
	if err := <-errs; err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "I see a cat" {
		t.Errorf("got %q, want %q", result, "I see a cat")
	}
}

func TestGeminiClient_SendStream(t *testing.T) {
	sseBody := "data: {\"candidates\":[{\"content\":{\"parts\":[{\"text\":\"Hello\"}]}}]}\n\n" +
		"data: {\"candidates\":[{\"content\":{\"parts\":[{\"text\":\" world\"}]}}]}\n\n"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" {
			t.Error("Gemini should not send Authorization header")
		}
		if r.URL.Query().Get("key") == "" {
			t.Error("expected key query param")
		}
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

func TestGeminiClient_SendStreamThinkingLevel(t *testing.T) {
	sseBody := "data: {\"candidates\":[{\"content\":{\"parts\":[{\"text\":\"thinking\",\"thought\":true}]}}]}\n\n" +
		"data: {\"candidates\":[{\"content\":{\"parts\":[{\"text\":\"Answer\"}]}}]}\n\n"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		bodyStr := string(body)
		if !strings.Contains(bodyStr, "thinkingLevel") {
			t.Error("expected thinkingLevel in request for Gemini 3 model")
		}
		if strings.Contains(bodyStr, "thinkingBudget") {
			t.Error("should not contain thinkingBudget for Gemini 3 model")
		}
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, sseBody)
	}))
	defer server.Close()

	client := NewGeminiClient(server.URL, "test-key", "gemini-3.1-pro-preview")
	messages := []Message{{Role: "user", Content: "think"}}
	ctx := context.Background()
	chunks, errs := client.SendStreamChan(ctx, messages, 0.7, 8192, "high", 0)

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
	if thinking != "thinking" {
		t.Errorf("thinking: got %q, want %q", thinking, "thinking")
	}
}

func TestGeminiClient_SystemPrompt(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req map[string]interface{}
		json.Unmarshal(body, &req)
		if _, ok := req["systemInstruction"]; !ok {
			t.Error("expected systemInstruction field")
		}
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

func TestGeminiClient_BuildThinkingConfig(t *testing.T) {
	tests := []struct {
		name            string
		model           string
		reasoningEffort string
		budgetTokens    int
		wantLevel       string
		wantBudget      *int
		wantNil         bool
	}{
		{"gemini3 with reasoning_effort", "gemini-3.1-pro-preview", "low", 0, "low", nil, false},
		{"gemini3 budget maps to low", "gemini-3-flash-preview", "", 512, "low", nil, false},
		{"gemini3 budget maps to medium", "gemini-3-flash-preview", "", 4096, "medium", nil, false},
		{"gemini3 budget maps to high", "gemini-3-flash-preview", "", 16000, "high", nil, false},
		{"gemini3 no config", "gemini-3-flash-preview", "", 0, "", nil, true},
		{"gemini25 with budget", "gemini-2.5-pro", "", 5000, "", intPtr(5000), false},
		{"gemini25 no config", "gemini-2.5-pro", "", 0, "", nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := NewGeminiClient("", "key", tt.model)
			tc := c.buildThinkingConfig(tt.reasoningEffort, tt.budgetTokens)
			if tt.wantNil {
				if tc != nil {
					t.Errorf("expected nil, got %+v", tc)
				}
				return
			}
			if tc == nil {
				t.Fatal("expected non-nil thinking config")
			}
			if tc.ThinkingLevel != tt.wantLevel {
				t.Errorf("level: got %q, want %q", tc.ThinkingLevel, tt.wantLevel)
			}
			if tt.wantBudget != nil {
				if tc.ThinkingBudget == nil || *tc.ThinkingBudget != *tt.wantBudget {
					t.Errorf("budget: got %v, want %d", tc.ThinkingBudget, *tt.wantBudget)
				}
			}
		})
	}
}

func intPtr(v int) *int { return &v }

func TestGeminiClient_RoleMapping(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if strings.Contains(string(body), `"role":"assistant"`) {
			t.Error("assistant role should be mapped to model")
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
