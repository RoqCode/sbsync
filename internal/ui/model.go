package ui

import (
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"storyblok-sync/internal/config"
	"storyblok-sync/internal/sb"
)

// --- UI Styles ---
var (
	titleStyle      = lipgloss.NewStyle().Bold(true).Underline(true)
	subtleStyle     = lipgloss.NewStyle().Faint(true)
	okStyle         = lipgloss.NewStyle().Bold(true)
	warnStyle       = lipgloss.NewStyle().Bold(true)
	helpStyle       = lipgloss.NewStyle().Faint(true)
	dividerStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	focusStyle      = lipgloss.NewStyle().Bold(true)
	cursorLineStyle = lipgloss.NewStyle().Background(lipgloss.Color("#2A2B3D"))
	cursorBarStyle  = lipgloss.NewStyle().Background(lipgloss.Color("#FFAB78"))
	markBarStyle    = lipgloss.NewStyle().Background(lipgloss.Color("#3AC4BA"))

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

type Model struct {
	state         state
	cfg           config.Config
	hasSBRC       bool
	sbrcPath      string
	statusMsg     string
	validateErr   error
	width, height int

	// spinner for loading states
	spinner spinner.Model

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

	selection SelectionState
	filter    FilterState
	search    SearchState
	filterCfg FilterConfig // Konfiguration für Such- und Filterparameter

	// tree state
	treeRoots []*storyNode
	flatNodes []*storyNode
	collapsed map[int]bool
	indexByID map[int]int
	treeLines []string
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

type storyNode struct {
	story    sb.Story
	parent   *storyNode
	children []*storyNode
}
