# AGENTS

## Overview

Welcome to **storyblok-sync**, a Go-based terminal UI (TUI) for synchronising Stories and Folders between two Storyblok spaces. The codebase follows a clean separation of concerns and uses Bubble Tea for the UI.

### Repository layout

```
storyblok-sync/
├─ cmd/sbsync/            # application entry point
├─ internal/
│  ├─ ui/                 # Bubble Tea models, views and update logic
│  ├─ sb/                 # Storyblok API client
│  └─ config/             # token/config loading and saving
├─ go.mod / go.sum
└─ testdata/              # fixtures for tests
```

Future packages (e.g. `internal/core/`, `infra/`, etc.) should follow the structure outlined in the project roadmap.

## Coding conventions

- **Go version:** 1.25
- **Formatting:** run `go fmt` (uses tabs for indentation)
- **Imports:** standard library groups first, third‑party packages second, internal packages last.
- **Naming & comments:** use English for identifiers and comments; user-facing strings remain in German where appropriate.
- **Errors:** return errors explicitly; wrap with `fmt.Errorf("… %w", err)` when adding context.
- **Testing:** add tests for new behaviour, keep them small and deterministic. Place fixtures under `testdata/` when needed.
- **Commit messages:** imperative mood, short summary on first line. Use conventional commit prefixes without scope, eg: (“feat: add scan command”).

## Testing & checks

Before committing:

1. `go fmt ./...`
2. `go vet ./...`
3. `go test ./...`

All commands must succeed or the failure should be explained in the PR.

## PR guidelines

- Describe user-facing changes briefly.
- Note any deviations from these guidelines.
- Do not include secrets or tokens in commits or logs.

Thank you for contributing!
