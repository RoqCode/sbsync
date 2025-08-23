# Storyblok Sync TUI

Storyblok Sync is a terminal user interface (TUI) that synchronises Stories and Folders between two Storyblok spaces. It allows users to scan source and target spaces, select content, preview collisions and apply changes.

## Features
- Scan source and target spaces and list stories with metadata.
- Mark individual stories or entire folders for synchronisation.
- Fuzzy search over name, slug and path.
- Preflight collision check with per-item skip or overwrite decisions.
- Sync engine that creates or updates stories in the target space with progress and error reporting.
- Rescan at any time to refresh space data.

## User Flow
1. **Welcome/Auth** – enter a token or load it from `~/.sbrc`.
2. **SpaceSelect** – choose source and target spaces.
3. **Scanning** – fetch stories for both spaces.
4. **BrowseList** – navigate and mark stories.
5. **Preflight** – review collisions and choose actions.
6. **Syncing** – apply the sync plan.
7. **Report** – see which items succeeded or failed.

Interrupts: `r` to rescan, `q` to abort.

## Development Roadmap (v0)
- **T1 – Config & Auth:** load/save token and optional space IDs via `~/.sbrc`.
- **T2 – Storyblok Client:** HTTP client with retries, rate limiting and basic endpoints.
- **T3 – SpaceSelect Screen:** select source/target spaces in the UI.
- **T4 – Scan + List View:** scan both spaces and display a flat list of stories.
- **T5 – Fuzzy Search:** filter stories by name, slug or path.
- **T6 – Preflight Collisions:** detect path/slug collisions and let users skip or overwrite.
- **T7 – Sync Executor:** apply create/update operations with progress and retry logic.
- **T8 – Report & Rescan:** final summary and option to rescan or quit.
- **T9 – Keybind Help:** context-sensitive shortcut overlay.
- **T10 – Logging:** structured logging, no telemetry.

## Project Structure
```
storyblok-sync/
├─ cmd/sbsync/            # entry point
├─ internal/
│  ├─ ui/                 # Bubble Tea models, views and keybinds
│  ├─ sb/                 # Storyblok API client
│  └─ config/             # token/config loading and saving
└─ testdata/              # JSON fixtures
```
Future modules (`internal/core`, `infra`) will house the sync logic and infrastructure helpers described in the roadmap.

## Contributing
See [AGENTS.md](AGENTS.md) for coding conventions and testing requirements. Submit pull requests with a short description of the change and note any deviations from the guidelines.
