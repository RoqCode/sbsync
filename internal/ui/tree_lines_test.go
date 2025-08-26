package ui

import (
	"strings"
	"testing"

	"storyblok-sync/internal/sb"
)

func TestGenerateTreeLinesFromStoriesHierarchy(t *testing.T) {
	root := sb.Story{ID: 1, Name: "root", Slug: "root", FullSlug: "root", IsFolder: true}
	rootPtr := root.ID
	a := sb.Story{ID: 2, Name: "a", Slug: "a", FullSlug: "root/a", FolderID: &rootPtr}
	b := sb.Story{ID: 3, Name: "b", Slug: "b", FullSlug: "root/b", FolderID: &rootPtr}

	lines := generateTreeLinesFromStories([]sb.Story{root, a, b})
	if len(lines) < 3 {
		t.Fatalf("expected >=3 lines, got %d", len(lines))
	}
	if !strings.Contains(lines[0], "root") || !strings.Contains(lines[1], "a") || !strings.Contains(lines[2], "b") {
		t.Fatalf("unexpected lines: %v", lines[:3])
	}
}
