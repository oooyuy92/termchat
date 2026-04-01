# Message Versioning Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add assistant reply version generation, preview, switching, comparison, and user-message editing to `termchat` while keeping the main chat timeline linear and active-version-only.

**Architecture:** Replace replace-all conversation persistence with a row-based SQLite model where assistant versions are sibling rows in `messages`. Keep `chat.History` as the in-memory active timeline with stable row metadata, and implement preview/apply flows inside the upgraded double-`Esc` browser. Browser actions mutate the stored conversation through explicit storage operations, then reload the active timeline back into the tab session.

**Tech Stack:** Go, Bubble Tea, Lip Gloss, Glamour, SQLite (`modernc.org/sqlite`), Go test

---

## File Map

### Files to modify

- `internal/chat/history.go`
  - extend `chat.Message` with persistent row metadata and browser-facing flags
  - add helpers for active timeline access without introducing a tree model
- `internal/chat/history_test.go`
  - cover metadata-bearing messages and active timeline helpers
- `internal/storage/conversation.go`
  - replace overwrite-style persistence with row-based conversation/version operations
- `internal/storage/conversation_test.go`
  - cover schema behavior, version groups, branch/truncate, and edit flags
- `internal/ui/model.go`
  - add browser submode, per-pane scroll state, compare-card focus, edit state, and confirmation state
- `internal/ui/update.go`
  - persist user/assistant rows incrementally
  - open browser on the latest assistant turn
  - reload active timeline after browser mutations
- `internal/ui/messagebrowse.go`
  - replace single-message browser with turn browser, compare mode, preview/apply logic, edit flow, and confirmation flow
- `internal/ui/view.go`
  - render version count badges in the main chat timeline
- `internal/ui/resumepicker.go`
  - load active timeline rows with metadata into `chat.History`
- `internal/export/export.go`
  - export only active timeline rows while ignoring hidden assistant versions

### Files to create

- `internal/ui/messagebrowse_test.go`
  - targeted browser state and key-flow tests
- `internal/ui/messagebrowse_view_test.go`
  - render tests for two-pane message mode and compare mode

### Existing files to check while implementing

- `internal/ui/tabsession.go`
- `internal/ui/view_quote_wrap_test.go`
- `internal/ui/update_mouse_test.go`
- `main_test.go`

## Task 1: Extend `chat.Message` and `chat.History` for active timeline metadata

**Files:**
- Modify: `internal/chat/history.go`
- Test: `internal/chat/history_test.go`

- [ ] **Step 1: Write the failing metadata tests**

Add these tests to `internal/chat/history_test.go`:

```go
func TestHistory_MessagesPreserveMetadata(t *testing.T) {
	h := NewHistory()
	h.Add(Message{
		ID:                   42,
		Seq:                  7,
		Role:                 "assistant",
		Content:              "v2 reply",
		VersionGroupID:       40,
		VersionNumber:        2,
		TotalVersions:        4,
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
```

- [ ] **Step 2: Run the targeted tests and confirm failure**

Run:

```bash
go test ./internal/chat -run 'TestHistory_(MessagesPreserveMetadata|ReplaceMessages)$'
```

Expected:

- compile failure because `Message` lacks the new fields and `History` lacks `ReplaceMessages`

- [ ] **Step 3: Add persistent metadata fields and replacement helper**

Update `internal/chat/history.go` so `Message` can carry row IDs and browser flags, and add a bulk replacement helper:

```go
type Message struct {
	ID                    int64       `json:"id"`
	Seq                   int         `json:"seq"`
	Role                  string      `json:"role"`
	Content               string      `json:"content"`
	Images                []ImageData `json:"-"`
	VersionGroupID        int64       `json:"version_group_id"`
	VersionNumber         int         `json:"version_number"`
	TotalVersions         int         `json:"total_versions"`
	EditedAfterGeneration bool        `json:"edited_after_generation"`
	StaleAfterUserEdit    bool        `json:"stale_after_user_edit"`
}

func (h *History) ReplaceMessages(messages []Message) {
	h.messages = append(h.messages[:0], messages...)
}
```

Keep `Add`, `Clear`, `DeleteAt`, and `Truncate` working with the extended `Message`.

- [ ] **Step 4: Run the chat history tests until they pass**

Run:

```bash
go test ./internal/chat
```

Expected:

- all tests in `./internal/chat` pass

- [ ] **Step 5: Commit the metadata-only change**

Run:

```bash
git add internal/chat/history.go internal/chat/history_test.go
git commit -m "feat(chat): add version metadata to history messages"
```

## Task 2: Replace overwrite persistence with row-based version-aware storage

**Files:**
- Modify: `internal/storage/conversation.go`
- Test: `internal/storage/conversation_test.go`

- [ ] **Step 1: Write failing storage tests for active timeline and version groups**

Append these tests to `internal/storage/conversation_test.go`:

```go
func TestConversationStore_InsertAndLoadActiveTimeline(t *testing.T) {
	store := newTestStore(t)

	name := "conv"
	userID, err := store.AppendMessage(name, chat.Message{Seq: 1, Role: "user", Content: "question"})
	if err != nil {
		t.Fatalf("AppendMessage(user) error = %v", err)
	}
	replyID, err := store.AppendMessage(name, chat.Message{
		Seq:           1,
		Role:          "assistant",
		Content:       "answer v1",
		VersionNumber: 1,
	})
	if err != nil {
		t.Fatalf("AppendMessage(assistant) error = %v", err)
	}
	if err := store.InitVersionGroup(replyID); err != nil {
		t.Fatalf("InitVersionGroup() error = %v", err)
	}
	if _, err := store.AppendAssistantVersion(name, replyID, chat.Message{
		Seq:           1,
		Role:          "assistant",
		Content:       "answer v2",
		VersionNumber: 2,
	}); err != nil {
		t.Fatalf("AppendAssistantVersion() error = %v", err)
	}

	msgs, err := store.LoadActiveTimeline(name)
	if err != nil {
		t.Fatalf("LoadActiveTimeline() error = %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("len = %d, want 2", len(msgs))
	}
	if msgs[0].ID != userID {
		t.Fatalf("user ID = %d, want %d", msgs[0].ID, userID)
	}
	if msgs[1].Content != "answer v1" {
		t.Fatalf("assistant content = %q, want answer v1", msgs[1].Content)
	}
	if msgs[1].TotalVersions != 2 {
		t.Fatalf("TotalVersions = %d, want 2", msgs[1].TotalVersions)
	}
}

func TestConversationStore_SetActiveVersionAndTruncateAfterSeq(t *testing.T) {
	store := newTestStore(t)

	name := "conv"
	_, _ = store.AppendMessage(name, chat.Message{Seq: 1, Role: "user", Content: "q1"})
	replyID, _ := store.AppendMessage(name, chat.Message{Seq: 1, Role: "assistant", Content: "v1", VersionNumber: 1})
	_, _ = store.AppendMessage(name, chat.Message{Seq: 2, Role: "user", Content: "q2"})
	_, _ = store.AppendMessage(name, chat.Message{Seq: 2, Role: "assistant", Content: "a2", VersionNumber: 1})

	if err := store.InitVersionGroup(replyID); err != nil {
		t.Fatalf("InitVersionGroup() error = %v", err)
	}
	if _, err := store.AppendAssistantVersion(name, replyID, chat.Message{Seq: 1, Role: "assistant", Content: "v2", VersionNumber: 2}); err != nil {
		t.Fatalf("AppendAssistantVersion() error = %v", err)
	}
	if err := store.SetActiveVersion(name, replyID, 2); err != nil {
		t.Fatalf("SetActiveVersion() error = %v", err)
	}
	if err := store.TruncateAfterSeq(name, 1); err != nil {
		t.Fatalf("TruncateAfterSeq() error = %v", err)
	}

	msgs, err := store.LoadActiveTimeline(name)
	if err != nil {
		t.Fatalf("LoadActiveTimeline() error = %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("len = %d, want 2", len(msgs))
	}
	if msgs[1].Content != "v2" {
		t.Fatalf("assistant content = %q, want v2", msgs[1].Content)
	}
}
```

- [ ] **Step 2: Run the storage tests and confirm failure**

Run:

```bash
go test ./internal/storage -run 'TestConversationStore_(InsertAndLoadActiveTimeline|SetActiveVersionAndTruncateAfterSeq)$'
```

Expected:

- compile failure because the row-based storage methods do not exist

- [ ] **Step 3: Replace schema and add row-based storage APIs**

Refactor `internal/storage/conversation.go` around these methods:

```go
func (s *Store) AppendMessage(name string, msg chat.Message) (int64, error)
func (s *Store) InitVersionGroup(messageID int64) error
func (s *Store) AppendAssistantVersion(name string, anchorID int64, msg chat.Message) (int64, error)
func (s *Store) LoadActiveTimeline(name string) ([]chat.Message, error)
func (s *Store) ListVersions(name string, anchorID int64) ([]chat.Message, error)
func (s *Store) SetActiveVersion(name string, versionGroupID int64, versionNumber int) error
func (s *Store) TruncateAfterSeq(name string, seq int) error
func (s *Store) UpdateMessageContent(messageID int64, content string) error
func (s *Store) MarkTurnEdited(userID, assistantID int64) error
func (s *Store) ClearTurnEdited(userID, assistantID int64) error
```

Use this schema inside `migrate`:

```sql
CREATE TABLE IF NOT EXISTS messages (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	conversation_id INTEGER NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
	role TEXT NOT NULL,
	content TEXT NOT NULL,
	seq INTEGER NOT NULL,
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	version_group_id INTEGER,
	version_number INTEGER NOT NULL DEFAULT 1,
	is_active_version INTEGER NOT NULL DEFAULT 1,
	edited_after_generation INTEGER NOT NULL DEFAULT 0,
	stale_after_user_edit INTEGER NOT NULL DEFAULT 0
);
```

When loading the active timeline, compute `TotalVersions` like this:

```sql
CASE
	WHEN m.role != 'assistant' THEN 1
	WHEN m.version_group_id IS NULL THEN 1
	ELSE (
		SELECT COUNT(*)
		FROM messages mv
		WHERE mv.version_group_id = m.version_group_id
	)
END AS total_versions
```

- [ ] **Step 4: Update list/search/export-facing loaders to read active timeline only**

Keep these public methods working by delegating to the new row-based queries:

```go
func (s *Store) Load(name string) ([]chat.Message, error) {
	return s.LoadActiveTimeline(name)
}
```

For `LoadAllForSearch` and `ListWithDate`, only include:

- user rows
- active assistant rows

This preserves history picker and search behavior without leaking hidden versions.

- [ ] **Step 5: Run the full storage test suite until it passes**

Run:

```bash
go test ./internal/storage
```

Expected:

- all `internal/storage` tests pass

- [ ] **Step 6: Commit the storage rewrite**

Run:

```bash
git add internal/storage/conversation.go internal/storage/conversation_test.go
git commit -m "feat(storage): add version-aware conversation store"
```

## Task 3: Persist chat flow incrementally and reload active timeline after browser mutations

**Files:**
- Modify: `internal/ui/update.go`
- Modify: `internal/ui/resumepicker.go`
- Modify: `internal/export/export.go`
- Check: `internal/ui/tabsession.go`

- [ ] **Step 1: Write failing persistence-focused UI tests**

Create `internal/ui/messagebrowse_test.go` with these helpers and the first failing test:

```go
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

func seedConversation(t *testing.T, store *storage.Store, name string, msgs []chat.Message) {
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

func TestMessageBrowseApplyReloadsActiveTimeline(t *testing.T) {
	store := newBrowserTestStore(t)
	model := newBrowserTestModel(t, store)

	seedConversation(t, store, "conv", []chat.Message{
		{Seq: 1, Role: "user", Content: "q1"},
		{Seq: 1, Role: "assistant", Content: "v1", VersionGroupID: 2, VersionNumber: 1, TotalVersions: 2},
	})

	model.tabs[0].autoSaveName = "conv"
	model.tabs[0].history.ReplaceMessages(mustLoadActiveTimeline(t, store, "conv"))
	model.mode = modeMessageBrowse
	model.messageBrowse.turns = []browseTurn{{
		User: chat.Message{ID: 1, Seq: 1, Role: "user", Content: "q1"},
		AssistantVersions: []chat.Message{
			{ID: 2, Seq: 1, Role: "assistant", Content: "v1", VersionGroupID: 2, VersionNumber: 1, TotalVersions: 2},
			{ID: 3, Seq: 1, Role: "assistant", Content: "v2", VersionGroupID: 2, VersionNumber: 2, TotalVersions: 2},
		},
		ActiveVersion:  0,
		PreviewVersion: 0,
	}}

	model, _ = model.updateMessageBrowse(tea.KeyMsg{Type: tea.KeyRight})
	if model.messageBrowse.turns[0].PreviewVersion == model.messageBrowse.turns[0].ActiveVersion {
		t.Fatalf("preview should differ from active after right arrow")
	}
}
```

This intentionally references the new browser state that does not exist yet.

- [ ] **Step 2: Run the targeted UI test and confirm failure**

Run:

```bash
go test ./internal/ui -run 'TestMessageBrowseApplyReloadsActiveTimeline$'
```

Expected:

- compile failure because `messageBrowse` state and browser-aware reload plumbing do not exist

- [ ] **Step 3: Insert stored user/assistant rows during the normal send flow**

Update `internal/ui/update.go` so pressing `Enter` in chat mode persists the user row before streaming:

```go
userMsg := chat.Message{Seq: nextSeq(tab.history.Messages()), Role: "user", Content: input, Images: tab.pendingImages}
userID, err := m.store.AppendMessage(tab.autoSaveName, userMsg)
if err != nil {
	m.statusMsg = "Save failed: " + err.Error()
	return m, nil
}
userMsg.ID = userID
tab.history.Add(userMsg)
```

When streaming finishes, append the assistant row through the store before adding it to history:

```go
func nextSeq(msgs []chat.Message) int {
	if len(msgs) == 0 {
		return 1
	}
	return msgs[len(msgs)-1].Seq + 1
}

assistantMsg := chat.Message{
	Seq:           userMsg.Seq,
	Role:          "assistant",
	Content:       t.currentResp,
	VersionNumber: 1,
}
assistantID, err := m.store.AppendMessage(t.autoSaveName, assistantMsg)
if err == nil {
	assistantMsg.ID = assistantID
}
t.history.Add(assistantMsg)
```

- [ ] **Step 4: Replace history clearing/resume/export paths with active-timeline reload helpers**

Add a helper in `internal/ui/update.go`:

```go
func (m *Model) reloadActiveTimeline(tabIdx int) error {
	name := m.tabs[tabIdx].autoSaveName
	msgs, err := m.store.LoadActiveTimeline(name)
	if err != nil {
		return err
	}
	m.tabs[tabIdx].history.ReplaceMessages(msgs)
	m.tabs[tabIdx].viewport.SetContent(m.buildChatContent())
	m.tabs[tabIdx].viewport.GotoBottom()
	m.tabs[tabIdx].chatFollowBottom = true
	return nil
}
```

Use it from:

- browser apply/delete/branch flows
- `updateResumeMode`
- export path when the active conversation is not the in-memory one

- [ ] **Step 5: Stop using replace-all autosave for normal chat flow**

Remove or shrink the old autosave path:

```go
func (m Model) autoSaveCmd(tabIdx int) tea.Cmd {
	return func() tea.Msg {
		return autoSavedMsg{TabIdx: tabIdx}
	}
}
```

Keep the message so the status flow remains stable, but do not call `store.Save()` anymore during normal chat flow.

- [ ] **Step 6: Run UI and export tests**

Run:

```bash
go test ./internal/ui ./internal/export
```

Expected:

- existing UI/export tests stay green after the persistence rewrite

- [ ] **Step 7: Commit the incremental persistence integration**

Run:

```bash
git add internal/ui/update.go internal/ui/resumepicker.go internal/export/export.go
git commit -m "feat(ui): persist active timeline incrementally"
```

## Task 4: Introduce browser state structs and render the two-pane message mode

**Files:**
- Modify: `internal/ui/model.go`
- Modify: `internal/ui/messagebrowse.go`
- Create: `internal/ui/messagebrowse_view_test.go`

- [ ] **Step 1: Write failing render tests for the two-pane browser**

Create `internal/ui/messagebrowse_view_test.go` with:

```go
func newMessageBrowseViewModel(t *testing.T) Model {
	t.Helper()
	cfg := config.DefaultConfig()
	tab, err := newTabSession(cfg, &stubProvider{model: "test-model"}, 80)
	if err != nil {
		t.Fatalf("newTabSession() error = %v", err)
	}
	return Model{
		cfg:       cfg,
		theme:     DarkTheme,
		tabs:      []TabSession{tab},
		activeTab: 0,
		width:     80,
		height:    24,
	}
}

func TestViewMessageBrowse_RendersTurnPanes(t *testing.T) {
	m := newMessageBrowseViewModel(t)
	m.mode = modeMessageBrowse
	m.messageBrowse.mode = browseModeMessage
	m.messageBrowse.turns = []browseTurn{
		{
			User: chat.Message{Role: "user", Content: "left pane"},
			AssistantVersions: []chat.Message{
				{Role: "assistant", Content: "right pane", VersionNumber: 1, TotalVersions: 2},
			},
		},
	}

	out := m.viewMessageBrowse()
	if !strings.Contains(out, "User") {
		t.Fatalf("output missing User header: %q", out)
	}
	if !strings.Contains(out, "Assistant v1/2") {
		t.Fatalf("output missing Assistant version header: %q", out)
	}
}
```

- [ ] **Step 2: Run the render test and confirm failure**

Run:

```bash
go test ./internal/ui -run 'TestViewMessageBrowse_RendersTurnPanes$'
```

Expected:

- compile failure because `messageBrowse`, `browseModeMessage`, and `browseTurn` do not exist

- [ ] **Step 3: Add dedicated browser state to `Model`**

In `internal/ui/model.go`, add focused state instead of overloading `browseCursor`:

```go
type browseMode int

const (
	browseModeMessage browseMode = iota
	browseModeCompare
)

type browseTurn struct {
	User              chat.Message
	AssistantVersions []chat.Message
	ActiveVersion     int
	PreviewVersion    int
}

func (t browseTurn) VersionCount() int {
	return len(t.AssistantVersions)
}

type messageBrowseState struct {
	mode                browseMode
	turns               []browseTurn
	turnIdx             int
	leftScroll          int
	rightScroll         int
	compareCardIdx      int
	compareCardScrolls  map[int]int
}
```

Replace `browseCursor` in `Model` with:

```go
	messageBrowse messageBrowseState
```

- [ ] **Step 4: Rebuild `viewMessageBrowse()` around turn panes**

Update `internal/ui/messagebrowse.go` to render:

```go
left := renderBrowsePane("User", turn.User.Content, m.messageBrowse.leftScroll, paneWidth)
rightMsg := turn.AssistantVersions[turn.PreviewVersion]
rightTitle := fmt.Sprintf("Assistant v%d/%d", rightMsg.VersionNumber, rightMsg.TotalVersions)
right := renderBrowsePane(rightTitle, rightMsg.Content, m.messageBrowse.rightScroll, paneWidth)

body := lipgloss.JoinHorizontal(lipgloss.Top, left, right)
```

Keep the footer help concise:

```go
"↑↓: turn  ←→: version  e: edit  g: new version  v: compare  Enter: apply/confirm  Esc: back"
```

- [ ] **Step 5: Run the view test and the full UI package**

Run:

```bash
go test ./internal/ui -run 'TestViewMessageBrowse_RendersTurnPanes$'
go test ./internal/ui
```

Expected:

- the new render test passes
- pre-existing UI tests still pass or fail only on the next planned browser-state gaps

- [ ] **Step 6: Commit the browser state scaffold**

Run:

```bash
git add internal/ui/model.go internal/ui/messagebrowse.go internal/ui/messagebrowse_view_test.go
git commit -m "feat(ui): add two-pane message browser state"
```

## Task 5: Implement preview/apply version switching and compare mode

**Files:**
- Modify: `internal/ui/messagebrowse.go`
- Modify: `internal/ui/view.go`
- Create: `internal/ui/messagebrowse_test.go`

- [ ] **Step 1: Write failing tests for preview-only navigation and compare mode**

Append these tests to `internal/ui/messagebrowse_test.go`:

```go
func newVersionedBrowseModel(t *testing.T) Model {
	t.Helper()
	store := newBrowserTestStore(t)
	m := newBrowserTestModel(t, store)
	m.messageBrowse.turns = []browseTurn{
		{
			User: chat.Message{ID: 1, Seq: 1, Role: "user", Content: "q1"},
			AssistantVersions: []chat.Message{
				{ID: 2, Seq: 1, Role: "assistant", Content: "v1", VersionGroupID: 2, VersionNumber: 1, TotalVersions: 2},
				{ID: 3, Seq: 1, Role: "assistant", Content: "v2", VersionGroupID: 2, VersionNumber: 2, TotalVersions: 2},
			},
			ActiveVersion:  0,
			PreviewVersion: 0,
		},
		{
			User: chat.Message{ID: 4, Seq: 2, Role: "user", Content: "q2"},
			AssistantVersions: []chat.Message{
				{ID: 5, Seq: 2, Role: "assistant", Content: "a2", VersionNumber: 1, TotalVersions: 1},
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
```

- [ ] **Step 2: Run the targeted browser tests and confirm failure**

Run:

```bash
go test ./internal/ui -run 'TestMessageBrowse_(RightArrowChangesPreviewOnly|EnterOnPreviewWithLaterTurnsOpensConfirmation|VEntersCompareMode)$'
```

Expected:

- compile failure because confirmation state and compare-mode transitions are not implemented

- [ ] **Step 3: Add preview/apply confirmation state**

In `internal/ui/model.go`, add:

```go
type browseConfirmKind int

const (
	confirmNone browseConfirmKind = iota
	confirmApplyPreview
	confirmEditRegenerate
)

type browseConfirmState struct {
	kind   browseConfirmKind
	cursor int
}
```

Then add it to `messageBrowseState`:

```go
	pendingConfirm browseConfirmState
```

- [ ] **Step 4: Implement preview navigation, apply flow, and compare mode**

Update `internal/ui/messagebrowse.go` with this core control flow:

```go
func (m *Model) currentBrowseTurn() *browseTurn {
	return &m.messageBrowse.turns[m.messageBrowse.turnIdx]
}

func (m *Model) movePreviewVersion(delta int) {
	turn := m.currentBrowseTurn()
	next := turn.PreviewVersion + delta
	if next < 0 || next >= len(turn.AssistantVersions) {
		return
	}
	turn.PreviewVersion = next
}

func (m *Model) moveCompareCard(delta int) {
	next := m.messageBrowse.compareCardIdx + delta
	if next < 0 || next >= len(m.currentBrowseTurn().AssistantVersions) {
		return
	}
	m.messageBrowse.compareCardIdx = next
	m.currentBrowseTurn().PreviewVersion = next
}

case "left", "h":
	if m.messageBrowse.mode == browseModeCompare {
		m.moveCompareCard(-1)
	} else {
		m.movePreviewVersion(-1)
	}
case "right", "l":
	if m.messageBrowse.mode == browseModeCompare {
		m.moveCompareCard(1)
	} else {
		m.movePreviewVersion(1)
	}
case "v":
	if m.currentBrowseTurn().VersionCount() > 1 {
		m.messageBrowse.mode = browseModeCompare
		m.messageBrowse.compareCardIdx = m.currentBrowseTurn().PreviewVersion
	}
case "enter":
	if m.messageBrowse.pendingConfirm.kind != confirmNone {
		return m.applyBrowseConfirmation()
	}
	return m.confirmOrApplyPreview()
```

`confirmOrApplyPreview()` should:

- no-op if preview == active
- apply immediately if there are no later turns
- otherwise open a chooser with:
  - `Apply Here`
  - `Branch From Here`
  - `Cancel`

Use these signatures:

```go
func (m Model) confirmOrApplyPreview() (Model, tea.Cmd)
func (m Model) applyBrowseConfirmation() (Model, tea.Cmd)
```

- [ ] **Step 5: Render compare mode cards and main-chat version badges**

In `viewMessageBrowse()`, when `browseModeCompare` is active, render cards:

```go
for idx, msg := range turn.AssistantVersions {
	card := renderCompareCard(msg, m.messageBrowse.compareCardScrolls[idx], idx == m.messageBrowse.compareCardIdx)
	cards = append(cards, card)
}
```

In `internal/ui/view.go`, add a light version indicator after assistant labels:

```go
label := tab.client.Model() + ":"
if msg.TotalVersions > 1 {
	label = fmt.Sprintf("%s [%d versions]", label, msg.TotalVersions)
}
```

- [ ] **Step 6: Run the UI package until version browsing tests pass**

Run:

```bash
go test ./internal/ui
```

Expected:

- preview-only navigation works
- apply confirmation opens correctly
- compare mode renders and navigates

- [ ] **Step 7: Commit the version preview/compare flow**

Run:

```bash
git add internal/ui/messagebrowse.go internal/ui/view.go internal/ui/messagebrowse_test.go
git commit -m "feat(ui): add version preview and compare browser"
```

## Task 6: Implement user editing, save-only flags, and regenerate-in-place

**Files:**
- Modify: `internal/ui/messagebrowse.go`
- Modify: `internal/storage/conversation.go`
- Modify: `internal/storage/conversation_test.go`
- Modify: `internal/ui/messagebrowse_test.go`

- [ ] **Step 1: Write failing tests for save-only and regenerate confirmation**

Append these tests to `internal/ui/messagebrowse_test.go`:

```go
func newEditableBrowseModel(t *testing.T) Model {
	t.Helper()
	m := newVersionedBrowseModel(t)
	m.messageBrowse.turns[0].AssistantVersions[0].StaleAfterUserEdit = false
	return m
}

func TestConversationStore_MarkTurnEditedAndClear(t *testing.T) {
	store := newTestStore(t)

	name := "conv"
	userID, _ := store.AppendMessage(name, chat.Message{Seq: 1, Role: "user", Content: "old"})
	assistantID, _ := store.AppendMessage(name, chat.Message{Seq: 1, Role: "assistant", Content: "reply", VersionNumber: 1})

	if err := store.MarkTurnEdited(userID, assistantID); err != nil {
		t.Fatalf("MarkTurnEdited() error = %v", err)
	}
	msgs, _ := store.LoadActiveTimeline(name)
	if !msgs[0].EditedAfterGeneration || !msgs[1].StaleAfterUserEdit {
		t.Fatalf("expected edited/stale flags, got %+v", msgs)
	}

	if err := store.ClearTurnEdited(userID, assistantID); err != nil {
		t.Fatalf("ClearTurnEdited() error = %v", err)
	}
	msgs, _ = store.LoadActiveTimeline(name)
	if msgs[0].EditedAfterGeneration || msgs[1].StaleAfterUserEdit {
		t.Fatalf("expected edited/stale flags to clear, got %+v", msgs)
	}
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
```

- [ ] **Step 2: Run the targeted tests and confirm failure**

Run:

```bash
go test ./internal/storage -run 'TestConversationStore_MarkTurnEditedAndClear$'
go test ./internal/ui -run 'TestMessageBrowse_EditSaveOnlyMarksTurnWithoutTruncating$'
```

Expected:

- failures because edit flags and edit-mode browser flow are not wired

- [ ] **Step 3: Add edit-mode browser state**

Extend `messageBrowseState` in `internal/ui/model.go`:

```go
	editMode   bool
	editBuffer string
	editDirty  bool
```

Then wire `e` in `internal/ui/messagebrowse.go`:

```go
case "e":
	turn := m.currentBrowseTurn()
	m.messageBrowse.editMode = true
	m.messageBrowse.editBuffer = turn.User.Content
	m.messageBrowse.editDirty = false
```

- [ ] **Step 4: Implement `Save Only` and `Regenerate` confirmation paths**

Inside `updateMessageBrowse`, make `Enter` on edit mode open:

```go
m.messageBrowse.pendingConfirm = browseConfirmState{
	kind:   confirmEditRegenerate,
	cursor: 0, // Regenerate first, Save Only second
}
```

Then implement the two choices:

```go
func (m Model) reloadBrowseTurnState() (Model, tea.Cmd)
func (m Model) confirmOrRegenerateEditedTurn() (Model, tea.Cmd)

switch selected {
case 0: // Regenerate
	return m.confirmOrRegenerateEditedTurn()
case 1: // Save Only
	if err := m.store.UpdateMessageContent(turn.User.ID, m.messageBrowse.editBuffer); err != nil {
		m.statusMsg = "Update failed: " + err.Error()
		return m, nil
	}
	if err := m.store.MarkTurnEdited(turn.User.ID, activeAssistant.ID); err != nil {
		m.statusMsg = "Mark stale failed: " + err.Error()
		return m, nil
	}
	return m.reloadBrowseTurnState()
}
```

For regenerate:

- if there are later turns, offer:
  - `Regenerate Here`
  - `Branch And Regenerate`
  - `Cancel`
- if there are no later turns, overwrite the active assistant content in place and clear the flags

- [ ] **Step 5: Run the storage and UI test suites**

Run:

```bash
go test ./internal/storage ./internal/ui
```

Expected:

- edited/stale flags persist correctly
- save-only preserves later turns
- regenerate path opens the right confirmation state

- [ ] **Step 6: Commit the edit/save-only flow**

Run:

```bash
git add internal/storage/conversation.go internal/storage/conversation_test.go internal/ui/messagebrowse.go internal/ui/messagebrowse_test.go
git commit -m "feat(ui): add editable user turns and stale state"
```

## Task 7: Finish browser actions, regression pass, and docs touch-up

**Files:**
- Modify: `internal/ui/messagebrowse.go`
- Modify: `internal/ui/messagebrowse_test.go`
- Modify: `README.md`

- [ ] **Step 1: Add the remaining browser regressions**

Append these tests:

```go
func TestMessageBrowse_DeleteActiveVersionPromotesNearestRemainingVersion(t *testing.T) {
	m := newVersionedBrowseModel(t)
	m.mode = modeMessageBrowse
	m.messageBrowse.turns[0].ActiveVersion = 1
	m.messageBrowse.turns[0].PreviewVersion = 1

	m, _ = m.updateMessageBrowse(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})

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
	seedConversation(t, store, "conv", []chat.Message{
		{Seq: 1, Role: "user", Content: "q1"},
		{Seq: 1, Role: "assistant", Content: "v1", VersionGroupID: 2, VersionNumber: 1},
		{Seq: 2, Role: "user", Content: "q2"},
		{Seq: 2, Role: "assistant", Content: "a2", VersionNumber: 1},
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

func TestMessageBrowse_CompareModeMouseWheelSetsFocusedCard(t *testing.T) {
	m := newVersionedBrowseModel(t)
	m.mode = modeMessageBrowse
	m.messageBrowse.mode = browseModeCompare
	m.messageBrowse.compareCardIdx = 0

	next, _ := m.Update(tea.MouseMsg{X: 50, Y: 8, Action: tea.MouseActionPress, Button: tea.MouseButtonWheelDown})
	m = next.(Model)

	if m.messageBrowse.compareCardIdx == 0 {
		t.Fatalf("expected mouse wheel to move focus away from first card")
	}
}
```

- [ ] **Step 2: Implement version-aware delete and branch**

Update `internal/ui/messagebrowse.go` so:

```go
case "d":
	return m.deleteCurrentBrowseSelection()
case "b":
	return m.branchFromCurrentBrowseTurn()
```

`deleteCurrentBrowseSelection()` should:

- delete only the active assistant version when multiple versions exist
- promote the nearest remaining version
- remove the entire assistant slot only if no versions remain

`branchFromCurrentBrowseTurn()` should:

- clone the active path up to the current turn
- optionally apply the preview version when the branch was triggered from a non-active preview

Use these signatures:

```go
func (m Model) deleteCurrentBrowseSelection() (Model, tea.Cmd)
func (m Model) branchFromCurrentBrowseTurn() (Model, tea.Cmd)
```

- [ ] **Step 3: Document browser controls in the README**

Add a short section to `README.md`:

```md
### Message Version Browser

- Press `Esc` twice to open the browser.
- In message mode, `↑↓` switches turns and `←→` previews assistant versions.
- Press `v` to open compare mode for horizontally arranged version cards.
- Press `e` to edit the user message, then choose `Regenerate` or `Save Only`.
```

- [ ] **Step 4: Run the broad regression suite**

Run:

```bash
go test ./...
```

Expected:

- all packages pass

- [ ] **Step 5: Commit the browser completion pass**

Run:

```bash
git add internal/ui/messagebrowse.go internal/ui/messagebrowse_test.go README.md
git commit -m "feat(ui): complete message version browser flows"
```

## Self-Review Checklist

### Spec coverage

- assistant version generation: Task 5
- preview without immediate mutation: Task 5
- compare mode cards with mouse and keyboard scroll: Task 5
- user editing with save-only versus regenerate: Task 6
- lightweight edited/stale labels: Tasks 4 and 6
- explicit apply/branch confirmation instead of immediate truncation: Task 5
- active-timeline-only main chat view: Tasks 2, 3, and 5

### Placeholder scan

- no `TODO`, `TBD`, or "similar to" instructions remain in the tasks
- all tasks reference exact file paths
- every code-changing task includes concrete code snippets or method signatures

### Type consistency

This plan consistently uses:

- `chat.Message`
- `History.ReplaceMessages`
- `Store.LoadActiveTimeline`
- `Store.ListVersions`
- `Store.SetActiveVersion`
- `browseModeMessage`
- `browseModeCompare`
- `messageBrowseState`

## Recommended Execution Order

Run the tasks in order. Do not start browser UI work before Task 2 lands, because the browser needs the new storage APIs and stable row IDs.
