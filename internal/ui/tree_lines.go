package ui

import (
	"strings"

	tree "github.com/charmbracelet/lipgloss/tree"
	"storyblok-sync/internal/sb"
)

// generateTreeLinesFromStories builds a tree for a flat, ordered slice of stories
// and returns the rendered lines. The input order defines sibling ordering.
// generateTreeLinesLabeled builds a tree using a caller-provided label function.
func generateTreeLinesLabeled(stories []sb.Story, labelFn func(i int, st sb.Story) string) []string {
	if len(stories) == 0 {
		return []string{}
	}

	tr := tree.New()
	nodes := make(map[int]*tree.Tree, len(stories))

	// First pass: create all nodes
	for i, st := range stories {
		node := tree.Root(labelFn(i, st))
		nodes[st.ID] = node
	}

	// Second pass: attach children to parents, otherwise add as root child
	for _, st := range stories {
		node := nodes[st.ID]
		if st.FolderID != nil {
			pid := *st.FolderID
			// Treat parent_id == 0 as root
			if pid != 0 {
				if parent, ok := nodes[pid]; ok {
					parent.Child(node)
					continue
				}
			}
		}
		tr.Child(node)
	}

	lines := strings.Split(tr.String(), "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

// generateTreeLinesFromStories builds tree lines using the default story display label.
func generateTreeLinesFromStories(stories []sb.Story) []string {
	return generateTreeLinesLabeled(stories, func(_ int, st sb.Story) string { return displayStory(st) })
}
