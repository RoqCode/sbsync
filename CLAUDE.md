# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Development Commands

**Build and Run:**
```bash
go build -o sbsync ./cmd/sbsync
./sbsync
```

**Testing:**
```bash
go test ./...                    # Run all tests
go test ./internal/ui -v         # Run specific package tests with verbose output
go test -run TestFilterStories   # Run specific test
```

**Code Quality:**
```bash
go fmt ./...                     # Format code (required before commits)
go vet ./...                     # Static analysis
```

**Development Setup:**
- Go 1.25+ required
- Configuration stored in `~/.sbrc` or via `SB_TOKEN` environment variable
- Use `go run ./cmd/sbsync` for quick development testing

**Debugging:**
- Enable debug logging: `DEBUG=1 go run ./cmd/sbsync`
- View logs: `tail -f debug.log`
- Delve debugging: `dlv debug --headless --api-version=2 --listen=127.0.0.1:43000 .`
- See DEBUGGING.md for detailed debugging guide

## Architecture Overview

This is a Go-based Terminal User Interface (TUI) for synchronizing Storyblok content between spaces using the Bubble Tea framework.

**Core Architecture:**
- **Entry Point:** `cmd/sbsync/main.go` - Minimal main that initializes the Bubble Tea program
- **UI Layer:** `internal/ui/` - Complete TUI implementation with state machine pattern
- **API Client:** `internal/sb/` - Storyblok Management API client with pagination and error handling  
- **Configuration:** `internal/config/` - Token and space ID management via `.sbrc` file

**State Machine Flow:**
The UI follows a linear state progression:
1. `stateWelcome` → Token validation or prompt
2. `stateSpaceSelect` → Choose source/target spaces
3. `stateScanning` → Fetch stories from both spaces
4. `stateBrowseList` → Select stories with fuzzy search and filtering
5. `statePreflight` → Review collisions and sync actions
6. `stateSync` → Execute sync operations (planned)
7. `stateReport` → Display results summary (planned)

**Key Data Structures:**
- `Model` in `internal/ui/model.go` - Central application state with sub-states for selection, filtering, search, and preflight
- `Story` in `internal/sb/client.go` - Unified structure for stories and folders from Storyblok API
- `PreflightItem` - Wraps stories with collision detection and sync planning

**UI Components:**
- Fuzzy search using `github.com/sahilm/fuzzy` with configurable coverage/spread parameters
- Tree-like navigation with folder collapse/expand functionality
- Multi-selection with visual indicators (colored symbols: S=Story, F=Folder, R=Root)
- Collision detection and resolution (Create/Update/Skip states)

**Configuration Management:**
- Token loading priority: Environment variable `SB_TOKEN` → `.sbrc` file → user input
- `.sbrc` format supports `SB_TOKEN`, `SOURCE_SPACE_ID`, `TARGET_SPACE_ID`
- Config path defaults to `~/.sbrc`

## Key Implementation Details

**Storyblok API Client:**
- Base URL: `https://mapi.storyblok.com/v1`
- Automatic pagination for story listing
- Proper error handling with context support
- Stories and folders treated uniformly via `is_folder` field

**UI State Management:**
- Single `Model` struct holds all application state
- Clean separation between UI state (selection, search) and business logic
- Lipgloss styling with consistent color scheme and symbols

**Sync Implementation:**
- Follows Storyblok CLI v3.36.1 behavior for compatibility
- Correct sync order: folders first (by depth), then stories
- Complete Story data model with content, UUID, translated slugs
- Proper error handling with retry logic for rate limiting
- UUID management to maintain story identity across spaces
- Uses `with_slug` API parameter for accurate collision detection

**Testing Approach:**
- Unit tests for core logic (sorting, filtering, configuration)
- Test data stored in `testdata/` directory
- Focus on business logic rather than UI components
- Use debug logging to verify sync behavior in development