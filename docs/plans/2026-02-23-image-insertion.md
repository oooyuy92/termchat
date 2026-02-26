# Image Insertion Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Support pasting images via Ctrl+V and sending them to Gemini API as inline base64 multimodal content.

**Architecture:** Extend Message with optional ImageData, add clipboard image reading via system commands, modify Gemini client to build inline_data parts, and handle Ctrl+V in the Bubble Tea UI.

**Tech Stack:** Go, Bubble Tea, system clipboard tools (xclip/wl-paste/pngpaste)

---

### Task 1: Add ImageData struct and extend Message

**Files:**
- Modify: `internal/chat/history.go:1-7`
- Test: `internal/chat/history_test.go`

**Step 1: Write the failing test**

Add to `internal/chat/history_test.go`:

```go
func TestHistory_MessageWithImages(t *testing.T) {
	h := NewHistory()
	img := ImageData{MimeType: "image/png", Data: []byte("fake-png")}
	h.Add(Message{Role: "user", Content: "describe this", Images: []ImageData{img}})

	msgs := h.Messages()
	if len(msgs) != 1 {
		t.Fatalf("len = %d, want 1", len(msgs))
	}
	if len(msgs[0].Images) != 1 {
		t.Fatalf("images len = %d, want 1", len(msgs[0].Images))
	}
	if msgs[0].Images[0].MimeType != "image/png" {
		t.Errorf("mime = %q, want image/png", msgs[0].Images[0].MimeType)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/chat/ -run TestHistory_MessageWithImages -v`
Expected: FAIL - `ImageData` undefined

**Step 3: Write minimal implementation**

In `internal/chat/history.go`, add before `Message`:

```go
// ImageData holds raw image bytes for multimodal messages.
type ImageData struct {
	MimeType string
	Data     []byte
}
```

Extend `Message`:

```go
type Message struct {
	Role    string      `json:"role"`
	Content string      `json:"content"`
	Images  []ImageData `json:"-"`
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/chat/ -run TestHistory_MessageWithImages -v`
Expected: PASS

**Step 5: Run all existing tests to check no regressions**

Run: `go test ./...`
Expected: All PASS

**Step 6: Commit**

```bash
git add internal/chat/history.go internal/chat/history_test.go
git commit -m "feat: add ImageData struct and extend Message for multimodal support"
```

---

### Task 2: Extend Gemini client to support inline image data

**Files:**
- Modify: `internal/chat/client_gemini.go:42-49` (geminiPart struct)
- Modify: `internal/chat/client_gemini.go:83-99` (stream method)
- Test: `internal/chat/client_test.go`

**Step 1: Write the failing test**

Add to `internal/chat/client_test.go`:

```go
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
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/chat/ -run TestGeminiClient_SendStreamWithImage -v`
Expected: FAIL - request body won't contain `inline_data`

**Step 3: Write minimal implementation**

In `internal/chat/client_gemini.go`:

Add import `"encoding/base64"`.

Replace `geminiPart`:

```go
type geminiPart struct {
	Text       string        `json:"text,omitempty"`
	InlineData *geminiInline `json:"inline_data,omitempty"`
}

type geminiInline struct {
	MimeType string `json:"mime_type"`
	Data     string `json:"data"`
}
```

In the `stream` method, replace the content-building loop (lines ~86-98):

```go
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
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/chat/ -run TestGeminiClient_SendStreamWithImage -v`
Expected: PASS

**Step 5: Run all tests**

Run: `go test ./...`
Expected: All PASS

**Step 6: Commit**

```bash
git add internal/chat/client_gemini.go internal/chat/client_test.go
git commit -m "feat: Gemini client supports inline image data"
```

---

### Task 3: Add clipboard image reading

**Files:**
- Create: `internal/ui/clipboard_image.go`

**Step 1: Implement readImageFromClipboard**

Create `internal/ui/clipboard_image.go`:

```go
package ui

import (
	"fmt"
	"os/exec"
	"runtime"

	"github.com/termchat/termchat/internal/chat"
)

// readImageFromClipboard attempts to read image data from the system clipboard.
// Returns nil, nil if no image is found in the clipboard.
func readImageFromClipboard() (*chat.ImageData, error) {
	type attempt struct {
		mime string
		args []string
	}

	var attempts []attempt

	switch runtime.GOOS {
	case "darwin":
		// pngpaste writes PNG to stdout with "-"
		attempts = []attempt{
			{"image/png", []string{"pngpaste", "-"}},
		}
	case "windows":
		// PowerShell clipboard image reading
		attempts = []attempt{
			{"image/png", []string{"powershell", "-command", "Get-Clipboard -Format Image | ForEach-Object { $ms = New-Object System.IO.MemoryStream; $_.Save($ms, [System.Drawing.Imaging.ImageFormat]::Png); [Console]::OpenStandardOutput().Write($ms.ToArray(), 0, $ms.Length) }"}},
		}
	default:
		// Linux: try wl-paste (Wayland) first, then xclip (X11)
		attempts = []attempt{
			{"image/png", []string{"wl-paste", "--type", "image/png"}},
			{"image/jpeg", []string{"wl-paste", "--type", "image/jpeg"}},
			{"image/png", []string{"xclip", "-selection", "clipboard", "-t", "image/png", "-o"}},
			{"image/jpeg", []string{"xclip", "-selection", "clipboard", "-t", "image/jpeg", "-o"}},
		}
	}

	for _, a := range attempts {
		if _, err := exec.LookPath(a.args[0]); err != nil {
			continue
		}
		cmd := exec.Command(a.args[0], a.args[1:]...)
		out, err := cmd.Output()
		if err == nil && len(out) > 0 {
			return &chat.ImageData{MimeType: a.mime, Data: out}, nil
		}
	}

	return nil, nil
}

// readTextFromClipboard reads text content from the system clipboard.
func readTextFromClipboard() (string, error) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("pbpaste")
	case "windows":
		cmd = exec.Command("powershell", "-command", "Get-Clipboard")
	default:
		if _, err := exec.LookPath("wl-paste"); err == nil {
			cmd = exec.Command("wl-paste", "--no-newline")
		} else if _, err := exec.LookPath("xclip"); err == nil {
			cmd = exec.Command("xclip", "-selection", "clipboard", "-o")
		} else if _, err := exec.LookPath("xsel"); err == nil {
			cmd = exec.Command("xsel", "--clipboard", "--output")
		} else {
			return "", fmt.Errorf("no clipboard tool found")
		}
	}
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}
```

**Step 2: Verify it compiles**

Run: `go build ./...`
Expected: Success

**Step 3: Commit**

```bash
git add internal/ui/clipboard_image.go
git commit -m "feat: add clipboard image and text reading functions"
```

---

### Task 4: Add Ctrl+V handling and pendingImages in UI

**Files:**
- Modify: `internal/ui/model.go:151-193` (Model struct)
- Modify: `internal/ui/update.go:79-144` (key handling)

**Step 1: Add pendingImages fields to Model**

In `internal/ui/model.go`, add to the `Model` struct (after `input string`):

```go
pendingImages []chat.ImageData
imageCounter  int
```

**Step 2: Handle Ctrl+V in update.go**

In `internal/ui/update.go`, add a new case before the `default:` case in the chat mode key switch (around line 131):

```go
case "ctrl+v":
	return m, m.pasteFromClipboard()
```

Add a new message type in `model.go`:

```go
type clipboardImageMsg struct {
	Image *chat.ImageData
	Text  string
	Err   error
}
```

Add the paste command method (in `update.go` or a new section):

```go
func (m Model) pasteFromClipboard() tea.Cmd {
	return func() tea.Msg {
		img, err := readImageFromClipboard()
		if err != nil {
			return clipboardImageMsg{Err: err}
		}
		if img != nil {
			return clipboardImageMsg{Image: img}
		}
		// No image, try text
		text, err := readTextFromClipboard()
		return clipboardImageMsg{Text: text, Err: err}
	}
}
```

Handle the message in `Update()`, add a new case after `clipboardResultMsg`:

```go
case clipboardImageMsg:
	if msg.Err != nil {
		m.statusMsg = "Paste failed: " + msg.Err.Error()
		return m, nil
	}
	if msg.Image != nil {
		m.imageCounter++
		m.pendingImages = append(m.pendingImages, *msg.Image)
		m.input += fmt.Sprintf("[image %d]", m.imageCounter)
		m.statusMsg = fmt.Sprintf("Image %d pasted", m.imageCounter)
		return m, nil
	}
	// Text paste
	m.input += msg.Text
	if m.input == "/" {
		m.mode = modeSlashComplete
		m.slashAC = slashComplete{
			matches: filterSlashCmds("/"),
			cursor:  0,
			offset:  0,
		}
	}
	return m, nil
```

**Step 3: Attach images on Enter send**

In `update.go`, modify the `case "enter":` block. After `m.history.Add(...)`, attach images:

Replace:
```go
m.history.Add(chat.Message{Role: "user", Content: input})
```

With:
```go
m.history.Add(chat.Message{Role: "user", Content: input, Images: m.pendingImages})
m.pendingImages = nil
```

**Step 4: Clear images on /clear**

In `handleCommand`, in the `/clear` case, add:

```go
m.pendingImages = nil
m.imageCounter = 0
```

**Step 5: Verify it compiles**

Run: `go build ./...`
Expected: Success

**Step 6: Run all tests**

Run: `go test ./...`
Expected: All PASS

**Step 7: Commit**

```bash
git add internal/ui/model.go internal/ui/update.go
git commit -m "feat: Ctrl+V image paste support in chat UI"
```

---

### Task 5: Manual integration test

**Step 1: Build and run**

Run: `go build -o termchat . && ./termchat`

**Step 2: Test image paste**

1. Copy an image to clipboard (e.g. screenshot)
2. Press Ctrl+V in termchat
3. Verify `[image 1]` appears in input and status bar shows "Image 1 pasted"
4. Type a question like "describe this image" after the placeholder
5. Press Enter
6. Verify Gemini responds with image description (requires Gemini provider configured)

**Step 3: Test text paste fallback**

1. Copy text to clipboard
2. Press Ctrl+V
3. Verify text is pasted normally into input

**Step 4: Test /clear resets images**

1. Paste an image
2. Type `/clear`
3. Paste another image
4. Verify counter resets (shows `[image 1]` not `[image 2]`)

**Step 5: Final commit**

```bash
git add -A
git commit -m "feat: image insertion support for Gemini multimodal"
```
