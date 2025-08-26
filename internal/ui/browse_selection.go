package ui

import (
	tea "github.com/charmbracelet/bubbletea"
	"storyblok-sync/internal/sb"
)

// handleBrowseSelection handles item selection with space key
func (m Model) handleBrowseSelection() (Model, tea.Cmd) {
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
	
	// Move cursor down after selection
	if m.selection.listIndex < m.itemsLen()-1 {
		m.selection.listIndex++
		m.ensureCursorVisible()
	}
	m.updateBrowseViewport()
	
	return m, nil
}

// toggleFolderSelection toggles selection for a folder and all its children
func (m Model) toggleFolderSelection(st sb.Story) {
	// Determine new selection state based on current folder state
	mark := !m.selection.selected[st.FullSlug]
	prefix := st.FullSlug
	
	// Apply to folder and all children
	for _, story := range m.storiesSource {
		if story.FullSlug == prefix || (len(story.FullSlug) > len(prefix) && 
			story.FullSlug[:len(prefix)] == prefix && 
			len(story.FullSlug) > len(prefix) && 
			story.FullSlug[len(prefix)] == '/') {
			m.selection.selected[story.FullSlug] = mark
		}
	}
}