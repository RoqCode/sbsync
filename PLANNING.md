# Planning

This document collects concrete, actionable plans and notes. It links to the high‑level TODOs in the README and expands on the performance improvements track.

## Links to Roadmap / TODOs

- README: TODO (Next Steps) — see [README.md#todo-next-steps](README.md#todo-next-steps)
- Relevant items:
  - 2) Robust rate limiting & retries
  - 6) Performance & caching
  - 1) Extract domain core (helps isolate sync engine from UI and clients)

## Goals

- Maximize throughput while staying under Storyblok Management API (MA) limits (6 rps).
- Reduce total requests by batching/coalescing and skipping no‑ops.
- Keep the UI responsive and progress reporting accurate.
- Maintain correctness: no accidental overwrites, predictable retries, and clear errors.

## Strategy Overview

- Split read/write responsibilities:
  - Use Content Delivery API (CDA) for reads (faster, CDN), reserving MA budget for writes.
  - Keep MA for listing identifiers/metadata during scan and for all writes.
- Centralize rate limiting and retries:
  - Global limiter guarding every MA request; separate limiter for CDA.
  - Robust retry with jitter and `Retry-After` honoring for 429/5xx.
- Reduce request count:
  - Batch CDA reads via pagination and filters.
  - Skip no‑op writes via local diff/hashing.
  - De‑duplicate operations targeting the same story within a run.

## Detailed Plan: Performance Improvements

1) Clients and Rate Limiting
- Create `internal/sb/ma` (Management API client) and `internal/sb/cda` (Content Delivery client).
- Add limiters:
  - MA: `rate.NewLimiter(5 rps, burst 6)` by default (configurable). All writes must `Wait(ctx)`.
  - CDA: separate limiter with a higher ceiling (configurable), adaptive down on 429.
- Retry policy (both clients):
  - Retry on 429, 502, 503, 504, and transient I/O; honor `Retry-After` when present, else exp backoff with jitter (base 250ms, cap 5s, attempts 3–5).
  - Ensure request bodies are replayable (buffered or `GetBody`).

2) Read Path via CDA
- Scan phase: keep MA “list-stories” to enumerate IDs/slugs/metadata (authoritative set).
- Hydration phase: use CDA to fetch content for the enumerated stories:
  - Prefer large `per_page` (e.g., 100) and filters (e.g., by folder, by slugs/uuids) to minimize calls.
  - For drafts/unpublished sources, use a preview token and `version=draft`.
  - If a story is not retrievable from CDA (permission/state), mark and fall back to MA read for that item only.
- Cache coordination:
  - For published reads, pass `cv` (cache version) to ensure freshness; obtain once per run from MA if needed.
  - For draft reads, use `version=draft` (bypasses CDN cache).

3) Write Path via MA
- Enqueue create/update operations into a bounded worker pool (e.g., 6–8 workers).
- Every HTTP call goes through the MA limiter and retry transport.
- Idempotency:
  - Include item version/lock fields if required by MA to avoid races.
  - Guard non‑idempotent mutations with a per‑item idempotency key or at least ensure safe replays in the sync layer.

4) Request Reduction & No‑op Skips
- Local diffing:
  - Compare source vs target content; deep compare `content`, slug, path, publish state; ignore read‑only fields.
  - Optionally hash canonicalized JSON to quickly detect equality.
- De‑duplication:
  - Coalesce multiple operations for the same story during a run; last‑writer wins within the queue.
- Pagination sizing:
  - Use maximum safe `per_page` for CDA list endpoints to reduce round‑trips.

5) Concurrency Model
- Single global MA limiter; all write workers share it.
- Separate CDA limiter for read hydration.
- Optional priority: drain writes first once preflight decisions are locked; otherwise interleave based on queue readiness.

6) Configuration
- Env/flags via `internal/config`:
  - `SB_MA_RPS` (default 5), `SB_MA_BURST` (6), `SB_MA_RETRY_MAX` (4), `SB_MA_RETRY_BASE_MS` (250).
  - `SB_CDA_RPS` (default 10), `SB_CDA_RETRY_MAX` (3).
  - `SB_CDA_VERSION`: `published` or `draft` (when preview token present).
  - `SB_CDA_PER_PAGE`: default 100.
  - `SB_ADAPTIVE_LIMITING`: true/false.

7) Instrumentation & Observability
- Minimal counters/timers:
  - Requests by client (CDA/MA), status (2xx/4xx/5xx), retries, backoff sleeps.
  - Throughput (items/sec) and current limiter rates.
- Expose in debug logs and a light TUI “stats” panel during sync.

8) Edge Cases & Consistency
- Mid‑run source changes:
  - Snapshot source view: read space `cv` or timestamp at start; pass to CDA reads for consistency.
- Locales:
  - Ensure CDA hydration covers all locales needed for the write; merge into per‑locale payloads.
- Relations/assets:
  - Avoid CDA options that resolve references if the writer expects raw JSON; keep payloads canonical.

9) Testing Plan (deterministic)
- Transport tests:
  - 429 handling with and without `Retry-After`.
  - Exponential backoff with jitter bounds; context cancel respected.
  - Limiter pacing ~5 rps for MA under load.
- CDA hydration tests:
  - Batch pagination reduces call count; per‑story fallback when CDA denies access.
  - Draft vs published reads, with/without preview token.
- Sync engine tests:
  - No‑op skip via hash/deep‑equal; de‑dupe logic correctness.
  - Concurrency ordering deterministic under seeded scheduler/fake clock.
  - Locale merging correctness.
- Fixtures under `testdata/` covering: small stories, nested components, large payloads, multi‑locale.

10) Implementation Sketch (incremental)
- `internal/sb/transport.go`: shared retrying RoundTripper + limiter.
- `internal/sb/ma/client.go`: MA client using the transport; write methods.
- `internal/sb/cda/client.go`: CDA client using the transport; batch list/get.
- `internal/core/sync` (follow‑up refactor): orchestrate scan (MA list), hydrate (CDA), plan, and execute (MA writes).
- Wire config via `internal/config` and thread into clients.

## Open Questions
- Are there endpoints for true bulk writes we could leverage? If not, keep to parallelized single writes.
- Minimum MA rate we should default to for safety in shared environments?
- What’s the acceptable tail latency for a typical sync in the TUI (to tune pool sizes)?

## Next Steps
- Wire separate CDA/MA clients with transport + limiters.
- Switch hydration to CDA with batching; add fallback to MA for misses.
- Add diff+hash no‑op skip before enqueuing writes.
- Instrument and expose minimal stats; add deterministic tests.

