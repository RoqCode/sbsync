package sync

import (
	"context"
	"encoding/json"
	"testing"

	"storyblok-sync/internal/sb"
)

func TestNewBulkSyncer(t *testing.T) {
	api := &mockStorySyncAPI{}
	sourceStories := []sb.Story{
		{ID: 1, FullSlug: "test", IsFolder: false},
		{ID: 2, FullSlug: "folder", IsFolder: true},
	}

	syncer := NewBulkSyncer(api, sourceStories, 1, 2, true)

	if syncer == nil {
		t.Fatal("Expected bulk syncer to be created")
	}
	if syncer.sourceSpaceID != 1 {
		t.Errorf("Expected source space ID 1, got %d", syncer.sourceSpaceID)
	}
	if syncer.targetSpaceID != 2 {
		t.Errorf("Expected target space ID 2, got %d", syncer.targetSpaceID)
	}
	if len(syncer.sourceStories) != 2 {
		t.Errorf("Expected 2 source stories, got %d", len(syncer.sourceStories))
	}
}

func TestGetStoriesWithPrefix(t *testing.T) {
	sourceStories := []sb.Story{
		{ID: 1, FullSlug: "app", IsFolder: true},
		{ID: 2, FullSlug: "app/page1", IsFolder: false},
		{ID: 3, FullSlug: "app/sub/page2", IsFolder: false},
		{ID: 4, FullSlug: "other", IsFolder: false},
		{ID: 5, FullSlug: "app-other", IsFolder: false}, // Should not match
	}

	syncer := NewBulkSyncer(&mockStorySyncAPI{}, sourceStories, 1, 2, true)

	tests := []struct {
		prefix   string
		expected []string
	}{
		{
			prefix:   "app",
			expected: []string{"app", "app/page1", "app/sub/page2"},
		},
		{
			prefix:   "app/sub",
			expected: []string{"app/sub/page2"},
		},
		{
			prefix:   "other",
			expected: []string{"other"},
		},
		{
			prefix:   "nonexistent",
			expected: []string{},
		},
	}

	for _, test := range tests {
		t.Run("prefix_"+test.prefix, func(t *testing.T) {
			result := syncer.getStoriesWithPrefix(test.prefix)

			if len(result) != len(test.expected) {
				t.Errorf("Expected %d stories, got %d", len(test.expected), len(result))
			}

			for i, expected := range test.expected {
				if i >= len(result) {
					t.Errorf("Missing expected story: %s", expected)
					continue
				}
				if result[i].FullSlug != expected {
					t.Errorf("Expected story %s at position %d, got %s", expected, i, result[i].FullSlug)
				}
			}
		})
	}
}

func TestSortByTypeAndDepth(t *testing.T) {
	stories := []sb.Story{
		{ID: 1, FullSlug: "app/sub/page", IsFolder: false},
		{ID: 2, FullSlug: "app", IsFolder: true},
		{ID: 3, FullSlug: "app/sub", IsFolder: true},
		{ID: 4, FullSlug: "app/page", IsFolder: false},
		{ID: 5, FullSlug: "zzz", IsFolder: true},
	}

	syncer := NewBulkSyncer(&mockStorySyncAPI{}, stories, 1, 2, true)
	syncer.sortByTypeAndDepth(stories)

	expected := []string{
		"app",          // folder, depth 0
		"zzz",          // folder, depth 0 (alphabetical)
		"app/sub",      // folder, depth 1
		"app/page",     // story, depth 1
		"app/sub/page", // story, depth 2
	}

	if len(stories) != len(expected) {
		t.Fatalf("Expected %d stories, got %d", len(expected), len(stories))
	}

	for i, expectedSlug := range expected {
		if stories[i].FullSlug != expectedSlug {
			t.Errorf("Position %d: expected %s, got %s", i, expectedSlug, stories[i].FullSlug)
		}
	}
}

func TestSyncStartsWith(t *testing.T) {
	sourceStories := []sb.Story{
		{
			ID:       1,
			FullSlug: "app",
			IsFolder: true,
			Content:  json.RawMessage([]byte(`{"content_types":["page"]}`)),
		},
		{
			ID:       2,
			FullSlug: "app/page1",
			IsFolder: false,
			Content:  json.RawMessage([]byte(`{"component":"page"}`)),
		},
	}

	api := &mockStorySyncAPI{
		stories: make(map[string][]sb.Story),
		storyContent: map[int]sb.Story{
			1: sourceStories[0],
			2: sourceStories[1],
		},
	}

	syncer := NewBulkSyncer(api, sourceStories, 1, 2, true)

	err := syncer.SyncStartsWith("app")

	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}

	// Verify that both items were processed
	totalCalls := len(api.createCalls) + len(api.updateCalls)
	if totalCalls != 2 {
		t.Errorf("Expected 2 sync operations, got %d", totalCalls)
	}

	// Verify folder was processed first
	if len(api.createCalls) > 0 && !api.createCalls[0].IsFolder {
		t.Error("Expected folder to be synced first")
	}
}

func TestSyncStartsWithDetailed(t *testing.T) {
	sourceStories := []sb.Story{
		{
			ID:       1,
			FullSlug: "app",
			IsFolder: true,
			Content:  json.RawMessage([]byte(`{"content_types":["page"]}`)),
		},
		{
			ID:       2,
			FullSlug: "app/page1",
			IsFolder: false,
			Content:  json.RawMessage([]byte(`{"component":"page"}`)),
		},
	}

	api := &mockStorySyncAPI{
		stories: make(map[string][]sb.Story),
		storyContent: map[int]sb.Story{
			1: sourceStories[0],
			2: sourceStories[1],
		},
	}

	syncer := NewBulkSyncer(api, sourceStories, 1, 2, true)

	result, err := syncer.SyncStartsWithDetailed("app")

	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}

	if result == nil {
		t.Fatal("Expected result to be returned")
	}

	// Should report bulk operation
	if result.Operation != "bulk (2 created, 0 updated)" {
		t.Errorf("Expected bulk operation description, got: %s", result.Operation)
	}

	if result.Warning != "" {
		t.Errorf("Expected no warnings, got: %s", result.Warning)
	}
}

func TestSyncStartsWithDetailed_WithWarnings(t *testing.T) {
	sourceStories := []sb.Story{
		{
			ID:       1,
			FullSlug: "app",
			IsFolder: true,
			Content:  json.RawMessage([]byte(`{"content_types":["page"]}`)),
		},
	}

	// This test is more complex since we need to mock the StorySyncer
	// For now, we'll test the basic functionality without warnings
	api := &mockStorySyncAPI{
		stories: make(map[string][]sb.Story),
		storyContent: map[int]sb.Story{
			1: sourceStories[0],
		},
	}

	syncer := NewBulkSyncer(api, sourceStories, 1, 2, true)

	result, err := syncer.SyncStartsWithDetailed("app")

	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}

	// Since no warnings are generated in the simple case, no warning aggregation
	if result.Warning != "" && result.Warning != "app: test warning" {
		t.Errorf("Unexpected warning format, got: %s", result.Warning)
	}
}

func TestFindTarget(t *testing.T) {
	targetStories := []sb.Story{
		{ID: 1, FullSlug: "app"},
		{ID: 2, FullSlug: "app/page1"},
		{ID: 3, FullSlug: "other"},
	}

	tests := []struct {
		slug     string
		expected int
	}{
		{"app", 0},
		{"app/page1", 1},
		{"other", 2},
		{"nonexistent", -1},
	}

	for _, test := range tests {
		result := FindTarget(targetStories, test.slug)
		if result != test.expected {
			t.Errorf("FindTarget(%s) = %d, expected %d", test.slug, result, test.expected)
		}
	}
}

func TestFindSource(t *testing.T) {
	sourceStories := []sb.Story{
		{ID: 1, FullSlug: "app"},
		{ID: 2, FullSlug: "app/page1"},
		{ID: 3, FullSlug: "other"},
	}

	tests := []struct {
		slug     string
		expected bool
		id       int
	}{
		{"app", true, 1},
		{"app/page1", true, 2},
		{"other", true, 3},
		{"nonexistent", false, 0},
	}

	for _, test := range tests {
		story, found := FindSource(sourceStories, test.slug)
		if found != test.expected {
			t.Errorf("FindSource(%s) found = %t, expected %t", test.slug, found, test.expected)
		}
		if found && story.ID != test.id {
			t.Errorf("FindSource(%s) ID = %d, expected %d", test.slug, story.ID, test.id)
		}
	}
}

func TestNextTargetID(t *testing.T) {
	tests := []struct {
		name     string
		stories  []sb.Story
		expected int
	}{
		{
			name:     "empty list",
			stories:  []sb.Story{},
			expected: 1,
		},
		{
			name: "single story",
			stories: []sb.Story{
				{ID: 5},
			},
			expected: 6,
		},
		{
			name: "multiple stories",
			stories: []sb.Story{
				{ID: 1},
				{ID: 10},
				{ID: 3},
			},
			expected: 11,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := NextTargetID(test.stories)
			if result != test.expected {
				t.Errorf("Expected %d, got %d", test.expected, result)
			}
		})
	}
}

// Extended mock that can return warnings
type mockStorySyncAPIWithWarnings struct {
	mockStorySyncAPI
	warningMessage string
}

func (m *mockStorySyncAPIWithWarnings) CreateStoryWithPublish(ctx context.Context, spaceID int, st sb.Story, publish bool) (sb.Story, error) {
	// Call parent method
	result, err := m.mockStorySyncAPI.CreateStoryWithPublish(ctx, spaceID, st, publish)
	return result, err
}

// We need to test the StorySyncer methods that return detailed results with warnings
// This requires implementing a mock StorySyncer or using the existing one
type mockStorySyncerWithWarnings struct {
	api     *mockStorySyncAPIWithWarnings
	warning string
}

func (m *mockStorySyncerWithWarnings) SyncFolderDetailed(story sb.Story, publish bool) (*SyncItemResult, error) {
	return &SyncItemResult{
		Operation:   OperationCreate,
		TargetStory: &sb.Story{ID: 100, FullSlug: story.FullSlug},
		Warning:     m.warning,
	}, nil
}

func (m *mockStorySyncerWithWarnings) SyncStoryDetailed(story sb.Story, publish bool) (*SyncItemResult, error) {
	return &SyncItemResult{
		Operation:   OperationCreate,
		TargetStory: &sb.Story{ID: 100, FullSlug: story.FullSlug},
		Warning:     m.warning,
	}, nil
}
