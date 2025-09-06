package ui

import (
	"context"
	"storyblok-sync/internal/config"
	sync "storyblok-sync/internal/core/sync"
	"storyblok-sync/internal/sb"
	"time"

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
    stateModePicker
    stateScanning
    stateBrowseList
    statePreflight
    stateCopyAsNew
    stateFolderFork
    stateSync
    stateReport
    stateQuit
)

// syncMode selects which domain to sync in this run
type syncMode int

const (
    modeStories syncMode = iota
    modeComponents
)

type SelectionState struct {
	// browse list (source)
	listIndex int
	selected  map[string]bool // key: FullSlug (oder Full Path)
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

// Unified Preflight state with core
// State values: "create", "update", "skip"
// Run values:   "pending", "running", "success", "failed"
const (
	StateCreate = "create"
	StateUpdate = "update"
	StateSkip   = "skip"

	RunPending   = "pending"
	RunRunning   = "running"
	RunDone      = "success"
	RunCancelled = "failed"
)

// Use core PreflightItem directly (includes optional Issue field)
type PreflightItem = sync.PreflightItem

// recalcState updates the textual state based on Skip/CopyAsNew/Collision
func recalcState(it *PreflightItem) {
	switch {
	case it.Skip:
		it.State = StateSkip
	case it.CopyAsNew:
		it.State = StateCreate
	case it.Collision:
		it.State = StateUpdate
	default:
		it.State = StateCreate
	}
}

type PreflightState struct {
	items     []PreflightItem
	listIndex int
	// visibleIdx maps visible list positions to indices in items
	// to support folder collapse/expand like in browse view
	visibleIdx []int
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
	paused      bool               // pause flag to stop scheduling new work
	api         *sb.Client
	report      Report
	// Per-item metrics snapshots to compute rate-limit retry deltas
	syncStartMetrics map[int]sb.MetricsSnapshot

	// token input
	ti textinput.Model

    // spaces & selection
    spaces          []sb.Space
    selectedIndex   int
    selectingSource bool
    sourceSpace     *sb.Space
    targetSpace     *sb.Space
    // mode picker
    modePickerIndex int
    currentMode     syncMode

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

	// preserve browse collapse state when entering preflight
	collapsedBeforePreflight map[int]bool

	// Copy-as-new full-screen state
	copy struct {
		itemIdx          int
		parent           string
		baseSlug         string
		presets          []string
		selectedPreset   int
		input            textinput.Model
		appendCopyToName bool
		errorMsg         string
	}

	// Folder-fork full-screen state
	folder struct {
		itemIdx                     int
		parent                      string
		baseSlug                    string
		presets                     []string
		selectedPreset              int
		input                       textinput.Model
		appendCopyToFolderName      bool // default ON
		appendCopyToChildStoryNames bool // default OFF
		errorMsg                    string
	}

	// (no pre-hydration; on-demand MA reads during sync)

	// --- Stats (Phase 4: TUI Performance Panel) ---
	// Requests/sec sampling via transport metrics snapshots
	rpsCurrent   float64
	reqTimes     []time.Time
	reqSamples   []float64
	reqTotals    []int64
	reqWindow    time.Duration
	lastSnapTime time.Time
	lastSnap     sb.MetricsSnapshot
	// Reads/writes per second and success per second
	rpsReadCurrent  float64
	rpsWriteCurrent float64
	spsSuccess      float64
	readTotals      []int64
	writeTotals     []int64
	successTotals   []int64
	successTimes    []time.Time
	successTotal    int64
	// Warning/error rates over window (HTTP level)
	warningRate     float64 // 429 / total * 100
	errorRate       float64 // 5xx / total * 100
	status429Totals []int64
	status5xxTotals []int64
	// Previous values for trend arrows
	prevRPS      float64
	prevReadRPS  float64
	prevWriteRPS float64
	prevSuccS    float64
	prevWarnPct  float64
	prevErrPct   float64
	// Worker utilization
	maxWorkers     int
	workerBarWidth int
	rpsGraphWidth  int
	rpsGraphHeight int
	showRpsGraph   bool

	// --- Publish modes (UI-only) ---
	// Per-item chosen publish mode: "draft", "publish", "publish_changes"
	publishMode map[string]string // key: FullSlug
	// Items that should be unpublished after overwrite (special case)
	unpublishAfter map[string]bool // key: FullSlug

	// --- EMA estimates for rate-limit budgeting ---
	emaWritePerItem float64
	emaItemDurSec   float64
}
