package ui

import (
	"context"
	"storyblok-sync/internal/config"
	"storyblok-sync/internal/sb"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
)

// --- Model / State ---
type state int

const (
	stateWelcome state = iota
	stateTokenPrompt
	stateValidating
	stateSpaceSelect
	stateScanning
	stateBrowseList
	statePreflight
	stateSync
	stateReport
	stateQuit
)

type SelectionState struct {
	// browse list (source)
	listIndex    int
	listOffset   int
	listViewport int
	selected     map[string]bool // key: FullSlug (oder Full Path)
}

type FilterState struct {
	// prefix-filter
	prefixing   bool
	prefixInput textinput.Model
	prefix      string // z.B. "a__portal/de"
}

type SearchState struct {
	// searching
	searching   bool
	searchInput textinput.Model
	query       string // aktueller Suchstring
	filteredIdx []int  // Mapping: sichtbarer Index -> original Index
}

// SyncState represents the action that will be performed for a story.
// It is kept as a string to allow easy extension with additional states.
type SyncState string

const (
	StateCreate SyncState = "C"
	StateUpdate SyncState = "U"
	StateSkip   SyncState = "S"
)

// RunState marks the execution state of a sync item.
type RunState int

const (
	RunPending RunState = iota
	RunRunning
	RunDone
	RunCancelled
)

type PreflightItem struct {
	Story      sb.Story
	Collision  bool
	Skip       bool
	Selected   bool
	State      SyncState
	StartsWith bool
	Run        RunState
}

func (it *PreflightItem) RecalcState() {
	switch {
	case it.Skip:
		it.State = StateSkip
	case it.Collision:
		it.State = StateUpdate
	default:
		it.State = StateCreate
	}
}

type PreflightState struct {
	items        []PreflightItem
	listIndex    int
	listOffset   int
	listViewport int
}

type SyncPlan struct {
	Items []PreflightItem
}

type Model struct {
	state         state
	cfg           config.Config
	hasSBRC       bool
	sbrcPath      string
	statusMsg     string
	validateErr   error
	width, height int

	// viewport for scrollable content
	viewport viewport.Model

	// spinner for loading states
	spinner     spinner.Model
	syncing     bool
	syncIndex   int
	syncCancel  context.CancelFunc // for cancelling sync operations
	syncContext context.Context    // cancellable context for sync
	api         *sb.Client
	report      Report

	// token input
	ti textinput.Model

	// spaces & selection
	spaces          []sb.Space
	selectedIndex   int
	selectingSource bool
	sourceSpace     *sb.Space
	targetSpace     *sb.Space

	// scan results
	storiesSource []sb.Story
	storiesTarget []sb.Story

	// tree state
	storyIdx        map[int]int  // Story ID -> index in storiesSource
	folderCollapsed map[int]bool // Folder ID -> collapsed?
	visibleIdx      []int        // indices of visible storiesSource entries

	selection SelectionState
	filter    FilterState
	search    SearchState
	filterCfg FilterConfig // Konfiguration für Such- und Filterparameter

	preflight PreflightState
	plan      SyncPlan
}
