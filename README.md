# Storyblok Sync TUI

Storyblok Sync is a terminal user interface (TUI) that synchronises Stories and Folders between two Storyblok spaces. It allows users to scan source and target spaces, select content, preview collisions and apply changes.

See also:

- Architecture overview: [docs/ARCHITECTURE.md](./docs/ARCHITECTURE.md)
- Planning and roadmap details: [docs/PLANNING.md](./docs/PLANNING.md)
- Environment flags reference: [docs/env.md](./docs/env.md)

## Features

- Scan source and target spaces and list stories with metadata.
- Mark individual stories or entire folders for synchronisation.
- Fuzzy search over name, slug and path.
- Preflight collision check with per-item skip or overwrite decisions.
- Sync engine that creates or updates stories in the target space with progress and error reporting.
- Rescan at any time to refresh space data.
- Prefix filter for listing/selection only (no bulk "starts-with" execution).

## User Flow

1. **Welcome/Auth** – enter a token or load it from `~/.sbrc`.
2. **SpaceSelect** – choose source and target spaces.
3. **Scanning** – fetch stories for both spaces.
4. **BrowseList** – navigate and mark stories.
5. **Preflight** – review collisions and choose actions.
6. **Syncing** – apply the sync plan.
7. **Report** – see which items succeeded or failed.

Interrupts: `r` to rescan, `q` to abort.

## Status

- v1.0: Functional and stable TUI for syncing Stories and Folders between Storyblok spaces. Implemented flow: auth, space selection, scan, browse with search, preflight, sync with retries, and final report. Logging is available via `DEBUG` to `debug.log`.

## TODO (Next Steps)

1. Robust rate limiting & retries — completed

2. Copy-as-new under different slug (collision handling) - completed

3. UX improvements - completed

4. Toggle publish state - completed

5. Security & logging - completed

6. Component sync - completed

- Mode toggle: switch between Stories and Components in the UI.
- API: extend client to list/get/create/update components; handle groups and display names.
- Browse/search: fuzzy search by name, group, and schema keys; filter by group.
- Collision check: detect name/group conflicts and schema diffs.
- Diff/merge: JSON schema diff with collapse/expand; highlight breaking changes (type, required, enum shrink).
- Dependencies: resolve nested component references; compute sync order; warn on missing dependencies.
- Safety/validation: block breaking changes by default or gate behind confirmation; optional dry‑run validator to check impact on existing stories.
- Backups: export target component schemas before overwrite; store under `testdata/` or timestamped snapshots.
- Tests: fixtures for components and dependency graphs; diff and ordering tests.

7. CI & releases

   Goals

   - Fast, deterministic CI for PRs and main.
   - Automated, repeatable releases with checksums and multi-arch binaries.

   Continuous Integration (GitHub Actions)

   - Workflow: `.github/workflows/ci.yml` on `pull_request` and `push` to `main`.
   - Runners/Matrix: `ubuntu-latest`, Go `1.25.x` (extend to macOS/Windows later if needed).
   - Steps:
     - `actions/checkout@v4` (with fetch-depth: 0 for tags when releasing).
     - `actions/setup-go@v5` with caching on `go.sum`.
     - `go mod download` to populate cache.
     - Format check: `gofmt -l .` and fail if any files listed.
     - `go vet ./...` for static checks.
     - `staticcheck` install and run: `go install honnef.co/go/tools/cmd/staticcheck@latest` then `staticcheck ./...`.
     - Tests: `go test ./... -race -covermode=atomic -coverprofile=coverage.out`.
     - Build sanity: `go build ./cmd/sbsync` to catch linker issues.
     - Artifacts: upload `coverage.out` and the built binary for debugging (optional).
   - Best practices: enable concurrency cancellation per-branch; cache Go build/mod; keep CI under ~3–4 minutes.

   Releases (GoReleaser)

   - Config: `.goreleaser.yaml` with builds for linux/darwin/windows, `amd64` and `arm64`; binary name `sbsync`, main `./cmd/sbsync`.
   - Archive: tar/zip per-OS, checksums file, SBOM optional.
   - Changelog: conventional-commit grouping; exclude chore/docs/refactor by default.
   - Homebrew/NFPM: optional later; start with GitHub Releases only.
   - Workflow: `.github/workflows/release.yml` triggered on tag push `v*.*.*`:
     - Checkout with full history; setup Go 1.25.
     - Use `goreleaser/goreleaser-action@v5` to run `goreleaser release --clean`.
   - Permissions: set repository Actions “Workflow permissions” to “Read and write” so `GITHUB_TOKEN` can create releases.
   - Signing (optional later): add `cosign` for detached signatures and `provenance`/SBOM if supply-chain hardening is desired.

   Repository Settings (recommended)

   - Protect `main`: require PRs, 1–2 approvals, and the CI job to pass before merge.
   - Require linear history or squash merges for clean release notes.
   - Enable Dependabot (`.github/dependabot.yml`) for `gomod` and `github-actions`.
   - Create `CODEOWNERS` to enforce reviews; keep `AGENTS.md` as the contributor guide.
   - Actions: ensure GitHub Actions is enabled for the repo; set “Read and write” workflow permissions.

   Rollout Plan

   - 7.1 Add CI workflow file and verify on a PR.
   - 7.2 Add `.goreleaser.yaml` and release workflow; push a prerelease tag (`v1.1.0-rc.1`).
   - 7.3 Review artifacts (checksums, archives), test binaries on macOS/Linux.
   - 7.4 Cut `v1.1.0` with generated release notes.

8. Dry-run mode (low priority)

   - Core: no-op write layer that still produces full reports.
   - UI toggle; clear messaging in Report view.
   - Tests verifying zero write calls and identical plan.

9. CLI-only mode

   Goals

   - Enable non-interactive sync from CI or scripts without launching the TUI.
   - Parity with core features: stories and components, filters, preflight, dry-run, and reporting.

   CLI Shape

   - Subcommand or flags: `sbsync sync --from <spaceID> --to <spaceID> [--stories|--components] [--prefix <path>] [--group <name>] [--since <YYYY-MM-DD>] [--publish] [--concurrency N] [--overwrite|--skip|--fork-suffix "-copy"] [--preflight-only] [--dry-run] [--report out.json] [--yes]`.
   - Auth: read token from env (`STORYBLOK_TOKEN`) or `internal/config` store; forbid interactive prompts when `--yes` not set.
   - Output: human-readable progress to stdout; machine-readable JSON report file (same schema as TUI report).
   - Exit codes: `0` success; `2` preflight found blocking issues; `3` partial failures beyond threshold; `4` invalid flags/config.

   Scope

   - Reuse existing scan → preflight → plan → execute pipeline from the core packages.
   - Implement minimal flag parser (stdlib `flag` or cobra); keep command surface small and stable.
   - Deterministic logs without secrets; optional `--verbose` for debug.

   Tests

   - Table tests for flag parsing and validation.
   - Golden tests for JSON report output on fixtures.
   - E2E dry-run against testdata to assert planning parity with TUI.

10. Datasources sync

   Goals

   - Sync Storyblok Datasources and their entries between spaces for parity across environments.

   Scope

   - API: extend client with List/Get/Create/Update for datasources and datasource entries; handle paging.
   - Mapping: match datasources by slug; entries by key; upsert values; optional create-missing-only mode.
   - Preflight: detect missing datasources, key collisions, and potential destructive changes (deletes disabled by default).
   - UI/CLI: browse/search datasources and entries; filters by slug/prefix; include in CLI with `--datasources` and `--ds-slug` filters.
   - Safety: no deletes by default; add `--allow-delete` gate later with explicit confirmation.
   - Backups: optional export of target datasources to `testdata/` or timestamped snapshots before writes.

   Tests

   - Fixtures under `testdata/datasources/` and `testdata/datasource_entries/`.

11. Sync process refactor

   Goals

   - Reduce duplication between Stories and Components flows; centralize shared orchestration, decisions, and reporting.

   Approach

   - Extract a small sync engine in `internal/core/sync` (or `internal/core/engine`) with:
     - Scheduler/worker pool with SpaceLimiter and retries.
     - Preflight decision model (Skip/Apply/Fork) and persistence.
     - Common report/result structures and progress events.
     - Hooks/interfaces for domain-specific parts (scan, identity, classify, apply, validators).
   - Keep domain packages (`storiesync`, `componentsync`) thin: implement adapters to the shared engine.
   - Maintain behavior parity; avoid over-generalizing domain rules.

   Tests

   - Unit tests for engine components (scheduler, decisions, reporting).
   - Migration tests to ensure stories/components produce identical plans/reports before/after refactor.

## Project Structure

```
storyblok-sync/
├─ cmd/sbsync/            # entry point
├─ internal/
│  ├─ ui/                 # Bubble Tea models, views and keybinds
│  ├─ sb/                 # Storyblok API client
│  ├─ config/             # token/config loading and saving
│  └─ core/
│     └─ sync/            # domain sync core (planner/orchestrator/syncer)
└─ testdata/              # JSON fixtures
```

The sync core has been extracted to `internal/core/sync`. Future modules (`infra`) will house infrastructure helpers described in the roadmap. For a deeper dive into responsibilities and data flow, see [docs/ARCHITECTURE.md](./docs/ARCHITECTURE.md).

## Contributing

See [AGENTS.md](AGENTS.md) for coding conventions and testing requirements. Submit pull requests with a short description of the change and note any deviations from the guidelines.

## Future ideas

- Interactive Diff & Merge view for collisions

  - Side-by-side JSON diff, `_uid`-aware arrays, per-field choose left/right
  - Persist decisions to a merged payload used by sync

- RichText preview

  - Detect RichText fields (root `type=doc`) in story content.
  - Add preview toggle in Diff and Browse: raw JSON vs rendered preview.
  - Implement minimal renderer (paragraphs, headings, bold/italic, links, lists); truncate long blocks with expand.
  - Sanitize/link handling; no external fetches; keep it fast and safe.
  - Tests with fixtures under `testdata/` for common node types and edge cases.
