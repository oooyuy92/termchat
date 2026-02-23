// internal/chat/history.go
package chat

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

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

// DeleteAt removes the message at index i. No-op if i is out of bounds.
func (h *History) DeleteAt(i int) {
	if i < 0 || i >= len(h.messages) {
		return
	}
	h.messages = append(h.messages[:i], h.messages[i+1:]...)
}

// Truncate keeps only the first n messages, discarding the rest.
// No-op if n >= len(messages).
func (h *History) Truncate(n int) {
	if n >= len(h.messages) {
		return
	}
	h.messages = h.messages[:n]
}
