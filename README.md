# Storyblok Sync TUI

Storyblok Sync is a terminal user interface (TUI) that synchronises Stories, Folders, and Components between two Storyblok spaces. It lets you scan source/target, select content, preflight collisions, and apply changes — now including component group mapping, internal tags, and presets.

See also:

- Architecture overview: [docs/ARCHITECTURE.md](./docs/ARCHITECTURE.md)
- Planning and roadmap details: [docs/PLANNING.md](./docs/PLANNING.md)
- Environment flags reference: [docs/env.md](./docs/env.md)

## Disclaimer

- This project is under active development; behavior may change and things may break.
- No warranty, promise of support, or liability is provided. Use at your own risk. I accept no responsibility for unintended outcomes, including data loss.
- Sync operations can be destructive. Create backups/exports and test in non‑production spaces first where possible.
- Use with caution: review preflight results, start with small batches, and verify outcomes.
- After each sync, double‑check all affected items in Storyblok to confirm they were updated as intended.

## Installation

There are two supported ways to install `sbsync`:

### 1) Download a prebuilt binary (recommended)

We publish multi‑platform archives on GitHub Releases via GoReleaser:

- Linux (amd64/arm64): `sbsync_<version>_linux_<arch>.tar.gz`
- macOS (amd64/arm64): `sbsync_<version>_darwin_<arch>.tar.gz`
- Windows (amd64/arm64): `sbsync_<version>_windows_<arch>.zip`

Steps (Linux/macOS):

1. Download the archive matching your OS/arch from the latest release.
2. Verify checksum (optional but recommended):
   ```sh
   # in the folder with the downloaded files
   shasum -a 256 -c checksums.txt | grep sbsync_<version>_<os>_<arch>
   ```
3. Extract and place the binary on your `PATH`:
   ```sh
   tar -xzf sbsync_<version>_<os>_<arch>.tar.gz
   sudo install -m 0755 sbsync /usr/local/bin/sbsync
   ```
4. macOS Gatekeeper note: if you see a quarantine warning, allow the binary once or remove the flag:
   ```sh
   xattr -d com.apple.quarantine /usr/local/bin/sbsync
   ```

Steps (Windows):

1. Download the `.zip` for your architecture.
2. Extract `sbsync.exe` and add its folder to your `PATH`, or run it directly.

### 2) Build from source

Prerequisites: Go 1.25+

```sh
git clone <this-repo-url>
cd storyblok-sync
go build ./cmd/sbsync
# optional: place on PATH
sudo install -m 0755 sbsync /usr/local/bin/sbsync
```

### 3) Curl‑to‑install script (for CI)

Use the provided installer to fetch the latest (or a pinned) release and install into a directory on your PATH.

Latest:

```sh
curl -fsSL https://raw.githubusercontent.com/RoqCode/storyblok-sync/main/scripts/install.sh \
  | bash -s -- -b /usr/local/bin
```

Pinned version:

```sh
curl -fsSL https://raw.githubusercontent.com/RoqCode/storyblok-sync/main/scripts/install.sh \
  | bash -s -- -v v1.2.3 -b /usr/local/bin
```

Notes:

- The script verifies the `checksums.txt` when possible (requires `sha256sum` or `shasum`).
- Set `GITHUB_TOKEN` in CI to avoid GitHub API rate limiting when resolving the latest tag.
- Use `-b "$HOME/.local/bin"` for user‑local installs; add it to your PATH.
- Windows runners can use the script in GitHub Actions’ bash shell; it downloads the `.zip` and installs `sbsync.exe`.

### Quick start

```sh
# provide your Storyblok token via env or config file
export SB_TOKEN=<your_token>

# run the TUI
sbsync

# optional: enable debug logs to debug.log
DEBUG=1 sbsync --verbose
```

`sbsync` also reads a config file at `~/.sbrc` (created/saved by the app) with keys:

```
SB_TOKEN=<token>
SOURCE_SPACE_ID=<space_id>
TARGET_SPACE_ID=<space_id>
```

## Features

- Stories: scan, browse, fuzzy search, preflight, sync (create/update), report.
- Folders: hierarchy planning, create/update, publish mode handling.
- Components: scan, browse, preflight, and sync (create/update) with:
  - Group remapping: maps `component_group_uuid` and whitelist UUIDs via name.
  - Internal tags: ensures tags exist and sets `internal_tag_ids`.
  - Presets: parity with Storyblok’s flow (POST new, PUT existing by name), including image passthrough.
  - Force-Update toggle in preflight to update “no changes” items for preset propagation.
- Stats panel: live Req/s with instantaneous Read/Write RPS and success/sec, plus worker bar.
- Rescan and mode switch between Stories and Components.

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

- Components sync MVP complete: groups, internal tags, and presets in sync (create/update) with preflight Force-Update.
- Stories/Folder sync stable: end-to-end flow with retries, rate-limiting, and reporting.
- Logging available via `DEBUG=1` to `debug.log`.

## Next Steps

- Preset images: optional asset upload flow to target space (S3 signed URL) when cross-space URLs aren’t valid.
- Component deletions and preset cleanup: detect target-only presets/components (optional/guarded).
- Component diff UX: structural JSON diff and breaking-change highlighting (types/required/enums).
- Component browse filters: group filter and schema key search.
- Dry-run mode: no-op writes with full report and risk summary.
- CLI mode: non-interactive sync (stories/components) for CI.
- CI & releases: keep staticcheck, vet, tests enforced; GoReleaser for multi-arch binaries.

Completed (high level)

- Robust rate limiting & retries
- Copy-as-new (stories) and folder fork
- Publish mode toggle and unpublish-after overwrite handling
- Security/logging improvements
- Component sync MVP (groups, internal tags, presets)

8. CLI-only mode

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

9. Datasources sync

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

10. Sync process refactor

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

11. Dry-run mode (low priority)

- Core: no-op write layer that still produces full reports.
- UI toggle; clear messaging in Report view.
- Tests verifying zero write calls and identical plan.

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

## License

Licensed under the GNU General Public License, version 3 (GPL-3.0). See `LICENSE` for details.

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
