package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"storyblok-sync/internal/sb"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/sahilm/fuzzy"
)

// ---------- Update ----------
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.KeyMsg:
		key := msg.String()

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
			if m.prefixing {
				switch key {
				case "esc":
					m.prefixInput.Blur()
					if strings.TrimSpace(m.prefixInput.Value()) == "" {
						m.prefix = ""
					}
					m.prefixing = false
					m.applyFilter()
					return m, nil
				case "enter":
					m.prefix = strings.TrimSpace(m.prefixInput.Value())
					m.prefixing = false
					m.prefixInput.Blur()
					m.applyFilter()
					return m, nil
				case "ctrl+c", "q":
					return m, tea.Quit
				default:
					var cmd tea.Cmd
					m.prefixInput, cmd = m.prefixInput.Update(msg)
					return m, cmd
				}
			}

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
			case "F":
				m.query = ""
				m.searchInput.SetValue("")
				m.applyFilter()
				return m, nil

			case "p": // Prefix bearbeiten
				m.prefixing = true
				m.prefixInput.SetValue(m.prefix)
				m.prefixInput.CursorEnd()
				m.prefixInput.Focus()
				return m, nil
			case "P": // Prefix schnell löschen
				m.prefix = ""
				m.prefixInput.SetValue("")
				m.applyFilter()
				return m, nil

			case "c":
				m.query = ""
				m.prefix = ""
				m.searchInput.SetValue("")
				m.applyFilter()
				m.prefixInput.SetValue("")
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
		const chrome = 12
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
		m.listIndex, m.listOffset = 0, 0
		m.applyFilter()
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
	if m.listViewport <= 0 {
		m.listViewport = 10
	}

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

	if m.listIndex < m.listOffset {
		m.listOffset = m.listIndex
	}
	if m.listIndex >= m.listOffset+m.listViewport {
		m.listOffset = m.listIndex - m.listViewport + 1
	}

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

func (m *Model) itemsLen() int {
	if m.filteredIdx != nil {
		return len(m.filteredIdx)
	}
	return len(m.storiesSource)
}

func (m *Model) itemAt(visIdx int) sb.Story {
	if m.filteredIdx != nil {
		return m.storiesSource[m.filteredIdx[visIdx]]
	}
	return m.storiesSource[visIdx]
}

func (m *Model) applyFilter() {
	q := strings.TrimSpace(strings.ToLower(m.query))
	pref := strings.TrimSpace(strings.ToLower(m.prefix))

	base := make([]string, len(m.storiesSource))
	for i, st := range m.storiesSource {
		name := st.Name
		if name == "" {
			name = st.Slug
		}
		base[i] = strings.ToLower(name + "  " + st.Slug + "  " + st.FullSlug)
	}

	idx := make([]int, 0, len(m.storiesSource))
	if pref != "" {
		for i, st := range m.storiesSource {
			if strings.HasPrefix(strings.ToLower(st.FullSlug), pref) {
				idx = append(idx, i)
			}
		}
	} else {
		idx = idx[:0]
		for i := range m.storiesSource {
			idx = append(idx, i)
		}
	}

	if q == "" {
		m.filteredIdx = append(m.filteredIdx[:0], idx...)
		m.listIndex, m.listOffset = 0, 0
		m.ensureCursorVisible()
		return
	}

	sub := make([]int, 0, min(m.maxResults, len(idx)))
	for _, i := range idx {
		if strings.Contains(base[i], q) {
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

	subset := make([]string, len(idx))
	mapBack := make([]int, len(idx))
	for j, i := range idx {
		subset[j] = base[i]
		mapBack[j] = i
	}
	matches := fuzzy.Find(q, subset)

	pruned := make([]int, 0, len(matches))
	for _, mt := range matches {
		if matchCoverage(q, mt) < m.minCoverage {
			continue
		}
		if matchSpread(mt) > m.maxSpread {
			continue
		}
		pruned = append(pruned, mapBack[mt.Index])
		if len(pruned) >= m.maxResults {
			break
		}
	}
	if len(pruned) == 0 {
		for i := 0; i < len(matches) && i < m.maxResults; i++ {
			pruned = append(pruned, mapBack[matches[i].Index])
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
	return float64(len(m.MatchedIndexes)) / float64(len(q))
}

func matchSpread(m fuzzy.Match) int {
	if len(m.MatchedIndexes) == 0 {
		return 0
	}
	return m.MatchedIndexes[len(m.MatchedIndexes)-1] - m.MatchedIndexes[0]
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
