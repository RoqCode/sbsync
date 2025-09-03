package ui

import (
	"os"
	"storyblok-sync/internal/config"
	"storyblok-sync/internal/sb"
	"strings"
	"time"

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

	// copy-as-new input (full-screen view)
	ci := textinput.New()
	ci.Placeholder = "Neuer Slug (z.B. artikel-copy)"
	ci.CharLimit = 200
	ci.Width = 40
	m.copy.input = ci

	// spinner
	sp := spinner.New()
	sp.Spinner = spinner.Line
	sp.Style = subtleStyle
	m.spinner = sp

	// viewport
	vp := viewport.New(80, 24) // initial dimensions, will be updated in WindowSize
	m.viewport = vp

	// stats: requests per second window and visual widths
	m.reqWindow = 30 * time.Second
	m.reqTimes = nil
	m.reqSamples = nil
	m.reqTotals = nil
	m.readTotals = nil
	m.writeTotals = nil
	m.successTotals = nil
	m.successTimes = nil
	m.successTotal = 0
	m.status429Totals = nil
	m.status5xxTotals = nil
	m.workerBarWidth = 8
	m.rpsGraphWidth = 24
	m.rpsGraphHeight = 3
	// Feature flag: enable multi-row RPS bar graph when set
	m.showRpsGraph = enableFlag(os.Getenv("SB_TUI_GRAPH"))

	// metrics tracking for per-item retry deltas
	m.syncStartMetrics = make(map[int]sb.MetricsSnapshot)

	return m
}

func (m Model) Init() tea.Cmd { return nil }

// enableFlag returns true for common truthy values: 1, true, yes (case-insensitive)
func enableFlag(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "on", "enable", "enabled":
		return true
	default:
		return false
	}
}
