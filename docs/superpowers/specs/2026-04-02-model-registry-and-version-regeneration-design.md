# Model Registry And Version Regeneration Design

Date: 2026-04-02

## Goal

Clarify and implement the message-versioning behavior around `e`, `g`, `r`, and `/model` so that:

- each turn has exactly one `user` message version
- each turn can have multiple `assistant` versions
- each assistant version keeps its own generation snapshot
- editing a user message can regenerate all assistant versions using their own saved snapshots
- users can add a new assistant version by choosing a configured provider/model pair from a model registry
- day-to-day model switching moves out of `/settings` into `/model`

## Non-Goals

- full agent snapshots including tools
- arbitrary role switching mid-conversation
- user-message version history
- replacing the existing `/settings` editor for general config management

## Terminology

- `role`: the conversation-level system/role prompt selected before chat begins. In this phase it does not include tools.
- `generation snapshot`: the per-assistant-version data required to reproduce that version. In this phase it contains:
  - provider
  - model
  - role name
  - role prompt
- `model registry`: a persistent two-level config:
  - provider config: shared base URL and API key
  - models: named model entries under that provider config

## Product Rules

### User message semantics

- A turn has one `user` message only.
- Pressing `e` edits that single `user` message.
- Editing does not create a new `user` version.

### Assistant version semantics

- A turn may have zero or more `assistant` versions.
- Each assistant version stores its own generation snapshot.
- A stored error response is still an `assistant` version and is deletable like any other version.

### `e` behavior

- `e` opens edit mode for the current turn's `user` message.
- If the edited content is submitted unchanged, exit edit mode with no storage changes.
- If the edited content changed, show two actions:
  - `save only`
  - `regenerate all versions`
- `save only`:
  - updates the single `user` row
  - does not create new assistant versions
  - leaves existing assistant versions in place
  - marks all existing assistant versions in the turn as stale
- `regenerate all versions`:
  - updates the single `user` row first
  - iterates every existing assistant version in the turn
  - for each version, rebuilds the request using that version's own saved generation snapshot
  - replaces the content of that same assistant version in place
  - if generation fails, replaces that version's content with `Error: ...`
  - clears stale/edited flags for versions successfully regenerated

### `r` behavior

- `r` regenerates only the currently previewed assistant version.
- It uses that version's own generation snapshot.
- It overwrites that version in place.
- It does not create a new version.
- If generation fails, it overwrites the version with `Error: ...`

### `g` behavior

- `g` creates a new assistant version for the current turn.
- It does not modify the `user` message.
- It opens a two-step selector:
  - choose a provider config
  - choose a model under that provider config
- The conversation's existing role prompt is reused.
- A new generation snapshot is created from:
  - selected provider
  - selected model
  - current conversation role name
  - current conversation role prompt
- The generated response is appended as a new assistant version in the same version group.
- If generation fails, append a new assistant version whose content is `Error: ...` and whose snapshot is still the selected one.

### Role semantics

- The conversation role is chosen before normal chat begins.
- The role is treated as fixed for the conversation.
- `g` does not ask the user to reselect role.
- Even though role is fixed at the conversation level, each assistant version still stores a copy of the role name and role prompt in its own generation snapshot to keep regeneration reproducible.

### `/model` behavior

- `/model` replaces `/settings` as the primary model-switching workflow.
- `/model` has two responsibilities:
  - manage the model registry
  - switch the current tab's active provider/model
- Switching through `/model` affects only the current tab.
- It does not change the global default config for future tabs.

## Data Model

### Assistant message metadata

Extend `chat.Message` and storage for assistant rows with:

- `snapshot_provider`
- `snapshot_model`
- `snapshot_role_name`
- `snapshot_role_prompt`

These values are required for all newly created assistant rows, including error rows.

### Conversation-level state

Current tab/session state must keep:

- current role name
- current role prompt
- current provider config selection
- current model selection

The role values are needed when creating a brand-new assistant version via `g`.

### Model registry

Persist a new registry config file, separate from the current main config, at `models.yaml` in the same config directory. Structure:

```yaml
providers:
  - name: gemini-gateway
    provider: openai-compatible
    base_url: https://example.test/v1
    api_key: env-or-literal
    models:
      - name: gemini-3-flash
        model: gemini-3-flash-preview
      - name: gemini-3-pro
        model: gemini-3.1-pro-preview
  - name: anthropic-direct
    provider: anthropic
    base_url: https://api.anthropic.com
    api_key: env-or-literal
    models:
      - name: sonnet
        model: claude-sonnet-4-20250514
```

Rules:

- provider config names must be unique
- model names only need to be unique within one provider config
- one provider config owns one provider type, one base URL, and one API key
- one provider config can expose multiple models

## UI Design

### `/model` entry

`/model` opens a dedicated browser/editor instead of reusing `/settings`.

Primary views:

- provider-config list
- model list for selected provider config
- action row for:
  - switch current tab to this model
  - add provider config
  - edit provider config
  - delete provider config
  - add model
  - edit model
  - delete model

For this phase, keyboard-first interaction is sufficient.

### Browser help updates

Message browser help should include:

- `e: edit user`
- `r: regenerate version`
- `g: new version`
- `v: compare`

### `g` selector

`g` should reuse the same provider/model selection component logic as `/model`, but in selection-only mode:

- choose provider config
- choose model
- confirm

No role step is shown.

## Request Construction

### Base regeneration context

For `e -> regenerate all`, `r`, and `g`, build the API messages as:

- optional system message from the snapshot role prompt or conversation role prompt, depending on operation
- all active timeline messages before the current turn
- current turn user message

Operation-specific rule:

- `e -> regenerate all` and `r` use the target version's own snapshot role prompt
- `g` uses the conversation's fixed role prompt

Provider/model selection comes from:

- target version snapshot for `e -> regenerate all` and `r`
- user choice from registry for `g`

## Error Handling

- generation failure is stored as `assistant` content beginning with `Error: `
- storage failure during regeneration must stop the operation and report a status message
- partial completion for `e -> regenerate all` is allowed
- if partial completion happens:
  - successful versions remain updated
  - failed versions are overwritten with `Error: ...`
  - the browser is rebuilt from storage after the full pass

## Migration

- existing assistant rows without snapshot fields should remain readable
- legacy rows without a complete snapshot are treated as non-reproducible
- for non-reproducible legacy rows:
  - browsing, compare, delete, apply-preview, and branch continue to work
  - `r` is disabled for that version
  - `e -> regenerate all versions` is disabled for any turn that still contains one or more non-reproducible assistant versions
  - `g` remains available and creates new reproducible versions under the new schema

## Testing

Add coverage for:

- `e -> save only` keeps one user row and marks all assistant versions stale
- `e -> regenerate all` uses each assistant version's own snapshot
- `e -> regenerate all` overwrites failed versions with `Error: ...`
- `r` only regenerates the previewed version
- `g` appends a new assistant version with selected provider/model and current role snapshot
- `/model` switches only the current tab
- legacy rows without snapshots degrade gracefully
- browser labels show enough metadata to distinguish versions from different provider/model pairs

## Implementation Notes

- `/settings` remains for general parameters and raw config editing, but provider/model switching should move out of the main flow
- assistant-version headers in message mode and compare mode must surface provider/model labels so cross-model comparison is understandable
- future tool support should extend the generation snapshot rather than replace it
