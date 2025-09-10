# Component Preset Sync — MVP Parity Plan

Goal: Add preset synchronization to the existing component sync with parity (where sensible) to Storyblok’s reference implementation. No extras beyond optimizations that do not change behavior.

Scope Overview

- Read all presets from source space, filter per component.
- On component create: push all its source presets to target (POST).
- On component update: compute new vs existing by name; create new (POST) and update existing (PUT).
- Preserve and pass through `image` if present; no upload pipeline in MVP.
- Keep existing behaviors: group mapping, whitelist remap, internal tag ensuring, create→update fallback, rate limiting.

API Surface (internal/sb)

- Types:
  - `ComponentPreset { id, name, component_id, preset(json), image }`.
  - Extend `Component` to optionally decode `all_presets` when returned by API.
- Endpoints:
  - GET `spaces/{space}/presets` → list all presets.
  - POST `spaces/{space}/presets` with `{ "preset": { name, component_id, preset, image } }`.
  - PUT `spaces/{space}/presets/{id}` with same shape.

Core Helpers (internal/core/componentsync)

- `FilterPresetsForComponentID(all, compID)`: select presets for a component.
- `DiffPresetsByName(src, tgt)`: return `(new, update)` by comparing names; for updates carry target `id`.
- `NormalizePresetForTarget(p, targetComponentID)`: drop `id`, set `component_id`, keep `name`, `preset`, `image`.
- JSON equality for update minimization is optional; MVP can always PUT existing-name presets.

Execution Flow Changes (internal/ui)

- Pre‑apply (shared prep):
  - Fetch `srcPresets := ListPresets(sourceSpaceID)` once.
  - Fetch `tgtPresets := ListPresets(targetSpaceID)` once (for update diffing).
- Per component:
  - After mapping groups/whitelists and mapping internal tags:
    - Create path:
      - `created := CreateComponent(...)`
      - `srcCompPresets := FilterPresetsForComponentID(srcPresets, sourceComp.ID)`
      - For each: `CreatePreset(targetSpaceID, NormalizePresetForTarget(p, created.ID))`
    - Update path:
      - `srcCompPresets := FilterPresetsForComponentID(srcPresets, sourceComp.ID)`
      - `tgtCompPresets := FilterPresetsForComponentID(tgtPresets, targetComp.ID)` (or `all_presets` if available)
      - `(new, upd) := DiffPresetsByName(srcCompPresets, tgtCompPresets)`
      - POST each in `new`; PUT each in `upd`.
- Rate limits: reuse component write limiter for POST/PUT preset calls.
- Logging: report counts created/updated; final "Presets in sync" per component.

Edge Cases & Parity Decisions

- Name collisions: name is the matching key across spaces.
- Missing `all_presets`: space‑level `GET /presets` is the source of truth.
- Image handling: pass through source `image` field; no cross‑space re‑upload in MVP.
- Deletes: do not delete target‑only presets.
- Order: no ordering guarantees; Storyblok’s lib pushes sequentially; we can keep sequential per component while still using worker pool across components.

Step‑by‑Step Implementation

1) sb types and endpoints
- Add `ComponentPreset` type and JSON wiring.
- Extend `Component` with `AllPresets` (omitempty).
- Implement `ListPresets`, `CreatePreset`, `UpdatePreset` on `Client`.
- Tests: request paths, payload shape, response decoding.

2) Core helpers
- Add `presets.go` with filter/diff/normalize helpers.
- Tests: table‑driven diffing (name‑based), normalization sets `component_id`, preserves `name/preset/image`.

3) UI prep integration
- In components apply prep, fetch `srcPresets` and `tgtPresets` once.
- Plumb them into executor/init messages.

4) Create path wiring
- After successful `CreateComponent`, filter/normalize and POST presets for that component.
- Log count; handle errors per item without aborting other items.

5) Update path wiring
- Before `UpdateComponent` response handling completes, compute `(new, upd)` using `tgtPresets`.
- POST new, PUT existing; log counts and errors.

6) Concurrency & limits
- Reuse SpaceLimiter writes for preset POST/PUT.
- Keep per‑component preset operations sequential to match reference behavior.

7) Tests & fixtures
- Add unit tests for helpers; client tests for new endpoints.
- Add integration‑style executor test with fake API covering create and update scenarios.

8) Docs & acceptance
- Update README component sync notes to mention presets.
- Acceptance: For a component with presets in source, target ends up with same set by name, with payloads (including image) mirrored; existing presets updated; no deletes.

Non‑Goals (MVP)

- Image upload to target assets (S3 signed URL flow).
- Deleting target‑only presets.
- Advanced diff of `preset` JSON for minimal PUTs.

Rollout Plan

- Feature‑flag internally if needed; otherwise enable by default.
- Verify on a small subset of components; confirm counts and names.
- Monitor rate‑limits; adjust limiter nudges if needed.
