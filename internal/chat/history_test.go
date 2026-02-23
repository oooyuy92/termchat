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

func TestHistory_DeleteAt(t *testing.T) {
	h := NewHistory()
	h.Add(Message{Role: "user", Content: "a"})
	h.Add(Message{Role: "assistant", Content: "b"})
	h.Add(Message{Role: "user", Content: "c"})

	h.DeleteAt(1) // remove "b"

	msgs := h.Messages()
	if len(msgs) != 2 {
		t.Fatalf("len = %d, want 2", len(msgs))
	}
	if msgs[0].Content != "a" || msgs[1].Content != "c" {
		t.Errorf("got %v, want [a c]", msgs)
	}
}

func TestHistory_DeleteAt_OutOfBounds(t *testing.T) {
	h := NewHistory()
	h.Add(Message{Role: "user", Content: "a"})
	h.DeleteAt(-1)  // no-op
	h.DeleteAt(5)   // no-op
	if len(h.Messages()) != 1 {
		t.Errorf("len = %d, want 1 (out-of-bounds delete should be no-op)", len(h.Messages()))
	}
}

func TestHistory_Truncate(t *testing.T) {
	h := NewHistory()
	h.Add(Message{Role: "user", Content: "a"})
	h.Add(Message{Role: "assistant", Content: "b"})
	h.Add(Message{Role: "user", Content: "c"})

	h.Truncate(2) // keep first 2

	msgs := h.Messages()
	if len(msgs) != 2 {
		t.Fatalf("len = %d, want 2", len(msgs))
	}
	if msgs[0].Content != "a" || msgs[1].Content != "b" {
		t.Errorf("got %v, want [a b]", msgs)
	}
}

func TestHistory_Truncate_Noop(t *testing.T) {
	h := NewHistory()
	h.Add(Message{Role: "user", Content: "a"})
	h.Truncate(5) // n > len → no-op
	if len(h.Messages()) != 1 {
		t.Errorf("len = %d, want 1", len(h.Messages()))
	}
}
