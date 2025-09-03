package ui

import tea "github.com/charmbracelet/bubbletea"

// handleBrowseTreeNavigation handles tree expand/collapse navigation
func (m Model) handleBrowseTreeNavigation(key string) (Model, tea.Cmd) {
	switch key {
	case "l":
		// Expand folder
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
		// Collapse folder or navigate to parent
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
			if pid == 0 {
				// already at root
				return m, nil
			}
			m.folderCollapsed[pid] = true
			m.refreshVisible()
			if vis := m.visibleIndexByID(pid); vis >= 0 {
				m.selection.listIndex = vis
				m.ensureCursorVisible()
			}
			m.updateBrowseViewport()
		}
	case "H":
		// Collapse all folders
		m.collapseAllFolders()
		m.updateBrowseViewport()
	case "L":
		// Expand all folders
		m.expandAllFolders()
		m.updateBrowseViewport()
	}
	return m, nil
}

// handleBrowseCursorMovement handles cursor movement navigation
func (m Model) handleBrowseCursorMovement(key string) (Model, tea.Cmd) {
	switch key {
	case "j", "down":
		// Move cursor down
		if m.selection.listIndex < m.itemsLen()-1 {
			m.selection.listIndex++
			m.ensureCursorVisible()
			m.updateBrowseViewport()
		}
	case "k", "up":
		// Move cursor up
		if m.selection.listIndex > 0 {
			m.selection.listIndex--
			m.ensureCursorVisible()
			m.updateBrowseViewport()
		}
	case "ctrl+d", "pgdown":
		// Page down
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
		// Page up
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
	}
	return m, nil
}

// handleBrowseActions handles action keys (rescan, start sync)
func (m Model) handleBrowseActions(key string) (Model, tea.Cmd) {
	switch key {
	case "r":
		// Rescan stories
		m.state = stateScanning
		return m, m.scanStoriesCmd()
	case "s":
		// Start sync
		if len(m.selection.selected) == 0 {
			m.statusMsg = "Keine Elemente ausgewählt!"
			return m, nil
		}
		m.startPreflight()
		return m, nil
	}
	return m, nil
}
