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
