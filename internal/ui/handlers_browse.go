package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	tree "github.com/charmbracelet/lipgloss/tree"
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
			jump := m.viewport.Height
			if jump <= 0 {
				jump = 10
			}
			m.selection.listIndex += jump
			if m.selection.listIndex > m.itemsLen()-1 {
				m.selection.listIndex = m.itemsLen() - 1
			}
			m.ensureCursorVisible()
			m.updateBrowseViewport()
		}
	case "ctrl+u", "pgup":
		jump := m.viewport.Height
		if jump <= 0 {
			jump = 10
		}
		m.selection.listIndex -= jump
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
	n := m.itemsLen()
	if n == 0 {
		m.selection.listIndex = 0
		return
	}
	if m.selection.listIndex < 0 {
		m.selection.listIndex = 0
	}
	if m.selection.listIndex > n-1 {
		m.selection.listIndex = n - 1
	}

	// Calculate which line in the viewport content the cursor is on
	cursorLine := m.calculateCursorLine()

	// Check if cursor line is visible in viewport
	topLine := m.viewport.YOffset
	bottomLine := topLine + m.viewport.Height - 1

	// Scroll margin - start scrolling when cursor approaches edges
	scrollMargin := 3
	if m.viewport.Height < 8 {
		scrollMargin = 1
	}

	if cursorLine < topLine+scrollMargin {
		// Cursor too close to top - scroll up
		targetLine := cursorLine - scrollMargin
		if targetLine < 0 {
			targetLine = 0
		}
		m.viewport.SetYOffset(targetLine)
	} else if cursorLine > bottomLine-scrollMargin {
		// Cursor too close to bottom - scroll down
		targetLine := cursorLine - m.viewport.Height + scrollMargin + 1
		if targetLine < 0 {
			targetLine = 0
		}
		m.viewport.SetYOffset(targetLine)
	}
}

func (m *Model) calculateCursorLine() int {
	// Calculate the actual visual line number where the cursor appears
	// This must match exactly how browse view renders each line

	totalLines := 0

	// We need to recreate the exact same tree rendering logic as view_browse.go
	// Get all stories up to cursor position
	stories := make([]sb.Story, m.selection.listIndex)
	for i := 0; i < m.selection.listIndex && i < len(m.visibleIdx); i++ {
		stories[i] = m.itemAt(i)
	}

	if len(stories) == 0 {
		return 0
	}

	// Recreate the tree structure exactly as in view_browse.go
	treeLines := m.generateTreeLines(stories)

	// Calculate wrapped lines for each tree line with proper formatting
	contentWidth := m.width - 4 // Same as view: cursorCell + markCell + content styled width
	if contentWidth <= 0 {
		contentWidth = 80
	}

	for i, treeLine := range treeLines {
		// Apply the exact same styling as in view_browse.go
		var styledContent string
		if i == m.selection.listIndex-1 { // Last item is cursor item
			styledContent = cursorLineStyle.Width(contentWidth).Render(treeLine)
		} else {
			styledContent = lipgloss.NewStyle().Width(contentWidth).Render(treeLine)
		}

		// Create complete line: cursorCell + markCell + styledContent
		// But for line counting, we only care about the styled content wrapping
		wrappedLines := m.countWrappedLines(styledContent)
		totalLines += wrappedLines
	}

	return totalLines
}

func (m *Model) generateTreeLines(stories []sb.Story) []string {
	if len(stories) == 0 {
		return []string{}
	}

	// Use the same tree generation as view_browse.go
	tr := tree.New()
	nodes := make(map[int]*tree.Tree, len(stories))

	for _, st := range stories {
		node := tree.Root(displayStory(st))
		nodes[st.ID] = node
	}

	for _, st := range stories {
		node := nodes[st.ID]
		if st.FolderID != nil {
			if parent, ok := nodes[*st.FolderID]; ok {
				parent.Child(node)
				continue
			}
		}
		tr.Child(node)
	}

	// Get tree lines exactly as in view_browse.go
	lines := strings.Split(tr.String(), "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}

	return lines
}

func (m *Model) countWrappedLines(styledContent string) int {
	// Count actual newlines in the styled content
	lines := 1
	for _, char := range styledContent {
		if char == '\n' {
			lines++
		}
	}
	return lines
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
		m.selection.listIndex = 0
		m.refreshVisible()
		return
	}

	sub := filterBySubstring(q, base, idx, m.filterCfg)
	if len(sub) > 0 {
		m.search.filteredIdx = sub
		m.selection.listIndex = 0
		m.refreshVisible()
		return
	}

	m.search.filteredIdx = filterByFuzzy(q, base, idx, m.filterCfg)
	m.selection.listIndex = 0
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
