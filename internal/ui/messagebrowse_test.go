// internal/ui/messagebrowse_test.go
package ui

import (
	"context"
	"errors"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/termchat/termchat/internal/chat"
	"github.com/termchat/termchat/internal/config"
	"github.com/termchat/termchat/internal/storage"
)

type streamingStubProvider struct {
	stubProvider
	response          string
	received          []chat.Message
	temperature       float64
	maxTokens         int
	reasoningEffort   string
	budgetTokens      int
}

func (s *streamingStubProvider) SendStreamChan(_ context.Context, messages []chat.Message, temperature float64, maxTokens int, reasoningEffort string, budgetTokens int) (<-chan chat.StreamChunk, <-chan error) {
	s.received = append([]chat.Message(nil), messages...)
	s.temperature = temperature
	s.maxTokens = maxTokens
	s.reasoningEffort = reasoningEffort
	s.budgetTokens = budgetTokens

	chunks := make(chan chat.StreamChunk, 1)
	errs := make(chan error, 1)
	chunks <- chat.StreamChunk{Content: s.response}
	close(chunks)
	close(errs)
	return chunks, errs
}

type erroringProvider struct {
	model string
	err   error
}

func (p *erroringProvider) SendStreamChan(_ context.Context, _ []chat.Message, _ float64, _ int, _ string, _ int) (<-chan chat.StreamChunk, <-chan error) {
	chunks := make(chan chat.StreamChunk)
	errs := make(chan error, 1)
	errs <- p.err
	close(chunks)
	close(errs)
	return chunks, errs
}

func (p *erroringProvider) Model() string         { return p.model }
func (p *erroringProvider) SetModel(model string) { p.model = model }
func (p *erroringProvider) SetBaseURL(string)     {}
func (p *erroringProvider) SetAPIKey(string)      {}
func (p *erroringProvider) BaseURL() string       { return "" }
func (p *erroringProvider) APIKey() string        { return "" }
func (p *erroringProvider) SupportsVision() bool  { return false }

func newBrowserTestStore(t *testing.T) *storage.Store {
	t.Helper()
	store, err := storage.New(t.TempDir())
	if err != nil {
		t.Fatalf("storage.New() error = %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func newBrowserTestModel(t *testing.T, store *storage.Store) Model {
	t.Helper()
	cfg := config.DefaultConfig()
	tab, err := newTabSession(cfg, &stubProvider{model: "test-model"}, 80)
	if err != nil {
		t.Fatalf("newTabSession() error = %v", err)
	}
	return Model{
		cfg:       cfg,
		store:     store,
		theme:     DarkTheme,
		tabs:      []TabSession{tab},
		activeTab: 0,
		width:     80,
		height:    24,
	}
}

type seedTurn struct {
	user      string
	assistant []string
}

func seedConversation(t *testing.T, store *storage.Store, name string, turns []seedTurn) {
	t.Helper()
	seq := 1
	for _, turn := range turns {
		// Add user message
		userMsg := chat.Message{Role: "user", Content: turn.user, Seq: seq}
		_, err := store.AppendMessage(name, userMsg)
		if err != nil {
			t.Fatalf("AppendMessage(%q) error = %v", turn.user, err)
		}
		seq++

		// Add assistant version(s)
		if len(turn.assistant) > 0 {
			// First version: append as regular message, then init version group
			firstMsg := chat.Message{
				Role:             "assistant",
				Content:          turn.assistant[0],
				Seq:              seq,
				SnapshotProvider: "openai-compatible",
				SnapshotModel:    "test-model", SnapshotAPIFormat: "openai-compatible",
			}
			firstID, err := store.AppendMessage(name, firstMsg)
			if err != nil {
				t.Fatalf("AppendMessage(%q) error = %v", turn.assistant[0], err)
			}

			// Initialize version group using first message ID
			if err := store.InitVersionGroup(firstID); err != nil {
				t.Fatalf("InitVersionGroup() error = %v", err)
			}

			// Add additional versions
			for i := 1; i < len(turn.assistant); i++ {
				assistantMsg := chat.Message{
					Role:             "assistant",
					Content:          turn.assistant[i],
					Seq:              seq,
					VersionNumber:    i + 1,
					SnapshotProvider: "openai-compatible",
					SnapshotModel:    "test-model", SnapshotAPIFormat: "openai-compatible",
				}
				if _, err := store.AppendAssistantVersion(name, firstID, assistantMsg); err != nil {
					t.Fatalf("AppendAssistantVersion(%q) error = %v", turn.assistant[i], err)
				}
			}
			seq++
		}
	}
}

func seedConversationLegacy(t *testing.T, store *storage.Store, name string, msgs []chat.Message) {
	t.Helper()
	for _, msg := range msgs {
		if _, err := store.AppendMessage(name, msg); err != nil {
			t.Fatalf("AppendMessage(%q) error = %v", msg.Content, err)
		}
	}
}

func mustLoadActiveTimeline(t *testing.T, store *storage.Store, name string) []chat.Message {
	t.Helper()
	msgs, err := store.LoadActiveTimeline(name)
	if err != nil {
		t.Fatalf("LoadActiveTimeline() error = %v", err)
	}
	return msgs
}

func TestIncrementalPersistence(t *testing.T) {
	store := newBrowserTestStore(t)
	model := newBrowserTestModel(t, store)

	// Seed a conversation with a user message
	seedConversationLegacy(t, store, "conv", []chat.Message{
		{Seq: 1, Role: "user", Content: "q1"},
	})

	model.tabs[0].autoSaveName = "conv"
	model.tabs[0].history.ReplaceMessages(mustLoadActiveTimeline(t, store, "conv"))

	// Verify the message was loaded
	msgs := model.tabs[0].history.Messages()
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	if msgs[0].Content != "q1" {
		t.Fatalf("expected content 'q1', got %q", msgs[0].Content)
	}
}

func newVersionedBrowseModel(t *testing.T) Model {
	t.Helper()
	store := newBrowserTestStore(t)
	m := newBrowserTestModel(t, store)
	m.messageBrowse.turns = []browseTurn{
		{
			User: chat.Message{ID: 1, Seq: 1, Role: "user", Content: "q1"},
			AssistantVersions: []chat.Message{
				{ID: 2, Seq: 1, Role: "assistant", Content: "v1", VersionGroupID: 2, VersionNumber: 1, TotalVersions: 2, SnapshotProvider: "openai-compatible", SnapshotModel:    "test-model", SnapshotAPIFormat: "openai-compatible"},
				{ID: 3, Seq: 1, Role: "assistant", Content: "v2", VersionGroupID: 2, VersionNumber: 2, TotalVersions: 2, SnapshotProvider: "openai-compatible", SnapshotModel:    "test-model", SnapshotAPIFormat: "openai-compatible"},
			},
			ActiveVersion:  0,
			PreviewVersion: 0,
		},
		{
			User: chat.Message{ID: 4, Seq: 2, Role: "user", Content: "q2"},
			AssistantVersions: []chat.Message{
				{ID: 5, Seq: 2, Role: "assistant", Content: "a2", VersionNumber: 1, TotalVersions: 1, SnapshotProvider: "openai-compatible", SnapshotModel:    "test-model", SnapshotAPIFormat: "openai-compatible"},
			},
			ActiveVersion:  0,
			PreviewVersion: 0,
		},
	}
	return m
}

func TestMessageBrowse_RightArrowChangesPreviewOnly(t *testing.T) {
	m := newVersionedBrowseModel(t)
	m.mode = modeMessageBrowse
	m.messageBrowse.mode = browseModeMessage

	before := m.messageBrowse.turns[0].ActiveVersion
	m, _ = m.updateMessageBrowse(tea.KeyMsg{Type: tea.KeyRight})

	if m.messageBrowse.turns[0].ActiveVersion != before {
		t.Fatalf("active version changed during preview navigation")
	}
	if m.messageBrowse.turns[0].PreviewVersion == before {
		t.Fatalf("preview version did not move")
	}
}

func TestMessageBrowse_EnterOnPreviewWithLaterTurnsOpensConfirmation(t *testing.T) {
	m := newVersionedBrowseModel(t)
	m.mode = modeMessageBrowse
	m.messageBrowse.mode = browseModeMessage
	m.messageBrowse.turnIdx = 0
	m.messageBrowse.turns[0].PreviewVersion = 1

	m, _ = m.updateMessageBrowse(tea.KeyMsg{Type: tea.KeyEnter})
	if m.messageBrowse.pendingConfirm.kind != confirmApplyPreview {
		t.Fatalf("pending confirm kind = %v, want confirmApplyPreview", m.messageBrowse.pendingConfirm.kind)
	}
}

func TestMessageBrowse_VEntersCompareMode(t *testing.T) {
	m := newVersionedBrowseModel(t)
	m.mode = modeMessageBrowse
	m.messageBrowse.mode = browseModeMessage

	m, _ = m.updateMessageBrowse(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("v")})
	if m.messageBrowse.mode != browseModeCompare {
		t.Fatalf("mode = %v, want browseModeCompare", m.messageBrowse.mode)
	}
}

func newEditableBrowseModel(t *testing.T) Model {
	t.Helper()
	store := newBrowserTestStore(t)
	m := newBrowserTestModel(t, store)

	// Seed conversation with messages
	name := "conv"
	userID, _ := store.AppendMessage(name, chat.Message{Seq: 1, Role: "user", Content: "q1"})
	assistantID, _ := store.AppendMessage(name, chat.Message{Seq: 1, Role: "assistant", Content: "v1", VersionNumber: 1, SnapshotProvider: "openai-compatible", SnapshotModel:    "test-model", SnapshotAPIFormat: "openai-compatible"})
	_ = store.InitVersionGroup(assistantID)
	_, _ = store.AppendAssistantVersion(name, assistantID, chat.Message{Seq: 1, Role: "assistant", Content: "v2", VersionNumber: 2, SnapshotProvider: "openai-compatible", SnapshotModel:    "test-model", SnapshotAPIFormat: "openai-compatible"})
	user2ID, _ := store.AppendMessage(name, chat.Message{Seq: 2, Role: "user", Content: "q2"})
	assistant2ID, _ := store.AppendMessage(name, chat.Message{Seq: 2, Role: "assistant", Content: "a2", VersionNumber: 1, SnapshotProvider: "openai-compatible", SnapshotModel:    "test-model", SnapshotAPIFormat: "openai-compatible"})

	m.tabs[0].autoSaveName = name

	// Build browse turns
	m.messageBrowse.turns = []browseTurn{
		{
			User: chat.Message{ID: userID, Seq: 1, Role: "user", Content: "q1"},
			AssistantVersions: []chat.Message{
				{ID: assistantID, Seq: 1, Role: "assistant", Content: "v1", VersionGroupID: assistantID, VersionNumber: 1, TotalVersions: 2, SnapshotProvider: "openai-compatible", SnapshotModel:    "test-model", SnapshotAPIFormat: "openai-compatible"},
				{ID: assistantID + 1, Seq: 1, Role: "assistant", Content: "v2", VersionGroupID: assistantID, VersionNumber: 2, TotalVersions: 2, SnapshotProvider: "openai-compatible", SnapshotModel:    "test-model", SnapshotAPIFormat: "openai-compatible"},
			},
			ActiveVersion:  0,
			PreviewVersion: 0,
		},
		{
			User: chat.Message{ID: user2ID, Seq: 2, Role: "user", Content: "q2"},
			AssistantVersions: []chat.Message{
				{ID: assistant2ID, Seq: 2, Role: "assistant", Content: "a2", VersionNumber: 1, TotalVersions: 1, SnapshotProvider: "openai-compatible", SnapshotModel:    "test-model", SnapshotAPIFormat: "openai-compatible"},
			},
			ActiveVersion:  0,
			PreviewVersion: 0,
		},
	}
	return m
}

func TestMessageBrowse_EditSaveOnlyMarksTurnWithoutTruncating(t *testing.T) {
	m := newEditableBrowseModel(t)
	m.mode = modeMessageBrowse

	m, _ = m.updateMessageBrowse(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("e")})
	m.messageBrowse.editBuffer = "edited question"
	m, _ = m.updateMessageBrowse(tea.KeyMsg{Type: tea.KeyEnter})
	m.messageBrowse.pendingConfirm.cursor = 1 // Save Only
	m, _ = m.updateMessageBrowse(tea.KeyMsg{Type: tea.KeyEnter})

	turn := m.messageBrowse.turns[0]
	if !turn.User.EditedAfterGeneration {
		t.Fatalf("expected user turn to be marked edited")
	}
	if !turn.AssistantVersions[turn.ActiveVersion].StaleAfterUserEdit {
		t.Fatalf("expected assistant turn to be marked stale")
	}
	if len(m.messageBrowse.turns) < 2 {
		t.Fatalf("later turns should be preserved")
	}
}

func TestMessageBrowse_DeleteActiveVersionPromotesNearestRemainingVersion(t *testing.T) {
	store := newBrowserTestStore(t)
	m := newBrowserTestModel(t, store)

	const name = "conv"
	_, err := store.AppendMessage(name, chat.Message{Seq: 1, Role: "user", Content: "q1"})
	if err != nil {
		t.Fatalf("AppendMessage(user) error = %v", err)
	}
	assistantID, err := store.AppendMessage(name, chat.Message{Seq: 1, Role: "assistant", Content: "v1", VersionNumber: 1, SnapshotProvider: "openai-compatible", SnapshotModel:    "test-model", SnapshotAPIFormat: "openai-compatible"})
	if err != nil {
		t.Fatalf("AppendMessage(v1) error = %v", err)
	}
	if err := store.InitVersionGroup(assistantID); err != nil {
		t.Fatalf("InitVersionGroup() error = %v", err)
	}
	_, err = store.AppendAssistantVersion(name, assistantID, chat.Message{Seq: 1, Role: "assistant", Content: "v2", VersionNumber: 2, SnapshotProvider: "openai-compatible", SnapshotModel:    "test-model", SnapshotAPIFormat: "openai-compatible"})
	if err != nil {
		t.Fatalf("AppendAssistantVersion(v2) error = %v", err)
	}
	if err := store.SetActiveVersion(name, assistantID, 2); err != nil {
		t.Fatalf("SetActiveVersion() error = %v", err)
	}

	m.tabs[0].autoSaveName = name
	m.tabs[0].history.ReplaceMessages(mustLoadActiveTimeline(t, store, name))
	if err := m.buildBrowserState(); err != nil {
		t.Fatalf("buildBrowserState() error = %v", err)
	}
	m.mode = modeMessageBrowse

	m, _ = m.updateMessageBrowse(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	m.messageBrowse.pendingConfirm.cursor = deleteAssistantOnly
	m, _ = m.updateMessageBrowse(tea.KeyMsg{Type: tea.KeyEnter})

	if m.messageBrowse.turns[0].ActiveVersion != 0 {
		t.Fatalf("active version = %d, want 0", m.messageBrowse.turns[0].ActiveVersion)
	}
	if m.messageBrowse.turns[0].AssistantVersions[0].Content != "v1" {
		t.Fatalf("remaining active content = %q, want v1", m.messageBrowse.turns[0].AssistantVersions[0].Content)
	}
}

func TestMessageBrowse_BranchUsesPreviewVersionWhenConfirmed(t *testing.T) {
	store := newBrowserTestStore(t)
	m := newBrowserTestModel(t, store)
	seedConversationLegacy(t, store, "conv", []chat.Message{
		{Seq: 1, Role: "user", Content: "q1"},
		{Seq: 1, Role: "assistant", Content: "v1", VersionGroupID: 2, VersionNumber: 1, SnapshotProvider: "openai-compatible", SnapshotModel:    "test-model", SnapshotAPIFormat: "openai-compatible"},
		{Seq: 2, Role: "user", Content: "q2"},
		{Seq: 2, Role: "assistant", Content: "a2", VersionNumber: 1, SnapshotProvider: "openai-compatible", SnapshotModel:    "test-model", SnapshotAPIFormat: "openai-compatible"},
	})
	m.tabs[0].autoSaveName = "conv"
	m.messageBrowse = newVersionedBrowseModel(t).messageBrowse
	m.messageBrowse.turns[0].PreviewVersion = 1

	m, _ = m.updateMessageBrowse(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("b")})
	if m.tabs[0].autoSaveName == "conv" {
		t.Fatalf("expected branch to switch to a new conversation name")
	}
	branched, err := store.LoadActiveTimeline(m.tabs[0].autoSaveName)
	if err != nil {
		t.Fatalf("LoadActiveTimeline(branch) error = %v", err)
	}
	if len(branched) == 0 || branched[1].Content != "v2" {
		t.Fatalf("branch content = %+v, want preview version v2", branched)
	}
}

func TestMessageBrowse_CompareModeMouseWheelScrollsHoveredCard(t *testing.T) {
	m := newVersionedBrowseModel(t)
	m.mode = modeMessageBrowse
	m.messageBrowse.mode = browseModeCompare
	m.messageBrowse.compareCardIdx = 0
	m.width = 80
	m.height = 24

	next, _ := m.Update(tea.MouseMsg{X: 50, Y: 8, Action: tea.MouseActionPress, Button: tea.MouseButtonWheelDown})
	m = next.(Model)

	if m.messageBrowse.compareCardIdx != 1 {
		t.Fatalf("compareCardIdx = %d, want 1", m.messageBrowse.compareCardIdx)
	}
	if got := m.messageBrowse.compareCardScrolls[1]; got != 1 {
		t.Fatalf("compareCardScrolls[1] = %d, want 1", got)
	}
	if got := m.messageBrowse.compareCardScrolls[0]; got != 0 {
		t.Fatalf("compareCardScrolls[0] = %d, want 0", got)
	}
}

func TestMessageBrowse_InitializeFromHistory(t *testing.T) {
	store := newBrowserTestStore(t)
	m := newBrowserTestModel(t, store)

	// Seed conversation with 2 turns
	seedConversation(t, store, "test", []seedTurn{
		{user: "first", assistant: []string{"response 1"}},
		{user: "second", assistant: []string{"response 2a", "response 2b"}},
	})

	// Load into tab
	msgs := mustLoadActiveTimeline(t, store, "test")
	m.tabs[0].history.ReplaceMessages(msgs)
	m.tabs[0].autoSaveName = "test"

	// Simulate double-Esc
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = next.(Model)
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = next.(Model)

	// Verify browser state initialized
	if m.mode != modeMessageBrowse {
		t.Fatal("expected modeMessageBrowse")
	}
	if len(m.messageBrowse.turns) != 2 {
		t.Fatalf("expected 2 turns, got %d", len(m.messageBrowse.turns))
	}
	if m.messageBrowse.turnIdx != 1 {
		t.Fatalf("expected turnIdx=1 (last turn), got %d", m.messageBrowse.turnIdx)
	}

	// Verify turn 1
	turn0 := m.messageBrowse.turns[0]
	if turn0.User.Content != "first" {
		t.Errorf("turn 0 user: got %q", turn0.User.Content)
	}
	if len(turn0.AssistantVersions) != 1 {
		t.Fatalf("turn 0: expected 1 version, got %d", len(turn0.AssistantVersions))
	}

	// Verify turn 2
	turn1 := m.messageBrowse.turns[1]
	if turn1.User.Content != "second" {
		t.Errorf("turn 1 user: got %q", turn1.User.Content)
	}
	if len(turn1.AssistantVersions) != 2 {
		t.Fatalf("turn 1: expected 2 versions, got %d", len(turn1.AssistantVersions))
	}
	if turn1.ActiveVersion != 0 {
		t.Errorf("turn 1: expected ActiveVersion=0, got %d", turn1.ActiveVersion)
	}
	if turn1.PreviewVersion != 0 {
		t.Errorf("turn 1: expected PreviewVersion=0, got %d", turn1.PreviewVersion)
	}
}

func TestMessageBrowse_UpDownMovesTurnSelection(t *testing.T) {
	m := newVersionedBrowseModel(t)
	m.mode = modeMessageBrowse
	m.messageBrowse.mode = browseModeMessage
	m.messageBrowse.turnIdx = 0

	m, _ = m.updateMessageBrowse(tea.KeyMsg{Type: tea.KeyDown})
	if m.messageBrowse.turnIdx != 1 {
		t.Fatalf("turnIdx after down = %d, want 1", m.messageBrowse.turnIdx)
	}

	m, _ = m.updateMessageBrowse(tea.KeyMsg{Type: tea.KeyUp})
	if m.messageBrowse.turnIdx != 0 {
		t.Fatalf("turnIdx after up = %d, want 0", m.messageBrowse.turnIdx)
	}
}

func TestMessageBrowse_ApplyPreviewPersistsWithoutLaterTurns(t *testing.T) {
	store := newBrowserTestStore(t)
	m := newBrowserTestModel(t, store)

	seedConversation(t, store, "conv", []seedTurn{
		{user: "q1", assistant: []string{"v1", "v2"}},
	})

	m.tabs[0].autoSaveName = "conv"
	m.tabs[0].history.ReplaceMessages(mustLoadActiveTimeline(t, store, "conv"))
	if err := m.buildBrowserState(); err != nil {
		t.Fatalf("buildBrowserState() error = %v", err)
	}
	m.mode = modeMessageBrowse

	m, _ = m.updateMessageBrowse(tea.KeyMsg{Type: tea.KeyRight})
	m, _ = m.updateMessageBrowse(tea.KeyMsg{Type: tea.KeyEnter})

	reloaded := mustLoadActiveTimeline(t, store, "conv")
	if got := reloaded[1].Content; got != "v2" {
		t.Fatalf("active timeline assistant = %q, want v2", got)
	}
	if got := m.tabs[0].history.Messages()[1].Content; got != "v2" {
		t.Fatalf("tab history assistant = %q, want v2", got)
	}
	if got := m.messageBrowse.turns[0].ActiveVersion; got != 1 {
		t.Fatalf("ActiveVersion = %d, want 1", got)
	}
}

func TestMessageBrowse_DeletePersistsToStorage(t *testing.T) {
	store := newBrowserTestStore(t)
	m := newBrowserTestModel(t, store)

	const name = "conv"
	_, err := store.AppendMessage(name, chat.Message{Seq: 1, Role: "user", Content: "q1"})
	if err != nil {
		t.Fatalf("AppendMessage(user) error = %v", err)
	}
	assistantID, err := store.AppendMessage(name, chat.Message{Seq: 1, Role: "assistant", Content: "v1", VersionNumber: 1, SnapshotProvider: "openai-compatible", SnapshotModel:    "test-model", SnapshotAPIFormat: "openai-compatible"})
	if err != nil {
		t.Fatalf("AppendMessage(v1) error = %v", err)
	}
	if err := store.InitVersionGroup(assistantID); err != nil {
		t.Fatalf("InitVersionGroup() error = %v", err)
	}
	v2ID, err := store.AppendAssistantVersion(name, assistantID, chat.Message{Seq: 1, Role: "assistant", Content: "v2", VersionNumber: 2, SnapshotProvider: "openai-compatible", SnapshotModel:    "test-model", SnapshotAPIFormat: "openai-compatible"})
	if err != nil {
		t.Fatalf("AppendAssistantVersion(v2) error = %v", err)
	}
	if err := store.SetActiveVersion(name, assistantID, 2); err != nil {
		t.Fatalf("SetActiveVersion() error = %v", err)
	}

	m.tabs[0].autoSaveName = name
	m.tabs[0].history.ReplaceMessages(mustLoadActiveTimeline(t, store, name))
	if err := m.buildBrowserState(); err != nil {
		t.Fatalf("buildBrowserState() error = %v", err)
	}
	m.mode = modeMessageBrowse

	m, _ = m.updateMessageBrowse(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	m.messageBrowse.pendingConfirm.cursor = deleteAssistantOnly
	m, _ = m.updateMessageBrowse(tea.KeyMsg{Type: tea.KeyEnter})

	reloaded := mustLoadActiveTimeline(t, store, name)
	if got := reloaded[1].Content; got != "v1" {
		t.Fatalf("active timeline assistant = %q, want v1", got)
	}
	if got := m.tabs[0].history.Messages()[1].ID; got != assistantID {
		t.Fatalf("tab history assistant ID = %d, want %d", got, assistantID)
	}
	if got := m.messageBrowse.turns[0].ActiveVersion; got != 0 {
		t.Fatalf("ActiveVersion = %d, want 0", got)
	}
	if got := m.messageBrowse.turns[0].AssistantVersions[0].ID; got == v2ID {
		t.Fatalf("deleted version %d still present in browser", v2ID)
	}
}

func TestMessageBrowse_DeleteAssistantRemovesPreviewVersion(t *testing.T) {
	store := newBrowserTestStore(t)
	m := newBrowserTestModel(t, store)

	const name = "conv"
	_, err := store.AppendMessage(name, chat.Message{Seq: 1, Role: "user", Content: "q1"})
	if err != nil {
		t.Fatalf("AppendMessage(user) error = %v", err)
	}
	assistantID, err := store.AppendMessage(name, chat.Message{Seq: 1, Role: "assistant", Content: "v1", VersionNumber: 1, SnapshotProvider: "openai-compatible", SnapshotModel:    "test-model", SnapshotAPIFormat: "openai-compatible"})
	if err != nil {
		t.Fatalf("AppendMessage(v1) error = %v", err)
	}
	if err := store.InitVersionGroup(assistantID); err != nil {
		t.Fatalf("InitVersionGroup() error = %v", err)
	}
	v2ID, err := store.AppendAssistantVersion(name, assistantID, chat.Message{
		Seq:              1,
		Role:             "assistant",
		Content:          "v2",
		VersionNumber:    2,
		SnapshotProvider: "openai-compatible",
		SnapshotModel:    "test-model", SnapshotAPIFormat: "openai-compatible",
	})
	if err != nil {
		t.Fatalf("AppendAssistantVersion(v2) error = %v", err)
	}

	m.tabs[0].autoSaveName = name
	m.tabs[0].history.ReplaceMessages(mustLoadActiveTimeline(t, store, name))
	if err := m.buildBrowserState(); err != nil {
		t.Fatalf("buildBrowserState() error = %v", err)
	}
	m.mode = modeMessageBrowse
	m.messageBrowse.turns[0].PreviewVersion = 1

	m, _ = m.updateMessageBrowse(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	if m.messageBrowse.pendingConfirm.kind != confirmDeleteSelection {
		t.Fatalf("pendingConfirm = %v, want confirmDeleteSelection", m.messageBrowse.pendingConfirm.kind)
	}
	m.messageBrowse.pendingConfirm.cursor = deleteAssistantOnly
	m, _ = m.updateMessageBrowse(tea.KeyMsg{Type: tea.KeyEnter})

	if len(m.messageBrowse.turns) != 1 {
		t.Fatalf("turn count = %d, want 1", len(m.messageBrowse.turns))
	}
	if len(m.messageBrowse.turns[0].AssistantVersions) != 1 {
		t.Fatalf("assistant version count = %d, want 1", len(m.messageBrowse.turns[0].AssistantVersions))
	}
	if got := m.messageBrowse.turns[0].AssistantVersions[0].ID; got == v2ID {
		t.Fatalf("preview version %d still present", v2ID)
	}

	reloaded, err := store.LoadBrowseMessages(name)
	if err != nil {
		t.Fatalf("LoadBrowseMessages() error = %v", err)
	}
	var deletedPreview bool
	for _, msg := range reloaded {
		if msg.ID == v2ID {
			deletedPreview = msg.Deleted
		}
	}
	if !deletedPreview {
		t.Fatalf("preview version %d not marked deleted", v2ID)
	}
}

func TestMessageBrowse_DeleteLastAssistantKeepsEmptyTurn(t *testing.T) {
	store := newBrowserTestStore(t)
	m := newBrowserTestModel(t, store)

	const name = "conv"
	_, err := store.AppendMessage(name, chat.Message{Seq: 1, Role: "user", Content: "q1"})
	if err != nil {
		t.Fatalf("AppendMessage(user) error = %v", err)
	}
	_, err = store.AppendMessage(name, chat.Message{Seq: 1, Role: "assistant", Content: "v1", VersionNumber: 1, SnapshotProvider: "openai-compatible", SnapshotModel:    "test-model", SnapshotAPIFormat: "openai-compatible"})
	if err != nil {
		t.Fatalf("AppendMessage(v1) error = %v", err)
	}

	m.tabs[0].autoSaveName = name
	m.tabs[0].history.ReplaceMessages(mustLoadActiveTimeline(t, store, name))
	if err := m.buildBrowserState(); err != nil {
		t.Fatalf("buildBrowserState() error = %v", err)
	}
	m.mode = modeMessageBrowse

	m, _ = m.updateMessageBrowse(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	m.messageBrowse.pendingConfirm.cursor = deleteAssistantOnly
	m, _ = m.updateMessageBrowse(tea.KeyMsg{Type: tea.KeyEnter})

	if len(m.messageBrowse.turns) != 1 {
		t.Fatalf("turn count = %d, want 1", len(m.messageBrowse.turns))
	}
	if m.messageBrowse.turns[0].User.Deleted {
		t.Fatalf("user should remain visible")
	}
	if got := len(m.messageBrowse.turns[0].AssistantVersions); got != 0 {
		t.Fatalf("assistant version count = %d, want 0", got)
	}
}

func TestMessageBrowse_DeleteBothRemovesTurnAndUndoRestoresIt(t *testing.T) {
	store := newBrowserTestStore(t)
	m := newBrowserTestModel(t, store)

	const name = "conv"
	seedConversation(t, store, name, []seedTurn{
		{user: "q1", assistant: []string{"a1"}},
	})
	m.tabs[0].autoSaveName = name
	m.tabs[0].history.ReplaceMessages(mustLoadActiveTimeline(t, store, name))
	if err := m.buildBrowserState(); err != nil {
		t.Fatalf("buildBrowserState() error = %v", err)
	}
	m.mode = modeMessageBrowse

	m, _ = m.updateMessageBrowse(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	m.messageBrowse.pendingConfirm.cursor = deleteBothSides
	m, _ = m.updateMessageBrowse(tea.KeyMsg{Type: tea.KeyEnter})

	if len(m.messageBrowse.turns) != 0 {
		t.Fatalf("turn count after delete both = %d, want 0", len(m.messageBrowse.turns))
	}

	m, _ = m.updateMessageBrowse(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("u")})

	if len(m.messageBrowse.turns) != 1 {
		t.Fatalf("turn count after undo = %d, want 1", len(m.messageBrowse.turns))
	}
	if got := m.messageBrowse.turns[0].User.Content; got != "q1" {
		t.Fatalf("restored user = %q, want q1", got)
	}
	if got := m.messageBrowse.turns[0].AssistantVersions[0].Content; got != "a1" {
		t.Fatalf("restored assistant = %q, want a1", got)
	}
}

func TestMessageBrowse_BranchReloadsFreshMessageIDs(t *testing.T) {
	store := newBrowserTestStore(t)
	m := newBrowserTestModel(t, store)

	const name = "conv"
	userID, err := store.AppendMessage(name, chat.Message{Seq: 1, Role: "user", Content: "q1"})
	if err != nil {
		t.Fatalf("AppendMessage(user) error = %v", err)
	}
	assistantID, err := store.AppendMessage(name, chat.Message{Seq: 1, Role: "assistant", Content: "v1", VersionNumber: 1, SnapshotProvider: "openai-compatible", SnapshotModel:    "test-model", SnapshotAPIFormat: "openai-compatible"})
	if err != nil {
		t.Fatalf("AppendMessage(v1) error = %v", err)
	}
	if err := store.InitVersionGroup(assistantID); err != nil {
		t.Fatalf("InitVersionGroup() error = %v", err)
	}
	v2ID, err := store.AppendAssistantVersion(name, assistantID, chat.Message{Seq: 1, Role: "assistant", Content: "v2", VersionNumber: 2, SnapshotProvider: "openai-compatible", SnapshotModel:    "test-model", SnapshotAPIFormat: "openai-compatible"})
	if err != nil {
		t.Fatalf("AppendAssistantVersion(v2) error = %v", err)
	}
	_, err = store.AppendMessage(name, chat.Message{Seq: 2, Role: "user", Content: "q2"})
	if err != nil {
		t.Fatalf("AppendMessage(user2) error = %v", err)
	}
	_, err = store.AppendMessage(name, chat.Message{Seq: 2, Role: "assistant", Content: "a2", VersionNumber: 1, SnapshotProvider: "openai-compatible", SnapshotModel:    "test-model", SnapshotAPIFormat: "openai-compatible"})
	if err != nil {
		t.Fatalf("AppendMessage(a2) error = %v", err)
	}

	m.tabs[0].autoSaveName = name
	m.tabs[0].history.ReplaceMessages(mustLoadActiveTimeline(t, store, name))
	if err := m.buildBrowserState(); err != nil {
		t.Fatalf("buildBrowserState() error = %v", err)
	}
	m.mode = modeMessageBrowse
	m.messageBrowse.turnIdx = 0
	m.messageBrowse.turns[0].PreviewVersion = 1

	m, _ = m.updateMessageBrowse(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("b")})

	if m.tabs[0].autoSaveName == name {
		t.Fatalf("expected branch to switch to a new conversation name")
	}

	branched := mustLoadActiveTimeline(t, store, m.tabs[0].autoSaveName)
	historyMsgs := m.tabs[0].history.Messages()
	if got := branched[0].ID; got == userID {
		t.Fatalf("branched user ID = %d, want fresh row ID", got)
	}
	if got := historyMsgs[0].ID; got != branched[0].ID {
		t.Fatalf("tab history user ID = %d, want %d", got, branched[0].ID)
	}
	if got := historyMsgs[1].ID; got != branched[1].ID {
		t.Fatalf("tab history assistant ID = %d, want %d", got, branched[1].ID)
	}
	if got := historyMsgs[1].ID; got == v2ID {
		t.Fatalf("tab history assistant ID = %d, want fresh row ID", got)
	}
	if got := historyMsgs[1].Content; got != "v2" {
		t.Fatalf("tab history assistant content = %q, want v2", got)
	}
}

func TestMessageBrowse_RegenerateEditedTurnPersistsAndClearsFlags(t *testing.T) {
	store := newBrowserTestStore(t)
	cfg := config.DefaultConfig()
	provider := &streamingStubProvider{
		stubProvider: stubProvider{model: "test-model"},
		response:     "regenerated answer",
	}
	tab, err := newTabSession(cfg, provider, 80)
	if err != nil {
		t.Fatalf("newTabSession() error = %v", err)
	}

	m := Model{
		cfg:       cfg,
		store:     store,
		theme:     DarkTheme,
		tabs:      []TabSession{tab},
		activeTab: 0,
		width:     80,
		height:    24,
		providerFactory: func(apiFormat, baseURL, apiKey, model string) chat.Provider {
			return provider
		},
		modelRegistry: config.ModelRegistry{
				Providers: []config.ProviderEntry{
					{
						Name:    "test-provider",
						BaseURL: "https://example.test/v1",
						APIKey:  "k",
						Models:  []config.ModelEntry{{Name: "test-model", Model: "test-model", APIFormat: "openai-compatible"}},
					},
				},
		},
	}

	seedConversationLegacy(t, store, "conv", []chat.Message{
		{Seq: 1, Role: "user", Content: "old question"},
		{Seq: 1, Role: "assistant", Content: "old answer", VersionNumber: 1, SnapshotProvider: "test-provider", SnapshotModel:    "test-model", SnapshotAPIFormat: "openai-compatible", SnapshotRoleName: "writer", SnapshotRolePrompt: "system prompt"},
	})

	m.tabs[0].autoSaveName = "conv"
	m.tabs[0].history.ReplaceMessages(mustLoadActiveTimeline(t, store, "conv"))
	m.tabs[0].history.SetSystemPrompt("system prompt")
	if err := m.buildBrowserState(); err != nil {
		t.Fatalf("buildBrowserState() error = %v", err)
	}
	m.mode = modeMessageBrowse

	m, _ = m.updateMessageBrowse(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("e")})
	m.messageBrowse.editBuffer = "edited question"
	m.messageBrowse.editDirty = true
	m, _ = m.updateMessageBrowse(tea.KeyMsg{Type: tea.KeyEnter})
	m.messageBrowse.pendingConfirm.cursor = 0
	m, _ = m.updateMessageBrowse(tea.KeyMsg{Type: tea.KeyEnter})

	reloaded := mustLoadActiveTimeline(t, store, "conv")
	if got := reloaded[0].Content; got != "edited question" {
		t.Fatalf("user content = %q, want edited question", got)
	}
	if got := reloaded[1].Content; got != "regenerated answer" {
		t.Fatalf("assistant content = %q, want regenerated answer", got)
	}
	if reloaded[0].EditedAfterGeneration {
		t.Fatalf("user EditedAfterGeneration = true, want false")
	}
	if reloaded[1].StaleAfterUserEdit {
		t.Fatalf("assistant StaleAfterUserEdit = true, want false")
	}
	if m.messageBrowse.editMode {
		t.Fatalf("editMode = true, want false")
	}
	if m.messageBrowse.pendingConfirm.kind != confirmNone {
		t.Fatalf("pendingConfirm = %v, want confirmNone", m.messageBrowse.pendingConfirm.kind)
	}
	if len(provider.received) != 2 {
		t.Fatalf("provider received %d messages, want 2", len(provider.received))
	}
	if provider.received[0].Role != "system" || provider.received[0].Content != "system prompt" {
		t.Fatalf("provider first message = %+v, want system prompt", provider.received[0])
	}
	if provider.received[1].Role != "user" || provider.received[1].Content != "edited question" {
		t.Fatalf("provider second message = %+v, want edited question", provider.received[1])
	}
}

func TestStreamErrorPersistsAsAssistantMessageBrowsableAndDeletable(t *testing.T) {
	store := newBrowserTestStore(t)
	m := newBrowserTestModel(t, store)
	m.modelRegistry = config.ModelRegistry{
		Providers: []config.ProviderEntry{
			{
				Name:    "gateway",
				BaseURL: "https://example.test/v1",
				APIKey:  "secret",
				Models: []config.ModelEntry{
					{Name: "flash", Model: "test-model", APIFormat: "openai-compatible", Temperature: 0.25, MaxTokens: 4096},
				},
			},
		},
	}

	const name = "conv"
	userID, err := store.AppendMessage(name, chat.Message{
		Seq:     1,
		Role:    "user",
		Content: "q1",
	})
	if err != nil {
		t.Fatalf("AppendMessage(user) error = %v", err)
	}

	m.tabs[0].autoSaveName = name
	m.tabs[0].providerConfigName = "gateway"
	m.tabs[0].modelConfigName = "flash"
	m.tabs[0].apiFormat = "openai-compatible"
	m.tabs[0].history.ReplaceMessages(mustLoadActiveTimeline(t, store, name))
	m.tabs[0].streaming = true

	next, _ := m.Update(streamErrMsg{
		TabIdx: 0,
		Err:    errors.New("stream: rate limited"),
	})
	m = next.(Model)

	reloaded := mustLoadActiveTimeline(t, store, name)
	if len(reloaded) != 2 {
		t.Fatalf("active timeline len = %d, want 2", len(reloaded))
	}
	if got := reloaded[1].Role; got != "assistant" {
		t.Fatalf("stored error role = %q, want assistant", got)
	}
	if got := reloaded[1].Seq; got != reloaded[0].Seq {
		t.Fatalf("stored error seq = %d, want %d", got, reloaded[0].Seq)
	}
	if got := reloaded[1].Content; got != "Error: stream: rate limited" {
		t.Fatalf("stored error content = %q", got)
	}

	if err := m.buildBrowserState(); err != nil {
		t.Fatalf("buildBrowserState() error = %v", err)
	}
	if len(m.messageBrowse.turns) != 1 {
		t.Fatalf("turn count = %d, want 1", len(m.messageBrowse.turns))
	}
	turn := m.messageBrowse.turns[0]
	if got := turn.User.ID; got != userID {
		t.Fatalf("turn user ID = %d, want %d", got, userID)
	}
	if len(turn.AssistantVersions) != 1 {
		t.Fatalf("assistant version count = %d, want 1", len(turn.AssistantVersions))
	}
	if got := turn.AssistantVersions[0].Content; got != "Error: stream: rate limited" {
		t.Fatalf("browser assistant content = %q", got)
	}
}

func TestStreamDonePersistsAssistantSnapshotFromBoundModel(t *testing.T) {
	store := newBrowserTestStore(t)
	m := newBrowserTestModel(t, store)
	m.modelRegistry = config.ModelRegistry{
		Providers: []config.ProviderEntry{
			{
				Name:    "gateway",
				BaseURL: "https://example.test/v1",
				APIKey:  "k",
				Models: []config.ModelEntry{
					{Name: "flash", Model: "test-model", APIFormat: "openai-compatible", Temperature: 0.2, MaxTokens: 4096},
				},
			},
		},
	}
	m.tabs[0].history.SetSystemPrompt("")
	m.tabs[0].autoSaveName = "conv"
	m.tabs[0].providerConfigName = "gateway"
	m.tabs[0].modelConfigName = "flash"
	m.tabs[0].apiFormat = "openai-compatible"
	m.activeRole = ""

	userID, err := store.AppendMessage("conv", chat.Message{
		Seq:     1,
		Role:    "user",
		Content: "q1",
	})
	if err != nil {
		t.Fatalf("AppendMessage(user) error = %v", err)
	}
	m.tabs[0].history.ReplaceMessages([]chat.Message{{ID: userID, Seq: 1, Role: "user", Content: "q1"}})
	m.tabs[0].currentResp = "a1"

	next, _ := m.Update(streamDoneMsg{TabIdx: 0})
	m = next.(Model)

	reloaded := mustLoadActiveTimeline(t, store, "conv")
	if len(reloaded) != 2 {
		t.Fatalf("active timeline len = %d, want 2", len(reloaded))
	}
	if got := reloaded[1].SnapshotProvider; got != "gateway" {
		t.Fatalf("SnapshotProvider = %q, want gateway", got)
	}
	if got := reloaded[1].SnapshotModel; got != "test-model" {
		t.Fatalf("SnapshotModel = %q, want test-model", got)
	}
	if !reloaded[1].HasGenerationSnapshot() {
		t.Fatalf("HasGenerationSnapshot = false, want true")
	}
}

func TestMessageBrowse_RegeneratePreviewRequiresRegisteredCurrentTabProvider(t *testing.T) {
	store := newBrowserTestStore(t)
	m := newBrowserTestModel(t, store)

	const name = "conv"
	seedConversationLegacy(t, store, name, []chat.Message{
		{Seq: 1, Role: "user", Content: "q1"},
		{Seq: 1, Role: "assistant", Content: "v1", VersionNumber: 1, SnapshotProvider: "openai-compatible", SnapshotModel:    "test-model", SnapshotAPIFormat: "openai-compatible", SnapshotRolePrompt: ""},
	})

	m.tabs[0].autoSaveName = name
	m.tabs[0].history.ReplaceMessages(mustLoadActiveTimeline(t, store, name))
	if err := m.buildBrowserState(); err != nil {
		t.Fatalf("buildBrowserState() error = %v", err)
	}
	m.mode = modeMessageBrowse

	m, _ = m.updateMessageBrowse(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")})

	reloaded := mustLoadActiveTimeline(t, store, name)
	if got := reloaded[1].Content; got != "v1" {
		t.Fatalf("assistant content = %q, want unchanged v1", got)
	}
	if got := m.statusMsg; got != `Regenerate failed: current session provider "openai-compatible" not found` {
		t.Fatalf("statusMsg = %q, want current session provider error", got)
	}
}

func TestMessageBrowse_RegenerateCurrentPreviewVersionUsesCurrentTabModelAndUpdatesSnapshot(t *testing.T) {
	store := newBrowserTestStore(t)
	tab, err := newTabSession(config.DefaultConfig(), &stubProvider{model: "live-model"}, 80)
	if err != nil {
		t.Fatalf("newTabSession() error = %v", err)
	}

	m := Model{
		cfg:       config.DefaultConfig(),
		store:     store,
		theme:     DarkTheme,
		tabs:      []TabSession{tab},
		activeTab: 0,
		width:     80,
		height:    24,
		providerFactory: func(apiFormat, baseURL, apiKey, model string) chat.Provider {
			if apiFormat != "openai-compatible" {
				t.Fatalf("apiFormat = %q, want openai-compatible", apiFormat)
			}
			if baseURL != "https://gateway.example/v1" {
				t.Fatalf("baseURL = %q, want https://gateway.example/v1", baseURL)
			}
			if apiKey != "gateway-key" {
				t.Fatalf("apiKey = %q, want gateway-key", apiKey)
			}
			if model != "live-model" {
				t.Fatalf("model = %q, want live-model", model)
			}
			return &streamingStubProvider{
				stubProvider: stubProvider{model: model},
				response:     "rewritten by current tab model",
			}
		},
		modelRegistry: config.ModelRegistry{
				Providers: []config.ProviderEntry{
					{
						Name:    "gateway",
						BaseURL: "https://gateway.example/v1",
						APIKey:  "gateway-key",
						Models:  []config.ModelEntry{{Name: "live", Model: "live-model", APIFormat: "openai-compatible", Temperature: 0.3, MaxTokens: 4096}},
					},
					{
						Name:    "anthropic-direct",
						BaseURL: "https://api.anthropic.com",
						APIKey:  "k",
						Models:  []config.ModelEntry{{Name: "sonnet", Model: "claude-sonnet-4", APIFormat: "anthropic"}},
					},
				},
		},
	}

	const name = "conv"
	seedConversationLegacy(t, store, name, []chat.Message{
		{Seq: 1, Role: "user", Content: "q1"},
		{Seq: 1, Role: "assistant", Content: "v1", VersionNumber: 1, SnapshotProvider: "anthropic-direct", SnapshotModel: "claude-sonnet-4", SnapshotAPIFormat: "anthropic", SnapshotRoleName: "writer", SnapshotRolePrompt: "be concise"},
	})

	m.tabs[0].autoSaveName = name
	m.tabs[0].providerConfigName = "gateway"
	m.tabs[0].modelConfigName = "live"
	m.tabs[0].apiFormat = "openai-compatible"
	m.tabs[0].history.ReplaceMessages(mustLoadActiveTimeline(t, store, name))
	m.tabs[0].history.SetSystemPrompt("new prompt")
	m.activeRole = "reviewer"
	if err := m.buildBrowserState(); err != nil {
		t.Fatalf("buildBrowserState() error = %v", err)
	}
	m.mode = modeMessageBrowse

	m, _ = m.updateMessageBrowse(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")})

	reloaded := mustLoadActiveTimeline(t, store, name)
	if got := reloaded[1].Content; got != "rewritten by current tab model" {
		t.Fatalf("assistant content = %q, want rewritten by current tab model", got)
	}
	if got := reloaded[1].SnapshotProvider; got != "gateway" {
		t.Fatalf("SnapshotProvider = %q, want gateway", got)
	}
	if got := reloaded[1].SnapshotModel; got != "live-model" {
		t.Fatalf("SnapshotModel = %q, want live-model", got)
	}
	if got := reloaded[1].SnapshotRoleName; got != "reviewer" {
		t.Fatalf("SnapshotRoleName = %q, want reviewer", got)
	}
	if got := reloaded[1].SnapshotRolePrompt; got != "new prompt" {
		t.Fatalf("SnapshotRolePrompt = %q, want new prompt", got)
	}
}

func TestMessageBrowse_RegenerateCurrentPreviewVersionRequiresExactTabModelBinding(t *testing.T) {
	store := newBrowserTestStore(t)
	tab, err := newTabSession(config.DefaultConfig(), &stubProvider{model: "live-model"}, 80)
	if err != nil {
		t.Fatalf("newTabSession() error = %v", err)
	}

	m := Model{
		cfg:       config.DefaultConfig(),
		store:     store,
		theme:     DarkTheme,
		tabs:      []TabSession{tab},
		activeTab: 0,
		width:     80,
		height:    24,
		modelRegistry: config.ModelRegistry{
			Providers: []config.ProviderEntry{
				{
					Name:    "gateway",
					BaseURL: "https://gateway.example/v1",
					APIKey:  "gateway-key",
					Models:  []config.ModelEntry{{Name: "live", Model: "live-model", APIFormat: "openai-compatible", Temperature: 0.3, MaxTokens: 4096}},
				},
			},
		},
	}

	const name = "conv"
	seedConversationLegacy(t, store, name, []chat.Message{
		{Seq: 1, Role: "user", Content: "q1"},
		{Seq: 1, Role: "assistant", Content: "v1", VersionNumber: 1, SnapshotProvider: "gateway", SnapshotModel: "live-model", SnapshotAPIFormat: "openai-compatible"},
	})

	m.tabs[0].autoSaveName = name
	m.tabs[0].providerConfigName = "gateway"
	m.tabs[0].modelConfigName = ""
	m.tabs[0].apiFormat = "openai-compatible"
	m.tabs[0].history.ReplaceMessages(mustLoadActiveTimeline(t, store, name))
	if err := m.buildBrowserState(); err != nil {
		t.Fatalf("buildBrowserState() error = %v", err)
	}
	m.mode = modeMessageBrowse

	m, _ = m.updateMessageBrowse(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")})

	reloaded := mustLoadActiveTimeline(t, store, name)
	if got := reloaded[1].Content; got != "v1" {
		t.Fatalf("assistant content = %q, want unchanged v1", got)
	}
	if got := m.statusMsg; got != "Regenerate failed: current session model is not selected" {
		t.Fatalf("statusMsg = %q, want missing model binding error", got)
	}
}

func TestMessageBrowse_EditRegenerateAllVersionsRewritesEachVersionOrError(t *testing.T) {
	store := newBrowserTestStore(t)
	cfg := config.DefaultConfig()
	tab, err := newTabSession(cfg, &stubProvider{model: "unused-live-model"}, 80)
	if err != nil {
		t.Fatalf("newTabSession() error = %v", err)
	}

	m := Model{
		cfg:       cfg,
		store:     store,
		theme:     DarkTheme,
		tabs:      []TabSession{tab},
		activeTab: 0,
		width:     80,
		height:    24,
		providerFactory: func(apiFormat, baseURL, apiKey, model string) chat.Provider {
			switch model {
			case "claude-sonnet-4":
				return &streamingStubProvider{
					stubProvider: stubProvider{model: model},
					response:     "claude rewrite",
				}
			case "gemini-3-flash-preview":
				return &erroringProvider{model: model, err: errors.New("429")}
			default:
				return &streamingStubProvider{
					stubProvider: stubProvider{model: model},
					response:     "fallback rewrite",
				}
			}
		},
		modelRegistry: config.ModelRegistry{
				Providers: []config.ProviderEntry{
					{
						Name:    "anthropic-direct",
						BaseURL: "https://api.anthropic.com",
						APIKey:  "k1",
						Models:  []config.ModelEntry{{Name: "sonnet", Model: "claude-sonnet-4", APIFormat: "anthropic"}},
					},
					{
						Name:    "gateway",
						BaseURL: "https://example.test/v1",
						APIKey:  "k2",
						Models:  []config.ModelEntry{{Name: "flash", Model: "gemini-3-flash-preview", APIFormat: "openai-compatible"}},
					},
				},
		},
	}

	const name = "conv"
	seedConversationLegacy(t, store, name, []chat.Message{
		{Seq: 1, Role: "user", Content: "old question"},
		{Seq: 1, Role: "assistant", Content: "old a", VersionNumber: 1, SnapshotProvider: "anthropic-direct", SnapshotModel: "claude-sonnet-4", SnapshotAPIFormat: "anthropic", SnapshotRoleName: "writer", SnapshotRolePrompt: "be concise"},
		{Seq: 1, Role: "assistant", Content: "old b", VersionNumber: 2, SnapshotProvider: "gateway", SnapshotModel:      "gemini-3-flash-preview", SnapshotAPIFormat:  "openai-compatible", SnapshotRoleName: "writer", SnapshotRolePrompt: "be concise"},
	})

	m.tabs[0].autoSaveName = name
	m.tabs[0].history.ReplaceMessages(mustLoadActiveTimeline(t, store, name))
	if err := m.buildBrowserState(); err != nil {
		t.Fatalf("buildBrowserState() error = %v", err)
	}
	m.mode = modeMessageBrowse

	m, _ = m.updateMessageBrowse(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("e")})
	m.messageBrowse.editBuffer = "edited question"
	m.messageBrowse.editDirty = true
	m, _ = m.updateMessageBrowse(tea.KeyMsg{Type: tea.KeyEnter})
	m.messageBrowse.pendingConfirm.cursor = 0
	m, _ = m.updateMessageBrowse(tea.KeyMsg{Type: tea.KeyEnter})

	reloaded, err := store.LoadBrowseMessages(name)
	if err != nil {
		t.Fatalf("LoadBrowseMessages() error = %v", err)
	}
	if got := reloaded[0].Content; got != "edited question" {
		t.Fatalf("user content = %q, want edited question", got)
	}
	if got := reloaded[1].Content; got != "claude rewrite" {
		t.Fatalf("assistant v1 content = %q, want claude rewrite", got)
	}
	if got := reloaded[2].Content; got != "Error: 429" {
		t.Fatalf("assistant v2 content = %q, want Error: 429", got)
	}
}

func TestMessageBrowse_NewVersionUsesSelectedModelParametersWithoutChangingTabBinding(t *testing.T) {
	store := newBrowserTestStore(t)
	m := newBrowserTestModel(t, store)
	seedConversation(t, store, "conv", []seedTurn{
		{user: "q1", assistant: []string{"a1"}},
	})

	m.tabs[0].autoSaveName = "conv"
	m.tabs[0].providerConfigName = "default-gateway"
	m.tabs[0].modelConfigName = "default-model"
	m.tabs[0].history.ReplaceMessages(mustLoadActiveTimeline(t, store, "conv"))
	if err := m.buildBrowserState(); err != nil {
		t.Fatalf("buildBrowserState() error = %v", err)
	}

	var provider *streamingStubProvider
	m.providerFactory = func(apiFormat, baseURL, apiKey, model string) chat.Provider {
		provider = &streamingStubProvider{
			stubProvider: stubProvider{model: model},
			response:     "new version",
		}
		return provider
	}

	selectedProvider := config.ProviderEntry{
		Name:    "gateway",
		BaseURL: "https://example.test/v1",
		APIKey:  "secret",
	}
	selectedModel := config.ModelEntry{
		Name:            "flash",
		Model:           "gemini-3-flash-preview",
		APIFormat:       "openai-compatible",
		Temperature:     0.15,
		MaxTokens:       4096,
		ReasoningEffort: "low",
		BudgetTokens:    256,
	}

	m, _ = m.appendAssistantVersionFromSelection(selectedProvider, selectedModel)

	if provider == nil {
		t.Fatal("providerFactory was not called")
	}
	if provider.temperature != 0.15 {
		t.Fatalf("temperature = %f, want 0.15", provider.temperature)
	}
	if provider.maxTokens != 4096 {
		t.Fatalf("maxTokens = %d, want 4096", provider.maxTokens)
	}
	if provider.reasoningEffort != "low" {
		t.Fatalf("reasoningEffort = %q, want low", provider.reasoningEffort)
	}
	if provider.budgetTokens != 256 {
		t.Fatalf("budgetTokens = %d, want 256", provider.budgetTokens)
	}
	if got := m.tabs[0].providerConfigName; got != "default-gateway" {
		t.Fatalf("providerConfigName = %q, want default-gateway", got)
	}
	if got := m.tabs[0].modelConfigName; got != "default-model" {
		t.Fatalf("modelConfigName = %q, want default-model", got)
	}
}

func TestMessageBrowse_GCreatesAssistantVersionFromSelectedRegistryModel(t *testing.T) {
	store := newBrowserTestStore(t)
	m := newBrowserTestModel(t, store)
	m.providerFactory = func(apiFormat, baseURL, apiKey, model string) chat.Provider {
		return &streamingStubProvider{
			stubProvider: stubProvider{model: model},
			response:     "new version from registry",
		}
	}
	m.modelRegistry = config.ModelRegistry{
		Providers: []config.ProviderEntry{
			{
				Name:    "gateway",
				BaseURL: "https://example.test/v1",
				APIKey:  "k",
				Models: []config.ModelEntry{
					{Name: "flash", Model: "gemini-3-flash-preview", APIFormat: "openai-compatible", Temperature: 0.25, MaxTokens: 4096},
				},
			},
		},
	}
	_, err := store.AppendMessage("conv", chat.Message{Seq: 1, Role: "user", Content: "q1"})
	if err != nil {
		t.Fatalf("AppendMessage(user) error = %v", err)
	}
	assistantID, err := store.AppendMessage("conv", chat.Message{
		Seq:                1,
		Role:               "assistant",
		Content:            "v1",
		VersionNumber:      1,
		SnapshotProvider:   "gateway",
		SnapshotModel:      "gemini-3-flash-preview", SnapshotAPIFormat:  "openai-compatible",
		SnapshotRoleName:   "writer",
		SnapshotRolePrompt: "be concise",
	})
	if err != nil {
		t.Fatalf("AppendMessage(assistant) error = %v", err)
	}
	if err := store.InitVersionGroup(assistantID); err != nil {
		t.Fatalf("InitVersionGroup() error = %v", err)
	}
	m.tabs[0].autoSaveName = "conv"
	m.tabs[0].history.ReplaceMessages(mustLoadActiveTimeline(t, store, "conv"))
	m.tabs[0].history.SetSystemPrompt("be concise")
	m.activeRole = "writer"
	if err := m.buildBrowserState(); err != nil {
		t.Fatalf("buildBrowserState() error = %v", err)
	}
	m.mode = modeMessageBrowse

	m, _ = m.updateMessageBrowse(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("g")})
	if m.mode != modeModelSelector {
		t.Fatalf("mode = %v, want modeModelSelector", m.mode)
	}
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRight})
	m = next.(Model)
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(Model)

	reloaded, err := store.LoadBrowseMessages("conv")
	if err != nil {
		t.Fatalf("LoadBrowseMessages() error = %v", err)
	}
	if len(reloaded) != 3 {
		t.Fatalf("browse len = %d, want 3", len(reloaded))
	}
	if got := reloaded[2].Content; got != "new version from registry" {
		t.Fatalf("new version content = %q, want new version from registry", got)
	}
	if got := reloaded[2].SnapshotProvider; got != "gateway" {
		t.Fatalf("SnapshotProvider = %q, want gateway", got)
	}
	if got := reloaded[2].SnapshotModel; got != "gemini-3-flash-preview" {
		t.Fatalf("SnapshotModel = %q, want gemini-3-flash-preview", got)
	}
}

func TestMessageBrowse_EditModeAcceptsTypingAndEscCancelsEdit(t *testing.T) {
	m := newEditableBrowseModel(t)
	m.mode = modeMessageBrowse

	m, _ = m.updateMessageBrowse(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("e")})
	if !m.messageBrowse.editMode {
		t.Fatal("editMode = false, want true")
	}

	m, _ = m.updateMessageBrowse(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
	if got := m.messageBrowse.editBuffer; got != "q1x" {
		t.Fatalf("editBuffer = %q, want q1x", got)
	}

	m, _ = m.updateMessageBrowse(tea.KeyMsg{Type: tea.KeyEsc})
	if m.messageBrowse.editMode {
		t.Fatal("editMode = true, want false after esc")
	}
	if m.mode != modeMessageBrowse {
		t.Fatalf("mode = %v, want modeMessageBrowse", m.mode)
	}
}

func TestMessageBrowse_CompareEscAndVReturnToMessageMode(t *testing.T) {
	m := newVersionedBrowseModel(t)
	m.mode = modeMessageBrowse
	m.messageBrowse.mode = browseModeCompare

	m, _ = m.updateMessageBrowse(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("v")})
	if got := m.messageBrowse.mode; got != browseModeMessage {
		t.Fatalf("browse mode after v = %v, want browseModeMessage", got)
	}

	m.messageBrowse.mode = browseModeCompare
	m, _ = m.updateMessageBrowse(tea.KeyMsg{Type: tea.KeyEsc})
	if m.mode != modeMessageBrowse {
		t.Fatalf("mode after esc = %v, want modeMessageBrowse", m.mode)
	}
	if got := m.messageBrowse.mode; got != browseModeMessage {
		t.Fatalf("browse mode after esc = %v, want browseModeMessage", got)
	}
}
