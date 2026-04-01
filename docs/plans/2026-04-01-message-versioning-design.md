# Message Versioning Design

Date: 2026-04-01
Project: `termchat`
Reference: `orion-chat-rs` message version generation and switching

## 1. Goal

Add message version generation and switching to `termchat`, migrating the core behavior from `orion-chat-rs` while adapting it to the terminal UI.

Phase 1 must support:

- generating multiple assistant reply versions for the same user turn
- browsing and previewing versions without immediately mutating the main timeline
- switching the active version with explicit user confirmation
- comparing versions in a dedicated browser mode
- editing the user message of a turn
- choosing between regenerating after edit or saving the edit only

Phase 1 does not need:

- a tree-shaped conversation model
- a separate full-screen compare page outside the existing double-`Esc` browser
- compatibility with old persisted history

The user has explicitly accepted dropping old local history compatibility, so the schema can be replaced instead of migrated incrementally.

## 2. Product Decisions

These decisions are fixed for Phase 1.

### 2.1 Timeline semantics

- The main chat view continues to show a single linear active timeline.
- Assistant version groups are hidden behind that active timeline.
- Generating a new assistant version does not immediately replace the active timeline.
- Previewing another version does not immediately delete following turns.
- Applying a non-active version may require a confirmation step if later turns exist.

### 2.2 Browser entry and placement

- The existing double-`Esc` message browser remains the entry point.
- The browser is upgraded instead of replaced.
- The browser becomes the main place for version browsing, version comparison, user-message editing, rollback, delete, branch, and copy.

### 2.3 Main chat UI scope

- The main chat view keeps a lightweight shortcut to jump into the browser focused on the latest assistant turn.
- Complex version browsing and comparison stay inside the browser.
- The main chat view does not gain message-level focus navigation.

### 2.4 Message and compare modes

- The browser has two submodes:
  - `message mode`
  - `compare mode`
- `message mode` is the default.
- `compare mode` is entered explicitly from `message mode`.

### 2.5 Editing semantics

- `g` means "generate another assistant answer version for the same user turn".
- `e` means "edit the current user message".
- `e` does not create a new assistant version.
- After editing, `Enter` offers:
  - `Regenerate`
  - `Save Only`
- `Save Only` preserves later turns.
- `Save Only` marks the current turn as edited/stale with lightweight labels only.

## 3. Data Model

Phase 1 should follow the storage strategy used by `orion-chat-rs`: versions are stored as sibling rows in the `messages` table, not in a separate versions table.

### 3.1 Conversation model

At the product level, the browser works on a `turn`.

A `turn` is:

- one user message on the left
- one assistant slot on the right
- the assistant slot may have one or more versions

At the storage level, the database still stores rows, not explicit turn objects. The application layer reconstructs turns from the linear active timeline.

### 3.2 Messages table

Target schema for `messages`:

- `id INTEGER PRIMARY KEY AUTOINCREMENT`
- `conversation_id INTEGER NOT NULL REFERENCES conversations(id) ON DELETE CASCADE`
- `role TEXT NOT NULL`
- `content TEXT NOT NULL`
- `seq INTEGER NOT NULL`
- `created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP`
- `version_group_id INTEGER`
- `version_number INTEGER NOT NULL DEFAULT 1`
- `is_active_version INTEGER NOT NULL DEFAULT 1`
- `edited_after_generation INTEGER NOT NULL DEFAULT 0`
- `stale_after_user_edit INTEGER NOT NULL DEFAULT 0`

Field semantics:

- `seq`
  - logical slot order in the active timeline
  - all assistant versions of the same turn share the same `seq`
- `version_group_id`
  - null for non-versioned rows
  - when a reply first becomes versioned, the original assistant row is updated so `version_group_id = id`
- `version_number`
  - starts at `1`
  - increments monotonically inside a version group
- `is_active_version`
  - exactly one active assistant version per group
- `edited_after_generation`
  - meaningful on user rows
  - indicates the user message was edited after its assistant reply was generated
- `stale_after_user_edit`
  - meaningful on assistant rows
  - indicates this assistant reply no longer matches the latest saved user content

### 3.3 Query rules

Main timeline queries:

- only read rows where:
  - `role != 'assistant'`, or
  - `is_active_version = 1`
- order by `seq ASC`, then `id ASC`

Version queries:

- fetch all assistant versions for a turn with:
  - `WHERE version_group_id = ?`
  - `ORDER BY version_number ASC`

Applying a version:

- update the target group's `is_active_version`
- optionally truncate later turns or branch into a new conversation

### 3.4 No old-history compatibility

Because old local history may be dropped:

- the schema can be recreated directly
- `storage.Save` no longer needs to support the old "delete all and rewrite all rows" model
- autosave becomes incremental and version-aware

## 4. Core Storage Operations

The storage layer needs explicit operations instead of a single replace-all save path.

### 4.1 Required operations

- `LoadConversationTimeline(conversationName)`
  - returns the active linear timeline only
- `LoadTurns(conversationName)`
  - reconstructs browser turns from the active timeline
- `ListVersions(conversationName, assistantMessageID)`
  - returns all versions in the message's version group
- `GenerateAssistantVersion(conversationName, assistantMessageID)`
  - initializes the group if needed
  - inserts a new non-active assistant version row
- `SetActiveVersion(conversationName, versionGroupID, versionNumber)`
  - flips the group's active flag
- `TruncateAfterSeq(conversationName, seq)`
  - deletes all rows after a logical turn
- `BranchConversationAtSeq(conversationName, seq, versionOverride?)`
  - creates a new conversation from the active path up to a turn
  - optionally replaces the branch turn's assistant version with a selected preview version
- `UpdateUserMessage(messageID, content)`
  - updates the user row content
- `MarkTurnEdited(userMessageID, assistantVersionID)`
  - sets:
    - `edited_after_generation = 1` on the user row
    - `stale_after_user_edit = 1` on the active assistant row
- `ClearTurnEdited(userMessageID, assistantVersionID)`
  - clears the above flags after regenerate succeeds
- `OverwriteAssistantInPlace(assistantVersionID, newContent)`
  - used by `e -> Regenerate`
  - does not create a new version

### 4.2 Why not keep replace-all autosave

Current `termchat` storage rewrites the whole conversation on autosave. That approach breaks versioning because:

- it destroys stable row identity
- it collapses multiple assistant versions into one visible row
- it makes preview/generated-but-not-applied versions impossible to persist correctly

Phase 1 therefore requires moving from replace-all persistence to row-based persistence.

## 5. Browser Architecture

The existing browser in `internal/ui/messagebrowse.go` becomes a turn browser with two submodes.

### 5.1 Browser state

Browser-local state:

- current mode:
  - `message`
  - `compare`
- selected turn index
- selected preview version index for the current turn
- compare-mode active card index
- left pane scroll offset
- right pane scroll offset
- compare card scroll offsets per version
- optional pending confirmation state
- optional edit state for the user pane

### 5.2 Preview versus active version

The browser must distinguish:

- `active version`
  - the version currently used by the main chat timeline
- `preview version`
  - the version currently being inspected in the browser

Rules:

- entering a turn initializes `preview = active`
- left/right in `message mode` only changes `preview`
- left/right in `compare mode` only changes the focused compare card and preview selection
- no destructive timeline change happens until the user confirms an apply action

### 5.3 Turn reconstruction

A turn is reconstructed from the active timeline as:

- one user message
- the nearest assistant message immediately following it, if present

Special cases:

- if the timeline ends on a user message with no assistant reply yet, that final turn has only the left pane
- system prompt is not treated as a turn

## 6. Message Mode

`message mode` upgrades the old single-message browser into a two-pane turn viewer.

### 6.1 Layout

- left pane: current user message
- right pane: current assistant preview version
- both panes have independent scroll positions
- header shows:
  - turn index
  - total turn count
  - current assistant version, if present
- footer help line shows available keys

### 6.2 Labels

Left pane header examples:

- `User`
- `User [edited]`

Right pane header examples:

- `Assistant v1/1`
- `Assistant v2/4`
- `Assistant v2/4 [stale]`
- `Assistant v3/4 [preview]`

### 6.3 Keys

- `Up` / `Down`
  - move between turns
- `j` / `k`
  - aliases for turn movement in message mode
- `Left` / `Right`
  - preview previous or next assistant version for the current turn
- `Enter`
  - apply the current preview version if it differs from active
  - confirm edit actions
  - keep existing rollback functionality only when no preview/edit confirmation is pending
- `g`
  - generate a new assistant version for the current turn
- `e`
  - edit the current user message
- `d`
  - delete current turn or current assistant active version as version-aware delete
- `b`
  - branch conversation from the current active path up to this turn
- `c`
  - copy the visible pane content
- `v`
  - enter compare mode when the current assistant has multiple versions
- `Esc`
  - leave the browser and return to the main chat view

### 6.4 Scrolling

- left and right panes both support vertical scrolling
- mouse wheel scrolls the pane under the pointer
- keyboard scrolling is only needed for whichever pane is currently focused during edit state
- outside edit state, message mode uses `Up` and `Down` for turn navigation, not pane scrolling

## 7. Compare Mode

Compare mode is the version-focused view for the selected turn.

### 7.1 Layout

- horizontally arranged assistant version cards
- each card has:
  - version header
  - optional model name if available in future
  - independent scrollable content area
- active/focused card has a clear highlight

### 7.2 Behavior

- entering compare mode focuses the current preview version card
- each card keeps its own scroll offset
- compare mode does not change the active timeline by itself

### 7.3 Keys

- `Left` / `Right`
  - move the focused card
- `Up` / `Down`
  - scroll the focused card
- `j` / `k`
  - scroll the focused card
- `PgUp` / `PgDn`
  - fast scroll the focused card
- `Home` / `End`
  - jump inside the focused card
- `Enter`
  - attempt to apply the focused preview version
- `c`
  - copy focused card content
- `g`
  - generate a new assistant version and refresh cards
- `Esc`
  - return to message mode

### 7.4 Mouse

- mouse wheel scrolls the card under the pointer
- scrolling a card also makes it the focused card
- this keeps keyboard and mouse focus aligned without introducing a global focus system

## 8. Generate Version Flow

`g` means "generate a different assistant answer for the same user turn".

### 8.1 Flow

1. User selects a turn with an assistant reply.
2. User presses `g`.
3. The system generates a new assistant version from:
   - all active timeline turns before this turn
   - the current user message content of this turn
4. The new version is inserted into the same assistant version group.
5. The new version is stored as non-active initially.
6. Browser preview switches to the newly generated version.
7. Main timeline remains unchanged until the user explicitly applies it.

### 8.2 Apply behavior

If the previewed version is not active and the user presses `Enter`:

- if there are no later turns:
  - set the preview version active immediately
- if there are later turns:
  - open a confirmation chooser

Confirmation options:

- `Apply Here`
  - set preview version active
  - truncate later turns
- `Branch From Here`
  - create a new conversation using the active path up to this turn
  - make the preview version active in the new conversation
- `Cancel`

## 9. Edit User Message Flow

`e` means "edit the current user message". It must stay separate from assistant version generation.

### 9.1 Flow

1. User presses `e` on a turn.
2. Left pane enters editable mode.
3. User edits content.
4. User presses `Enter`.

If content is unchanged:

- exit edit mode

If content changed:

- open a chooser:
  - `Regenerate`
  - `Save Only`
  - `Cancel`

### 9.2 Save Only

`Save Only` does:

- persist the new user content
- keep the current assistant reply unchanged
- keep later turns unchanged
- mark:
  - left pane as `[edited]`
  - right pane as `[stale]`

It does not:

- create a new assistant version
- auto-regenerate
- truncate later turns

### 9.3 Regenerate

`Regenerate` does not create a new version. It updates the current assistant reply in place.

If there are no later turns:

- overwrite the active assistant reply for this turn
- clear edited/stale labels on success

If there are later turns:

- open a confirmation chooser

Confirmation options:

- `Regenerate Here`
  - overwrite current active assistant reply
  - truncate later turns
- `Branch And Regenerate`
  - create a new conversation from this turn
  - use the edited user content and regenerated assistant reply in the new conversation
- `Cancel`

## 10. Delete, Rollback, Branch, and Copy

Existing browser actions remain, but become version-aware.

### 10.1 Delete

If the current turn has a non-versioned assistant reply:

- delete the turn as before

If the current turn has multiple assistant versions:

- deleting the active version removes that version only
- if other versions remain:
  - the nearest version becomes active automatically
- if no versions remain:
  - remove the assistant slot from the turn

### 10.2 Rollback

Rollback remains available, but acts on the active path:

- keep all turns up to the selected turn
- discard everything after it

### 10.3 Branch

Branch copies the active path up to the current turn into a new conversation.

If a non-active preview version is selected, the branch operation may optionally use that preview version in the new conversation.

### 10.4 Copy

- in message mode, copy the visible user or assistant pane content
- in compare mode, copy the focused card content

## 11. Main Chat View Integration

The main chat view remains simple.

Changes:

- continue rendering only the active timeline
- assistant rows with multiple versions should show a lightweight version indicator
- add a shortcut that opens the browser focused on the latest assistant turn

The main chat view should not:

- support in-place version browsing
- support in-place compare cards
- support direct user-message editing

## 12. Error Handling

Phase 1 must handle these explicitly.

### 12.1 Generation failures

- if `g` fails, keep the existing active version unchanged
- show status feedback in the browser
- do not leave a broken preview selected

### 12.2 Apply failures

- if switching active version fails, keep preview state but do not mutate main timeline
- show status feedback

### 12.3 Edit/regenerate failures

- if regenerate fails after user edit:
  - keep edited user content if already saved
  - preserve stale labels
  - keep the old assistant content unless overwrite had already been committed

### 12.4 Branch failures

- if branch creation fails, keep the current conversation untouched

## 13. Testing Strategy

Phase 1 needs storage tests, browser state tests, and rendering tests.

### 13.1 Storage tests

- create a version group from an original assistant reply
- insert additional versions with shared `seq`
- query active timeline correctly
- list versions correctly
- apply a different version
- truncate after a selected turn
- branch from a selected turn
- save-only edit sets edited/stale flags
- regenerate clears edited/stale flags

### 13.2 Browser tests

- message mode left/right changes preview only
- applying preview with no later turns switches active version immediately
- applying preview with later turns enters confirmation state
- compare mode preserves per-card scroll offsets
- compare mode mouse/keyboard focus alignment works
- edit flow offers regenerate vs save-only
- save-only preserves later turns

### 13.3 Rendering tests

- message mode draws two panes
- edited/stale labels render correctly
- compare mode draws horizontal cards
- focused compare card highlight updates correctly

## 14. Implementation Notes

This design implies targeted rewrites in:

- `internal/storage/conversation.go`
- `internal/chat/history.go`
- `internal/ui/messagebrowse.go`
- `internal/ui/update.go`
- `internal/ui/view.go`

Minimal impact areas:

- provider clients
- settings
- roles
- shortcuts
- tab management

## 15. Recommendation

Build Phase 1 around these principles:

- use SQLite rows as the source of truth for assistant versions
- keep the main chat timeline linear and active-version-only
- make the browser preview-first, not destructive-first
- reserve destructive timeline changes for explicit confirmation
- keep editing and version generation as separate user intents

This keeps the migrated behavior close to `orion-chat-rs` at the storage layer while making the interaction model fit a terminal application.
