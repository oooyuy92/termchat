# termchat Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build a terminal AI chat tool in Go that connects to OpenAI-compatible APIs with streaming output, Markdown rendering, and conversation persistence.

**Architecture:** Bubbletea (Elm architecture: Model-Update-View) drives the TUI. The chat package handles OpenAI-compatible API streaming via net/http. Config is loaded from YAML. Conversations are persisted as JSON files.

**Tech Stack:** Go, bubbletea, lipgloss, glamour, gopkg.in/yaml.v3

---

### Task 1: Project Scaffolding

**Files:**
- Create: `go.mod`
- Create: `main.go`

**Step 1: Initialize Go module**

Run: `go mod init github.com/termchat/termchat`
Expected: `go.mod` created

**Step 2: Install dependencies**

Run:
```bash
go get github.com/charmbracelet/bubbletea
go get github.com/charmbracelet/lipgloss
go get github.com/charmbracelet/glamour
go get gopkg.in/yaml.v3
```

**Step 3: Create minimal main.go**

```go
package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Println("termchat")
	os.Exit(0)
}
```

**Step 4: Verify build**

Run: `go build -o termchat . && ./termchat`
Expected: prints "termchat"

**Step 5: Commit**

```bash
git add go.mod go.sum main.go
git commit -m "feat: project scaffolding with dependencies"
```

---

### Task 2: Config Module

**Files:**
- Create: `internal/config/config.go`
- Create: `internal/config/config_test.go`
- Create: `config.example.yaml`

**Step 1: Write the failing test**

```go
// internal/config/config_test.go
package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	// Create temp config file
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	content := []byte(`api:
  base_url: "https://api.example.com/v1"
  api_key: "test-key"
  model: "gpt-4o"
parameters:
  temperature: 0.7
  max_tokens: 4096
storage:
  dir: "/tmp/termchat/conversations"
`)
	if err := os.WriteFile(configPath, content, 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.API.BaseURL != "https://api.example.com/v1" {
		t.Errorf("BaseURL = %q, want %q", cfg.API.BaseURL, "https://api.example.com/v1")
	}
	if cfg.API.APIKey != "test-key" {
		t.Errorf("APIKey = %q, want %q", cfg.API.APIKey, "test-key")
	}
	if cfg.API.Model != "gpt-4o" {
		t.Errorf("Model = %q, want %q", cfg.API.Model, "gpt-4o")
	}
	if cfg.Parameters.Temperature != 0.7 {
		t.Errorf("Temperature = %f, want %f", cfg.Parameters.Temperature, 0.7)
	}
	if cfg.Parameters.MaxTokens != 4096 {
		t.Errorf("MaxTokens = %d, want %d", cfg.Parameters.MaxTokens, 4096)
	}
}

func TestLoadConfigDefaults(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	content := []byte(`api:
  api_key: "test-key"
`)
	if err := os.WriteFile(configPath, content, 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.API.BaseURL != "https://api.openai.com/v1" {
		t.Errorf("default BaseURL = %q, want %q", cfg.API.BaseURL, "https://api.openai.com/v1")
	}
	if cfg.API.Model != "gpt-4o" {
		t.Errorf("default Model = %q, want %q", cfg.API.Model, "gpt-4o")
	}
	if cfg.Parameters.Temperature != 0.7 {
		t.Errorf("default Temperature = %f, want %f", cfg.Parameters.Temperature, 0.7)
	}
	if cfg.Parameters.MaxTokens != 4096 {
		t.Errorf("default MaxTokens = %d, want %d", cfg.Parameters.MaxTokens, 4096)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/config/...`
Expected: FAIL — package does not exist

**Step 3: Write implementation**

```go
// internal/config/config.go
package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	API        APIConfig        `yaml:"api"`
	Parameters ParametersConfig `yaml:"parameters"`
	Storage    StorageConfig    `yaml:"storage"`
}

type APIConfig struct {
	BaseURL string `yaml:"base_url"`
	APIKey  string `yaml:"api_key"`
	Model   string `yaml:"model"`
}

type ParametersConfig struct {
	Temperature float64 `yaml:"temperature"`
	MaxTokens   int     `yaml:"max_tokens"`
}

type StorageConfig struct {
	Dir string `yaml:"dir"`
}

func DefaultConfig() Config {
	return Config{
		API: APIConfig{
			BaseURL: "https://api.openai.com/v1",
			Model:   "gpt-4o",
		},
		Parameters: ParametersConfig{
			Temperature: 0.7,
			MaxTokens:   4096,
		},
		Storage: StorageConfig{
			Dir: "~/.local/share/termchat/conversations",
		},
	}
}

func Load(path string) (Config, error) {
	cfg := DefaultConfig()

	data, err := os.ReadFile(path)
	if err != nil {
		return cfg, err
	}

	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return cfg, err
	}

	return cfg, nil
}

// ConfigDir returns the default config directory path.
func ConfigDir() string {
	home, _ := os.UserHomeDir()
	return home + "/.config/termchat"
}

// DefaultConfigPath returns the default config file path.
func DefaultConfigPath() string {
	return ConfigDir() + "/config.yaml"
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/config/... -v`
Expected: PASS

**Step 5: Create example config**

```yaml
# config.example.yaml
api:
  base_url: "https://api.openai.com/v1"
  api_key: "your-api-key-here"
  model: "gpt-4o"

parameters:
  temperature: 0.7
  max_tokens: 4096

storage:
  dir: "~/.local/share/termchat/conversations"
```

**Step 6: Commit**

```bash
git add internal/config/ config.example.yaml
git commit -m "feat: add config module with YAML loading and defaults"
```

---

### Task 3: Chat History (Message Types)

**Files:**
- Create: `internal/chat/history.go`
- Create: `internal/chat/history_test.go`

**Step 1: Write the failing test**

```go
// internal/chat/history_test.go
package chat

import "testing"

func TestHistory_AddAndGet(t *testing.T) {
	h := NewHistory()

	h.Add(Message{Role: "user", Content: "hello"})
	h.Add(Message{Role: "assistant", Content: "hi there"})

	msgs := h.Messages()
	if len(msgs) != 2 {
		t.Fatalf("Messages() len = %d, want 2", len(msgs))
	}
	if msgs[0].Role != "user" || msgs[0].Content != "hello" {
		t.Errorf("msgs[0] = %+v, want user/hello", msgs[0])
	}
	if msgs[1].Role != "assistant" || msgs[1].Content != "hi there" {
		t.Errorf("msgs[1] = %+v, want assistant/hi there", msgs[1])
	}
}

func TestHistory_Clear(t *testing.T) {
	h := NewHistory()
	h.Add(Message{Role: "user", Content: "hello"})
	h.Clear()

	if len(h.Messages()) != 0 {
		t.Errorf("after Clear(), len = %d, want 0", len(h.Messages()))
	}
}

func TestHistory_ToAPIMessages(t *testing.T) {
	h := NewHistory()
	h.SetSystemPrompt("You are helpful.")
	h.Add(Message{Role: "user", Content: "hi"})

	msgs := h.ToAPIMessages()
	if len(msgs) != 2 {
		t.Fatalf("ToAPIMessages() len = %d, want 2", len(msgs))
	}
	if msgs[0].Role != "system" {
		t.Errorf("first message role = %q, want system", msgs[0].Role)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/chat/... -v`
Expected: FAIL

**Step 3: Write implementation**

```go
// internal/chat/history.go
package chat

// Message represents a single chat message.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// History manages the conversation message list.
type History struct {
	messages     []Message
	systemPrompt string
}

func NewHistory() *History {
	return &History{}
}

func (h *History) SetSystemPrompt(prompt string) {
	h.systemPrompt = prompt
}

func (h *History) Add(msg Message) {
	h.messages = append(h.messages, msg)
}

func (h *History) Messages() []Message {
	return h.messages
}

func (h *History) Clear() {
	h.messages = nil
}

// ToAPIMessages returns messages formatted for the API, including system prompt.
func (h *History) ToAPIMessages() []Message {
	var msgs []Message
	if h.systemPrompt != "" {
		msgs = append(msgs, Message{Role: "system", Content: h.systemPrompt})
	}
	msgs = append(msgs, h.messages...)
	return msgs
}

func (h *History) Count() int {
	return len(h.messages)
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/chat/... -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/chat/
git commit -m "feat: add chat history with message management"
```

---

### Task 4: OpenAI-Compatible Streaming Client

**Files:**
- Create: `internal/chat/client.go`
- Create: `internal/chat/client_test.go`

**Step 1: Write the failing test**

```go
// internal/chat/client_test.go
package chat

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestClient_SendStream(t *testing.T) {
	// Mock SSE server
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

	err := client.SendStream(messages, 0.7, 100, func(chunk string) {
		collected.WriteString(chunk)
	})
	if err != nil {
		t.Fatalf("SendStream() error = %v", err)
	}

	if collected.String() != "Hello world!" {
		t.Errorf("collected = %q, want %q", collected.String(), "Hello world!")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/chat/... -v -run TestClient`
Expected: FAIL — NewClient not defined

**Step 3: Write implementation**

```go
// internal/chat/client.go
package chat

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// Client handles communication with OpenAI-compatible APIs.
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

type chatRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Stream      bool      `json:"stream"`
	Temperature float64   `json:"temperature,omitempty"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
}

type chatChunk struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
		FinishReason *string `json:"finish_reason"`
	} `json:"choices"`
	Usage *struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

// SendStream sends a chat completion request and calls onChunk for each token.
func (c *Client) SendStream(messages []Message, temperature float64, maxTokens int, onChunk func(string)) error {
	reqBody := chatRequest{
		Model:       c.model,
		Messages:    messages,
		Stream:      true,
		Temperature: temperature,
		MaxTokens:   maxTokens,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", c.baseURL+"/chat/completions", bytes.NewReader(body))
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

		if len(chunk.Choices) > 0 && chunk.Choices[0].Delta.Content != "" {
			onChunk(chunk.Choices[0].Delta.Content)
		}
	}

	return scanner.Err()
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/chat/... -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/chat/
git commit -m "feat: add OpenAI-compatible streaming chat client"
```

---

### Task 5: Conversation Storage (Save/Load)

**Files:**
- Create: `internal/storage/conversation.go`
- Create: `internal/storage/conversation_test.go`

**Step 1: Write the failing test**

```go
// internal/storage/conversation_test.go
package storage

import (
	"path/filepath"
	"testing"

	"github.com/termchat/termchat/internal/chat"
)

func TestSaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	store := New(dir)

	messages := []chat.Message{
		{Role: "user", Content: "hello"},
		{Role: "assistant", Content: "hi there"},
	}

	err := store.Save("test-conv", messages)
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	loaded, err := store.Load("test-conv")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if len(loaded) != 2 {
		t.Fatalf("loaded len = %d, want 2", len(loaded))
	}
	if loaded[0].Content != "hello" {
		t.Errorf("loaded[0].Content = %q, want %q", loaded[0].Content, "hello")
	}

	// Verify file exists
	path := filepath.Join(dir, "test-conv.json")
	if !fileExists(path) {
		t.Errorf("expected file %s to exist", path)
	}
}

func TestList(t *testing.T) {
	dir := t.TempDir()
	store := New(dir)

	store.Save("conv-a", []chat.Message{{Role: "user", Content: "a"}})
	store.Save("conv-b", []chat.Message{{Role: "user", Content: "b"}})

	names, err := store.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	if len(names) != 2 {
		t.Fatalf("List() len = %d, want 2", len(names))
	}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
```

Note: add `"os"` to imports.

**Step 2: Run test to verify it fails**

Run: `go test ./internal/storage/... -v`
Expected: FAIL

**Step 3: Write implementation**

```go
// internal/storage/conversation.go
package storage

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/termchat/termchat/internal/chat"
)

// Store handles conversation persistence.
type Store struct {
	dir string
}

func New(dir string) *Store {
	return &Store{dir: dir}
}

// Save writes messages to a JSON file.
func (s *Store) Save(name string, messages []chat.Message) error {
	if err := os.MkdirAll(s.dir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(messages, "", "  ")
	if err != nil {
		return err
	}

	path := filepath.Join(s.dir, name+".json")
	return os.WriteFile(path, data, 0644)
}

// Load reads messages from a JSON file.
func (s *Store) Load(name string) ([]chat.Message, error) {
	path := filepath.Join(s.dir, name+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var messages []chat.Message
	if err := json.Unmarshal(data, &messages); err != nil {
		return nil, err
	}
	return messages, nil
}

// List returns names of all saved conversations.
func (s *Store) List() ([]string, error) {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var names []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") {
			names = append(names, strings.TrimSuffix(e.Name(), ".json"))
		}
	}
	return names, nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/storage/... -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/storage/
git commit -m "feat: add conversation save/load storage"
```

---

### Task 6: TUI — Model & Status Bar

**Files:**
- Create: `internal/ui/model.go`
- Create: `internal/ui/statusbar.go`

**Step 1: Create the main TUI model**

```go
// internal/ui/model.go
package ui

import (
	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/termchat/termchat/internal/chat"
	"github.com/termchat/termchat/internal/config"
	"github.com/termchat/termchat/internal/storage"
)

// streamChunkMsg carries a token from the streaming response.
type streamChunkMsg struct {
	Content string
}

// streamDoneMsg signals the stream has finished.
type streamDoneMsg struct{}

// streamErrMsg signals a streaming error.
type streamErrMsg struct {
	Err error
}

// commandResultMsg carries output from a slash command.
type commandResultMsg struct {
	Text string
}

type Model struct {
	cfg       config.Config
	client    *chat.Client
	history   *chat.History
	store     *storage.Store
	renderer  *glamour.TermRenderer

	// UI state
	input       string
	streaming   bool
	currentResp string
	statusMsg   string
	totalTokens int
	width       int
	height      int
	err         error
}

func NewModel(cfg config.Config) (Model, error) {
	renderer, err := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(0), // will set dynamically based on terminal width
	)
	if err != nil {
		return Model{}, err
	}

	storageDir := cfg.Storage.Dir
	// Expand ~ in path
	if len(storageDir) > 0 && storageDir[0] == '~' {
		home, _ := os.UserHomeDir()
		storageDir = home + storageDir[1:]
	}

	return Model{
		cfg:      cfg,
		client:   chat.NewClient(cfg.API.BaseURL, cfg.API.APIKey, cfg.API.Model),
		history:  chat.NewHistory(),
		store:    storage.New(storageDir),
		renderer: renderer,
	}, nil
}

func (m Model) Init() tea.Cmd {
	return nil
}
```

Note: add `"os"` to imports.

**Step 2: Create the status bar**

```go
// internal/ui/statusbar.go
package ui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

var statusBarStyle = lipgloss.NewStyle().
	Background(lipgloss.Color("236")).
	Foreground(lipgloss.Color("252")).
	Padding(0, 1)

var statusKeyStyle = lipgloss.NewStyle().
	Background(lipgloss.Color("63")).
	Foreground(lipgloss.Color("230")).
	Padding(0, 1).
	Bold(true)

func (m Model) renderStatusBar() string {
	model := statusKeyStyle.Render("model") + statusBarStyle.Render(m.client.Model())
	tokens := statusKeyStyle.Render("tokens") + statusBarStyle.Render(fmt.Sprintf("%d", m.totalTokens))
	msgs := statusKeyStyle.Render("msgs") + statusBarStyle.Render(fmt.Sprintf("%d", m.history.Count()))

	status := model + " " + tokens + " " + msgs

	if m.statusMsg != "" {
		status += "  " + statusBarStyle.Render(m.statusMsg)
	}

	bar := lipgloss.NewStyle().
		Width(m.width).
		Background(lipgloss.Color("236")).
		Render(status)

	return bar
}
```

**Step 3: Verify build**

Run: `go build ./internal/ui/...`
Expected: compiles without errors

**Step 4: Commit**

```bash
git add internal/ui/
git commit -m "feat: add TUI model and status bar"
```

---

### Task 7: TUI — View (Markdown Rendering)

**Files:**
- Create: `internal/ui/view.go`

**Step 1: Implement the View method**

```go
// internal/ui/view.go
package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	userLabelStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("86")).
		Bold(true)

	assistantLabelStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("212")).
		Bold(true)

	inputPromptStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("241"))

	errStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("196")).
		Bold(true)
)

func (m Model) View() string {
	var b strings.Builder

	// Render conversation history
	for _, msg := range m.history.Messages() {
		switch msg.Role {
		case "user":
			b.WriteString(userLabelStyle.Render("You:") + "\n")
			b.WriteString(msg.Content + "\n\n")
		case "assistant":
			b.WriteString(assistantLabelStyle.Render("Assistant:") + "\n")
			rendered, err := m.renderer.Render(msg.Content)
			if err != nil {
				b.WriteString(msg.Content + "\n\n")
			} else {
				b.WriteString(rendered + "\n")
			}
		}
	}

	// Render current streaming response
	if m.streaming && m.currentResp != "" {
		b.WriteString(assistantLabelStyle.Render("Assistant:") + "\n")
		rendered, err := m.renderer.Render(m.currentResp)
		if err != nil {
			b.WriteString(m.currentResp)
		} else {
			b.WriteString(rendered)
		}
		b.WriteString("▊\n") // cursor indicator
	}

	// Render error
	if m.err != nil {
		b.WriteString(errStyle.Render(fmt.Sprintf("Error: %v", m.err)) + "\n\n")
	}

	// Input area
	if !m.streaming {
		b.WriteString(inputPromptStyle.Render("> ") + m.input)
	}

	// Status bar at bottom
	content := b.String()

	return content + "\n" + m.renderStatusBar()
}
```

**Step 2: Verify build**

Run: `go build ./internal/ui/...`
Expected: compiles

**Step 3: Commit**

```bash
git add internal/ui/view.go
git commit -m "feat: add TUI view with markdown rendering"
```

---

### Task 8: TUI — Update (Input & Streaming)

**Files:**
- Create: `internal/ui/update.go`

**Step 1: Implement the Update method**

```go
// internal/ui/update.go
package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/termchat/termchat/internal/chat"
)

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		if m.streaming {
			// Ignore input while streaming (except quit)
			if msg.String() == "ctrl+c" {
				return m, tea.Quit
			}
			return m, nil
		}

		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit

		case "enter":
			input := strings.TrimSpace(m.input)
			if input == "" {
				return m, nil
			}
			m.input = ""

			// Handle slash commands
			if strings.HasPrefix(input, "/") {
				return m.handleCommand(input)
			}

			// Send message
			m.history.Add(chat.Message{Role: "user", Content: input})
			m.streaming = true
			m.currentResp = ""
			m.err = nil

			return m, m.sendStreamCmd()

		case "backspace":
			if len(m.input) > 0 {
				m.input = m.input[:len(m.input)-1]
			}

		default:
			if len(msg.String()) == 1 || msg.String() == " " {
				m.input += msg.String()
			}
		}
		return m, nil

	case streamChunkMsg:
		m.currentResp += msg.Content
		return m, nil

	case streamDoneMsg:
		m.streaming = false
		if m.currentResp != "" {
			m.history.Add(chat.Message{Role: "assistant", Content: m.currentResp})
		}
		m.currentResp = ""
		return m, nil

	case streamErrMsg:
		m.streaming = false
		m.err = msg.Err
		m.currentResp = ""
		return m, nil

	case commandResultMsg:
		m.statusMsg = msg.Text
		return m, nil
	}

	return m, nil
}

// sendStreamCmd returns a tea.Cmd that streams the API response.
func (m Model) sendStreamCmd() tea.Cmd {
	messages := m.history.ToAPIMessages()
	client := m.client
	temp := m.cfg.Parameters.Temperature
	maxTok := m.cfg.Parameters.MaxTokens

	return func() tea.Msg {
		err := client.SendStream(messages, temp, maxTok, func(chunk string) {
			// We need to send chunks through the program.
			// This will be handled via tea.Program.Send() in a goroutine approach.
			// For now, we collect the full response.
		})
		if err != nil {
			return streamErrMsg{Err: err}
		}
		return streamDoneMsg{}
	}
}
```

**Note:** The streaming integration with bubbletea requires using `tea.Program.Send()` from a goroutine. This will be refined in Task 9 when we wire everything together.

**Step 2: Implement slash command handler**

Add to `internal/ui/update.go`:

```go
func (m Model) handleCommand(input string) (tea.Model, tea.Cmd) {
	parts := strings.Fields(input)
	cmd := parts[0]

	switch cmd {
	case "/exit":
		return m, tea.Quit

	case "/clear":
		m.history.Clear()
		m.totalTokens = 0
		m.statusMsg = "Conversation cleared"
		return m, nil

	case "/model":
		if len(parts) < 2 {
			m.statusMsg = "Usage: /model <name>"
			return m, nil
		}
		m.client.SetModel(parts[1])
		m.statusMsg = "Model set to " + parts[1]
		return m, nil

	case "/save":
		name := "default"
		if len(parts) >= 2 {
			name = parts[1]
		}
		err := m.store.Save(name, m.history.Messages())
		if err != nil {
			m.statusMsg = "Save failed: " + err.Error()
		} else {
			m.statusMsg = "Saved as " + name
		}
		return m, nil

	case "/load":
		name := "default"
		if len(parts) >= 2 {
			name = parts[1]
		}
		msgs, err := m.store.Load(name)
		if err != nil {
			m.statusMsg = "Load failed: " + err.Error()
		} else {
			m.history.Clear()
			for _, msg := range msgs {
				m.history.Add(msg)
			}
			m.statusMsg = "Loaded " + name
		}
		return m, nil

	case "/list":
		names, err := m.store.List()
		if err != nil {
			m.statusMsg = "List failed: " + err.Error()
		} else if len(names) == 0 {
			m.statusMsg = "No saved conversations"
		} else {
			m.statusMsg = "Saved: " + strings.Join(names, ", ")
		}
		return m, nil

	default:
		m.statusMsg = "Unknown command: " + cmd
		return m, nil
	}
}
```

**Step 3: Verify build**

Run: `go build ./internal/ui/...`
Expected: compiles

**Step 4: Commit**

```bash
git add internal/ui/update.go
git commit -m "feat: add TUI update with input handling and slash commands"
```

---

### Task 9: Wire Everything Together — Streaming Integration & main.go

**Files:**
- Modify: `internal/ui/model.go` — add program reference for Send()
- Modify: `internal/ui/update.go` — fix streaming to use goroutine + program.Send()
- Modify: `main.go` — wire up config, model, and tea.Program

**Step 1: Update Model to support program reference**

Add to `internal/ui/model.go`:

```go
// SetProgram stores a reference to the tea.Program for sending messages from goroutines.
func (m *Model) SetProgram(p *tea.Program) {
	m.program = p
}
```

Add `program *tea.Program` field to the Model struct.

**Step 2: Rewrite sendStreamCmd to use goroutine + program.Send()**

Replace `sendStreamCmd` in `internal/ui/update.go`:

```go
func (m Model) sendStreamCmd() tea.Cmd {
	messages := m.history.ToAPIMessages()
	client := m.client
	temp := m.cfg.Parameters.Temperature
	maxTok := m.cfg.Parameters.MaxTokens
	p := m.program

	return func() tea.Msg {
		err := client.SendStream(messages, temp, maxTok, func(chunk string) {
			if p != nil {
				p.Send(streamChunkMsg{Content: chunk})
			}
		})
		if err != nil {
			return streamErrMsg{Err: err}
		}
		return streamDoneMsg{}
	}
}
```

**Step 3: Write main.go**

```go
// main.go
package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/termchat/termchat/internal/config"
	"github.com/termchat/termchat/internal/ui"
)

func main() {
	configPath := config.DefaultConfigPath()

	// Allow overriding config path via flag
	if len(os.Args) > 2 && os.Args[1] == "--config" {
		configPath = os.Args[2]
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config from %s: %v\n", configPath, err)
		fmt.Fprintf(os.Stderr, "Create a config file or use --config <path>\n")
		fmt.Fprintf(os.Stderr, "See config.example.yaml for reference.\n")
		os.Exit(1)
	}

	model, err := ui.NewModel(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize: %v\n", err)
		os.Exit(1)
	}

	p := tea.NewProgram(model, tea.WithAltScreen())
	model.SetProgram(p)

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
```

**Step 4: Verify build**

Run: `go build -o termchat .`
Expected: compiles without errors

**Step 5: Commit**

```bash
git add main.go internal/ui/
git commit -m "feat: wire up main entry point with streaming integration"
```

---

### Task 10: Manual End-to-End Test

**Step 1: Create a test config**

Create `~/.config/termchat/config.yaml` with a real API key and endpoint.

**Step 2: Run the application**

Run: `go run .`

**Step 3: Test checklist**

- [ ] App starts and shows input prompt
- [ ] Type a message and press Enter — streaming response appears
- [ ] Markdown in response renders with syntax highlighting
- [ ] Status bar shows model name, token count, message count
- [ ] `/model deepseek-chat` changes the model
- [ ] `/save test1` saves conversation
- [ ] `/clear` clears conversation
- [ ] `/load test1` restores conversation
- [ ] `/list` shows saved conversations
- [ ] Ctrl+C exits cleanly

**Step 4: Fix any issues found during testing**

**Step 5: Commit final fixes**

```bash
git add -A
git commit -m "fix: polish from manual testing"
```

---

### Task 11: Rune-Aware Input & Multi-byte Character Support

**Step 1: Update key handling in update.go**

The current input handling is byte-based. Replace the default key case to properly handle multi-byte characters (Chinese, emoji, etc.):

```go
case tea.KeyMsg:
    // In the default case, replace string length check with rune handling:
    switch msg.Type {
    case tea.KeyRunes:
        m.input += string(msg.Runes)
    }
```

Update backspace to use rune slicing:

```go
case "backspace":
    runes := []rune(m.input)
    if len(runes) > 0 {
        m.input = string(runes[:len(runes)-1])
    }
```

**Step 2: Verify Chinese input works**

Run the app, type Chinese characters, verify they appear correctly.

**Step 3: Commit**

```bash
git add internal/ui/update.go
git commit -m "fix: support multi-byte character input (CJK, emoji)"
```

---

## Summary

| Task | Description | Dependencies |
|------|-------------|-------------|
| 1 | Project scaffolding | — |
| 2 | Config module | 1 |
| 3 | Chat history | 1 |
| 4 | Streaming client | 3 |
| 5 | Conversation storage | 3 |
| 6 | TUI model & status bar | 2, 3, 4, 5 |
| 7 | TUI view (Markdown) | 6 |
| 8 | TUI update (input & commands) | 6 |
| 9 | Wire everything (main.go) | 7, 8 |
| 10 | Manual E2E test | 9 |
| 11 | Multi-byte input fix | 10 |

Tasks 3, 4, 5 can be done in parallel. Tasks 7, 8 can be done in parallel.
