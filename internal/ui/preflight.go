package ui

import (
	"storyblok-sync/internal/sb"
)

// SyncPlanItem represents a story selected for syncing.
// Collision indicates that a story with the same full slug exists in the target space.
// Skip marks whether the item should be skipped during sync.
type SyncPlanItem struct {
	Story     sb.Story
	Collision bool
	Skip      bool
}

// SyncPlan collects all selected stories and decisions for the upcoming sync.
type SyncPlan struct {
	Items []SyncPlanItem
}

// buildSyncPlan creates a SyncPlan from the current selection and target stories.
func (m *Model) buildSyncPlan() SyncPlan {
	tgt := make(map[string]sb.Story, len(m.storiesTarget))
	for _, st := range m.storiesTarget {
		tgt[st.FullSlug] = st
	}
	plan := SyncPlan{Items: make([]SyncPlanItem, 0, len(m.selection.selected))}
	for _, st := range m.storiesSource {
		if m.selection.selected[st.FullSlug] {
			_, coll := tgt[st.FullSlug]
			plan.Items = append(plan.Items, SyncPlanItem{Story: st, Collision: coll})
		}
	}
	return plan
}

// ensurePreflightCursorVisible keeps the preflight cursor inside the viewport.
func (m *Model) ensurePreflightCursorVisible() {
	vp := m.selection.listViewport
	if vp <= 0 {
		vp = 10
	}
	n := len(m.preflight.plan.Items)
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
