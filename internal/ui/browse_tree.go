package ui

import "storyblok-sync/internal/sb"

// collapseAllFolders collapses all folders in the tree
func (m *Model) collapseAllFolders() {
	for id := range m.folderCollapsed {
		m.folderCollapsed[id] = true
	}
	m.refreshVisible()
}

// expandAllFolders expands all folders in the tree
func (m *Model) expandAllFolders() {
	for id := range m.folderCollapsed {
		m.folderCollapsed[id] = false
	}
	m.refreshVisible()
}

// visibleIndexByID finds the visible index for a story by its ID
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

// itemsLen returns the count of visible items
func (m *Model) itemsLen() int {
	return len(m.visibleIdx)
}

// itemAt returns the story at the given visible index
func (m *Model) itemAt(visIdx int) sb.Story {
	if visIdx < 0 || visIdx >= len(m.visibleIdx) {
		return sb.Story{} // Return empty story as fallback
	}
	sourceIdx := m.visibleIdx[visIdx]
	if sourceIdx < 0 || sourceIdx >= len(m.storiesSource) {
		return sb.Story{} // Return empty story as fallback
	}
	return m.storiesSource[sourceIdx]
}
