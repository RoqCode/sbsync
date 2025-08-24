package ui

import (
	"testing"

	"storyblok-sync/internal/sb"
)

func TestSyncStructureCreatesFolders(t *testing.T) {
	root := sb.Story{ID: 1, Name: "a__portal", Slug: "a__portal", FullSlug: "a__portal", IsFolder: true}
	de := sb.Story{ID: 2, Name: "de", Slug: "de", FullSlug: "a__portal/de", FolderID: &root.ID, IsFolder: true}
	shop := sb.Story{ID: 3, Name: "shop", Slug: "shop", FullSlug: "a__portal/de/shop", FolderID: &de.ID, IsFolder: true}
	detail := sb.Story{ID: 4, Name: "detail", Slug: "detail", FullSlug: "a__portal/de/shop/detail", FolderID: &shop.ID, IsFolder: true}
	item := sb.Story{ID: 5, Name: "item1", Slug: "item1", FullSlug: "a__portal/de/shop/detail/item1", FolderID: &detail.ID}

	m := InitialModel()
	m.storiesSource = []sb.Story{root, de, shop, detail, item}

	if err := m.syncStructure(item); err != nil {
		t.Fatalf("syncStructure returned error: %v", err)
	}

	expected := []string{root.FullSlug, de.FullSlug, shop.FullSlug, detail.FullSlug}
	if len(m.storiesTarget) != len(expected) {
		t.Fatalf("expected %d folders, got %d", len(expected), len(m.storiesTarget))
	}
	for _, slug := range expected {
		if idx := m.findTarget(slug); idx < 0 {
			t.Fatalf("folder %s not created", slug)
		}
	}
}

func TestSyncStoryCreatesAndUpdates(t *testing.T) {
	folder := sb.Story{ID: 1, Name: "app", Slug: "app", FullSlug: "app", IsFolder: true}
	story := sb.Story{ID: 2, Name: "one", Slug: "one", FullSlug: "app/one", FolderID: &folder.ID}

	m := InitialModel()
	m.storiesSource = []sb.Story{folder, story}

	if err := m.syncStructure(story); err != nil {
		t.Fatalf("syncStructure: %v", err)
	}
	if err := m.syncStory(story); err != nil {
		t.Fatalf("syncStory create: %v", err)
	}
	if idx := m.findTarget(story.FullSlug); idx < 0 {
		t.Fatalf("story not created")
	}

	updated := story
	updated.Name = "eins"
	if err := m.syncStory(updated); err != nil {
		t.Fatalf("syncStory update: %v", err)
	}
	idx := m.findTarget(story.FullSlug)
	if got := m.storiesTarget[idx].Name; got != "eins" {
		t.Fatalf("expected updated name 'eins', got %q", got)
	}
	if len(m.storiesTarget) != 2 {
		t.Fatalf("expected 2 items total, got %d", len(m.storiesTarget))
	}
}

func TestSyncStartsWithCopiesSubtree(t *testing.T) {
	parent := sb.Story{ID: 1, Name: "app", Slug: "app", FullSlug: "app", IsFolder: true}
	child1 := sb.Story{ID: 2, Name: "one", Slug: "one", FullSlug: "app/one", FolderID: &parent.ID}
	child2 := sb.Story{ID: 3, Name: "two", Slug: "two", FullSlug: "app/two", FolderID: &parent.ID}

	m := InitialModel()
	m.storiesSource = []sb.Story{parent, child1, child2}

	if err := m.syncStartsWith("app"); err != nil {
		t.Fatalf("syncStartsWith: %v", err)
	}
	if len(m.storiesTarget) != 3 {
		t.Fatalf("expected 3 items, got %d", len(m.storiesTarget))
	}
	for _, slug := range []string{parent.FullSlug, child1.FullSlug, child2.FullSlug} {
		if idx := m.findTarget(slug); idx < 0 {
			t.Fatalf("missing %s", slug)
		}
	}
	folderIdx := m.findTarget(parent.FullSlug)
	folderID := m.storiesTarget[folderIdx].ID
	for _, slug := range []string{child1.FullSlug, child2.FullSlug} {
		idx := m.findTarget(slug)
		if m.storiesTarget[idx].FolderID == nil || *m.storiesTarget[idx].FolderID != folderID {
			t.Fatalf("child %s does not reference folder", slug)
		}
	}
}
