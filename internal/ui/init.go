package ui

import (
	"storyblok-sync/internal/config"
	"storyblok-sync/internal/sb"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

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

	// viewport
	vp := viewport.New(80, 24) // initial dimensions, will be updated in WindowSize
	m.viewport = vp

	// metrics tracking for per-item retry deltas
	m.syncStartMetrics = make(map[int]sb.MetricsSnapshot)

	return m
}

func (m Model) Init() tea.Cmd { return nil }
