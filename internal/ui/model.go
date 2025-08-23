package ui

import (
	"storyblok-sync/internal/config"
	"storyblok-sync/internal/sb"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// --- UI Styles ---
var (
	titleStyle   = lipgloss.NewStyle().Bold(true).Underline(true)
	subtleStyle  = lipgloss.NewStyle().Faint(true)
	okStyle      = lipgloss.NewStyle().Bold(true)
	warnStyle    = lipgloss.NewStyle().Bold(true)
	helpStyle    = lipgloss.NewStyle().Faint(true)
	dividerStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	focusStyle   = lipgloss.NewStyle().Bold(true)
	selStyle     = lipgloss.NewStyle().Reverse(true)
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
	stateQuit
)

type Model struct {
	state         state
	cfg           config.Config
	hasSBRC       bool
	sbrcPath      string
	statusMsg     string
	validateErr   error
	width, height int

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

	// browse list (source)
	listIndex    int
	listOffset   int
	listViewport int
	selected     map[string]bool // key: FullSlug (oder Full Path)

	// searching
	searching   bool
	searchInput textinput.Model
	query       string // aktueller Suchstring
	filteredIdx []int  // Mapping: sichtbarer Index -> original Index
	// search tuning
	minCoverage float64 // Anteil der Query, der gematcht wurde (0.0–1.0)
	maxSpread   int     // max. Abstand zwischen 1. und letzter Match-Position
	maxResults  int     // harte Obergrenze für Ergebnisliste
	// prefix-filter
	prefixing   bool
	prefixInput textinput.Model
	prefix      string // z.B. "a__portal/de"
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
	m.selected = make(map[string]bool)

	// search
	si := textinput.New()
	si.Placeholder = "Fuzzy suchen…"
	si.CharLimit = 200
	si.Width = 40
	m.searchInput = si
	m.query = ""
	m.filteredIdx = nil
	m.minCoverage = 0.6 // strenger -> höher (z.B. 0.7)
	m.maxSpread = 40    // strenger -> kleiner (z.B. 25)
	m.maxResults = 200  // UI ruhig halten

	// prefix
	pi := textinput.New()
	pi.Placeholder = "Slug-Prefix (z.B. a__portal/de)"
	pi.CharLimit = 200
	pi.Width = 40
	m.prefixInput = pi
	m.prefix = ""

	return m
}

func (m Model) Init() tea.Cmd { return nil }
