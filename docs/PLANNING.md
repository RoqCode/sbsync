# Component Sync — MVP and Future Plan

This plan restructures TODO #6 (Component sync) into a minimal, shippable MVP that mirrors Storyblok’s blind overwrite behavior, and a set of future optimizations to layer on later. Each MVP iteration is small and independently testable.

MVP Goals

- Blind overwrite parity: create/update components in target based on source without schema diffing or merge logic.
- Sync Mode picker: screen after space selection to choose what to sync (Stories or Components); accessible again from list and success views.
- Usable browse experience: search, sorting (updated/created/name), and a date‑cutoff filter for components. (Group filter deferred.)
- Robust mapping: ensure component groups and internal tags exist in target and are mapped correctly.

MVP Non‑Goals (deferred)

- Schema diffing, breaking‑change detection, and merge UI.
- Dependency analysis and topological ordering.
- Backups/snapshots and dry‑run validator.

Vocabulary

- Component: Storyblok component with `name`, optional `display_name`, optional `component_group_uuid`, and JSON `schema`.
- Group map: name→UUID mapping to remap `component_group_uuid` and any `component_group_whitelist` entries within field schemas (e.g., bloks/richtext).

High‑Level Architecture (MVP)

- `internal/sb`: List/Get/Create/Update components; List/Create component groups; List/Create internal tags. Reuse transport, retries, and metrics.
- `internal/core/componentsync`: Planner/executor for blind overwrite; helpers for group/whitelist remapping and internal tag ensuring.
- `internal/ui`: Add Sync Mode picker; components list tree (group → components) with search/sort/filter; preflight with name collision handling (Skip/Fork); success view with navigation back to picker.
- Sync engine parity: Reuse the stories worker-pool pattern for concurrent execution and progress reporting.

Architecture Decision

- Separate flows: Implement Components as a dedicated flow to keep concerns clean and avoid Story-specific assumptions (slugs, folders, publish modes). Use `internal/core/componentsync` with parallel UI under `internal/ui/components`.
- Reuse patterns: Mirror Stories patterns for list tree, preflight decisions (Skip/Apply/Fork), worker scheduling with SpaceLimiter, and reporting, without forcing a generic Story abstraction.
- Minimal shared helpers: Optionally extract tiny utilities (selection toggles, a small worker scheduler helper) where it reduces duplication without coupling domain specifics. Keep SpaceLimiter shared as-is.
- Refactor later: After MVP, evaluate safe consolidation opportunities based on duplication observed in practice.

MVP Iterations

1) Sync Mode Picker & Navigation — DONE
- Outcome: New screen after SpaceSelect to choose Stories vs Components; reachable via a keybind from the Components list and Success views to return to picker.
- Scope: `ModePicker` model/view, route wiring, and help footer updates.
- Tests: Model routing tests; verify navigation from list/success back to picker.

2) Core Types & Fixtures — DONE (types), fixtures TBD
- Outcome: Types for `Component` and `Group`; fixtures under `testdata/components/` and `testdata/component_groups/`.
- Scope: `Component{Name, DisplayName, GroupUUID, Schema, ID, CreatedAt, UpdatedAt}`; `Group{UUID, Name}`; fixture loaders.
- Tests: Unmarshal from fixtures; timestamp parsing; stable normalization.

3) SB Client: List/Get (Components, Groups) — DONE
- Outcome: `ListComponents`, `GetComponentByName`, `ListComponentGroups`.
- Scope: Parse API JSON; paging if applicable. Build `GroupMap` (name→uuid).
- Tests: Parsing fixtures and error surfacing.

4) SB Client: Create/Update (Components), Groups Create, Internal Tags — DONE
- Outcome: `CreateComponent`, `UpdateComponent`, `CreateComponentGroup`, `ListInternalTags`, `CreateInternalTag`.
- Scope: Implement payload shaping. Treat 409/422 on create as idempotent. Ensure `internal_tag_ids` can be set explicitly.
- Tests: Request/response shaping; idempotent create behavior.

5) Browse Components (Search, Sort, Date‑Cutoff) — IN PROGRESS (Group filter deferred)
- Outcome: Flat components list with:
  - Search by name (basic toggle now; text input follows)
  - Sorting: by `UpdatedAt`, `CreatedAt`, or `Name` (asc/desc) [DONE]
  - Filters: date‑cutoff quick toggle (today on/off) [DONE]; group filter [Deferred]
- Scope: Client‑side sort/filter; accept textual date input in a later step; selection toggles mirror Stories UX; show updated date per row.
- Tests: Sorting/date‑cutoff tests [DONE]; add search input tests later.

6) Component Groups Sync & Mapping — DONE
- Outcome: Ensure all source groups exist in target; build `GroupMap` for mapping.
- Scope: During scan or preflight, list groups, create missing target groups; map `component_group_uuid` and `component_group_whitelist` via `GroupMap`.
- Tests: Fixtures with missing/existing groups; verify whitelist remapping.

7) Internal Tags Ensure — DONE
- Outcome: Ensure component internal tags exist in target and apply via `internal_tag_ids`.
- Scope: Read source `internal_tags_list`; create missing tags in target (`object_type=component`); set `internal_tag_ids` on create/update.
- Tests: Existing vs missing tags; payload contains final IDs; error propagation as item issues.

8) Preflight (Name Collisions, Skip/Fork) — DONE
- Outcome: Preflight screen summarizing actions with per-item decision: Skip, Apply (overwrite), or Fork.
- Scope: Collision check by name only; if a source component name exists in target, default to Apply (overwrite) but allow Skip or Fork. Fork prompts for a new component name (suffix suggestion), and schedules a create under that name.
- Tests: Model tests for decision cycling, fork name entry/validation, and persistence of choices.

9) Planner (Blind Overwrite + Decisions) — DONE
- Outcome: Plan with Create vs Update using target name→ID map and user decisions from Preflight; honor list filters (including date‑cutoff).
- Scope: Classify items; transform Fork decisions into Create actions with the chosen name; collect mapping info for executor.
- Tests: Table tests for classification and fork transformation.

10) Executor (Blind Overwrite, Concurrent Workers) — DONE
- Outcome: Execute Create/Update with retries; map groups/whitelists; set `internal_tag_ids`; handle 422 as update fallback.
- Scope: Use the stories worker-pool pattern (configurable concurrency). Ensure group creation step has completed before execution. Payload uses source fields; for schema use source verbatim except remapped whitelist. Progress + per‑item result.
- Tests: Stub client; assert call order/payloads; concurrency respects worker limits; cover 422 fallback path.

11) Reporting & Success View — DONE
- Outcome: Integrate results into existing report; success view offers navigation back to Mode Picker.
- Scope: Extend report minimally; update help/footer with return action.
- Tests: Golden report coverage and navigation tests.

Future Optimizations

- Schema Diff Engine & Safety: Structured diffs, breaking‑change detection, and gated overrides.
- Dependency Analysis: Build DAG of component references and sync in topological order; warn on cycles/missing deps.
- Diff UI: Expandable tree and summaries for breaking vs non‑breaking changes.
- Backups/Snapshots: Export target component schemas before overwrites to a timestamped location.
- Dry‑Run Validator: Estimate impact on stories; report risks without writes.
- Presets Advanced Sync: Full diff and partial updates beyond basic create/update.
- Rate‑Limit Budgeting: Mode‑aware worker sizing based on measured API costs.
- Shared abstractions: Identify and consolidate duplicate scheduler/preflight utilities across Stories and Components where it improves maintainability without over-coupling.

Testing & Tooling Notes

- Table‑driven tests; avoid network. Fixtures: `testdata/components/`, `testdata/component_groups/`, `testdata/internal_tags/`.
- Date parsing: accept `YYYY-MM-DD` and full RFC3339 (e.g., `2025-09-06T12:00:00Z`); default to local midnight when only a date is provided.
- Keep packages cohesive: `internal/core/componentsync/{plan,exec,map}` for MVP.
- Run: `go fmt ./... && go vet ./... && go test ./...` before merging each iteration.

Acceptance Criteria (MVP)

- Sync Mode picker appears after selecting spaces and is reachable from Components list and Success views.
- Components browse supports selection toggles, search, sorting by updated/created/name (asc/desc), and date‑cutoff filtering. (Group filter not required.)
- Preflight exists for components and mirrors Stories UX with name-based collision handling: Skip, Apply (overwrite), and Fork (copy-as-new with rename input).
- Groups are created and mapped before component execution; `component_group_uuid` and field `component_group_whitelist` are correctly remapped to target UUIDs.
- Internal component tags are created as needed and applied via `internal_tag_ids` on both create and update.
- Executor uses concurrent workers analogous to Stories; blind overwrite executes using deterministic create/update decisions (name→ID map) with create→update fallback; progress and reporting are shown.
