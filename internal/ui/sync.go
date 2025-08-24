package ui

import (
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"storyblok-sync/internal/sb"
)

type syncResultMsg struct {
	index int
	err   error
}

// optimizePreflight deduplicates entries and merges full folder selections into starts_with tasks.
func (m *Model) optimizePreflight() {
	selected := make(map[string]*PreflightItem)
	for i := range m.preflight.items {
		it := &m.preflight.items[i]
		if it.Skip {
			continue
		}
		if _, ok := selected[it.Story.FullSlug]; ok {
			it.Skip = true
			continue
		}
		selected[it.Story.FullSlug] = it
	}
	for _, it := range selected {
		if !it.Story.IsFolder {
			continue
		}
		prefix := it.Story.FullSlug + "/"
		all := true
		for _, st := range m.storiesSource {
			if strings.HasPrefix(st.FullSlug, prefix) {
				if _, ok := selected[st.FullSlug]; !ok {
					all = false
					break
				}
			}
		}
		if all {
			it.StartsWith = true
			for slug, ch := range selected {
				if slug != it.Story.FullSlug && strings.HasPrefix(slug, prefix) {
					ch.Skip = true
				}
			}
		}
	}
	optimized := make([]PreflightItem, 0, len(m.preflight.items))
	for _, it := range m.preflight.items {
		if it.Skip {
			continue
		}
		it.Run = RunPending
		optimized = append(optimized, it)
	}
	m.preflight.items = optimized
}

func (m *Model) runNextItem() tea.Cmd {
	if m.syncIndex >= len(m.preflight.items) {
		return nil
	}
	idx := m.syncIndex
	m.preflight.items[idx].Run = RunRunning
	return func() tea.Msg {
		it := m.preflight.items[idx]
		// Step 1: ensure structure
		_ = m.syncStructure(it.Story)
		// Step 2: sync content
		switch {
		case it.StartsWith:
			_ = m.syncStartsWith(it.Story.FullSlug)
		case it.Story.IsFolder:
			// nothing more
		default:
			_ = m.syncStory(it.Story)
		}
		time.Sleep(50 * time.Millisecond)
		return syncResultMsg{index: idx, err: nil}
	}
}

func (m *Model) syncStructure(st sb.Story) error {
	// placeholder: ensure folder path exists
	_ = st
	return nil
}

func (m *Model) syncStory(st sb.Story) error {
	// placeholder: sync single story
	_ = st
	return nil
}

func (m *Model) syncStartsWith(slug string) error {
	// placeholder: bulk sync for folder
	_ = slug
	return nil
}
