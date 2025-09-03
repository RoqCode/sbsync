package ui

import (
	"fmt"
	"strings"

	"storyblok-sync/internal/sb"
)

// ------ utils -------

func containsSpaceID(spacesSlice []sb.Space, spaceID string) (sb.Space, bool) {
	for _, space := range spacesSlice {
		if fmt.Sprint(space.ID) == spaceID {
			return space, true
		}
	}
	return sb.Space{}, false
}

func (m *Model) indexBySlug(slug string) int {
	for i, st := range m.storiesSource {
		if st.FullSlug == slug {
			return i
		}
	}
	return -1
}

func (m *Model) includeAncestors(idx int, inc map[int]bool) {
	for {
		if inc[idx] {
			return
		}
		inc[idx] = true
		st := m.storiesSource[idx]
		// Treat parent_id == 0 as root (no parent)
		if st.FolderID == nil || (st.FolderID != nil && *st.FolderID == 0) {
			return
		}
		pIdx, ok := m.storyIdx[*st.FolderID]
		if !ok {
			return
		}
		idx = pIdx
	}
}

// isRootStory returns true when the story is at root level.
// Storyblok MA may encode root as parent_id = 0 instead of null.
func isRootStory(st sb.Story) bool {
	if st.FolderID == nil {
		return true
	}
	return *st.FolderID == 0
}

func (m *Model) rebuildStoryIndex() {
	m.storyIdx = make(map[int]int, len(m.storiesSource))
	m.folderCollapsed = make(map[int]bool)
	for i, st := range m.storiesSource {
		m.storyIdx[st.ID] = i
		if st.IsFolder {
			m.folderCollapsed[st.ID] = true
		}
	}
}

func (m Model) hasSelectedDescendant(slug string) bool {
	prefix := slug + "/"
	for s, v := range m.selection.selected {
		if v && strings.HasPrefix(s, prefix) {
			return true
		}
	}
	return false
}

func (m Model) hasSelectedDirectChild(slug string) bool {
	prefix := slug + "/"
	for s, v := range m.selection.selected {
		if v && strings.HasPrefix(s, prefix) {
			rest := strings.TrimPrefix(s, prefix)
			if !strings.Contains(rest, "/") {
				return true
			}
		}
	}
	return false
}

// selectableSpaces returns the list of spaces available for the current selection step.
// When selecting the target space, it excludes the already picked source space.
func (m Model) selectableSpaces() []sb.Space {
	if m.selectingSource || m.sourceSpace == nil {
		return m.spaces
	}
	filtered := make([]sb.Space, 0, len(m.spaces))
	for _, sp := range m.spaces {
		if sp.ID != m.sourceSpace.ID {
			filtered = append(filtered, sp)
		}
	}
	return filtered
}
