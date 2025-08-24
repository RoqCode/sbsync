package ui

import (
	"storyblok-sync/internal/config"
	"storyblok-sync/internal/sb"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// --- UI Styles ---
var (
	titleStyle       = lipgloss.NewStyle().Bold(true).Underline(true)
	subtleStyle      = lipgloss.NewStyle().Faint(true)
	okStyle          = lipgloss.NewStyle().Bold(true)
	warnStyle        = lipgloss.NewStyle().Bold(true)
	helpStyle        = lipgloss.NewStyle().Faint(true)
	dividerStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	focusStyle       = lipgloss.NewStyle().Bold(true)
	cursorLineStyle  = lipgloss.NewStyle().Background(lipgloss.Color("#2A2B3D"))
	cursorBarStyle   = lipgloss.NewStyle().Background(lipgloss.Color("#FFAB78"))
	markBarStyle     = lipgloss.NewStyle().Background(lipgloss.Color("#3AC4BA"))
	markNestedStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#3AC4BA"))
	collisionSign    = lipgloss.NewStyle().Foreground(lipgloss.Color("11")).Render("!")
	stateCreateStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	stateUpdateStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("11"))
	stateSkipStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	stateDoneStyle   = lipgloss.NewStyle().Background(lipgloss.Color("10")).Foreground(lipgloss.Color("0")).Bold(true)

	// markers for different story types (colored squares)
	symbolStory  = fgSymbol("#8942E1", "S")
	symbolFolder = fgSymbol("#3AC4BA", "F")
	symbolRoot   = fgSymbol("214", "R")
)

func fgSymbol(col, ch string) string {
	s := lipgloss.NewStyle().Foreground(lipgloss.Color(col)).Render(ch)
	const reset = "\x1b[0m"
	return strings.TrimSuffix(s, reset) + "\x1b[39m"
}

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

var stateStyles = map[SyncState]lipgloss.Style{
	StateCreate: stateCreateStyle,
	StateUpdate: stateUpdateStyle,
	StateSkip:   stateSkipStyle,
}

// RunState marks the execution state of a sync item.
type RunState int

const (
	RunPending RunState = iota
	RunRunning
	RunDone
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

	// spinner for loading states
	spinner   spinner.Model
	syncing   bool
	syncIndex int

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

func InitialModel() Model {
	p := config.DefaultPath()
	cfg, err := config.Load(p)
	hasFile := err == nil

	m := Model{
		state:     stateWelcome,
		cfg:       cfg,
		hasSBRC:   hasFile,
		sbrcPath:  p,
		statusMsg: "",
	}

	if cfg.Token == "" {
		m.statusMsg = "Keine ~/.sbrc oder kein Token – drück Enter für Token-Eingabe."
	} else {
		m.statusMsg = "Token gefunden – Enter zum Validieren, q zum Beenden."
	}

	// textinput
	ti := textinput.New()
	ti.Placeholder = "Storyblok Personal Access Token"
	ti.Focus()
	ti.EchoMode = textinput.EchoPassword
	ti.CharLimit = 200
	m.ti = ti
	m.selection.selected = make(map[string]bool)
	m.folderCollapsed = make(map[int]bool)
	m.storyIdx = make(map[int]int)

	// search
	si := textinput.New()
	si.Placeholder = "Fuzzy suchen…"
	si.CharLimit = 200
	si.Width = 40
	m.search.searchInput = si
	m.search.query = ""
	m.search.filteredIdx = nil
	m.filterCfg = FilterConfig{
		MinCoverage: 0.6, // strenger -> höher (z.B. 0.7)
		MaxSpread:   40,  // strenger -> kleiner (z.B. 25)
		MaxResults:  200, // UI ruhig halten
	}

	// prefix
	pi := textinput.New()
	pi.Placeholder = "Slug-Prefix (z.B. a__portal/de)"
	pi.CharLimit = 200
	pi.Width = 40
	m.filter.prefixInput = pi
	m.filter.prefix = ""

	// spinner
	sp := spinner.New()
	sp.Spinner = spinner.Line
	sp.Style = subtleStyle
	m.spinner = sp

	return m
}

func (m Model) Init() tea.Cmd { return nil }
