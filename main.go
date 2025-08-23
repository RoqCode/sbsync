package main

import (
	"context"
	"fmt"
	"os"
	"storyblok-sync/internal/config"
	"storyblok-sync/internal/sb"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/sahilm/fuzzy"
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

type model struct {
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
}

func initialModel() model {
	p := config.DefaultPath()
	cfg, err := config.Load(p)
	hasFile := err == nil

	m := model{
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

	return m
}

func (m model) Init() tea.Cmd {
	return nil
}

// ---------- Update ----------
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.KeyMsg:
		key := strings.ToLower(msg.String())

		switch m.state {

		case stateWelcome:
			switch key {
			case "ctrl+c", "q":
				return m, tea.Quit
			case "enter":
				if m.cfg.Token == "" {
					m.state = stateTokenPrompt
					m.statusMsg = "Bitte gib deinen Token ein."
				} else {
					m.state = stateValidating
					m.statusMsg = "Validiere Token…"
					return m, m.validateTokenCmd()
				}
			}

		case stateTokenPrompt:
			switch key {
			case "esc":
				m.state = stateWelcome
				m.statusMsg = "Zurück zum Welcome."
				return m, nil
			case "enter":
				m.cfg.Token = strings.TrimSpace(m.ti.Value())
				if m.cfg.Token == "" {
					m.statusMsg = "Token leer."
					return m, nil
				}
				m.state = stateValidating
				m.statusMsg = "Validiere Token…"
				return m, m.validateTokenCmd()
			default:
				var cmd tea.Cmd
				m.ti, cmd = m.ti.Update(msg)
				return m, cmd
			}

		case stateValidating:
			if key == "q" {
				return m, tea.Quit
			}

		case stateSpaceSelect:
			switch key {
			case "ctrl+c", "q":
				// zurück? Für MVP beenden wir lieber nicht – aber User erwartet "q" to quit:
				return m, tea.Quit
			case "j", "down":
				if m.selectedIndex < len(m.spaces)-1 {
					m.selectedIndex++
				}
			case "k", "up":
				if m.selectedIndex > 0 {
					m.selectedIndex--
				}
			case "enter":
				if len(m.spaces) == 0 {
					return m, nil
				}
				chosen := m.spaces[m.selectedIndex]
				if m.selectingSource {
					// Source wählen; Target-Auswahl vorbereiten
					m.sourceSpace = &chosen
					m.selectingSource = false
					// optional: Target nicht gleich Source erlauben?
					// wir lassen es erstmal zu; man kann später coden, dass source!=target sein muss.
					m.statusMsg = fmt.Sprintf("Source gesetzt: %s (%d). Wähle jetzt Target.", chosen.Name, chosen.ID)
					// Reset index für Target-Auswahl
					m.selectedIndex = 0
				} else {
					m.targetSpace = &chosen
					m.statusMsg = fmt.Sprintf("Target gesetzt: %s (%d). Scanne jetzt Stories…", chosen.Name, chosen.ID)
					m.state = stateScanning
					return m, m.scanStoriesCmd()
				}
			}

		case stateScanning:
			// Platzhalter – später starten wir hier den echten Scan und wechseln nach BrowseList.
			if key == "q" {
				return m, tea.Quit
			}

		case stateBrowseList:
			if m.searching {
				switch key {
				case "esc":
					// ESC: wenn Query leer -> Suche schließen, sonst nur löschen
					if strings.TrimSpace(m.query) == "" {
						m.searching = false
						m.searchInput.Blur()
						return m, nil
					}
					m.query = ""
					m.searchInput.SetValue("")
					m.applyFilter()
					return m, nil
				case "enter":
					// Enter: Suche schließen, Ergebnis bleibt aktiv
					m.searching = false
					m.searchInput.Blur()
					return m, nil
					// in stateBrowseList:
				case "+":
					m.minCoverage += 0.05
					if m.minCoverage > 0.95 {
						m.minCoverage = 0.95
					}
					m.applyFilter()
				case "-":
					m.minCoverage -= 0.05
					if m.minCoverage < 0.3 {
						m.minCoverage = 0.3
					}
					m.applyFilter()
				case "ctrl+c", "q":
					return m, tea.Quit
				default:
					var cmd tea.Cmd
					m.searchInput, cmd = m.searchInput.Update(msg)
					newQ := m.searchInput.Value()
					if newQ != m.query {
						m.query = newQ
						m.applyFilter()
					}
					return m, cmd
				}
			}

			switch key {
			case "ctrl+c", "q":
				return m, tea.Quit
				// Suche togglen
			case "f":
				m.searching = true
				m.searchInput.SetValue(m.query)
				m.searchInput.CursorEnd()
				m.searchInput.Focus()
				return m, nil

			case "c":
				m.query = ""
				m.searchInput.SetValue("")
				m.applyFilter()
				return m, nil

			// Navigation mit aktueller Länge
			case "j", "down":
				if m.listIndex < m.itemsLen()-1 {
					m.listIndex++
					m.ensureCursorVisible()
				}
			case "k", "up":
				if m.listIndex > 0 {
					m.listIndex--
					m.ensureCursorVisible()
				}
			case "ctrl+d", "pgdown":
				if m.itemsLen() > 0 {
					m.listIndex += m.listViewport
					if m.listIndex > m.itemsLen()-1 {
						m.listIndex = m.itemsLen() - 1
					}
					m.ensureCursorVisible()
				}
			case "ctrl+u", "pgup":
				m.listIndex -= m.listViewport
				if m.listIndex < 0 {
					m.listIndex = 0
				}
				m.ensureCursorVisible()

			// Markieren – beachte filteredIdx beim Zugriff
			case " ":
				if m.itemsLen() == 0 {
					return m, nil
				}
				st := m.itemAt(m.listIndex)
				if m.selected == nil {
					m.selected = make(map[string]bool)
				}
				m.selected[st.FullSlug] = !m.selected[st.FullSlug]

			// Rescan bleibt gleich
			case "r":
				m.state = stateScanning
				m.statusMsg = "Rescan…"
				return m, m.scanStoriesCmd()
			case "s":
				// Weiter zu Preflight in T6 – hier nur Platzhalter
				m.statusMsg = "Preflight (T6) folgt …"
			}
		}

	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		// grobe Reserve für Header, Divider, Titel, Footer/Hilfe
		const chrome = 8
		vp := m.height - chrome
		if vp < 3 {
			vp = 3
		}
		m.listViewport = vp

	case validateMsg:
		if msg.err != nil {
			m.validateErr = msg.err
			m.statusMsg = "Validierung fehlgeschlagen: " + msg.err.Error()
			m.state = stateTokenPrompt
			return m, nil
		}
		m.spaces = msg.spaces
		m.statusMsg = fmt.Sprintf("Token ok. %d Spaces gefunden.", len(m.spaces))
		// check if we have spaces configured and validate if their ids are in m.spaces
		if m.cfg.SourceSpace != "" && m.cfg.TargetSpace != "" {
			sourceSpace, sourceIdIsOk := containsSpaceID(m.spaces, m.cfg.SourceSpace)
			targetSpace, targetIdIsOk := containsSpaceID(m.spaces, m.cfg.TargetSpace)

			if sourceIdIsOk && targetIdIsOk {
				m.sourceSpace = &sourceSpace
				m.targetSpace = &targetSpace
				m.statusMsg = fmt.Sprintf("Target gesetzt: %s (%d). Scanne jetzt Stories…", sourceSpace.Name, sourceSpace.ID)
				m.state = stateScanning
				return m, m.scanStoriesCmd()
			}
		}
		m.state = stateSpaceSelect
		m.selectingSource = true
		m.selectedIndex = 0
		return m, nil

	case scanMsg:
		if msg.err != nil {
			m.statusMsg = "Scan-Fehler: " + msg.err.Error()
			m.state = stateSpaceSelect // zurück; du kannst auch einen Fehler-Screen bauen
			return m, nil
		}
		m.storiesSource = msg.src
		m.storiesTarget = msg.tgt
		m.listIndex = 0
		m.listOffset = 0
		if m.selected == nil {
			m.selected = make(map[string]bool)
		} else {
			// optional: Selektion leeren, da sich die Liste geändert hat
			clear(m.selected)
		}
		m.statusMsg = fmt.Sprintf("Scan ok. Source: %d Stories, Target: %d Stories.", len(m.storiesSource), len(m.storiesTarget))
		m.state = stateBrowseList
		return m, nil
	}

	return m, nil
}

// ---------- Messages / Cmds ----------
type validateMsg struct {
	spaces []sb.Space
	err    error
}

type scanMsg struct {
	src []sb.Story
	tgt []sb.Story
	err error
}

func (m model) validateTokenCmd() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		c := sb.New(m.cfg.Token)
		spaces, err := c.ListSpaces(ctx)
		if err != nil {
			return validateMsg{err: err}
		}
		return validateMsg{spaces: spaces, err: nil}
	}
}

func (m model) scanStoriesCmd() tea.Cmd {
	srcID, tgtID := 0, 0
	if m.sourceSpace != nil {
		srcID = m.sourceSpace.ID
	}
	if m.targetSpace != nil {
		tgtID = m.targetSpace.ID
	}
	token := m.cfg.Token

	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		c := sb.New(token)

		// Parallel wäre nice-to-have, hier sequentiell für Klarheit
		src, err := c.ListStories(ctx, sb.ListStoriesOpts{SpaceID: srcID, PerPage: 50})
		if err != nil {
			return scanMsg{err: fmt.Errorf("source scan: %w", err)}
		}
		tgt, err := c.ListStories(ctx, sb.ListStoriesOpts{SpaceID: tgtID, PerPage: 50})
		if err != nil {
			return scanMsg{err: fmt.Errorf("target scan: %w", err)}
		}
		return scanMsg{src: src, tgt: tgt, err: nil}
	}
}

func (m model) View() string {
	if m.state == stateQuit {
		return ""
	}
	var b strings.Builder
	b.WriteString(titleStyle.Render("Storyblok Sync (TUI)"))
	b.WriteString("\n")
	b.WriteString(dividerStyle.Render(strings.Repeat("─", max(10, m.width-2))))
	b.WriteString("\n\n")

	switch m.state {

	case stateWelcome:
		b.WriteString("Willkommen! Diese App hilft dir, Stories zwischen Spaces zu synchronisieren.\n\n")
		if m.cfg.Token != "" {
			b.WriteString(okStyle.Render("✓ Token vorhanden\n"))
		} else {
			b.WriteString(warnStyle.Render("! Kein Token gefunden (~/.sbrc oder SB_TOKEN)\n"))
		}
		if m.hasSBRC {
			b.WriteString(subtleStyle.Render(fmt.Sprintf("Konfiguration gefunden: %s\n", m.sbrcPath)))
		}
		b.WriteString(subtleStyle.Render(m.statusMsg) + "\n\n")
		b.WriteString(helpStyle.Render("Tasten: Enter weiter  |  q beenden"))

	case stateTokenPrompt:
		b.WriteString("Bitte gib deinen Storyblok Token ein:\n\n")
		b.WriteString(m.ti.View() + "\n\n")
		if m.validateErr != nil {
			b.WriteString(warnStyle.Render(m.validateErr.Error()) + "\n\n")
		}
		b.WriteString(helpStyle.Render("Enter bestätigen  |  Esc zurück"))

	case stateValidating:
		b.WriteString("Validiere Token…\n\n")
		b.WriteString(helpStyle.Render("q abbrechen"))

	case stateSpaceSelect:
		if m.selectingSource {
			b.WriteString("Spaces (wähle **Source**):\n\n")
		} else {
			b.WriteString("Spaces (wähle **Target**):\n\n")
			if m.sourceSpace != nil {
				b.WriteString(subtleStyle.Render(fmt.Sprintf("Source: %s (%d)\n\n", m.sourceSpace.Name, m.sourceSpace.ID)))
			}
		}
		if len(m.spaces) == 0 {
			b.WriteString(warnStyle.Render("Keine Spaces gefunden.\n"))
		} else {
			for i, sp := range m.spaces {
				cursor := "  "
				if i == m.selectedIndex {
					cursor = "> "
				}
				line := fmt.Sprintf("%s%s (id %d)", cursor, sp.Name, sp.ID)
				if i == m.selectedIndex {
					line = focusStyle.Render(line)
				}
				b.WriteString(line + "\n")
			}
		}
		b.WriteString("\n" + helpStyle.Render("j/k bewegen  |  Enter wählen  |  q beenden"))

	case stateScanning:
		src := "(none)"
		tgt := "(none)"
		if m.sourceSpace != nil {
			src = fmt.Sprintf("%s (%d)", m.sourceSpace.Name, m.sourceSpace.ID)
		}
		if m.targetSpace != nil {
			tgt = fmt.Sprintf("%s (%d)", m.targetSpace.Name, m.targetSpace.ID)
		}
		b.WriteString("Scanning…\n\n")
		b.WriteString(fmt.Sprintf("Source: %s\nTarget: %s\n\n", src, tgt))
		b.WriteString(subtleStyle.Render("Hole Stories (flach)…") + "\n\n")
		b.WriteString(helpStyle.Render("q beenden"))

	case stateBrowseList:
		srcCount := len(m.storiesSource)
		tgtCount := len(m.storiesTarget)
		b.WriteString(fmt.Sprintf("Browse (Source Stories) – %d Items  |  Target: %d\n\n", srcCount, tgtCount))
		if srcCount == 0 {
			b.WriteString(warnStyle.Render("Keine Stories im Source gefunden.\n"))
		} else {
			// sichtbaren Bereich bestimmen
			total := m.itemsLen()
			start := m.listOffset
			end := min(start+m.listViewport, total)

			// Suchleiste (falls aktiv oder Query gesetzt)
			if m.searching || strings.TrimSpace(m.query) != "" {
				label := "Suche: "
				if m.searching {
					b.WriteString(label + m.searchInput.View() + "\n\n")
				} else {
					b.WriteString(label + m.query + "\n\n")
				}
			}

			if total == 0 {
				b.WriteString(warnStyle.Render("Keine Stories gefunden (Filter aktiv?).\n"))
			} else {
				for i := start; i < end; i++ {
					st := m.itemAt(i)
					cursor := "  "
					if i == m.listIndex {
						cursor = "▶ "
					}

					mark := "[ ]"
					if m.selected[st.FullSlug] {
						mark = "[x]"
					}

					line := fmt.Sprintf("%s%s  %s", cursor, mark, displayStory(st))
					if i == m.listIndex {
						line = selStyle.Render(line)
					}
					b.WriteString(line + "\n")
				}
			}

			shown := 0
			if end > start {
				shown = end - start
			}

			// Anzeige: Filterstatus + Range
			suffix := ""
			if m.filteredIdx != nil {
				suffix = fmt.Sprintf("  |  gefiltert: %d", total)
			}
			b.WriteString("\n")
			b.WriteString(subtleStyle.Render(
				fmt.Sprintf("Zeilen %d–%d von %d (sichtbar: %d)%s",
					start+1, end, total, shown, suffix),
			))
			b.WriteString("\n")

		}
		// Footer / Hilfe
		checked := 0
		for _, v := range m.selected {
			if v {
				checked++
			}
		}
		b.WriteString("\n")
		b.WriteString(subtleStyle.Render(fmt.Sprintf("Markiert: %d", checked)) + "\n")
		b.WriteString(helpStyle.Render(fmt.Sprintf("j/k bewegen  |  Story markieren  |  f suchen  |  +/- strenger/lockerer (cov=%.2f, spread=%d)  |  c Filter löschen  |  Enter schließen  |  Esc löschen/zurück  |  r rescan  |  s preflight  |  q beenden", m.minCoverage, m.maxSpread)))

	}

	b.WriteString("\n")
	return b.String()
}

// ------ utils -------

func containsSpaceID(spacesSlice []sb.Space, spaceID string) (sb.Space, bool) {
	for _, space := range spacesSlice {
		if fmt.Sprint(space.ID) == spaceID {
			return space, true
		}
	}
	return sb.Space{}, false
}

func displayStory(st sb.Story) string {
	name := st.Name
	if name == "" {
		name = st.Slug
	}
	return fmt.Sprintf("%s  (%s)", name, st.FullSlug)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func (m *model) ensureCursorVisible() {
	if m.listViewport <= 0 {
		m.listViewport = 10
	}

	// clamp index gegen aktuelle Länge
	n := m.itemsLen()
	if n == 0 {
		m.listIndex = 0
		m.listOffset = 0
		return
	}
	if m.listIndex < 0 {
		m.listIndex = 0
	}
	if m.listIndex > n-1 {
		m.listIndex = n - 1
	}

	// scroll up/down
	if m.listIndex < m.listOffset {
		m.listOffset = m.listIndex
	}
	if m.listIndex >= m.listOffset+m.listViewport {
		m.listOffset = m.listIndex - m.listViewport + 1
	}

	// clamp offset
	maxStart := n - m.listViewport
	if maxStart < 0 {
		maxStart = 0
	}
	if m.listOffset > maxStart {
		m.listOffset = maxStart
	}
	if m.listOffset < 0 {
		m.listOffset = 0
	}
}

// Gesamtlänge der aktuell sichtbaren Items (gefiltert oder voll)
func (m *model) itemsLen() int {
	if m.filteredIdx != nil {
		return len(m.filteredIdx)
	}
	return len(m.storiesSource)
}

// original Story für einen sichtbaren Index holen
func (m *model) itemAt(visIdx int) sb.Story {
	if m.filteredIdx != nil {
		return m.storiesSource[m.filteredIdx[visIdx]]
	}
	return m.storiesSource[visIdx]
}

// Fuzzy anwenden (Name, Slug, FullSlug)
func (m *model) applyFilter() {
	q := strings.TrimSpace(strings.ToLower(m.query))
	if q == "" {
		m.filteredIdx = nil
		m.listIndex, m.listOffset = 0, 0
		return
	}

	// Kandidaten zusammensetzen
	base := make([]string, len(m.storiesSource))
	for i, st := range m.storiesSource {
		name := st.Name
		if name == "" {
			name = st.Slug
		}
		base[i] = strings.ToLower(name + "  " + st.Slug + "  " + st.FullSlug)
	}

	// Pass 1: Substring (präzise, schnell)
	sub := make([]int, 0, 128)
	for i, s := range base {
		if strings.Contains(s, q) {
			sub = append(sub, i)
			if len(sub) >= m.maxResults {
				break
			}
		}
	}
	if len(sub) > 0 {
		m.filteredIdx = sub
		m.listIndex, m.listOffset = 0, 0
		m.ensureCursorVisible()
		return
	}

	// Pass 2: Fuzzy (Fallback, aber gedrosselt)
	matches := fuzzy.Find(q, base)
	pruned := make([]int, 0, len(matches))
	for _, mt := range matches {
		if matchCoverage(q, mt) < m.minCoverage {
			continue
		}
		if matchSpread(mt) > m.maxSpread {
			continue
		}
		pruned = append(pruned, mt.Index)
		if len(pruned) >= m.maxResults {
			break
		}
	}

	if len(pruned) == 0 {
		// falls alles zu streng: zeig die Top N rohen Fuzzy-Ergebnisse
		for i := 0; i < len(matches) && i < m.maxResults; i++ {
			pruned = append(pruned, matches[i].Index)
		}
	}
	m.filteredIdx = pruned
	m.listIndex, m.listOffset = 0, 0
	m.ensureCursorVisible()
}

func matchCoverage(q string, m fuzzy.Match) float64 {
	if len(q) == 0 {
		return 1
	}
	// m.MatchedIndexes Länge = wie viele Query-Zeichen gematcht wurden
	return float64(len(m.MatchedIndexes)) / float64(len(q))
}

func matchSpread(m fuzzy.Match) int {
	if len(m.MatchedIndexes) == 0 {
		return 0
	}
	return m.MatchedIndexes[len(m.MatchedIndexes)-1] - m.MatchedIndexes[0]
}

// --- main ---
func main() {
	if _, err := tea.NewProgram(
		initialModel(),
		tea.WithAltScreen(),
		// optional: tea.WithMouseCellMotion(),
	).Run(); err != nil {
		fmt.Println("error:", err)
		os.Exit(1)
	}
}
