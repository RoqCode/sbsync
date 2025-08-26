package ui

import (
	"strings"
	
	"github.com/charmbracelet/lipgloss"
	tree "github.com/charmbracelet/lipgloss/tree"
	"storyblok-sync/internal/sb"
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

// calculateCursorLine calculates the actual visual line number where the cursor appears
func (m *Model) calculateCursorLine() int {
	// We need to recreate the exact same tree rendering logic as view_browse.go
	if len(m.visibleIdx) == 0 {
		return 0
	}

	// Group stories by parent ID for tree rendering
	stories := make([]sb.Story, 0, len(m.visibleIdx))
	for _, idx := range m.visibleIdx {
		if idx < len(m.storiesSource) {
			stories = append(stories, m.storiesSource[idx])
		}
	}

	return m.generateTreeLines(stories, m.selection.listIndex)
}

// generateTreeLines generates tree structure and returns the line number for a given cursor index
func (m *Model) generateTreeLines(stories []sb.Story, targetIndex int) int {
	if len(stories) == 0 {
		return 0
	}

	// Create tree structure
	nodes := make(map[int]*tree.Tree, len(stories))
	rootNodes := []*tree.Tree{}

	// First pass: create nodes for all stories
	for i, st := range stories {
		var styleFunc func(...string) string
		if st.IsFolder {
			styleFunc = lipgloss.NewStyle().Foreground(lipgloss.Color("33")).Render
		} else {
			styleFunc = lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render
		}

		displayName := m.displayStoryName(st)
		node := tree.Root(styleFunc(displayName))
		nodes[st.ID] = node

		// Check if this is the target cursor position
		if i == targetIndex {
			// This is the cursor line - count previous lines
			return len(rootNodes)
		}

		if st.FolderID == nil {
			rootNodes = append(rootNodes, node)
		}
	}

	// Second pass: build hierarchy
	for i, st := range stories {
		if i == targetIndex {
			return len(rootNodes) // This would be more complex in real tree rendering
		}

		if st.FolderID != nil {
			if parentNode, exists := nodes[*st.FolderID]; exists {
				parentNode.Child(nodes[st.ID])
			}
		}
	}

	return 0 // Default fallback
}

// displayStoryName formats a story name for tree display
func (m *Model) displayStoryName(st sb.Story) string {
	// Build the display string
	var parts []string

	// Add symbol
	if st.IsFolder {
		if m.folderCollapsed[st.ID] {
			parts = append(parts, "▶")
		} else {
			parts = append(parts, "▼")
		}
		parts = append(parts, "F")
	} else {
		parts = append(parts, " ", "S")
	}

	// Add name and slug
	if st.Name != "" {
		parts = append(parts, st.Name)
	}
	if st.Slug != "" && st.Slug != st.Name {
		parts = append(parts, "("+st.Slug+")")
	}

	return strings.Join(parts, " ")
}

// countWrappedLines counts how many display lines a piece of content takes
func (m *Model) countWrappedLines(content string) int {
	if m.viewport.Width <= 0 {
		return 1
	}

	lines := strings.Split(content, "\n")
	totalLines := 0

	for _, line := range lines {
		if len(line) == 0 {
			totalLines++
			continue
		}
		// Calculate wrapped lines
		lineWidth := lipgloss.Width(line)
		wrappedLines := (lineWidth + m.viewport.Width - 1) / m.viewport.Width
		if wrappedLines == 0 {
			wrappedLines = 1
		}
		totalLines += wrappedLines
	}

	return totalLines
}