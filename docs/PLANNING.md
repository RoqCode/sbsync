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

- Create CDA client
- Add shared retrying transport used by both MA and CDA clients.
- Add per‑host rate limiters (MA and CDA) and simple counters.

Details

- Retries: trigger on 429/502/503/504 and transient I/O.
- Retry‑After: parse seconds and HTTP‑date formats; cap total attempt time.
- Backoff: exponential with jitter (base 250ms, cap 5s, attempts 3–5); respect context cancel.
- Limiters: MA default 5 rps, burst 6; CDA default 10 rps.
- Instrumentation: counters for requests by client, status buckets, retries, backoff sleeps.

Acceptance

- Requests exhibit correct pacing and backoff in tests.
- Retries honor Retry‑After and stop on context cancel.

Tests

- Deterministic transport tests for header parsing, jitter bounds, limiter pacing, cancellation.

### Phase 2: CDA Token Resolution During Scanning (per selected space)

Scope

- Resolve CDA tokens up front and store per run; fall back cleanly.

Details (single source of truth)

- Discover the CDA public/preview token for the selected source space via Management API (if permissions allow).
- If discovery fails or is not permitted, mark as “no CDA token” and use MA reads for hydration as needed.

Security

- Never log tokens; redact in any error messages; keep in memory only; clear on exit.

Acceptance

- Token resolution follows precedence; failures fall back to MA reads without breaking sync.

Tests

- discovery > fallback flows; discovery denied; logs redacted.

### Phase 3: Pre‑Hydration Stage via CDA (before writes)

Scope

- Perform a dedicated “hydration” stage for all selected stories before starting per‑item writes. Continue to list by MA (authoritative); use CDA for content.

Details

- Build a hydration queue from the finalized preflight plan (stories only; skip folders).
- Batch hydration: group slugs/UUIDs (e.g., 50–100 per request) to maximize CDA efficiency and bound memory.
- Draft/unpublished: use preview token + `version=draft`.
- Published & Changes handling:
  - For stories that are published but have unpublished changes (“Published & Changes”), fetch BOTH published and draft versions during hydration.
  - Detection strategies (pick one, based on API capabilities):
    - Prefer API metadata that flags this state (if available in list/get responses).
    - Or, when preview token is available: fetch both versions and compare canonicalized `content`; if they differ, mark as “has unpublished changes”.
- Hydration cache records may carry up to two content blobs per story when needed: `publishedContent` and `draftContent` (to minimize memory, store only present variants).
- Hydration cache: store `content` blobs keyed by story ID/slug; wire ContentManager to read from this cache first.
- Fallback: for items missing in the cache (hydration miss/failure), let ContentManager fall back to MA `GetStoryWithContent` during the sync step for that item only.
- Memory guards: cap concurrent batches; store only `content` (not full payloads); progressively clear cache once items are processed.
- UI: show a brief “Hydrating content …” progress step (hydrated/total) before writes begin.

Acceptance

- Pre‑hydration runs before writes; hydration cache hits for most items; MA fallback used only for unhydrated items.
- Significant reduction in MA reads; correctness preserved for draft/published.

Tests

- Batch reduces call count; ContentManager uses hydration cache; fallback to MA correct; memory caps honored.

### Phase 4: Write Worker Pool, De‑dup/Idempotency (folders pre‑created)

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

### Phase 5: TUI Performance Panel (Throughput & Workers)

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

### Phase 6: Target Hydration + Diff / No‑op Detection (Optional)

Scope

- Avoid updates when target payload would be identical to source by comparing pre‑hydrated source content against target content hydrated in bulk.

Details

- After Phase 3 pre‑hydrates source content, perform a matching “target hydration” stage:
  - Enumerate target stories in scope (by slug set from the preflight plan).
  - Hydrate target content in CDA batches (or MA if CDA not available), using the same batching/memory guards.
- Canonicalize both source and target content: sort keys; strip read‑only/transient fields; for arrays, match by `_uid`.
- Compare canonical forms; if equal, mark the item as no‑op and drop it from the final write plan.
- Safety: still keep raw‑payload invariants for remaining updates; do not change existing write ordering (e.g., Published & Changes sequence).

Acceptance

- No‑op updates are skipped; collision and publish logic unaffected.
- Hydration cache usage verified for both source and target; memory caps respected.

Tests

- Canonicalization correctness (maps/arrays); skip logic avoids false positives.
- Target hydration reduces per‑item reads; memory bounded; end‑to‑end write plan excludes no‑ops.

### Phase 7: Dynamic Concurrency & Circuit Breakers

Scope

- Adapt concurrency based on error rates; protect the system during server issues.

Details

- Ramp up worker count gradually to a ceiling under low error rates.
- Back off on bursts of 429/5xx; add a lightweight breaker that reduces concurrency temporarily.

Acceptance

- Under normal conditions, concurrency rises to the ceiling; under errors, it backs off and recovers.

Tests

- Ramp‑up/down behavior; breaker trips and recovers; throughput preserved.

## Configuration

- `SB_MA_RPS`, `SB_MA_BURST`, `SB_MA_RETRY_MAX`, `SB_MA_RETRY_BASE_MS`.
- `SB_CDA_RPS`, `SB_CDA_RETRY_MAX`, `SB_CDA_PER_PAGE`, `SB_CDA_VERSION`.
- CDA tokens are discovered per selected space (Phase 2). Avoid pre‑configuring tokens in env/config to prevent mismatch with selected spaces.
- Security: never log tokens; redact sensitive values.

## Observability

- Counters/timers: requests by client, status buckets, retries, backoff sleeps.
- Throughput (items/sec) and limiter rates; TUI stats panel (Phase 5).
- Debug logs gated by `DEBUG`; avoid logging large payloads or secrets.

## Edge Cases & Consistency

- Mid‑run source changes: consider snapshotting space `cv`/timestamp and passing to CDA.
- Locales: ensure hydration covers required locales; merge correctly in payloads.
- Relations/assets: avoid CDA options that resolve references if writer expects raw JSON.

## Test Matrix (Per Phase)

- Phase 1: retry/backoff/limiting; Retry‑After parsing; context cancel.
- Phase 2: token precedence; MA discovery allowed/denied; fallback path; redaction.
- Phase 3: batch hydration; draft vs published; fallback correctness; memory caps.
- Phase 4: concurrent folder creation; per‑path locking; coalescing; idempotency.
- Phase 5: stats sampling bounded; non‑blocking updates.
- Phase 6: canonicalization and skip; no false positives.
- Phase 7: dynamic concurrency adjustments; breaker behavior and recovery.
