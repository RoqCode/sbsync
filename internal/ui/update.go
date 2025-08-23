package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"

	"storyblok-sync/internal/sb"
)

// ---------- Update ----------
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.KeyMsg:
		key := msg.String()

		// global shortcuts
		if key == "ctrl+c" || key == "q" {
			return m, tea.Quit
		}

		switch m.state {
		case stateWelcome:
			return m.handleWelcomeKey(key)
		case stateTokenPrompt:
			return m.handleTokenPromptKey(msg)
		case stateValidating:
			return m.handleValidatingKey(key)
		case stateSpaceSelect:
			return m.handleSpaceSelectKey(key)
		case stateScanning:
			return m.handleScanningKey(key)
		case stateBrowseList:
			return m.handleBrowseListKey(msg)
		}

	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		// grobe Reserve für Header, Divider, Titel, Footer/Hilfe
		const chrome = 12
		vp := m.height - chrome
		if vp < 3 {
			vp = 3
		}
		m.selection.listViewport = vp

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
				return m, tea.Batch(m.spinner.Tick, m.scanStoriesCmd())
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
		m.selection.listIndex, m.selection.listOffset = 0, 0
		m.applyFilter()
		if m.selection.selected == nil {
			m.selection.selected = make(map[string]bool)
		} else {
			// optional: Selektion leeren, da sich die Liste geändert hat
			clear(m.selection.selected)
		}
		m.statusMsg = fmt.Sprintf("Scan ok. Source: %d Stories, Target: %d Stories.", len(m.storiesSource), len(m.storiesTarget))
		m.state = stateBrowseList
		return m, nil

	case spinner.TickMsg:
		if m.state == stateValidating || m.state == stateScanning {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
		return m, nil
	}

	return m, nil
}

// ---------- Handlers ----------

func (m Model) handleWelcomeKey(key string) (Model, tea.Cmd) {
	switch key {
	case "enter":
		if m.cfg.Token == "" {
			m.state = stateTokenPrompt
			m.statusMsg = "Bitte gib deinen Token ein."
			return m, nil
		}
		m.state = stateValidating
		m.statusMsg = "Validiere Token…"
		return m, tea.Batch(m.spinner.Tick, m.validateTokenCmd())
	}
	return m, nil
}

func (m Model) handleTokenPromptKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	key := msg.String()
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
		return m, tea.Batch(m.spinner.Tick, m.validateTokenCmd())
	default:
		var cmd tea.Cmd
		m.ti, cmd = m.ti.Update(msg)
		return m, cmd
	}
}

func (m Model) handleValidatingKey(key string) (Model, tea.Cmd) {
	return m, nil
}

func (m Model) handleSpaceSelectKey(key string) (Model, tea.Cmd) {
	switch key {
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
			return m, tea.Batch(m.spinner.Tick, m.scanStoriesCmd())
		}
	}
	return m, nil
}

func (m Model) handleScanningKey(key string) (Model, tea.Cmd) {
	// Platzhalter – später starten wir hier den echten Scan und wechseln nach BrowseList.
	return m, nil
}

func (m Model) handleBrowseListKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	key := msg.String()

	if m.filter.prefixing {
		switch key {
		case "esc":
			m.filter.prefixInput.Blur()
			if strings.TrimSpace(m.filter.prefixInput.Value()) == "" {
				m.filter.prefix = ""
			}
			m.filter.prefixing = false
			m.applyFilter()
			return m, nil
		case "enter":
			m.filter.prefix = strings.TrimSpace(m.filter.prefixInput.Value())
			m.filter.prefixing = false
			m.filter.prefixInput.Blur()
			m.applyFilter()
			return m, nil
		case "ctrl+c", "q":
			return m, tea.Quit
		default:
			var cmd tea.Cmd
			m.filter.prefixInput, cmd = m.filter.prefixInput.Update(msg)
			return m, cmd
		}
	}

	if m.search.searching {
		switch key {
		case "esc":
			// ESC: wenn Query leer -> Suche schließen, sonst nur löschen
			if strings.TrimSpace(m.search.query) == "" {
				m.search.searching = false
				m.search.searchInput.Blur()
				return m, nil
			}
			m.search.query = ""
			m.search.searchInput.SetValue("")
			m.applyFilter()
			return m, nil
		case "enter":
			// Enter: Suche schließen, Ergebnis bleibt aktiv
			m.search.searching = false
			m.search.searchInput.Blur()
			return m, nil
		case "+":
			m.filterCfg.MinCoverage += 0.05
			if m.filterCfg.MinCoverage > 0.95 {
				m.filterCfg.MinCoverage = 0.95
			}
			m.applyFilter()
		case "-":
			m.filterCfg.MinCoverage -= 0.05
			if m.filterCfg.MinCoverage < 0.3 {
				m.filterCfg.MinCoverage = 0.3
			}
			m.applyFilter()
		case "ctrl+c", "q":
			return m, tea.Quit
		default:
			var cmd tea.Cmd
			m.search.searchInput, cmd = m.search.searchInput.Update(msg)
			newQ := m.search.searchInput.Value()
			if newQ != m.search.query {
				m.search.query = newQ
				m.applyFilter()
			}
			return m, cmd
		}
	}

	switch key {
	case "f":
		m.search.searching = true
		m.search.searchInput.SetValue(m.search.query)
		m.search.searchInput.CursorEnd()
		m.search.searchInput.Focus()
		return m, nil
	case "F":
		m.search.query = ""
		m.search.searchInput.SetValue("")
		m.applyFilter()
		return m, nil

	case "p": // Prefix bearbeiten
		m.filter.prefixing = true
		m.filter.prefixInput.SetValue(m.filter.prefix)
		m.filter.prefixInput.CursorEnd()
		m.filter.prefixInput.Focus()
		return m, nil
	case "P": // Prefix schnell löschen
		m.filter.prefix = ""
		m.filter.prefixInput.SetValue("")
		m.applyFilter()
		return m, nil

	case "c":
		m.search.query = ""
		m.filter.prefix = ""
		m.search.searchInput.SetValue("")
		m.applyFilter()
		m.filter.prefixInput.SetValue("")
		m.applyFilter()
		return m, nil

	case "j", "down":
		if m.selection.listIndex < m.itemsLen()-1 {
			m.selection.listIndex++
			m.ensureCursorVisible()
		}
	case "k", "up":
		if m.selection.listIndex > 0 {
			m.selection.listIndex--
			m.ensureCursorVisible()
		}
	case "ctrl+d", "pgdown":
		if m.itemsLen() > 0 {
			m.selection.listIndex += m.selection.listViewport
			if m.selection.listIndex > m.itemsLen()-1 {
				m.selection.listIndex = m.itemsLen() - 1
			}
			m.ensureCursorVisible()
		}
	case "ctrl+u", "pgup":
		m.selection.listIndex -= m.selection.listViewport
		if m.selection.listIndex < 0 {
			m.selection.listIndex = 0
		}
		m.ensureCursorVisible()

	case " ":
		if m.itemsLen() == 0 {
			return m, nil
		}
		st := m.itemAt(m.selection.listIndex)
		if m.selection.selected == nil {
			m.selection.selected = make(map[string]bool)
		}
		m.selection.selected[st.FullSlug] = !m.selection.selected[st.FullSlug]

	case "r":
		m.state = stateScanning
		m.statusMsg = "Rescan…"
		return m, m.scanStoriesCmd()
	case "s":
		// Weiter zu Preflight in T6 – hier nur Platzhalter
		m.statusMsg = "Preflight (T6) folgt …"
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

func (m Model) validateTokenCmd() tea.Cmd {
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

func (m Model) scanStoriesCmd() tea.Cmd {
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

// ------ utils -------

func containsSpaceID(spacesSlice []sb.Space, spaceID string) (sb.Space, bool) {
	for _, space := range spacesSlice {
		if fmt.Sprint(space.ID) == spaceID {
			return space, true
		}
	}
	return sb.Space{}, false
}

func (m *Model) ensureCursorVisible() {
	if m.selection.listViewport <= 0 {
		m.selection.listViewport = 10
	}

	n := m.itemsLen()
	if n == 0 {
		m.selection.listIndex = 0
		m.selection.listOffset = 0
		return
	}
	if m.selection.listIndex < 0 {
		m.selection.listIndex = 0
	}
	if m.selection.listIndex > n-1 {
		m.selection.listIndex = n - 1
	}

	if m.selection.listIndex < m.selection.listOffset {
		m.selection.listOffset = m.selection.listIndex
	}
	if m.selection.listIndex >= m.selection.listOffset+m.selection.listViewport {
		m.selection.listOffset = m.selection.listIndex - m.selection.listViewport + 1
	}

	maxStart := n - m.selection.listViewport
	if maxStart < 0 {
		maxStart = 0
	}
	if m.selection.listOffset > maxStart {
		m.selection.listOffset = maxStart
	}
	if m.selection.listOffset < 0 {
		m.selection.listOffset = 0
	}
}

func (m *Model) itemsLen() int {
	if m.search.filteredIdx != nil {
		return len(m.search.filteredIdx)
	}
	return len(m.storiesSource)
}

func (m *Model) itemAt(visIdx int) sb.Story {
	if m.search.filteredIdx != nil {
		return m.storiesSource[m.search.filteredIdx[visIdx]]
	}
	return m.storiesSource[visIdx]
}

func (m *Model) applyFilter() {
	q := strings.TrimSpace(strings.ToLower(m.search.query))
	pref := strings.TrimSpace(strings.ToLower(m.filter.prefix))

	base := make([]string, len(m.storiesSource))
	for i, st := range m.storiesSource {
		name := st.Name
		if name == "" {
			name = st.Slug
		}
		base[i] = strings.ToLower(name + "  " + st.Slug + "  " + st.FullSlug)
	}

	idx := filterByPrefix(m.storiesSource, pref)

	if q == "" {
		m.search.filteredIdx = append(m.search.filteredIdx[:0], idx...)
		m.selection.listIndex, m.selection.listOffset = 0, 0
		m.ensureCursorVisible()
		return
	}

	sub := filterBySubstring(q, base, idx, m.filterCfg)
	if len(sub) > 0 {
		m.search.filteredIdx = sub
		m.selection.listIndex, m.selection.listOffset = 0, 0
		m.ensureCursorVisible()
		return
	}

	m.search.filteredIdx = filterByFuzzy(q, base, idx, m.filterCfg)
	m.selection.listIndex, m.selection.listOffset = 0, 0
	m.ensureCursorVisible()
}
