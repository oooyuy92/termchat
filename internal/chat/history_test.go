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
	h.DeleteAt(-1) // no-op
	h.DeleteAt(5)  // no-op
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

func TestHistory_Truncate_Noop(t *testing.T) {
	h := NewHistory()
	h.Add(Message{Role: "user", Content: "a"})
	h.Truncate(5) // n > len → no-op
	if len(h.Messages()) != 1 {
		t.Errorf("len = %d, want 1", len(h.Messages()))
	}
}

func TestHistory_MessagesPreserveMetadata(t *testing.T) {
	h := NewHistory()
	h.Add(Message{
		ID:                    42,
		Seq:                   7,
		Role:                  "assistant",
		Content:               "v2 reply",
		VersionGroupID:        40,
		VersionNumber:         2,
		TotalVersions:         4,
		EditedAfterGeneration: false,
		StaleAfterUserEdit:    true,
	})

	msgs := h.Messages()
	if len(msgs) != 1 {
		t.Fatalf("len = %d, want 1", len(msgs))
	}
	if msgs[0].ID != 42 || msgs[0].Seq != 7 {
		t.Fatalf("got message IDs %+v, want ID=42 seq=7", msgs[0])
	}
	if msgs[0].VersionGroupID != 40 || msgs[0].VersionNumber != 2 || msgs[0].TotalVersions != 4 {
		t.Fatalf("got version metadata %+v", msgs[0])
	}
	if !msgs[0].StaleAfterUserEdit {
		t.Fatalf("expected stale flag to be preserved")
	}
}

func TestHistory_ReplaceMessages(t *testing.T) {
	h := NewHistory()
	h.Add(Message{Role: "user", Content: "first"})
	h.ReplaceMessages([]Message{
		{ID: 1, Seq: 1, Role: "user", Content: "edited"},
		{ID: 2, Seq: 1, Role: "assistant", Content: "active reply", VersionGroupID: 2, VersionNumber: 1, TotalVersions: 3},
	})

	msgs := h.Messages()
	if len(msgs) != 2 {
		t.Fatalf("len = %d, want 2", len(msgs))
	}
	if msgs[0].Content != "edited" || msgs[1].Content != "active reply" {
		t.Fatalf("ReplaceMessages() = %+v", msgs)
	}
}

func TestMessage_HasGenerationSnapshotAllowsEmptyRolePrompt(t *testing.T) {
	msg := Message{
		SnapshotProvider:   "gateway",
		SnapshotModel:      "gemini-3-flash-preview",
		SnapshotAPIFormat:  "gemini",
		SnapshotRolePrompt: "",
	}

	if !msg.HasGenerationSnapshot() {
		t.Fatalf("HasGenerationSnapshot() = false, want true when provider/model/api_format are present")
	}
}
