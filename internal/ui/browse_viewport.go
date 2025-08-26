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
	total := m.itemsLen()
	if total == 0 || m.selection.listIndex < 0 {
		return 0
	}

	// Get ALL visible stories (not just up to cursor)
	stories := make([]sb.Story, total)
	for i := 0; i < total; i++ {
		stories[i] = m.itemAt(i)
	}

	// Generate the complete tree structure exactly as in view_browse.go
	treeLines := m.generateTreeLines(stories)

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
func (m *Model) generateTreeLines(stories []sb.Story) []string {
	if len(stories) == 0 {
		return []string{}
	}

	// Recreate the exact same tree generation logic as view_browse.go (lines 63-85)
	tr := tree.New()
	nodes := make(map[int]*tree.Tree, len(stories))
	
	// First pass: create all nodes
	for _, st := range stories {
		node := tree.Root(displayStory(st))
		nodes[st.ID] = node
	}

	// Second pass: build parent-child relationships
	for _, st := range stories {
		node := nodes[st.ID]
		if st.FolderID != nil {
			if parent, ok := nodes[*st.FolderID]; ok {
				parent.Child(node)
				continue
			}
		}
		tr.Child(node)
	}

	// Render tree lines exactly as in view_browse.go
	lines := strings.Split(tr.String(), "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}

	return lines
}


// countWrappedLines counts how many display lines a piece of styled content takes
func (m *Model) countWrappedLines(styledContent string) int {
	// For the tree view, each story takes exactly one line
	// The lipgloss styling doesn't introduce additional line breaks in our case
	// Just count newlines in the content
	if styledContent == "" {
		return 1
	}
	lines := strings.Count(styledContent, "\n") + 1
	return lines
}