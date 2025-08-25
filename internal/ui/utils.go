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
		if st.FolderID == nil {
			return
		}
		pIdx, ok := m.storyIdx[*st.FolderID]
		if !ok {
			return
		}
		idx = pIdx
	}
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
