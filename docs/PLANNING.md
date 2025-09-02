## Interactive Diff & Merge (Preflight)

This document proposes an incremental plan to add an interactive Diff & Merge view for colliding stories discovered during Preflight. Users will be able to review differences and produce a merged story payload that the sync engine applies to the target space.

### Goals
- Allow resolving collisions by producing a merged story payload, not just skip/overwrite
- Side‑by‑side, collapsible JSON diff focused on `content`, slugs, and essential metadata
- Keyboard‑first controls; fast on large JSON payloads; deterministic output
- Persist merge decisions per item and integrate cleanly into the sync flow

### Non‑Goals (Initial)
- RichText rendering (covered by separate task)
- Component schema diff/validation beyond JSON structure
- Cross‑item merge operations (only per story)

---

### UX Flow (Phase 1)
1. In Preflight, collision items get an "Open Diff" action (e.g. press `d` or `enter`).
2. Enter a new screen state `stateDiff` with:
   - Header: story name, slug, collision summary
   - Two panes: Source (left) and Target (right)
   - Tree view of fields; unchanged nodes collapsed by default
   - A decision column per changed node showing: left | right | mixed
3. Navigation/Actions (initial set):
   - Arrows / `j` `k` `h` `l`: move/select/expand/collapse
   - `space` or `←/→`: choose left/right for the selected node
   - `A` accept‑all left; `D` accept‑all right (within current subtree)
   - `/` search by JSON path substring; `n` next match, `N` prev
   - `s` save decisions and return to Preflight; `q` cancel changes and return
   - `?` help overlay
4. Saving produces a merged payload snapshot for that item and marks the item as "merge" in Preflight (distinct from skip/update).
5. During Sync, items with a merged payload use it for write operations instead of plain source payload.

---

### Data Model & Integration
- Extend `internal/core/sync.PreflightItem` with optional merge fields (names indicative):
  - `MergeMode string` // "none" | "merged"
  - `MergeSummary string` // short human‑readable summary for Preflight row
  - `MergedPayload json.RawMessage` // normalized Storyblok payload to write
- UI will maintain an in‑memory `UIDiffPlan` (path → choice) while in `stateDiff` and serialize to `MergedPayload` on save.
- Sync path changes:
  - If `MergedPayload != nil`, the writer uses it instead of fetched source payload for create/update (still honoring invariants below).
  - Publish behavior and UUID handling remain governed by existing plan/policy.

Note: If modifying core types is undesirable in Phase 1, the UI can keep `MergedPayload` in a sidecache keyed by slug and inject it via the sync adapter. Phase 2 can migrate to typed fields in `PreflightItem`.

---

### Diff Model
- Input: two normalized JSON payloads per story (Source, Target)
  - Normalize by removing read‑only/noise fields and schema‑invariant differences prior to diff
  - Focus fields: `name`, `slug`, `parent_id` (derived from folder), `content` subtree, `translated_slugs` (as attributes without IDs)
- Ignored by diff (handled by invariants in writer): `id`, `uuid` (synced via separate call), `created_at`, `updated_at`, `first_published_at`, `published_at`, `full_slug`, `cv`
- Node types: map, array, scalar
- Map diff: key union; classify `added`, `removed`, `modified`, `unchanged`
- Array diff: component arrays are matched by `_uid` when present; otherwise positional with best‑effort matching by `component` and key subset
  - For `_uid` arrays: detect add/remove/modify (moves initially treated as modify)
  - For plain arrays: treat as positional; consider an optional tolerance for small reorder (Phase 2)

Representation:
- Each node has a path (JSON Pointer‑like, e.g., `/content/body/1/props/title`)
- Each node carries a `diffKind`: unchanged | added_left | added_right | removed_left | removed_right | modified
- Each node carries a `decision`: auto | left | right | mixed (derived)

---

### Merge Rules (Phase 1)
- Default: choose Source for all changed fields (consistent with overwrite), but allow overriding per node
- Per‑node decisions cascade to children; children can override parent
- Array merging:
  - `_uid` arrays: union by `_uid`, apply per‑element decisions; new elements from either side included if their parent node decision allows
  - Positional arrays: choose entire array left or right, or per‑index override where length aligns; deep granular per‑index merge in Phase 2
- Special handling:
  - `slug`/`full_slug`: allow explicit choice; warn if parent path constraints would break; writer still ensures folder path correctness
  - `translated_slugs`: compare by `lang` key; drop IDs; union with per‑lang choice
  - `parent_id`: not directly chosen; resolved from folder path by planner/writer
- Writer invariants after merge:
  - Strip read‑only fields; convert `translated_slugs` → `translated_slugs_attributes` without IDs
  - Ensure `parent_id` is set appropriately
  - Apply publish flag per plan/policy
  - After write, update UUID to match Source if present (existing behavior)

---

### UI Design (Bubble Tea)
- New state: `stateDiff`
- New view: `internal/ui/view_diff.go`
  - Renders header, side‑by‑side panes, tree list with diff markers
  - Shows decision column; unchanged collapsed by default; counters for changed nodes
- Update handlers: `internal/ui/handlers_diff.go`
  - Key handling for navigation, decisions, search, save/cancel
  - Maintains `UIDiffPlan` and selection
- Integration points:
  - Preflight list adds an action to open diff for collision items
  - Preflight item row shows a small badge when a merge has been saved

---

### Fetching Data
- On opening diff:
  - Fetch full Source and Target story payloads via `internal/sb` (draft + published as appropriate)
  - Apply a normalization pass before diff (remove read‑only, format fields, drop nulls that are semantically absent)
- Caching: retain fetched/normalized payloads while in diff to avoid re‑fetch on expand/search

---

### Performance & UX Guarantees
- No background goroutines mutating model; all updates via messages
- Lazy tree materialization: build node lists on expand to keep initial render fast
- Limit heavy recomputations; only recompute decisions/diff for affected subtree
- Provide progress indicator when fetching payloads; handle errors gracefully

---

### Keybindings (Phase 1)
- Navigation: `↑/↓/←/→` or `j/k/h/l`
- Expand/Collapse: `→/←` or `l/h`
- Choose Left/Right: `←` = left, `→` = right, `space` toggles between left/right
- Accept All (subtree): `A` left, `D` right
- Search path: `/` open, `enter` confirm, `n`/`N` cycle
- Save: `s` (serialize merged payload and return)
- Cancel: `q` (discard changes and return)
- Help: `?`

---

### Phased Implementation
1) Phase 1 – Minimal viable merge
   - Side‑by‑side diff for maps/scalars; `_uid` array support (add/remove/modify as items)
   - Per‑node left/right choice; accept‑all on subtree
   - Save merges to `MergedPayload`; Preflight marks item as merged
   - Sync consumes `MergedPayload` for write operations
   - Acceptance:
     - Can resolve a collision by merging and successfully syncing the merged result
     - Large payloads remain responsive (initial load < 300ms on typical payloads)

2) Phase 2 – Array granularity and polish
   - Positional array per‑index overrides; move detection for `_uid` arrays (visual cue only)
   - Better search, breadcrumbs, and change counters; partial rendering optimizations
   - Acceptance: merging arrays with mixed adds/removes/modifies works predictably

3) Phase 3 – Validation & ergonomics
   - Pre‑save validation: basic schema expectations, required fields present
   - Conflict warnings for slugs/parent path; clearer summaries in Preflight row
   - Acceptance: merge UI prevents obvious invalid payloads; summaries aid review

---

### Testing Strategy
- Unit tests (pure functions):
  - Normalization: removal of read‑only fields and ID stripping for `translated_slugs`
  - Map/array diff with `_uid` matching; scalar comparisons; path correctness
  - Merge application from decisions → merged JSON
- Integration tests (UI):
  - Open diff for a fixture collision; make decisions; save; Preflight reflects merge
  - Sync path uses `MergedPayload` and writes successfully to the mocked API
- Performance checks: large nested JSON with hundreds of nodes

---

### Risks & Mitigations
- Complex array diffing: start with `_uid` keyed arrays; defer reorders for Phase 2
- Invalid payloads post‑merge: add a light validator and rely on writer invariants
- State bloat in model: keep `UIDiffPlan` compact (path → enum) and compute merged only on save

---

### Open Questions
- Should we support a three‑way merge with a common base (future)?
- Do we need per‑field validators (e.g., slugs/locales) in Phase 1, or rely on writer?
- How to display binary/asset changes succinctly?
