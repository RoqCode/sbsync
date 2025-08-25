package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"storyblok-sync/internal/sb"
)

// handleBrowseListKey handles all key events in the browse list state.
// This includes navigation, search, filtering, and selection logic for the browse screen.
func (m Model) handleBrowseListKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	key := msg.String()

	// The viewport will be updated after cursor movement via updateBrowseViewport()
	// Don't let viewport handle navigation keys directly as it conflicts with cursor system

	if m.filter.prefixing {
		switch key {
		case "esc":
			m.filter.prefixInput.Blur()
			if strings.TrimSpace(m.filter.prefixInput.Value()) == "" {
				m.filter.prefix = ""
			}
			m.filter.prefixing = false
			m.applyFilter()
			m.updateBrowseViewport()
			return m, nil
		case "enter":
			m.filter.prefix = strings.TrimSpace(m.filter.prefixInput.Value())
			m.filter.prefixing = false
			m.filter.prefixInput.Blur()
			m.applyFilter()
			m.updateBrowseViewport()
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
				m.collapseAllFolders()
				return m, nil
			}
			m.search.query = ""
			m.search.searchInput.SetValue("")
			m.collapseAllFolders()
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
				if m.search.query == "" && newQ != "" {
					m.expandAllFolders()
				}
				if m.search.query != "" && newQ == "" {
					m.collapseAllFolders()
				}
				m.search.query = newQ
				m.applyFilter()
				m.updateBrowseViewport()
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
		m.collapseAllFolders()
		m.applyFilter()
		m.updateBrowseViewport()
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
		m.updateBrowseViewport()
		return m, nil

	case "c":
		m.search.query = ""
		m.filter.prefix = ""
		m.search.searchInput.SetValue("")
		m.filter.prefixInput.SetValue("")
		m.collapseAllFolders()
		m.applyFilter()
		m.updateBrowseViewport()
		return m, nil

	case "l":
		if m.itemsLen() == 0 {
			return m, nil
		}
		st := m.itemAt(m.selection.listIndex)
		if st.IsFolder {
			m.folderCollapsed[st.ID] = false
			m.refreshVisible()
			m.updateBrowseViewport()
		}
	case "h":
		if m.itemsLen() == 0 {
			return m, nil
		}
		st := m.itemAt(m.selection.listIndex)
		if st.IsFolder {
			m.folderCollapsed[st.ID] = true
			m.refreshVisible()
			m.updateBrowseViewport()
		} else if st.FolderID != nil {
			pid := *st.FolderID
			m.folderCollapsed[pid] = true
			m.refreshVisible()
			if vis := m.visibleIndexByID(pid); vis >= 0 {
				m.selection.listIndex = vis
				m.ensureCursorVisible()
			}
			m.updateBrowseViewport()
		}
	case "H":
		m.collapseAllFolders()
		m.updateBrowseViewport()
	case "L":
		m.expandAllFolders()
		m.updateBrowseViewport()
	case "j", "down":
		if m.selection.listIndex < m.itemsLen()-1 {
			m.selection.listIndex++
			m.ensureCursorVisible()
			m.updateBrowseViewport()
		}
	case "k", "up":
		if m.selection.listIndex > 0 {
			m.selection.listIndex--
			m.ensureCursorVisible()
			m.updateBrowseViewport()
		}
	case "ctrl+d", "pgdown":
		if m.itemsLen() > 0 {
			m.selection.listIndex += m.selection.listViewport
			if m.selection.listIndex > m.itemsLen()-1 {
				m.selection.listIndex = m.itemsLen() - 1
			}
			m.ensureCursorVisible()
			m.updateBrowseViewport()
		}
	case "ctrl+u", "pgup":
		m.selection.listIndex -= m.selection.listViewport
		if m.selection.listIndex < 0 {
			m.selection.listIndex = 0
		}
		m.ensureCursorVisible()
		m.updateBrowseViewport()

	case " ":
		if m.itemsLen() == 0 {
			return m, nil
		}
		st := m.itemAt(m.selection.listIndex)
		if m.selection.selected == nil {
			m.selection.selected = make(map[string]bool)
		}
		if st.IsFolder {
			m.toggleFolderSelection(st)
		} else {
			m.selection.selected[st.FullSlug] = !m.selection.selected[st.FullSlug]
		}
		if m.selection.listIndex < m.itemsLen()-1 {
			m.selection.listIndex++
			m.ensureCursorVisible()
		}
		m.updateBrowseViewport()

	case "r":
		m.state = stateScanning
		m.statusMsg = "Rescan…"
		return m, m.scanStoriesCmd()
	case "s":
		m.startPreflight()
		return m, nil
	}
	return m, nil
}

// Helper functions used by handleBrowseListKey

func (m *Model) itemsLen() int {
	return len(m.visibleIdx)
}

func (m *Model) itemAt(visIdx int) sb.Story {
	return m.storiesSource[m.visibleIdx[visIdx]]
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
		m.refreshVisible()
		return
	}

	sub := filterBySubstring(q, base, idx, m.filterCfg)
	if len(sub) > 0 {
		m.search.filteredIdx = sub
		m.selection.listIndex, m.selection.listOffset = 0, 0
		m.refreshVisible()
		return
	}

	m.search.filteredIdx = filterByFuzzy(q, base, idx, m.filterCfg)
	m.selection.listIndex, m.selection.listOffset = 0, 0
	m.refreshVisible()
}

func (m *Model) refreshVisible() {
	base := m.search.filteredIdx
	if base == nil {
		base = make([]int, len(m.storiesSource))
		for i := range m.storiesSource {
			base[i] = i
		}
	}

	included := make(map[int]bool, len(base))
	for _, idx := range base {
		included[idx] = true
	}

	children := make(map[int][]int)
	roots := make([]int, 0)
	for _, idx := range base {
		st := m.storiesSource[idx]
		if st.FolderID != nil {
			if pIdx, ok := m.storyIdx[*st.FolderID]; ok && included[pIdx] {
				children[pIdx] = append(children[pIdx], idx)
				continue
			}
		}
		roots = append(roots, idx)
	}

	m.visibleIdx = m.visibleIdx[:0]
	var walk func(int)
	walk = func(idx int) {
		m.visibleIdx = append(m.visibleIdx, idx)
		st := m.storiesSource[idx]
		if st.IsFolder && m.folderCollapsed[st.ID] {
			return
		}
		for _, ch := range children[idx] {
			walk(ch)
		}
	}
	for _, r := range roots {
		walk(r)
	}
	m.ensureCursorVisible()
}

func (m *Model) collapseAllFolders() {
	for id := range m.folderCollapsed {
		m.folderCollapsed[id] = true
	}
	m.refreshVisible()
}

func (m *Model) expandAllFolders() {
	for id := range m.folderCollapsed {
		m.folderCollapsed[id] = false
	}
	m.refreshVisible()
}

func (m *Model) visibleIndexByID(id int) int {
	if idx, ok := m.storyIdx[id]; ok {
		for i, v := range m.visibleIdx {
			if v == idx {
				return i
			}
		}
	}
	return -1
}

func (m *Model) toggleFolderSelection(st sb.Story) {
	mark := !m.selection.selected[st.FullSlug]
	prefix := st.FullSlug
	for _, story := range m.storiesSource {
		if story.FullSlug == prefix || strings.HasPrefix(story.FullSlug, prefix+"/") {
			m.selection.selected[story.FullSlug] = mark
		}
	}
}
