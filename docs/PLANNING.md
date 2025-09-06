# Component Sync — Iteration Plan

This plan breaks TODO #6 (Component sync) into small, stackable iterations. Each iteration is independently testable with deterministic fixtures under `testdata/` and unit tests using the standard `testing` package.

Goals

- Add a Components mode in the TUI to browse, diff, and sync component schemas from a source space to a target space.
- Detect and communicate collisions, schema diffs, and dependency ordering.
- Default to safe behavior; block or gate breaking changes. Provide backups and dry‑run validation paths.

Non‑Goals (for now)

- Field‑level merge UI for component schemas.
- Automatic migration of existing Stories to satisfy breaking changes.
- Full RichText or content preview for component examples.

Vocabulary

- Component: Storyblok component, identified by `name`, optional `display_name`, optional `group`, and JSON `schema`.
- Schema diff: Structural comparison of JSON (type/required/enum changes, added/removed keys, option changes).
- Breaking change: A change likely to invalidate existing content (e.g., type change, required added, enum shrink).
- Dependency: Reference to other components (e.g., `component`, `blocks`, or `options.source=internal` with component lists) establishing a sync order.

High‑Level Architecture

- `internal/sb`: Extend client to List/Get/Create/Update components; List/Create component groups; List/Create internal tags; presets helpers. Reuse retries/metrics like Stories.
- `internal/core/componentsync`: New focused package with planner, diff, dependency analysis, and executor.
- `internal/ui`: Mode toggle (Stories/Components) and views for browse, preflight, diff, and report.

Iteration 1 — Core Types & Fixtures

- Outcome: Common types for components, groups, and schema nodes; initial test fixtures committed under `testdata/components/`.
- Scope: Define `corecomponents.Component` with `Name`, `DisplayName`, `Group`, `Schema` (as `map[string]any` or `json.RawMessage`), and `ID` (target/source). Define `corecomponents.Group` with `UUID`, `Name`. Provide a `GroupMap` helper (name→uuid) and fixture loaders used by tests.
- Tests: Load fixture pairs (source/target) to ensure type unmarshalling and stable JSON normalisation (key order insensitive).

Iteration 2 — SB Client: List/Get

- Outcome: `sb.Client.ListComponents(ctx, spaceID)` and `GetComponent(ctx, spaceID, name)`.
- Scope: Implement calls with existing transport, error wrapping, and metrics. Convert API JSON into `corecomponents.Component`.
- Tests: Use canned JSON in tests (no network). Table tests for parsing, pagination (if applicable), and error surfacing.

Iteration 2a — SB Client: Component Groups

- Outcome: `sb.Client.ListComponentGroups(ctx, spaceID)` and `CreateComponentGroup(ctx, spaceID, name)`.
- Scope: Map API JSON to `corecomponents.Group`. Provide `GroupMap` builder (name→uuid) for mapping and diffing. Handle idempotent create semantics (ignore 422/conflict).
- Tests: Parsing fixtures and idempotent create behavior.

Iteration 3 — SB Client: Create/Update

- Outcome: `CreateComponent(ctx, spaceID, Component)` and `UpdateComponent(ctx, spaceID, Component)`.
- Scope: Implement minimal payloads (name/group/display_name/schema). Ensure idempotent update semantics.
- Tests: Request body shaping and response parsing via fixtures; retry paths for 429/5xx simulated by stub transport.

Iteration 4 — UI Mode Toggle & Skeleton

- Outcome: Toggle between Stories and Components. Empty list screen wired to fetch action later.
- Scope: Add global mode state and a basic Components route in Bubble Tea; keybinding to switch modes.
- Tests: Model update unit tests: mode toggles and view routing work; help footer shows correct hints.

Iteration 5 — Scan & Browse Components

- Outcome: Fetch and list components for both spaces with counts; basic fuzzy search by name.
- Scope: Wire `ListComponents` in a scan step. Render list with `group/name` and a status badge (New/Exists/OnlyTarget).
- Tests: Model tests using fixtures; search filters results; status computation matches expectations.

Iteration 5.1 — Component Groups Sync & Mapping

- Outcome: Ensure all source groups exist in target; compute `GroupMap` for mapping UUIDs by group name.
- Scope: During scan, list groups for both spaces; create any missing target groups; build `GroupMap`. Expose `GroupMap` to planner/executor. This runs before component diff/plan to avoid spurious diffs from missing groups.
- Tests: Fixtures with missing/existing groups; assert create calls and resulting map.

Iteration 6 — Collision Detection (Name/Group)

- Outcome: Identify collisions where same `name` has differing `group`/`display_name`. Mark as Conflict in UI.
- Scope: Matching strategy by `name` (canonical). Show per‑row badges: `[New]`, `[Update]`, `[Conflict]`, `[TargetOnly]`.
- Tests: Table tests for pairing and conflict classification.

Iteration 7 — Schema Diff Engine (Minimal)

- Outcome: Pure function that compares two component schemas and returns a structured diff with severity.
- Scope: Detect added/removed fields, type changes, required changes, enum additions/removals. Flag breaking vs non‑breaking. Canonicalize fields referencing groups (`component_group_whitelist`) by comparing against group names (via `GroupMap`) to avoid false diffs from differing UUIDs.
- Tests: Fixtures for each case; stable output regardless of key order; readable string summary for the UI.

Iteration 8 — Dependency Analysis

- Outcome: Extract dependencies from schema (component/blocks references). Build DAG and provide topological order.
- Scope: Handle nested/array cases and guard against cycles with a clear error (cycle path in message).
- Tests: Graph tests for simple chains, branching, and cycles. Ensure order respects prerequisites.

Iteration 9 — Planner (Preflight Plan)

- Outcome: Build an executable plan from source/target snapshots, collisions, diffs, and dependency order.
- Scope: Items: Create, Update, Skip, Conflict. Carry diff summary and dependency info. Default policy: block breaking changes.
- Tests: Plan construction over fixture pairs; verifies ordering and correct gating on breaking diffs.

Iteration 10 — UI Preflight View

- Outcome: Show components with status, diff summary, and allow per‑item decision: Skip/Apply (and override to allow breaking with confirmation).
- Scope: List view with filter by group; keybind to open a detail pane with diff summary. Gated confirmation for breaking changes.
- Tests: Model update tests for toggles and confirmation flow; rendering smoke tests (string contains badges).

Iteration 11 — Executor (Safe Path)

- Outcome: Execute Create/Update per plan with retries and progress reporting. Breaking changes blocked unless explicitly allowed in item.
- Scope: Use a deterministic create/update decision from the target name→ID map (avoid create‑then‑update), but handle 422 as a fallback to update. Map `component_group_uuid` to the target UUID via `GroupMap`. For `bloks`/`richtext` fields, rewrite `component_group_whitelist` using `GroupMap`. Use `CreateComponent`/`UpdateComponent`. Respect dependency order (topo) and per‑item decisions. Produce a result report.
- Tests: Stub client to capture calls; assert order and payloads. Verify blocked items are skipped with issue note.

Iteration 11.1 — Internal Tags

- Outcome: Ensure internal component tags exist in target and are applied explicitly on create and update.
- Scope: Add client methods: `ListInternalTags(spaceID)` and `CreateInternalTag(spaceID, name, objectType=component)`. Before syncing a component, ensure all `internal_tags_list` exist in target; collect IDs and set `internal_tag_ids` explicitly on both create and update (backend merge can be non‑recursive). Avoid non‑awaited loops; in Go use simple loops or `errgroup` for parallelism with proper waits.
- Tests: Fixtures for existing/missing tags; assert creation calls and final payload IDs. Error handling when tag creation fails should be surfaced as item issues but not crash the whole sync.

Iteration 11.2 — Presets Sync (Optional)

- Outcome: Sync component presets alongside component create/update.
- Scope: Client helpers to list presets for a space/component and create/update them. On component create: add all source presets. On update: diff presets (by name/key) and submit creates/updates minimally.
- Tests: Fixtures for presets on source/target; verify correct create/update sets and association with component IDs.

Iteration 12 — Backups (Target Snapshot)

- Outcome: Write pre‑update target component schemas to `testdata/snapshots/<timestamp>/components.json` (or a configured path).
- Scope: Add backup step when any Update is scheduled. Ensure directory creation and JSON formatting are deterministic.
- Tests: Filesystem tests writing to a temp dir; JSON contains expected set and shapes.

Iteration 13 — Diff UI (Expand/Highlight)

- Outcome: Expandable diff view highlighting breaking changes (type/required/enum shrink). Collapse non‑breaking by default.
- Scope: Minimal viewer: left/right summaries and a focused field list for breaking changes. No full tree view yet.
- Tests: Rendering unit tests for diff summaries; breaking markers appear as expected.

Iteration 14 — Optional Dry‑Run Validator

- Outcome: Optional analysis to estimate impact on stories by scanning known schemas and referencing fields. No writes.
- Scope: Flag as experimental; behind `SB_COMPONENT_DRYRUN=1`. Summarize potential risk (counts by component reference in source snapshot if available).
- Tests: Deterministic counts via fixtures; UI shows a concise note when enabled.

Iteration 15 — Reporting & Telemetry

- Outcome: Include component operations in the existing sync report (JSON). Add per‑item issues and diff summaries.
- Scope: Reuse report plumbing; extend types minimally. Ensure no secrets leak.
- Tests: Golden report tests with redaction.

Iteration 16 — Polishing & Docs

- Outcome: Finalize help text, errors, and docs. Add README feature notes and usage tips.
- Scope: Review keybindings, badges, and messages; update `docs/PLANNING.md` status; document environment flags.
- Tests: Lint/vet clean; CI passes.

Testing & Tooling Notes

- Use table‑driven tests; avoid network. Fixtures live under `testdata/components/` and `testdata/components_graphs/`.
- JSON normalization helper for stable diffs (marshal with sorted keys for tests if needed).
- Prefer small, focused packages: `internal/core/componentsync/{diff,plan,deps,exec}` for cohesion.
- Run: `go test ./...` and `go vet ./...` before merging each iteration.

Open Questions

- Naming collisions across groups: do we allow auto‑rename for target creates? (Out of scope initially.)
- Display name divergence: treat as non‑breaking but visible change.
- Schema option diffs (e.g., plugin options) breadth; start minimal and expand with real cases.

Acceptance Criteria (Feature)

- Components mode lists source/target components with status and supports search/filter by name and group.
- Component groups are synced first and mapped by name; component `component_group_uuid` and field `component_group_whitelist` are correctly remapped to target UUIDs.
- Preflight shows diff summaries; breaking changes highlighted and blocked unless explicitly confirmed per item.
- Sync uses deterministic create/update decisions (name→ID map) with 422 fallback, executes in dependency order, and produces a report.
- Internal component tags are created as needed and applied via `internal_tag_ids` on both create and update.
- Target schemas are snapshotted before overwrites.
