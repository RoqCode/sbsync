# Publish Toggle Plan (Publish, Draft, Publish&Changes)

This plan specifies UX, data flow, and sync semantics for a three‑state publish toggle per story: Publish, Draft, and Publish&Changes. Focus is on predictable behavior and clear constraints for Storyblok’s dual‑version “publish with pending changes” mode.

## Goals

- Allow users to choose, per story, one of: Publish, Draft, Publish&Changes.
- Default choices sensibly from source state and target space plan.
- Prevent invalid combinations (e.g., Publish&Changes when target isn’t published or on creates).
- Keep UI simple and consistent across Browse, Preflight, and Sync views.

## State Definitions

- **Draft**: sync content to target without publishing. Target’s published version (if any) remains unchanged; target stays draft if it didn’t exist.
- **Publish**: sync content and publish it (subject to plan limits and dev‑mode constraints).
- **Publish&Changes**: sync content without publishing it, but keep target’s existing published version live. This implies the target already has a published version; the new content becomes the draft version. Equivalent API behavior: perform update with `publish=false` while the target story is in “published” state. UI badge: `[Pub+∆]`.

## Constraints & Rules

- **Folders**: no publish state; the toggle applies to stories only.
- **Creates**: Publish&Changes is invalid; there is no prior published version. Disable or automatically fall back to Draft.
- **Target not published**: Publish&Changes is invalid; fall back to Draft and show a hint.
- **Dev mode target space**: Publish requests may be blocked/limited; we still allow selecting Publish but visually hint “[Dev]”. Runtime errors are surfaced per item.
- **Copy‑as‑new (fork)**: always Draft initially; Publish/Publish&Changes disabled by default. User may switch to Publish explicitly; Publish&Changes stays disabled.

## Defaulting Policy

On preflight build, for each story item (not folder):

1. Determine `existsInTarget` and `targetPublished`.
2. If `source.Published == true` and `shouldPublish() == true`:
   - Default to Publish (overwrite) regardless of whether the target is published.
3. If `source.Published == false`: default to Draft.
4. If target space is in dev mode: retain default but display a `[Dev]` badge; actual publish may be limited by API.

Rationale: conservative when target is already live; otherwise mirror source intent.

## UX Design

Preflight view (primary surface):

- Per‑item toggle cycling order: Draft → Publish → Publish&Changes → Draft.
- Keybinds:
  - `p`: cycle publish state for current story item.
  - `P`: apply current item’s state broadly:
    - If cursor is on a folder: apply to all children in that folder (direct descendants) and their subtrees (stories only; skip folders).
    - If cursor is on a story: apply to all siblings within the same folder and their subtrees where applicable (i.e., for any sibling that is a folder, apply to its descendant stories).
- Badges on list items:
  - `[Draft]`, `[Pub]`, `[Pub+∆]` for stories.
  - Add `[Dev]` badge when target space is dev mode.
  - For forked items, keep `[Fork]`; combine compactly e.g., `[Fork][Draft]`.
- Disabled states: when a state is invalid (e.g., Publish&Changes on create), the cycle skips it; footer notes why (“Publish&Changes requires published target”).

Browse view (secondary):

- Show current effective publish badge for source items as a hint, but the toggle is only available in Preflight.

Sync view:

- Display the chosen state next to each story for visibility; color consistent with Preflight styling.

Footer help (Preflight):

- Add: `p: Publish/Draft/Publish&Changes`. If dev mode: append “Ziel im Dev‑Modus: Publish kann ignoriert werden”.

## Data Model

- Introduce an explicit per‑item enum independent of `sb.Story.Published`:
  - New UI field on preflight item: `PublishMode` with values `draft|publish|publish_changes`.
  - Continue using `sb.Story.Published` to reflect source state and for display; do not overload it for plan state.
- Persistence: store `PublishMode` on `ui.PreflightItem` (UI layer) and compute the effective `publish` boolean at execution time based on the mode and context.
- For copy‑as‑new: set `PublishMode = draft` initially; allow switching to `publish`; keep `publish_changes` disabled.

Note: Core sync orchestrator currently uses `story.Published` to decide publishing. UI will map `PublishMode` → `publish bool` and call the syncer with that flag (no core changes required).

## Sync Semantics

For each story item at scheduling time:

- Draft: create/update with `publish=false`.
- Publish: create/update with `publish=true` (if `shouldPublish() == true`; otherwise `false` but keep UI badge and show `[Dev]`).
- Publish&Changes: update with `publish=false` (never publish). Only valid if `existsInTarget && targetPublished`. For creates or unpublished target, force `publish=false`, set an inline issue, continue. UI badge `[Pub+∆]`.

Special case (requested behavior):

- If the source story is Published but its `PublishMode` is set to Draft, and the target story is currently published, then:
  - Overwrite the target’s content (perform an update), and
  - Afterwards unpublish the story so the final state is Draft.
  - Implementation requires an explicit Unpublish call; see Implementation Steps.

Implementation detail: The UI already constructs a `StorySyncer` and invokes `SyncStoryDetailed`. We will pass the computed `publish` flag derived from `PublishMode` into that call. For the “overwrite then unpublish” case, after a successful update we call `UnpublishStory(spaceID, storyID)`.

## Edge Cases

- Dev mode + Publish selected: API may return dev‑mode publish limit errors; surface as warnings/errors. Keep item completed with issue text; continue flow.
- Publish&Changes on create: treat as Draft; add Issue: “Publish&Changes benötigt veröffentlichtes Ziel; als Draft synchronisiert”.
- Publish&Changes on unpublished target: same fallback as above.
- Folders: ignore toggle keys; no badges.

## Implementation Steps

1) Model & Wiring
- Add `PublishMode string` to `internal/ui/types.go`’s `PreflightItem` (UI alias or wrapper). Allowed: `draft`, `publish`, `publish_changes`.
- Initialize `PublishMode` in `startPreflight()` per Defaulting Policy (build a map `targetPublishedBySlug`).

2) Key Handling (Preflight)
- In `handlePreflightKey`: on `p`, cycle `PublishMode` for the current visible story item, skipping invalid states based on existence/targetPublished. Re-render; keep skipped/deselected untouched.
- `P` propagation as specified above (siblings + subtree).

3) Rendering
- `view_preflight.go`: append `[Draft]/[Pub]/[Pub+∆]` badges for stories; add `[Dev]` when applicable.
- `view_browse.go`: show a small `[Pub]/[Draft]` hint for source items (optional).
- `view_sync.go`: include the selected mode in each story’s line.
- Update footer help strings accordingly.

4) Execution Mapping
- In `runNextItem()` command closure (or where `StorySyncer` is called):
  - Compute `publishFlag` from `PublishMode` and `shouldPublish()`.
  - For `publish_changes`, enforce update path assumption; if it’s a create, set `publishFlag=false` and set an inline Issue.
  - For the “Published source + Draft mode + Target published” case: perform update with `publish=false` (or `true` if needed to guarantee overwrite semantics), then call `UnpublishStory` to ensure final draft state.
  - Pass `publishFlag` into `SyncStoryDetailed` (UI already calls it with a boolean).

New API in `internal/sb`:

- Add `UnpublishStory(ctx context.Context, spaceID, storyID int) error` to client; implement via Storyblok Management API. Use existing retry/transport; add tests.

5) Validation & Issues
- On invalid mode usage (create/unpublished target with `publish_changes`), set a concise `item.Issue`. Keep it visible in Sync and Report.

6) Tests
- Unit/UI:
  - Defaulting from combinations of source/target publish flags (including “both published” → default Publish).
  - Cycling skips invalid states and wraps correctly.
  - Publish&Changes falls back to Draft for creates or unpublished targets and sets Issue; badge shows `[Pub+∆]` where applicable.
  - Dev mode hint rendering.
- Propagation: `P` on folder applies to all children and their subtrees; `P` on a story applies to siblings and their subtrees.
- Integration (sync path): stub API and assert publish flag per mode; verify “overwrite then unpublish” sequence occurs when specified.

## Open Questions / Later Improvements

- Global apply: quick actions to set all selected stories to Draft/Publish?
- Reports: include chosen publish mode in saved report entries.
- Core API: add a formal publish override to reduce UI coupling (later).

## Acceptance Criteria

- Preflight shows and allows cycling Publish/Draft/Publish&Changes for stories; folders unaffected.
- Defaulting follows the updated policy (source state mirrored; overwrite when both published); dev‑mode hints appear when applicable.
- Publish&Changes only applies to updates of already published target stories; otherwise falls back to Draft with an inline issue; badge `[Pub+∆]` used consistently.
- When a published source is set to Draft and the target is published, the tool overwrites content and unpublishes the target so the final state is Draft.
- Sync honors the chosen mode and surfaces any API limits/errors.

---

## Rate‑Limit Budget & Worker Allocation (Final Optimization)

We will optimize concurrency to honor Storyblok limits while maximizing throughput, accounting for added API calls (notably `UnpublishStory`). This is a separate, last step after the publish toggle lands.

### Current State

- Per‑host limiter in `internal/sb/transport.go` with adaptive RPS and retries.
- Per‑space read/write buckets in `internal/core/sync/space_limiter.go` used by `StorySyncer` around MA calls.
- UI schedules up to `maxWorkers` (currently 1 during folder phase, then up to 6) without explicit knowledge of expected API “cost” per item.

### Objective

- Tune `maxWorkers` dynamically so the expected write pressure does not exceed the per‑space write bucket and avoids 429s, while keeping workers busy.
- Incorporate different API footprints per publish mode, especially the “overwrite then unpublish” case which costs an extra write.

### API Cost Model (initial)

- Story Update/Create (Draft/Publish/Publish&Changes): ~1 write, 1–2 reads (raw fetch + existence check fallback).
- Overwrite then Unpublish (Published source set to Draft on published target): ~2 writes (update + unpublish), 1–2 reads.
- Folder Create: 1 write (+ small read overhead).

We will refine these using measured metrics deltas per item.

### Plan

1) Instrumentation
- Use existing `sb.MetricsSnapshot` deltas per item (already captured) to compute `writesPerItem` and `readsPerItem` rolling averages by item type/mode.
- Extend per‑item result to include `WriteDelta`/`ReadDelta` derived from snapshots for faster averaging (UI already tracks `Retry429`).

2) Desired Concurrency Estimation
- Maintain rolling averages: `avgWritePerItem`, `avgReadPerItem`, and `avgItemDuration` over the recent window.
- Estimate sustainable concurrent items from current write bucket capacity and host limiter ceiling:
  - `targetWriteRPS = min(spaceWriteRPS, hostWriteCeiling)` (derive from limiter configs and recent adaptive cap).
  - `writesPerSecondPerWorker ≈ avgWritePerItem / avgItemDurationSeconds`.
  - `desiredWorkers = clamp( floor(targetWriteRPS / writesPerSecondPerWorker), 1, 12 )`.
- Apply smoothing (EMA) to avoid oscillation and only change by ±1 at a time.

3) Mode‑Aware Budgeting
- When scheduling next items, sum the expected writes of currently running items plus the candidate’s expected writes (use cost model by mode). Only schedule if the sum over the next `Δt` remains under the write bucket burst. This prevents spikes from multiple “update+unpublish” tasks in parallel.

4) Adaptive Backoff on 429s
- On spikes in 429 rate (windowed `warningRate`), immediately reduce `maxWorkers` by 1–2 and increase again slowly after the rate subsides (like TCP congestion control).

5) Integration with SpaceLimiter
- Option A (non‑intrusive): keep SpaceLimiter as is; use it as a guard at API call time, while the scheduler attempts to keep measured pressure under capacity.
- Option B (later): expose a lightweight `TryReserveWrites(n)` from SpaceLimiter to pre‑reserve budget before scheduling; if reservation fails, delay scheduling.

6) UI Wiring
- Reuse `statsTick()` to recompute desired workers every 500ms based on the latest averages and warning/error rates; update `m.maxWorkers` accordingly (still honoring folder‑first sequencing).

7) Tests
- Simulate higher write cost (e.g., many items needing unpublish) → controller reduces workers.
- Simulate no 429s and low cost → controller gently increases workers up to ceiling.
- Ensure the controller never schedules stories during folder phase and never exceeds configured caps.

### Out of Scope (for now)

- Cross‑space global back‑pressure; we scope per‑run/per‑space only.
- Fine‑grained per‑endpoint budgets; method‑level read/write split is sufficient.
