package sync

import (
	"testing"

	"storyblok-sync/internal/sb"
)

func TestPrepareStoryForCreation(t *testing.T) {
	story := sb.Story{
		ID:        123,
		Name:      "Test Story",
		Slug:      "test-story",
		FullSlug:  "test-story",
		Content:   map[string]interface{}{"component": "page"},
		UUID:      "test-uuid",
		CreatedAt: "2023-01-01T00:00:00Z",
		UpdatedAt: "2023-01-02T00:00:00Z",
		Position:  10,
		FolderID:  &[]int{456}[0],
	}

	result := PrepareStoryForCreation(story)

	// ID should be cleared for creation
	if result.ID != 0 {
		t.Errorf("Expected ID to be cleared (0), got %d", result.ID)
	}

	// Timestamps should be cleared for creation
	if result.CreatedAt != "" {
		t.Errorf("Expected CreatedAt to be cleared, got %s", result.CreatedAt)
	}

	if result.UpdatedAt != "" {
		t.Errorf("Expected UpdatedAt to be cleared, got %s", result.UpdatedAt)
	}

	// Content and other important fields should be preserved
	if result.Name != story.Name {
		t.Errorf("Expected Name to be preserved, got %s", result.Name)
	}

	if result.Slug != story.Slug {
		t.Errorf("Expected Slug to be preserved, got %s", result.Slug)
	}

	if result.FullSlug != story.FullSlug {
		t.Errorf("Expected FullSlug to be preserved, got %s", result.FullSlug)
	}

	if result.UUID != story.UUID {
		t.Errorf("Expected UUID to be preserved, got %s", result.UUID)
	}

	if result.Content == nil {
		t.Error("Expected Content to be preserved")
	}

	if result.Position != story.Position {
		t.Errorf("Expected Position to be preserved, got %d", result.Position)
	}

	if result.FolderID == nil || *result.FolderID != *story.FolderID {
		t.Error("Expected FolderID to be preserved")
	}
}

func TestPrepareStoryForUpdate(t *testing.T) {
	source := sb.Story{
		ID:        999, // This should be ignored
		Name:      "Updated Name",
		Slug:      "updated-slug",
		FullSlug:  "updated-slug",
		Content:   map[string]interface{}{"component": "updated"},
		UUID:      "source-uuid",
		CreatedAt: "2023-01-01T00:00:00Z", // Should be ignored
		UpdatedAt: "2023-01-02T00:00:00Z", // Should be ignored
		Position:  20,
		FolderID:  &[]int{789}[0],
	}

	target := sb.Story{
		ID:        123, // This should be preserved
		Name:      "Old Name",
		Slug:      "old-slug",
		FullSlug:  "old-slug",
		Content:   map[string]interface{}{"component": "old"},
		UUID:      "target-uuid",
		CreatedAt: "2022-01-01T00:00:00Z", // Should be preserved
		UpdatedAt: "2022-01-02T00:00:00Z", // Should be cleared
		Position:  10,
		FolderID:  &[]int{456}[0],
	}

	result := PrepareStoryForUpdate(source, target)

	// Target's ID and CreatedAt should be preserved
	if result.ID != target.ID {
		t.Errorf("Expected target ID to be preserved (%d), got %d", target.ID, result.ID)
	}

	if result.CreatedAt != target.CreatedAt {
		t.Errorf("Expected target CreatedAt to be preserved (%s), got %s", target.CreatedAt, result.CreatedAt)
	}

	// UpdatedAt should be cleared
	if result.UpdatedAt != "" {
		t.Errorf("Expected UpdatedAt to be cleared, got %s", result.UpdatedAt)
	}

	// Source values should be used for content fields
	if result.Name != source.Name {
		t.Errorf("Expected source Name (%s), got %s", source.Name, result.Name)
	}

	if result.Slug != source.Slug {
		t.Errorf("Expected source Slug (%s), got %s", source.Slug, result.Slug)
	}

	if result.FullSlug != source.FullSlug {
		t.Errorf("Expected source FullSlug (%s), got %s", source.FullSlug, result.FullSlug)
	}

	if result.UUID != source.UUID {
		t.Errorf("Expected source UUID (%s), got %s", source.UUID, result.UUID)
	}

	if result.Content == nil {
		t.Error("Expected Content to be set")
	} else {
		component := result.Content["component"]
		if component != "updated" {
			t.Errorf("Expected source Content, got component: %v", component)
		}
	}

	if result.Position != source.Position {
		t.Errorf("Expected source Position (%d), got %d", source.Position, result.Position)
	}

	if result.FolderID == nil || *result.FolderID != *source.FolderID {
		t.Error("Expected source FolderID to be used")
	}
}

func TestEnsureDefaultContent_NonFolder(t *testing.T) {
	story := sb.Story{
		ID:       1,
		Slug:     "test",
		FullSlug: "test",
		IsFolder: false,
		Content:  nil, // No content initially
	}

	result := EnsureDefaultContent(story)

	if result.Content == nil {
		t.Fatal("Expected Content to be created for non-folder story")
	}

	component := result.Content["component"]
	if component != "page" {
		t.Errorf("Expected default component 'page', got %v", component)
	}
}

func TestEnsureDefaultContent_NonFolderWithExistingContent(t *testing.T) {
	story := sb.Story{
		ID:       1,
		Slug:     "test",
		FullSlug: "test",
		IsFolder: false,
		Content:  map[string]interface{}{"component": "article", "title": "Existing"},
	}

	result := EnsureDefaultContent(story)

	if result.Content == nil {
		t.Fatal("Expected Content to be preserved")
	}

	// Existing content should be preserved
	component := result.Content["component"]
	if component != "article" {
		t.Errorf("Expected existing component 'article', got %v", component)
	}

	title := result.Content["title"]
	if title != "Existing" {
		t.Errorf("Expected existing title 'Existing', got %v", title)
	}
}

func TestEnsureDefaultContent_Folder(t *testing.T) {
	story := sb.Story{
		ID:       1,
		Slug:     "folder",
		FullSlug: "folder",
		IsFolder: true,
		Content:  nil, // No content initially
	}

	result := EnsureDefaultContent(story)

	// Folders should not get default content
	if result.Content != nil {
		t.Errorf("Expected Content to remain nil for folders, got %v", result.Content)
	}
}

func TestGetFolderPaths_SingleLevel(t *testing.T) {
	paths := GetFolderPaths("story")
	if len(paths) != 0 {
		t.Errorf("Expected 0 folder paths for single level, got %d: %v", len(paths), paths)
	}
}

func TestGetFolderPaths_TwoLevels(t *testing.T) {
	paths := GetFolderPaths("folder/story")
	expected := []string{"folder"}
	
	if len(paths) != 1 {
		t.Fatalf("Expected 1 folder path, got %d: %v", len(paths), paths)
	}
	
	if paths[0] != expected[0] {
		t.Errorf("Expected path '%s', got '%s'", expected[0], paths[0])
	}
}

func TestGetFolderPaths_ThreeLevels(t *testing.T) {
	paths := GetFolderPaths("app/section/story")
	expected := []string{"app", "app/section"}
	
	if len(paths) != 2 {
		t.Fatalf("Expected 2 folder paths, got %d: %v", len(paths), paths)
	}
	
	for i, expectedPath := range expected {
		if paths[i] != expectedPath {
			t.Errorf("Expected path[%d] '%s', got '%s'", i, expectedPath, paths[i])
		}
	}
}

func TestGetFolderPaths_DeepNesting(t *testing.T) {
	paths := GetFolderPaths("app/section/subsection/story")
	expected := []string{"app", "app/section", "app/section/subsection"}
	
	if len(paths) != 3 {
		t.Fatalf("Expected 3 folder paths, got %d: %v", len(paths), paths)
	}
	
	for i, expectedPath := range expected {
		if paths[i] != expectedPath {
			t.Errorf("Expected path[%d] '%s', got '%s'", i, expectedPath, paths[i])
		}
	}
}

func TestGetFolderPaths_EmptyString(t *testing.T) {
	paths := GetFolderPaths("")
	if len(paths) != 0 {
		t.Errorf("Expected 0 folder paths for empty string, got %d: %v", len(paths), paths)
	}
}

func TestGetFolderPaths_TrailingSlash(t *testing.T) {
	paths := GetFolderPaths("folder/story/")
	expected := []string{"folder", "folder/story"}
	
	if len(paths) != 2 {
		t.Fatalf("Expected 2 folder paths, got %d: %v", len(paths), paths)
	}
	
	for i, expectedPath := range expected {
		if paths[i] != expectedPath {
			t.Errorf("Expected path[%d] '%s', got '%s'", i, expectedPath, paths[i])
		}
	}
}

func TestParentSlug(t *testing.T) {
	tests := []struct {
		input    string
		expected string
		name     string
	}{
		{"story", "", "single level"},
		{"folder/story", "folder", "two levels"},
		{"app/section/story", "app/section", "three levels"},
		{"app/section/subsection/story", "app/section/subsection", "deep nesting"},
		{"", "", "empty string"},
		{"/story", "", "leading slash"},
		{"folder/", "folder", "trailing slash"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := ParentSlug(test.input)
			if result != test.expected {
				t.Errorf("ParentSlug(%q) = %q, expected %q", test.input, result, test.expected)
			}
		})
	}
}

func TestItemType(t *testing.T) {
	// Test folder
	folder := sb.Story{IsFolder: true}
	if ItemType(folder) != "folder" {
		t.Errorf("Expected 'folder' for folder story, got %s", ItemType(folder))
	}

	// Test story
	story := sb.Story{IsFolder: false}
	if ItemType(story) != "story" {
		t.Errorf("Expected 'story' for non-folder story, got %s", ItemType(story))
	}
}