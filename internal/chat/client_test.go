// internal/chat/client_test.go
package chat

import (
	"context"
	"fmt"
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

	client := NewClient(server.URL, "test-key", "test-model")

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

	client := NewClient(server.URL, "test-key", "test-model")
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

	client := NewClient(server.URL, "test-key", "test-model")
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
