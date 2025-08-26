package sync

import (
	"testing"

	"storyblok-sync/internal/sb"
)

func TestNewPreflightPlanner(t *testing.T) {
	sourceStories := []sb.Story{
		{ID: 1, FullSlug: "source1"},
	}
	targetStories := []sb.Story{
		{ID: 2, FullSlug: "target1"},
	}

	planner := NewPreflightPlanner(sourceStories, targetStories)

	if planner == nil {
		t.Fatal("Expected preflight planner to be created")
	}
	if len(planner.sourceStories) != 1 {
		t.Errorf("Expected 1 source story, got %d", len(planner.sourceStories))
	}
	if len(planner.targetStories) != 1 {
		t.Errorf("Expected 1 target story, got %d", len(planner.targetStories))
	}
}

func TestOptimizePreflight_Basic(t *testing.T) {
	sourceStories := []sb.Story{
		{ID: 1, FullSlug: "app", IsFolder: true},
		{ID: 2, FullSlug: "app/page1", IsFolder: false},
	}

	planner := NewPreflightPlanner(sourceStories, []sb.Story{})

	items := []PreflightItem{
		{
			Story:    sourceStories[1], // app/page1 (story)
			Skip:     false,
			Selected: true,
			State:    StateCreate,
		},
		{
			Story:    sourceStories[0], // app (folder)
			Skip:     false,
			Selected: true,
			State:    StateCreate,
		},
	}

	optimized := planner.OptimizePreflight(items)

	if len(optimized) != 2 {
		t.Errorf("Expected 2 optimized items, got %d", len(optimized))
	}

	// Folders should come first
	if !optimized[0].Story.IsFolder {
		t.Error("Expected first item to be a folder")
	}
	if optimized[1].Story.IsFolder {
		t.Error("Expected second item to be a story")
	}

	// All items should have Run status set to pending
	for i, item := range optimized {
		if item.Run != RunPending {
			t.Errorf("Item %d: expected run status %s, got %s", i, RunPending, item.Run)
		}
	}
}

func TestOptimizePreflight_SkipDuplicates(t *testing.T) {
	story := sb.Story{ID: 1, FullSlug: "test", IsFolder: false}
	
	sourceStories := []sb.Story{story}
	planner := NewPreflightPlanner(sourceStories, []sb.Story{})

	items := []PreflightItem{
		{
			Story:    story,
			Skip:     false,
			Selected: true,
			State:    StateCreate,
		},
		{
			Story:    story, // Duplicate
			Skip:     false,
			Selected: true,
			State:    StateUpdate,
		},
	}

	optimized := planner.OptimizePreflight(items)

	if len(optimized) != 1 {
		t.Errorf("Expected 1 optimized item (duplicate removed), got %d", len(optimized))
	}
}

func TestOptimizePreflight_SkipMarkedItems(t *testing.T) {
	sourceStories := []sb.Story{
		{ID: 1, FullSlug: "keep", IsFolder: false},
		{ID: 2, FullSlug: "skip", IsFolder: false},
	}

	planner := NewPreflightPlanner(sourceStories, []sb.Story{})

	items := []PreflightItem{
		{
			Story:    sourceStories[0],
			Skip:     false,
			Selected: true,
			State:    StateCreate,
		},
		{
			Story:    sourceStories[1],
			Skip:     true, // This should be filtered out
			Selected: true,
			State:    StateCreate,
		},
	}

	optimized := planner.OptimizePreflight(items)

	if len(optimized) != 1 {
		t.Errorf("Expected 1 optimized item (skipped item removed), got %d", len(optimized))
	}
	if optimized[0].Story.FullSlug != "keep" {
		t.Errorf("Expected remaining item to be 'keep', got %s", optimized[0].Story.FullSlug)
	}
}

func TestOptimizePreflight_AddMissingFolders(t *testing.T) {
	sourceStories := []sb.Story{
		{ID: 1, FullSlug: "app", IsFolder: true},
		{ID: 2, FullSlug: "app/sub", IsFolder: true},
		{ID: 3, FullSlug: "app/sub/page", IsFolder: false},
	}

	// Target has no folders
	planner := NewPreflightPlanner(sourceStories, []sb.Story{})

	// Only selecting the deepest story
	items := []PreflightItem{
		{
			Story:    sourceStories[2], // app/sub/page
			Skip:     false,
			Selected: true,
			State:    StateCreate,
		},
	}

	optimized := planner.OptimizePreflight(items)

	// Should auto-add missing folders: app, app/sub
	if len(optimized) != 3 {
		t.Errorf("Expected 3 items (1 story + 2 auto-added folders), got %d", len(optimized))
	}

	// Check order: folders first, then stories
	expectedOrder := []string{"app", "app/sub", "app/sub/page"}
	for i, item := range optimized {
		if item.Story.FullSlug != expectedOrder[i] {
			t.Errorf("Position %d: expected %s, got %s", i, expectedOrder[i], item.Story.FullSlug)
		}
	}

	// Auto-added folders should be marked for creation
	for i := 0; i < 2; i++ { // First two are folders
		if optimized[i].State != StateCreate {
			t.Errorf("Auto-added folder %s should have state %s, got %s", 
				optimized[i].Story.FullSlug, StateCreate, optimized[i].State)
		}
		if !optimized[i].Selected {
			t.Errorf("Auto-added folder %s should be selected", optimized[i].Story.FullSlug)
		}
	}
}

func TestOptimizePreflight_DontAddExistingFolders(t *testing.T) {
	sourceStories := []sb.Story{
		{ID: 1, FullSlug: "app", IsFolder: true},
		{ID: 2, FullSlug: "app/sub", IsFolder: true},
		{ID: 3, FullSlug: "app/sub/page", IsFolder: false},
	}

	// Target already has the folders
	targetStories := []sb.Story{
		{ID: 100, FullSlug: "app", IsFolder: true},
		{ID: 101, FullSlug: "app/sub", IsFolder: true},
	}

	planner := NewPreflightPlanner(sourceStories, targetStories)

	items := []PreflightItem{
		{
			Story:    sourceStories[2], // app/sub/page
			Skip:     false,
			Selected: true,
			State:    StateCreate,
		},
	}

	optimized := planner.OptimizePreflight(items)

	// Should not auto-add folders that already exist
	if len(optimized) != 1 {
		t.Errorf("Expected 1 item (no auto-added folders), got %d", len(optimized))
	}
	if optimized[0].Story.FullSlug != "app/sub/page" {
		t.Errorf("Expected story app/sub/page, got %s", optimized[0].Story.FullSlug)
	}
}

func TestFindMissingFolderPaths(t *testing.T) {
	sourceStories := []sb.Story{
		{ID: 1, FullSlug: "app", IsFolder: true},
		{ID: 2, FullSlug: "app/sub", IsFolder: true},
		{ID: 3, FullSlug: "other", IsFolder: true},
	}

	targetStories := []sb.Story{
		{ID: 100, FullSlug: "app", IsFolder: true}, // app exists
		// app/sub and other are missing
	}

	planner := NewPreflightPlanner(sourceStories, targetStories)

	items := []PreflightItem{
		{Story: sb.Story{FullSlug: "app/sub/page"}},        // Needs app/sub
		{Story: sb.Story{FullSlug: "other/page"}},          // Needs other
		{Story: sb.Story{FullSlug: "app/existing/page"}},   // Needs app/existing (not in source)
	}

	missing := planner.FindMissingFolderPaths(items)

	expectedPaths := map[string]bool{
		"app/sub": true,
		"other":   true,
		// app/existing should not be found since it's not in source stories
	}

	if len(missing) != len(expectedPaths) {
		t.Errorf("Expected %d missing folders, got %d", len(expectedPaths), len(missing))
	}

	for _, folder := range missing {
		if !expectedPaths[folder.FullSlug] {
			t.Errorf("Unexpected missing folder: %s", folder.FullSlug)
		}
		if !folder.IsFolder {
			t.Errorf("Missing folder %s should be marked as folder", folder.FullSlug)
		}
	}
}

func TestBuildTargetFolderMap(t *testing.T) {
	targetStories := []sb.Story{
		{ID: 1, FullSlug: "folder1", IsFolder: true},
		{ID: 2, FullSlug: "story1", IsFolder: false}, // Should not be included
		{ID: 3, FullSlug: "folder2", IsFolder: true},
	}

	planner := NewPreflightPlanner([]sb.Story{}, targetStories)

	folderMap := planner.BuildTargetFolderMap()

	if len(folderMap) != 2 {
		t.Errorf("Expected 2 folders in map, got %d", len(folderMap))
	}

	if _, exists := folderMap["folder1"]; !exists {
		t.Error("Expected folder1 in map")
	}
	if _, exists := folderMap["folder2"]; !exists {
		t.Error("Expected folder2 in map")
	}
	if _, exists := folderMap["story1"]; exists {
		t.Error("Expected story1 not in folder map")
	}
}

func TestProcessTranslatedSlugs_NoExisting(t *testing.T) {
	sourceStory := sb.Story{
		ID:       1,
		FullSlug: "test",
		TranslatedSlugs: []sb.TranslatedSlug{
			{Lang: "en", Name: "Test", Path: "test"},
			{Lang: "de", Name: "Test DE", Path: "test-de"},
		},
	}

	result := ProcessTranslatedSlugs(sourceStory, []sb.Story{})

	// TranslatedSlugs should be cleared
	if len(result.TranslatedSlugs) != 0 {
		t.Error("Expected TranslatedSlugs to be cleared")
	}

	// TranslatedSlugsAttributes should be set
	if len(result.TranslatedSlugsAttributes) != 2 {
		t.Errorf("Expected 2 TranslatedSlugsAttributes, got %d", len(result.TranslatedSlugsAttributes))
	}

	// IDs should be nil for new story
	for _, attr := range result.TranslatedSlugsAttributes {
		if attr.ID != nil {
			t.Errorf("Expected nil ID for new translated slug, got %v", *attr.ID)
		}
	}
}

func TestProcessTranslatedSlugs_WithExisting(t *testing.T) {
	sourceStory := sb.Story{
		ID:       1,
		FullSlug: "test",
		TranslatedSlugs: []sb.TranslatedSlug{
			{Lang: "en", Name: "Test", Path: "test"},
			{Lang: "de", Name: "Test DE", Path: "test-de"},
			{Lang: "fr", Name: "Test FR", Path: "test-fr"}, // New language
		},
	}

	existingStory := sb.Story{
		ID:       2,
		FullSlug: "test",
		TranslatedSlugs: []sb.TranslatedSlug{
			{ID: &[]int{100}[0], Lang: "en", Name: "Old Test", Path: "old-test"},
			{ID: &[]int{101}[0], Lang: "de", Name: "Old Test DE", Path: "old-test-de"},
			// No French in existing
		},
	}

	result := ProcessTranslatedSlugs(sourceStory, []sb.Story{existingStory})

	if len(result.TranslatedSlugsAttributes) != 3 {
		t.Errorf("Expected 3 TranslatedSlugsAttributes, got %d", len(result.TranslatedSlugsAttributes))
	}

	// Check that existing IDs are preserved
	idMap := make(map[string]*int)
	for _, attr := range result.TranslatedSlugsAttributes {
		idMap[attr.Lang] = attr.ID
	}

	if idMap["en"] == nil || *idMap["en"] != 100 {
		t.Error("Expected English ID to be preserved as 100")
	}
	if idMap["de"] == nil || *idMap["de"] != 101 {
		t.Error("Expected German ID to be preserved as 101")
	}
	if idMap["fr"] != nil {
		t.Error("Expected French ID to be nil (new language)")
	}
}

func TestProcessTranslatedSlugs_EmptyTranslatedSlugs(t *testing.T) {
	sourceStory := sb.Story{
		ID:              1,
		FullSlug:        "test",
		TranslatedSlugs: []sb.TranslatedSlug{}, // Empty
	}

	result := ProcessTranslatedSlugs(sourceStory, []sb.Story{})

	// Should return unchanged story
	if len(result.TranslatedSlugs) != 0 {
		t.Error("Expected TranslatedSlugs to remain empty")
	}
	if len(result.TranslatedSlugsAttributes) != 0 {
		t.Error("Expected TranslatedSlugsAttributes to remain empty")
	}
}

func TestParentSlug(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"app/sub/page", "app/sub"},
		{"app/page", "app"},
		{"page", ""},
		{"", ""},
		{"app/sub/deep/page", "app/sub/deep"},
	}

	for _, test := range tests {
		result := ParentSlug(test.input)
		if result != test.expected {
			t.Errorf("ParentSlug(%q) = %q, expected %q", test.input, result, test.expected)
		}
	}
}

func TestItemType(t *testing.T) {
	tests := []struct {
		story    sb.Story
		expected string
	}{
		{
			story:    sb.Story{IsFolder: true},
			expected: "folder",
		},
		{
			story:    sb.Story{IsFolder: false},
			expected: "story",
		},
	}

	for _, test := range tests {
		result := ItemType(test.story)
		if result != test.expected {
			t.Errorf("ItemType(IsFolder: %t) = %q, expected %q", test.story.IsFolder, result, test.expected)
		}
	}
}

func TestOptimizePreflight_ComplexSorting(t *testing.T) {
	sourceStories := []sb.Story{
		{ID: 1, FullSlug: "z-last", IsFolder: false},
		{ID: 2, FullSlug: "app", IsFolder: true},
		{ID: 3, FullSlug: "app/sub/deep", IsFolder: true},
		{ID: 4, FullSlug: "app/sub", IsFolder: true},
		{ID: 5, FullSlug: "app/page", IsFolder: false},
		{ID: 6, FullSlug: "app/sub/page", IsFolder: false},
		{ID: 7, FullSlug: "a-first", IsFolder: true},
	}

	planner := NewPreflightPlanner(sourceStories, []sb.Story{})

	items := make([]PreflightItem, len(sourceStories))
	for i, story := range sourceStories {
		items[i] = PreflightItem{
			Story:    story,
			Skip:     false,
			Selected: true,
			State:    StateCreate,
		}
	}

	optimized := planner.OptimizePreflight(items)

	expectedOrder := []string{
		"a-first",      // folder, depth 0
		"app",          // folder, depth 0
		"app/sub",      // folder, depth 1
		"app/sub/deep", // folder, depth 2
		"z-last",       // story, depth 0 (stories are sorted by depth first, then name)
		"app/page",     // story, depth 1
		"app/sub/page", // story, depth 2
	}

	if len(optimized) != len(expectedOrder) {
		t.Errorf("Expected %d items, got %d", len(expectedOrder), len(optimized))
	}

	for i, expectedSlug := range expectedOrder {
		if i >= len(optimized) {
			t.Errorf("Missing item at position %d: expected %s", i, expectedSlug)
			continue
		}
		if optimized[i].Story.FullSlug != expectedSlug {
			t.Errorf("Position %d: expected %s, got %s", i, expectedSlug, optimized[i].Story.FullSlug)
		}
	}
}

func TestOptimizePreflight_AutoAddFolderAlreadyInList(t *testing.T) {
	sourceStories := []sb.Story{
		{ID: 1, FullSlug: "app", IsFolder: true},
		{ID: 2, FullSlug: "app/page", IsFolder: false},
	}

	planner := NewPreflightPlanner(sourceStories, []sb.Story{})

	items := []PreflightItem{
		{
			Story:    sourceStories[0], // app folder - already in list
			Skip:     false,
			Selected: true,
			State:    StateCreate,
		},
		{
			Story:    sourceStories[1], // app/page - would need app folder
			Skip:     false,
			Selected: true,
			State:    StateCreate,
		},
	}

	optimized := planner.OptimizePreflight(items)

	// Should not duplicate the app folder
	if len(optimized) != 2 {
		t.Errorf("Expected 2 items (no duplication), got %d", len(optimized))
	}

	folderCount := 0
	for _, item := range optimized {
		if item.Story.IsFolder {
			folderCount++
		}
	}

	if folderCount != 1 {
		t.Errorf("Expected 1 folder, got %d", folderCount)
	}
}