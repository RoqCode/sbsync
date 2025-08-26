package ui

import (
	"strings"
)

// applyFilter applies search and prefix filters to the story list
func (m *Model) applyFilter() {
	q := strings.TrimSpace(strings.ToLower(m.search.query))
	pref := strings.TrimSpace(strings.ToLower(m.filter.prefix))

	// Build searchable text for each story
	base := make([]string, len(m.storiesSource))
	for i, st := range m.storiesSource {
		name := st.Name
		if name == "" {
			name = st.Slug
		}
		base[i] = strings.ToLower(name + "  " + st.Slug + "  " + st.FullSlug)
	}

	// Apply prefix filter first
	idx := filterByPrefix(m.storiesSource, pref)

	// If no search query, use prefix-filtered results
	if q == "" {
		m.search.filteredIdx = append(m.search.filteredIdx[:0], idx...)
		m.selection.listIndex = 0
		m.refreshVisible()
		return
	}

	// Try substring search first (faster and more predictable)
	sub := filterBySubstring(q, base, idx, m.filterCfg)
	if len(sub) > 0 {
		m.search.filteredIdx = sub
		m.selection.listIndex = 0
		m.refreshVisible()
		return
	}

	// Fall back to fuzzy search
	m.search.filteredIdx = filterByFuzzy(q, base, idx, m.filterCfg)
	m.selection.listIndex = 0
	m.refreshVisible()
}

// refreshVisible updates the visible item indices based on current filters and hierarchy
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

	// Build parent-child mapping for hierarchy handling
	children := make(map[int][]int)
	for i, story := range m.storiesSource {
		if story.FolderID != nil {
			pid := *story.FolderID
			children[pid] = append(children[pid], i)
		}
	}

	// Recursive function to include visible items respecting folder collapse state
	var addVisible func(int) bool
	addVisible = func(idx int) bool {
		if idx >= len(m.storiesSource) {
			return false
		}

		story := m.storiesSource[idx]
		
		// If this item matches the filter, include it
		shouldInclude := included[idx]
		
		if shouldInclude {
			m.visibleIdx = append(m.visibleIdx, idx)
		}

		// If it's a folder and not collapsed, process children
		if story.IsFolder && !m.folderCollapsed[story.ID] {
			for _, childIdx := range children[story.ID] {
				addVisible(childIdx)
			}
		}

		return shouldInclude
	}

	// Reset visible indices
	m.visibleIdx = m.visibleIdx[:0]

	// Add root level items and their visible children
	for i, story := range m.storiesSource {
		if story.FolderID == nil {
			addVisible(i)
		}
	}
}

// These filtering functions are already defined in filter.go
// We'll use the existing implementations