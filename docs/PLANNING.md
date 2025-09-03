# Copy-as-new under different slug (Collision handling)

This plan adds a lean alternative to overwrite/skip when Preflight detects a collision: create a new story in the target space using the source content but under a unique slug.

## Goals

- Let users resolve collisions by creating a copy of the source story in target.
- Keep UI simple: no interactive diffing; few predictable choices.
- Ensure generated slugs are valid and unique (including translated slugs).
- Integrate with existing Preflight → Sync flow and writer invariants.

## Scope (Phase 1)

- Stories only (not folders).
- Copy content from Source → Target with adjusted slug(s) and paths.
- Only create copy as draft (prevent unintentional duplicates)
- Do not handle UUID. UUID is going to be blocked by original. Let the user resolve internal links by hand.

---

## UX Flow

1. Preflight marks collision items as usual (state: update). For collisions, provide a new action: "Fork as new" (key: `f`).
2. Pressing `f` opens a small modal:
   - Preset patterns:
     - `{slug}-copy`
     - `{slug}-{hhmmss-ddmmyyyy}` (local time)
   - Manual edit field for final slug (pre‑filled by chosen preset).
   - Optional checkbox: append " (copy)" to Name (default: off).
3. On confirm:
   - The item is marked with an action badge "Fork" in the Preflight list.
   - The computed new slug and per‑locale paths are stored with the item.
4. Sync uses the create path to make a new story in target under the chosen slug.

Keyboard:

- `c` Copy as new
- `enter` confirm in modal; `esc` cancel

---

## Data Model & Integration

- Extend `internal/core/sync.PreflightItem`:
  - `CopyAsNew bool`
  - `NewSlug string` // normalized final slug for default locale
  - `NewTranslatedPaths map[string]string` // lang → new full path (for translated slugs)
  - `AppendCopySuffixToName bool`
- UI sets these fields. Planner continues to optimize ordering (folders first). Item uses `State=create` for UI clarity; a label shows "Fork" or "F" to avoid confusion with "C" (Create).
- Sync layer checks `CopyAsNew` and forces the create path

---

## Slug & Path Generation

- Base slug is the last path segment of the source story (`source.Story.Slug`). Normalize to kebab‑case ASCII and lowercase.
- Presets:
  - {alter-slug}-copy
  - {slug}-v2
  - {slug}-new
  - {slug}-{yyyyMMdd-HHmm}
- Uniqueness:
  - If `${parent}/${newSlug}` already exists in target (including folders), append `-1`, `-2`, … until unique.
  - Same logic applies per locale paths derived below.
- Translated slugs:
  - For each `translated_slugs` entry, compute new `path` by replacing the last segment of the existing `path` with `newSlug` (same suffix across locales).
  - Drop IDs (writer will map to `translated_slugs_attributes`).
- Parent folder:
  - Keep the same parent folder path as source (planner already ensures folders exist or adds them).
- Name:
  - If `AppendCopySuffixToName`, append ` " (copy)"` to `name`.

---

## Writer Behavior (unchanged invariants)

- Treat as create:
  - Use source payload; override `slug` and per‑locale `translated_slugs` paths derived above.
  - Resolve `parent_id` from folder path as today.
  - Strip read‑only fields; convert `translated_slugs` → `translated_slugs_attributes` without IDs.
- Publish policy: follow current plan/policy (published/draft handling).
- UUID: after create, don't update UUID to avoid collisions

---

## Error Handling

- Validation in modal: ensure slug not empty and unique in folder, valid chars, fits Storyblok rules.
- If uniqueness resolution fails due to rapid concurrent changes, retry with incremented numeric suffix.
- On API error, show inline issue on the item and leave it pending/cancelled according to current rules.

---

## Testing Strategy

Unit:

- Slug normalization: cases with spaces, umlauts, punctuation → kebab‑case.
- Uniqueness generator: existing slugs map → choose `-1`, `-2` correctly.
- Translated slugs mapping: replace last segment; drop IDs.

Integration (UI):

- Preflight collision → press `f` → choose preset → save → item shows "Copy" badge and holds slug data.
- Sync consumes `CopyAsNew` and executes create path with overridden slug.

Integration (core):

- With `CopyAsNew`, writer chooses create path and respects invariants; publish + UUID behavior intact.

---

## Phase 1 Acceptance Criteria

- Users can resolve a collision by creating a copy with `{slug}-copy` and sync succeeds.
- If target already contains `{slug}-copy`, system creates `{slug}-copy-1` (or `-2`, etc.).
- Translated slugs are updated per locale with new last segment, without IDs.
- Folders are unaffected; feature disabled for folder items.

---

## Implementation Steps

1. Model: add fields to `PreflightItem` (or implement UI cache for Phase 1 if preferred).
2. UI (Preflight): add `f` action and modal; implement slug generator and validator; store results.
3. Writer: in story sync code, if `CopyAsNew`, force create path and override slug/translated paths before sending.
4. Tests: add unit tests for slug generation and translated mapping; UI integration test for flow; core integration test for create path override.
