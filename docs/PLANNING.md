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

- Add a stats panel to the Sync view with live throughput and worker utilization.

Details

- Items/sec: maintain a rolling window via a small ring buffer; update ~500ms.
- Workers: display active vs total workers as compact bars.
- Non‑blocking: rendering must be inexpensive and must not block sync.

Acceptance

- Panel updates periodically during sync; no visible UI jitter or slowdown.

Tests

- Sampling logic bounded; panel updates without blocking; works under idle and loaded runs.

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
