package ui

import (
	"testing"

	"storyblok-sync/internal/sb"
)

func TestVisibleOrderBrowseMatchesItemAt(t *testing.T) {
	folder := sb.Story{ID: 1, Name: "folder", Slug: "folder", FullSlug: "folder", IsFolder: true}
	fptr := folder.ID
	child := sb.Story{ID: 2, Name: "child", Slug: "child", FullSlug: "folder/child", FolderID: &fptr}

	m := InitialModel()
	m.storiesSource = []sb.Story{folder, child}
	m.rebuildStoryIndex()
	m.applyFilter() // builds visibleIdx with folder collapsed -> only folder visible

	stories, order := m.visibleOrderBrowse()
	if len(stories) != 1 || len(order) != 1 {
		t.Fatalf("expected 1 visible story initially, got %d", len(stories))
	}
	if stories[0].ID != folder.ID {
		t.Fatalf("expected folder as visible story")
	}

	// expand folder and check order again
	m.folderCollapsed[folder.ID] = false
	m.refreshVisible()
	stories, order = m.visibleOrderBrowse()
	if len(stories) != 2 {
		t.Fatalf("expected 2 visible stories after expand, got %d", len(stories))
	}
	if stories[1].ID != child.ID {
		t.Fatalf("expected child to be second visible story")
	}
}

func TestVisibleOrderPreflightFallbackOrder(t *testing.T) {
	root := sb.Story{ID: 1, Name: "root", Slug: "root", FullSlug: "root"}
	m := InitialModel()
	m.preflight.items = []PreflightItem{{Story: root}}

	stories, order := m.visibleOrderPreflight()
	if len(stories) != 1 || len(order) != 1 {
		t.Fatalf("expected 1 visible preflight story, got %d", len(stories))
	}
	if stories[0].ID != root.ID || order[0] != 0 {
		t.Fatalf("unexpected visible preflight result: story id %d order %d", stories[0].ID, order[0])
	}
}
