package ui

import (
	"context"
	"fmt"
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
		err := m.syncStructure(it.Story)
		if err == nil {
			// Step 2: sync content
			switch {
			case it.StartsWith:
				err = m.syncStartsWith(it.Story.FullSlug)
			case it.Story.IsFolder:
				// nothing more
			default:
				err = m.syncStory(it.Story)
			}
		}
		if err == nil {
			m.report.AddSuccess(it.Story.FullSlug)
			time.Sleep(50 * time.Millisecond)
		} else {
			m.report.AddFailure(it.Story.FullSlug, err)
		}
		return syncResultMsg{index: idx, err: err}
	}
}

func (m *Model) syncStructure(st sb.Story) error {
	parts := strings.Split(st.FullSlug, "/")
	if !st.IsFolder {
		parts = parts[:len(parts)-1]
	}
	var parentID *int
	var path []string
	for _, p := range parts {
		path = append(path, p)
		full := strings.Join(path, "/")
		if idx := m.findTarget(full); idx >= 0 {
			id := m.storiesTarget[idx].ID
			parentID = &id
			continue
		}
		src, ok := m.findSource(full)
		if !ok {
			src = sb.Story{Name: p, Slug: p, FullSlug: full, IsFolder: true}
		}
		if parentID != nil {
			id := *parentID
			src.FolderID = &id
		} else {
			src.FolderID = nil
		}
		src.IsFolder = true
		if m.client != nil && m.targetSpace != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			created, err := m.client.CreateStory(ctx, m.targetSpace.ID, src)
			cancel()
			if err != nil {
				return fmt.Errorf("create folder %s: %w", full, err)
			}
			src = created
		} else {
			src.ID = m.nextTargetID()
		}
		m.storiesTarget = append(m.storiesTarget, src)
		id := src.ID
		parentID = &id
	}
	return nil
}

func (m *Model) syncStory(st sb.Story) error {
	if idx := m.findTarget(st.FullSlug); idx >= 0 {
		existing := m.storiesTarget[idx]
		st.ID = existing.ID
		st.FolderID = existing.FolderID
		if m.client != nil && m.targetSpace != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			_, err := m.client.UpdateStory(ctx, m.targetSpace.ID, st)
			cancel()
			if err != nil {
				return fmt.Errorf("update story %s: %w", st.FullSlug, err)
			}
		}
		m.storiesTarget[idx] = st
		return nil
	}
	if parent := parentSlug(st.FullSlug); parent != "" {
		if idx := m.findTarget(parent); idx >= 0 {
			id := m.storiesTarget[idx].ID
			st.FolderID = &id
		}
	}
	if m.client != nil && m.targetSpace != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		created, err := m.client.CreateStory(ctx, m.targetSpace.ID, st)
		cancel()
		if err != nil {
			return fmt.Errorf("create story %s: %w", st.FullSlug, err)
		}
		st = created
	} else {
		st.ID = m.nextTargetID()
	}
	m.storiesTarget = append(m.storiesTarget, st)
	return nil
}

func (m *Model) syncStartsWith(slug string) error {
	for _, st := range m.storiesSource {
		if st.FullSlug == slug || strings.HasPrefix(st.FullSlug, slug+"/") {
			if err := m.syncStructure(st); err != nil {
				return err
			}
			if !st.IsFolder {
				if err := m.syncStory(st); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (m *Model) findTarget(fullSlug string) int {
	for i, st := range m.storiesTarget {
		if st.FullSlug == fullSlug {
			return i
		}
	}
	return -1
}

func (m *Model) findSource(fullSlug string) (sb.Story, bool) {
	for _, st := range m.storiesSource {
		if st.FullSlug == fullSlug {
			return st, true
		}
	}
	return sb.Story{}, false
}

func (m *Model) nextTargetID() int {
	max := 0
	for _, st := range m.storiesTarget {
		if st.ID > max {
			max = st.ID
		}
	}
	return max + 1
}

func parentSlug(full string) string {
	if i := strings.LastIndex(full, "/"); i >= 0 {
		return full[:i]
	}
	return ""
}
