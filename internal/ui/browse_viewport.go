package ui

import (
    "github.com/charmbracelet/lipgloss"
)

// ensureCursorVisible ensures the cursor stays within the viewport bounds
func (m *Model) ensureCursorVisible() {
	n := m.itemsLen()
	if n == 0 {
		m.selection.listIndex = 0
		return
	}
	if m.selection.listIndex < 0 {
		m.selection.listIndex = 0
	}
	if m.selection.listIndex > n-1 {
		m.selection.listIndex = n - 1
	}

	// Calculate which line in the viewport content the cursor is on
	cursorLine := m.calculateCursorLine()

    // Adjust viewport using shared helper
    m.ensureCursorInViewport(cursorLine)
}

// calculateCursorLine calculates the actual visual line number where the cursor appears
func (m *Model) calculateCursorLine() int {
	total := m.itemsLen()
	if total == 0 || m.selection.listIndex < 0 {
		return 0
	}

    // Get ALL visible stories (not just up to cursor) via shared helper
    stories, _ := m.visibleOrderBrowse()

    // Generate the complete tree structure exactly as in view_browse.go
    treeLines := generateTreeLinesFromStories(stories)

	// Calculate visual lines up to the cursor position
	totalLines := 0
	contentWidth := m.width - 4 // Same as view: cursorCell + markCell + content styled width
	if contentWidth <= 0 {
		contentWidth = 80
	}

	// Count lines up to (but not including) the cursor position
	cursorPos := m.selection.listIndex
	if cursorPos > len(treeLines) {
		cursorPos = len(treeLines)
	}

	for i := 0; i < cursorPos; i++ {
		if i >= len(treeLines) {
			break
		}
		
		// Apply the same styling as in view_browse.go
		styledContent := lipgloss.NewStyle().Width(contentWidth).Render(treeLines[i])
		
    // Count wrapped lines for this styled content
    wrappedLines := m.countWrappedLines(styledContent)
    totalLines += wrappedLines
	}

	return totalLines
}

// generateTreeLines generates tree structure exactly as in view_browse.go
// generateTreeLines is now shared in tree_lines.go


// countWrappedLines counts how many display lines a piece of styled content takes
// countWrappedLines moved to viewport_shared.go for reuse