package ui

import (
	"context"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

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
			m.updateViewportContent()
		}
	case "k", "up":
		if m.preflight.listIndex > 0 {
			m.preflight.listIndex--
			m.ensurePreflightCursorVisible()
			m.updateViewportContent()
		}
	case "ctrl+d", "pgdown":
		jump := m.viewport.Height
		if jump <= 0 {
			jump = 10
		}
		if m.preflight.listIndex+jump < len(m.preflight.items) {
			m.preflight.listIndex += jump
		} else {
			m.preflight.listIndex = len(m.preflight.items) - 1
		}
		m.ensurePreflightCursorVisible()
		m.updateViewportContent()
	case "ctrl+u", "pgup":
		jump := m.viewport.Height
		if jump <= 0 {
			jump = 10
		}
		if m.preflight.listIndex-jump >= 0 {
			m.preflight.listIndex -= jump
		} else {
			m.preflight.listIndex = 0
		}
		m.ensurePreflightCursorVisible()
		m.updateViewportContent()
	case "x":
		if len(m.preflight.items) > 0 {
			it := &m.preflight.items[m.preflight.listIndex]
			if it.Collision && it.Selected {
				it.Skip = !it.Skip
				it.RecalcState()
				m.updateViewportContent()
			}
		}
	case "X":
		for i := range m.preflight.items {
			if m.preflight.items[i].Collision && m.preflight.items[i].Selected {
				m.preflight.items[i].Skip = true
				m.preflight.items[i].RecalcState()
			}
		}
		m.updateViewportContent()
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
		m.updateViewportContent()
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
		m.state = stateSync

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
		m.preflight = PreflightState{}
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
	m.preflight = PreflightState{items: items, listIndex: 0}
	m.state = statePreflight
	collisions := 0
	for _, it := range items {
		if it.Collision {
			collisions++
		}
	}
	m.statusMsg = fmt.Sprintf("Preflight: %d Items, %d Kollisionen", len(items), collisions)
	m.updateViewportContent()
}

func (m *Model) ensurePreflightCursorVisible() {
	n := len(m.preflight.items)
	if n == 0 {
		m.preflight.listIndex = 0
		return
	}
	if m.preflight.listIndex < 0 {
		m.preflight.listIndex = 0
	}
	if m.preflight.listIndex > n-1 {
		m.preflight.listIndex = n - 1
	}
	// Calculate which line in the viewport content the cursor is on
	cursorLine := m.calculatePreflightCursorLine()

	// Check if cursor line is visible in viewport
	topLine := m.viewport.YOffset
	bottomLine := topLine + m.viewport.Height - 1

	// Scroll margin - start scrolling when cursor approaches edges
	scrollMargin := 3
	if m.viewport.Height < 8 {
		scrollMargin = 1
	}

	if cursorLine < topLine+scrollMargin {
		// Cursor too close to top - scroll up
		targetLine := cursorLine - scrollMargin
		if targetLine < 0 {
			targetLine = 0
		}
		m.viewport.SetYOffset(targetLine)
	} else if cursorLine > bottomLine-scrollMargin {
		// Cursor too close to bottom - scroll down
		targetLine := cursorLine - m.viewport.Height + scrollMargin + 1
		if targetLine < 0 {
			targetLine = 0
		}
		m.viewport.SetYOffset(targetLine)
	}
}

func (m *Model) calculatePreflightCursorLine() int {
	// Calculate the actual visual line number where the cursor appears
	// This must match exactly how preflight view renders each line

	totalLines := 0

	// Preflight view uses width - 2 for content (cursorCell + stateCell + content)
	contentWidth := m.width - 2
	if contentWidth <= 0 {
		contentWidth = 80
	}

	for i := 0; i < m.preflight.listIndex && i < len(m.preflight.items); i++ {
		item := m.preflight.items[i]

		// Recreate the exact content formatting from view_preflight.go
		content := m.formatPreflightContent(item)

		// Apply the exact same styling as in view_preflight.go
		lineStyle := lipgloss.NewStyle().Width(contentWidth)
		if i == m.preflight.listIndex-1 { // If this will be the cursor item
			lineStyle = cursorLineStyle.Copy().Width(contentWidth)
		}
		if item.State == StateSkip {
			lineStyle = lineStyle.Faint(true)
		}

		styledContent := lineStyle.Render(content)

		// Count wrapped lines in styled content
		wrappedLines := m.countPreflightWrappedLines(styledContent)
		totalLines += wrappedLines
	}

	return totalLines
}

func (m *Model) formatPreflightContent(item PreflightItem) string {
	// Recreate the exact content formatting from view_preflight.go
	story := item.Story

	// Create story display content
	content := fmt.Sprintf("%s  (%s)", story.Name, story.FullSlug)

	// Add collision indicator exactly as in view_preflight.go
	if item.Collision {
		content = collisionSign + " " + content
	} else {
		content = "  " + content
	}

	return content
}

func (m *Model) countPreflightWrappedLines(styledContent string) int {
	// Count actual newlines in the styled content
	lines := 1
	for _, char := range styledContent {
		if char == '\n' {
			lines++
		}
	}
	return lines
}
