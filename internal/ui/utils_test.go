package ui

import (
	"reflect"
	"testing"

	"storyblok-sync/internal/sb"
)

func intPtr(i int) *int { return &i }

func TestContainsSpaceID(t *testing.T) {
	spaces := []sb.Space{{ID: 1}, {ID: 2}}
	if sp, ok := containsSpaceID(spaces, "2"); !ok || sp.ID != 2 {
		t.Fatalf("expected to find space 2, got %v %v", sp, ok)
	}
	if _, ok := containsSpaceID(spaces, "3"); ok {
		t.Fatalf("unexpected space found")
	}
}

func TestRebuildStoryIndexAndIncludeAncestors(t *testing.T) {
	root := sb.Story{ID: 1, FullSlug: "root", IsFolder: true}
	child := sb.Story{ID: 2, FullSlug: "root/child", IsFolder: true, FolderID: intPtr(1)}
	leaf := sb.Story{ID: 3, FullSlug: "root/child/leaf", FolderID: intPtr(2)}
	m := &Model{storiesSource: []sb.Story{root, child, leaf}}
	m.rebuildStoryIndex()
	if got := m.indexBySlug("root/child"); got != 1 {
		t.Fatalf("indexBySlug want 1 got %d", got)
	}
	if m.storyIdx[1] != 0 || m.storyIdx[2] != 1 || m.storyIdx[3] != 2 {
		t.Fatalf("storyIdx not built correctly: %v", m.storyIdx)
	}
	if !m.folderCollapsed[1] || !m.folderCollapsed[2] {
		t.Fatalf("folders not collapsed: %v", m.folderCollapsed)
	}
	if _, ok := m.folderCollapsed[3]; ok {
		t.Fatalf("leaf should not be in folderCollapsed")
	}
	inc := make(map[int]bool)
	m.includeAncestors(2, inc)
	want := map[int]bool{2: true, 1: true, 0: true}
	if !reflect.DeepEqual(inc, want) {
		t.Fatalf("includeAncestors want %v got %v", want, inc)
	}
}

func TestSelectedDescendantsAndChildren(t *testing.T) {
	m := Model{}
	m.selection.selected = map[string]bool{"root/child/leaf": true}
	if !m.hasSelectedDescendant("root") {
		t.Fatalf("expected descendant for root")
	}
	if m.hasSelectedDirectChild("root") {
		t.Fatalf("no direct child should be selected for root")
	}
	m.selection.selected["root/child"] = true
	if !m.hasSelectedDirectChild("root") {
		t.Fatalf("expected direct child for root")
	}
	if !m.hasSelectedDirectChild("root/child") {
		t.Fatalf("expected direct child for root/child")
	}
}
