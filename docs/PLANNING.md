# Planning

This plan presents concrete, incremental implementation steps to improve performance and robustness. Each phase is small, testable, and safe to ship independently. It builds on the current architecture: UI (Bubble Tea) → Core Sync (`internal/core/sync`) → Storyblok client (`internal/sb`).

Links:

- Top‑level roadmap: ../README.md#todo-next-steps
- Architecture: ./ARCHITECTURE.md

## Goals

- Respect Storyblok Management API (MA) limits while increasing throughput.
- Reduce read/write volume; avoid no‑ops; preserve raw‐payload invariants.
- Keep the TUI responsive with clear progress and live stats.
- Maintain determinism and strong test coverage.

## Phased Plan (Incremental)

Each phase specifies scope, acceptance criteria, and tests.

### Phase 1: Transport, Rate Limiting, Retries, Instrumentation

Scope

- Add a shared retrying HTTP transport used by the Management API client.
- Add per‑host rate limiter and simple counters.

Details

- Retries: trigger on 429/502/503/504 and transient I/O.
- Retry‑After: parse seconds and HTTP‑date formats; cap total attempt time.
- Backoff: exponential with jitter (base 250ms, cap 5s, attempts 3–5); respect context cancel.
- Limiter: default 5 rps, burst 6 for MA.
- Instrumentation: counters for requests, status buckets, retries, backoff sleeps.

Acceptance

- Requests exhibit correct pacing and backoff in tests.
- Retries honor Retry‑After and stop on context cancel.

Tests

- Deterministic transport tests for header parsing, jitter bounds, limiter pacing, cancellation.

### Phase 2: On‑Demand Reads, No Pre‑Hydration

Scope

- Fetch content on demand via MA for each item during sync; remove pre‑hydration logic.

Details

- ContentManager calls `GetStoryWithContent` only when needed; maintains a small in‑memory cache.
- Keep write flow unchanged (raw create/update, UUID alignment); use on‑demand reads for correctness.

Acceptance

- Sync reads exactly what it needs; no separate hydration step; correctness equivalent to MAPI payloads.

Tests

- Ensure `EnsureContent` uses MA and cache; verify sync flow remains correct.

### Phase 3: Write Worker Pool, De‑dup/Idempotency (folders pre‑created)

Scope

- Introduce a bounded worker pool for story/folder writes; rely on the preflight folder‑creation step to ensure all folders exist before story writes.

Details

- Worker pool: 6–8 workers sharing the MA limiter.
- Folders: created as explicit items from Phase 3’s optimized preflight (folders first). Story writes assume folders exist.
  - Current implementation note: we still call `EnsureFolderPathStatic` before each story as a safety net; we can remove this once confidence in the preflight is high.
- De‑dup: coalesce queued operations per slug (last‑writer wins within the run).
- Idempotency: continue using `force_update`; safe to replay updates.

Follow‑ups
- Add a temporary feature flag to disable the per‑story `EnsureFolderPathStatic` safety net.
- After validating in tests and a few real runs that folders‑first execution is reliable (no missing parent errors), remove the safety net and the flag.

Published & Changes write sequence
- For items marked “has unpublished changes” and with both payloads available:
  1) Write the published payload with `publish=1` (raw update or create), to set the published baseline.
  2) Immediately write the draft payload with `publish=0`, to re‑apply the unpublished changes.
- For published‑only items: single write with `publish=1`.
- For draft‑only (unpublished) items: single write with `publish=0`.
- Raw‑payload invariants remain in effect (preserve unknown fields, translate slugs, parent resolution, UUID alignment).

- Acceptance

- Folder items are executed first; subsequent story writes do not attempt to create folders.
- Duplicate enqueues for the same slug are coalesced; no wasted writes.
- “Published & Changes” items preserve state after sync (two writes in the correct order).

Tests

- Concurrency: parallel items target same path → exactly one create chain; coalescing verified.
- Published & Changes: verifies dual‑write order; published‑only and draft‑only paths verified; raw invariants preserved.

### Phase 4: TUI Performance Panel (Throughput & Workers)

Scope

- Add a non‑blocking stats panel to the Sync view with live throughput (items/sec) and worker utilization (active/total).

Design Overview

- Sampling model: record item completion timestamps in a fixed‑size ring buffer; compute moving throughput over a short window (e.g., last 15–30s).
- Update cadence: recompute display at most every 500ms via a Tea tick; no tight loops, no per‑frame heavy work.
- Worker utilization: maintain counters of active/running workers vs configured max; show as a compact bar and numeric fraction.
- Thread‑safety: stats updates are done only inside Bubble Tea’s Update loop, driven by messages (no background goroutines modifying model state).

Implementation Plan

1) Model additions (internal/ui/types.go)
   - Add `stats` struct to `Model`:
     - `completedTimes []time.Time` (ring buffer with capacity, e.g., 512)
     - `head int` (ring index)
     - `window time.Duration` (e.g., 15s)
     - `lastSample time.Time` (to throttle updates)
     - `itemsPerSec float64`
     - `activeWorkers int` and `maxWorkers int` (live view of current worker pool)
   - Keep fields small to avoid increasing copy costs of the model.

2) Sync integration (internal/ui/update_main.go)
   - On each `syncResultMsg`, append `time.Now()` to the ring buffer.
   - Maintain `activeWorkers` by:
     - Increment when scheduling a worker (`RunRunning` set) and decrement when a worker produces a `syncResultMsg`.
     - Set `maxWorkers` from the current pool policy (Phase 3 fixed value, Phase 5 will adapt).

3) Sampling command (internal/ui/update_main.go)
   - Introduce a `statsTickMsg` driven by `tea.Tick(500 * time.Millisecond, ...)` while in `stateSync`.
   - On `statsTickMsg`, prune timestamps older than `now - window`, compute `itemsPerSec = float64(len(windowEvents)) / window.Seconds()`.
   - Re‑enqueue the next tick only while `state == stateSync`.

4) View (internal/ui/view_sync.go)
   - Render a single‑line or two‑line panel under the progress header, e.g.:
     - `Throughput: 3.2 it/s  |  Workers: [◼◼◼◻◻◻] 3/6`
   - Use existing lipgloss styles; keep to a fixed width budget and truncate gracefully.
   - Only compute derived strings from already‑stored `itemsPerSec`, `activeWorkers`, `maxWorkers` in the view function (no scanning data here).

5) Ring buffer helper (internal/ui/utils.go or dedicated file)
   - Provide small helpers: `statsInit(capacity)`, `statsAppend(ts time.Time)`, `statsPruneBefore(cutoff time.Time)`.
   - Avoid allocations: reuse the fixed slice and wrap around via modulo.

6) Non‑blocking guarantees
   - No logging inside the tick handler; O(n) prune cost bounded by ring capacity (≤512).
   - Cap window to 30s; cap tick at ≥250ms if the UI becomes tight on small terminals.

7) Feature flag (optional)
   - `SB_TUI_STATS=0/1` env var to enable/disable the panel quickly if needed.

Acceptance Criteria

- The stats panel appears in `stateSync` and updates roughly twice per second.
- Under sustained load, no visible UI slowdown or input lag; CPU overhead remains negligible.
- Items/sec stabilizes to a reasonable value; worker bar reflects active workers.

Testing Strategy

- Unit tests for ring buffer prune/append behavior and items/sec calculation boundaries (empty window, full window, rolling).
- Update loop tests: feed synthetic `syncResultMsg` at known intervals; assert computed `itemsPerSec` within tolerance.
- View tests: ensure rendering uses precomputed fields and does not panic on small widths.
- Concurrency: verify counters update only via messages; no data races in tests with `-race`.

Rollout Steps

1) Land ring buffer and sampling without rendering (behind a no‑op view).
2) Add simple one‑line panel; verify correctness in tests and manual runs.
3) Iterate on styling and spacing; keep truncation to fit common terminal widths.
4) Optional: gate via `SB_TUI_STATS` if any regressions appear.


### Phase 5: Dynamic Concurrency & Circuit Breakers

Scope

- Adapt concurrency based on error rates; protect the system during server issues.

Details

- Ramp up worker count gradually to a ceiling under low error rates.
- Back off on bursts of 429/5xx; add a lightweight breaker that reduces concurrency temporarily.

Acceptance

- Under normal conditions, concurrency rises to the ceiling; under errors, it backs off and recovers.

Tests

- Ramp‑up/down behavior; breaker trips and recovers; throughput preserved.

### Phase 6: Sync Robustness & UX

Scope

- Improve pause/resume behavior, error surfacing, and reporting ergonomics. Optional persistence of run state.

Details

- Pause/cancel smoothing; ensure idempotent resume from the next pending item.
- Clearer per‑item errors in the Report view; grouped summaries.
- Optional: persist run state (preflight selection + per‑item status) to allow resume after program restart.

Acceptance

- Users can pause/cancel/resume smoothly; errors are easy to find and act on.
- (If enabled) A program restart can resume the run with the same preflight selection.

Tests

- Resume picks up the correct next item; idempotency preserved.
- Report formatting remains stable; summaries reflect actual outcomes.

## Configuration

- `SB_MA_RPS`, `SB_MA_BURST`, `SB_MA_RETRY_MAX`, `SB_MA_RETRY_BASE_MS`.
- Security: never log tokens; redact sensitive values.

## Observability

- Counters/timers: requests by client, status buckets, retries, backoff sleeps.
- Throughput (items/sec) and limiter rates; TUI stats panel (Phase 5).
- Debug logs gated by `DEBUG`; avoid logging large payloads or secrets.

## Edge Cases & Consistency

- Mid‑run source changes: consider snapshotting MA `cv`/timestamp where relevant.
- Locales: ensure MA reads cover required locales; raw invariants remain.
- Relations/assets: avoid resolving references if writer expects raw JSON.

## Test Matrix (Per Phase)

- Phase 1: retry/backoff/limiting; Retry‑After parsing; context cancel.
- Phase 2: on‑demand MA reads; cache behavior; correctness retained.
- Phase 3: concurrent folder creation; per‑path locking; coalescing; idempotency.
- Phase 4: stats sampling bounded; non‑blocking updates.
- Phase 5: dynamic concurrency adjustments; breaker behavior and recovery.
- Phase 6: resume behavior; report formatting and correctness.
