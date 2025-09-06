package ui

import (
	"fmt"
	"sort"
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

// --- Publish mode helpers ---

const (
	PublishModeDraft          = "draft"
	PublishModePublish        = "publish"
	PublishModePublishChanges = "publish_changes"
)

// getPublishMode returns the publish mode for a slug, defaulting to draft.
func (m *Model) getPublishMode(slug string) string {
	if m.publishMode == nil {
		return PublishModeDraft
	}
	if v, ok := m.publishMode[slug]; ok && v != "" {
		return v
	}
	return PublishModeDraft
}

func (m *Model) setPublishMode(slug, mode string) {
	if m.publishMode == nil {
		m.publishMode = make(map[string]string)
	}
	switch mode {
	case PublishModeDraft, PublishModePublish, PublishModePublishChanges:
		m.publishMode[slug] = mode
	default:
		m.publishMode[slug] = PublishModeDraft
	}
}

// cyclePublishModeForSlug cycles through allowed modes for the slug, skipping invalid states.
func (m *Model) cyclePublishModeForSlug(slug string) {
	var it *PreflightItem
	for i := range m.preflight.items {
		if m.preflight.items[i].Story.FullSlug == slug {
			it = &m.preflight.items[i]
			break
		}
	}
	if it == nil || it.Story.IsFolder {
		return
	}
	curr := m.getPublishMode(slug)
	next := func(x string) string {
		switch x {
		case PublishModeDraft:
			return PublishModePublish
		case PublishModePublish:
			return PublishModePublishChanges
		default:
			return PublishModeDraft
		}
	}
	for i := 0; i < 3; i++ {
		cand := next(curr)
		if m.isPublishModeValid(*it, cand) {
			m.setPublishMode(slug, cand)
			return
		}
		curr = cand
	}
	m.setPublishMode(slug, PublishModeDraft)
}

// isPublishModeValid checks if a mode is valid for the item based on existence/target state.
func (m *Model) isPublishModeValid(it PreflightItem, mode string) bool {
	if it.Story.IsFolder {
		return false
	}
	exists, tgtPublished := false, false
	for _, t := range m.storiesTarget {
		if t.FullSlug == it.Story.FullSlug {
			exists = true
			tgtPublished = t.Published
			break
		}
	}
	switch mode {
	case PublishModePublishChanges:
		return exists && tgtPublished
	case PublishModePublish, PublishModeDraft:
		return true
	default:
		return false
	}
}

// initDefaultPublishModes computes initial publish modes per item.
func (m *Model) initDefaultPublishModes() {
	if m.publishMode == nil {
		m.publishMode = make(map[string]string)
	}
	// Default policy: mirror source state (overwrite when both published)
	for _, it := range m.preflight.items {
		if it.Story.IsFolder || !it.Selected || it.Skip {
			continue
		}
		if it.Story.Published && m.shouldPublish() {
			m.publishMode[it.Story.FullSlug] = PublishModePublish
		} else {
			m.publishMode[it.Story.FullSlug] = PublishModeDraft
		}
	}
}

// sortStrings is a small helper used elsewhere; keep here to avoid import churn
func sortStrings(s []string) {
	sort.Strings(s)
}

// expectedWriteUnits estimates write cost for scheduling budget.
// Stories normally cost 1 write; Draft over published (overwrite+unpublish) costs 2.
// Folders cost 1.
func (m *Model) expectedWriteUnits(it PreflightItem) int {
	if it.Story.IsFolder {
		return 1
	}
	mode := m.getPublishMode(it.Story.FullSlug)
	exists, tgtPublished := false, false
	for _, t := range m.storiesTarget {
		if t.FullSlug == it.Story.FullSlug {
			exists = true
			tgtPublished = t.Published
			break
		}
	}
	if mode == PublishModeDraft && it.Story.Published && exists && tgtPublished {
		return 2
	}
	return 1
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
