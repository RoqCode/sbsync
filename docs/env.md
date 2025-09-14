# Environment Flags

This document lists all supported environment variables, what they do, and sensible defaults. Flags control authentication, transport limits/retries, and optional TUI features.

## How To Set

- macOS/Linux (temporary): `SB_MA_RPS=6 SB_TUI_GRAPH=1 go run ./cmd/sbsync`
- macOS/Linux (shell profile): `export SB_MA_RPS=6`
- Windows users: Use WSL2 and set env vars inside your Linux distribution as above.

## Authentication & Config

- SB_TOKEN: Personal Access Token for the Storyblok Management API.
  - Type: string
  - Default: none (read from `~/.sbrc` if present)
  - Notes: If not set via env, the app reads `SB_TOKEN` from `~/.sbrc`.

- SOURCE_SPACE_ID: Optional source space ID for preselection.
  - Type: string (numeric ID)
  - Default: none
  - Notes: Stored in `~/.sbrc` by the app when configured.

- TARGET_SPACE_ID: Optional target space ID for preselection.
  - Type: string (numeric ID)
  - Default: none
  - Notes: Stored in `~/.sbrc` by the app when configured.

## Transport: Rate Limits & Retries

These tune the retrying, rate-limited HTTP transport used by the Management API client.

- SB_MA_RPS: Max requests per second for Management API (per host, combined reads+writes).
  - Type: float
  - Default: `14` (allows ~7 read + ~7 write concurrently)
  - Example: `SB_MA_RPS=18`

- SB_MA_BURST: Token bucket burst capacity (host level).
  - Type: int
  - Default: `14`
  - Example: `SB_MA_BURST=8`

- SB_MA_RETRY_MAX: Max retry attempts on 429/5xx/transient errors.
  - Type: int
  - Default: `4` (i.e., up to 5 total attempts)
  - Example: `SB_MA_RETRY_MAX=2`

- SB_MA_RETRY_BASE_MS: Base backoff (ms) for exponential backoff.
  - Type: int
  - Default: `250`
  - Example: `SB_MA_RETRY_BASE_MS=200`

- SB_MA_RETRY_CAP_MS: Max backoff cap (ms).
  - Type: int
  - Default: `5000`
  - Example: `SB_MA_RETRY_CAP_MS=3000`

Notes:
- Retry-After headers are honored when present.
- The transport includes gentle adaptive nudging: small step-up on sustained 2xx, stronger step-down on 429/5xx/network errors. The ceiling remains SB_MA_RPS.

## TUI Features

- SB_TUI_GRAPH: Enable multi-row Req/s history graph in the Sync view.
  - Type: boolean (1/true/yes/on)
  - Default: disabled
  - Example: `SB_TUI_GRAPH=1`
  - Notes: The numeric stats (Req/s, Read, Write, Succ/s, Warn%, Err%) are always shown; this flag controls only the multi-row graph below them.

## Tips

- Combine transport tuning:
  - `SB_MA_RPS=6 SB_MA_BURST=8` to approach ~6 req/s under good conditions.
  - Reduce `SB_MA_RETRY_MAX` in fast-fail scenarios to minimize latency.
- Verify effects in the Sync stats line (Req/s, Read, Write, Succ/s, Warn%, Err%).
