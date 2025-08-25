package ui

import (
	"context"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"storyblok-sync/internal/sb"
)

func (m Model) handlePreflightKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	if m.syncing {
		return m, nil
	}
	key := msg.String()
	switch key {
	case "j", "down":
		if m.preflight.listIndex < len(m.preflight.items)-1 {
			m.preflight.listIndex++
			m.ensurePreflightCursorVisible()
		}
	case "k", "up":
		if m.preflight.listIndex > 0 {
			m.preflight.listIndex--
			m.ensurePreflightCursorVisible()
		}
	case "x":
		if len(m.preflight.items) > 0 {
			it := &m.preflight.items[m.preflight.listIndex]
			if it.Collision && it.Selected {
				it.Skip = !it.Skip
				it.RecalcState()
			}
		}
	case "X":
		for i := range m.preflight.items {
			if m.preflight.items[i].Collision && m.preflight.items[i].Selected {
				m.preflight.items[i].Skip = true
				m.preflight.items[i].RecalcState()
			}
		}
	case "c":
		removed := false
		for _, it := range m.preflight.items {
			if it.Skip {
				delete(m.selection.selected, it.Story.FullSlug)
				removed = true
			}
		}
		if removed {
			m.startPreflight()
		}
	case "esc", "q":
		m.state = stateBrowseList
		return m, nil
	case "enter":
		m.optimizePreflight()
		if len(m.preflight.items) == 0 {
			m.statusMsg = "Keine Items zum Sync"
			return m, nil
		}
		m.plan = SyncPlan{Items: append([]PreflightItem(nil), m.preflight.items...)}
		m.syncing = true
		m.syncIndex = 0
		m.api = sb.New(m.cfg.Token)

		// Set up cancellation context for sync operations
		m.syncContext, m.syncCancel = context.WithCancel(context.Background())

		// Initialize comprehensive report with space information
		sourceSpaceName := ""
		targetSpaceName := ""
		if m.sourceSpace != nil {
			sourceSpaceName = fmt.Sprintf("%s (%d)", m.sourceSpace.Name, m.sourceSpace.ID)
		}
		if m.targetSpace != nil {
			targetSpaceName = fmt.Sprintf("%s (%d)", m.targetSpace.Name, m.targetSpace.ID)
		}
		m.report = *NewReport(sourceSpaceName, targetSpaceName)

		m.statusMsg = fmt.Sprintf("Synchronisiere %d Items…", len(m.preflight.items))
		return m, tea.Batch(m.spinner.Tick, m.runNextItem())
	}
	return m, nil
}

func (m *Model) startPreflight() {
	target := make(map[string]bool, len(m.storiesTarget))
	for _, st := range m.storiesTarget {
		target[st.FullSlug] = true
	}
	included := make(map[int]bool)
	for slug, v := range m.selection.selected {
		if !v {
			continue
		}
		if idx := m.indexBySlug(slug); idx >= 0 {
			m.includeAncestors(idx, included)
		}
	}
	if len(included) == 0 {
		m.preflight = PreflightState{listViewport: m.selection.listViewport}
		m.statusMsg = "Keine Stories markiert."
		return
	}
	children := make(map[int][]int)
	roots := make([]int, 0)
	for i, st := range m.storiesSource {
		if !included[i] {
			continue
		}
		if st.FolderID != nil {
			if pIdx, ok := m.storyIdx[*st.FolderID]; ok && included[pIdx] {
				children[pIdx] = append(children[pIdx], i)
				continue
			}
		}
		roots = append(roots, i)
	}
	items := make([]PreflightItem, 0, len(included))
	var walk func(int)
	walk = func(idx int) {
		st := m.storiesSource[idx]
		sel := m.selection.selected[st.FullSlug]
		it := PreflightItem{Story: st, Collision: target[st.FullSlug], Selected: sel, Skip: !sel}
		it.RecalcState()
		items = append(items, it)
		for _, ch := range children[idx] {
			walk(ch)
		}
	}
	for _, r := range roots {
		walk(r)
	}
	m.preflight = PreflightState{items: items, listIndex: 0, listOffset: 0, listViewport: m.selection.listViewport}
	m.state = statePreflight
	collisions := 0
	for _, it := range items {
		if it.Collision {
			collisions++
		}
	}
	m.statusMsg = fmt.Sprintf("Preflight: %d Items, %d Kollisionen", len(items), collisions)
}

func (m *Model) ensurePreflightCursorVisible() {
	vp := m.preflight.listViewport
	if vp <= 0 {
		vp = 10
	}
	n := len(m.preflight.items)
	if n == 0 {
		m.preflight.listIndex = 0
		m.preflight.listOffset = 0
		return
	}
	if m.preflight.listIndex < 0 {
		m.preflight.listIndex = 0
	}
	if m.preflight.listIndex > n-1 {
		m.preflight.listIndex = n - 1
	}
	if m.preflight.listIndex < m.preflight.listOffset {
		m.preflight.listOffset = m.preflight.listIndex
	}
	if m.preflight.listIndex >= m.preflight.listOffset+vp {
		m.preflight.listOffset = m.preflight.listIndex - vp + 1
	}
	maxStart := n - vp
	if maxStart < 0 {
		maxStart = 0
	}
	if m.preflight.listOffset > maxStart {
		m.preflight.listOffset = maxStart
	}
	if m.preflight.listOffset < 0 {
		m.preflight.listOffset = 0
	}
}
