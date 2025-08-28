# Repository Guidelines

## Project Structure & Module Organization
- `cmd/sbsync/`: Application entry point (main package).
- `internal/ui/`: Bubble Tea TUI models, views, and update loop.
- `internal/sb/`: Storyblok API client.
- `internal/config/`: Token/config loading and saving.
- `testdata/`: Fixtures for deterministic tests.
- Future modules (e.g., `internal/core/`, `infra/`) should follow the same layout and separation of concerns.

## Build, Test, and Development
- `go build ./cmd/sbsync`: Build the `sbsync` binary.
- `go run ./cmd/sbsync`: Run the TUI locally.
- `go fmt ./...`: Format code (tabs; Go 1.25).
- `go vet ./...`: Static checks for common mistakes.
- `go test ./...`: Run the full test suite. Add `-cover` for coverage.

## Coding Style & Naming Conventions
- **Language:** Go 1.25; identifiers and comments in English. User‑facing strings may be German.
- **Formatting:** Use `go fmt` (tabs). Group imports: stdlib, third‑party, internal.
- **Errors:** Return explicitly; wrap with `fmt.Errorf("… %w", err)` when adding context.
- **Naming:** Clear, descriptive names; avoid abbreviations. Keep package APIs small and focused.

## Testing Guidelines
- **Framework:** Standard `testing` package; table‑driven tests preferred.
- **Placement:** `*_test.go` next to code; fixtures under `testdata/`.
- **Behavior:** Small, deterministic tests without network or timing flakiness.
- **Run:** `go test ./...` (optionally `-race -cover`).

## Commit & Pull Request Guidelines
- **Commits:** Conventional commits, imperative mood (e.g., `feat: add scan command`). Short summary line.
- **Before pushing:** `go fmt ./... && go vet ./... && go test ./...` must pass or be explained.
- **PRs:** Brief description of user‑facing changes, linked issues, and any deviations from these guidelines. No secrets or tokens in commits or logs.

## Security & Configuration
- Never commit Storyblok tokens or credentials. Use `internal/config` for local persistence; prefer env vars or OS‑level stores for secrets.
- Avoid logging sensitive values; redact where necessary.

## Architecture Notes
- Bubble Tea MVU for UI (`internal/ui`) with business logic and Storyblok access decoupled (`internal/sb`). Keep future domain logic under `internal/core/` to maintain clear boundaries.

