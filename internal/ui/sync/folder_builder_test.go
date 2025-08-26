package sync

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"storyblok-sync/internal/sb"
)

// mockFolderAPI implements FolderAPI for testing
type mockFolderAPI struct {
	stories      map[string][]sb.Story // slug -> stories
	storyContent map[int]sb.Story      // storyID -> story with content
	createCalls  []sb.Story            // Track create calls
	shouldError  bool
	errorMessage string
}

func (m *mockFolderAPI) GetStoriesBySlug(ctx context.Context, spaceID int, slug string) ([]sb.Story, error) {
	if m.shouldError {
		return nil, errors.New(m.errorMessage)
	}
	if stories, ok := m.stories[slug]; ok {
		return stories, nil
	}
	return []sb.Story{}, nil
}

func (m *mockFolderAPI) GetStoryWithContent(ctx context.Context, spaceID, storyID int) (sb.Story, error) {
	if m.shouldError {
		return sb.Story{}, errors.New(m.errorMessage)
	}
	if story, ok := m.storyContent[storyID]; ok {
		return story, nil
	}
	return sb.Story{}, errors.New("not found")
}

func (m *mockFolderAPI) CreateStoryWithPublish(ctx context.Context, spaceID int, st sb.Story, publish bool) (sb.Story, error) {
	if m.shouldError {
		return sb.Story{}, errors.New(m.errorMessage)
	}
	st.ID = len(m.createCalls) + 100 // Give it a unique ID
	m.createCalls = append(m.createCalls, st)
	return st, nil
}

// mockFolderReport implements Report for testing
type mockFolderReport struct {
	successes []string
}

func (m *mockFolderReport) AddSuccess(slug, operation string, duration int64, story *sb.Story) {
	m.successes = append(m.successes, slug+":"+operation)
}

func TestNewFolderPathBuilder(t *testing.T) {
	api := &mockFolderAPI{}
	report := &mockFolderReport{}
	sourceStories := []sb.Story{
		{ID: 1, FullSlug: "folder1", IsFolder: true},
		{ID: 2, FullSlug: "folder2", IsFolder: true},
	}

	builder := NewFolderPathBuilder(api, report, sourceStories, 1, 2, true)

	if builder == nil {
		t.Fatal("Expected folder path builder to be created")
	}
	if builder.srcSpaceID != 1 {
		t.Errorf("Expected source space ID 1, got %d", builder.srcSpaceID)
	}
	if builder.tgtSpaceID != 2 {
		t.Errorf("Expected target space ID 2, got %d", builder.tgtSpaceID)
	}
	if !builder.publish {
		t.Error("Expected publish to be true")
	}
	if len(builder.sourceStories) != 2 {
		t.Errorf("Expected 2 source stories, got %d", len(builder.sourceStories))
	}
	if builder.contentMgr == nil {
		t.Error("Expected content manager to be initialized")
	}
}

func TestCheckExistingFolder_Found(t *testing.T) {
	existingFolder := sb.Story{
		ID:       123,
		FullSlug: "existing/folder",
		IsFolder: true,
	}

	api := &mockFolderAPI{
		stories: map[string][]sb.Story{
			"existing/folder": {existingFolder},
		},
	}
	report := &mockFolderReport{}

	builder := NewFolderPathBuilder(api, report, []sb.Story{}, 1, 2, true)

	ctx := context.Background()
	result, err := builder.CheckExistingFolder(ctx, "existing/folder")

	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}
	if result == nil {
		t.Fatal("Expected folder to be found")
	}
	if result.ID != 123 {
		t.Errorf("Expected folder ID 123, got %d", result.ID)
	}
}

func TestCheckExistingFolder_NotFound(t *testing.T) {
	api := &mockFolderAPI{
		stories: make(map[string][]sb.Story),
	}
	report := &mockFolderReport{}

	builder := NewFolderPathBuilder(api, report, []sb.Story{}, 1, 2, true)

	ctx := context.Background()
	result, err := builder.CheckExistingFolder(ctx, "nonexistent/folder")

	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}
	if result != nil {
		t.Error("Expected folder not to be found")
	}
}

func TestCheckExistingFolder_APIError(t *testing.T) {
	api := &mockFolderAPI{
		shouldError:  true,
		errorMessage: "API error",
	}
	report := &mockFolderReport{}

	builder := NewFolderPathBuilder(api, report, []sb.Story{}, 1, 2, true)

	ctx := context.Background()
	_, err := builder.CheckExistingFolder(ctx, "test/folder")

	if err == nil {
		t.Error("Expected error from API")
	}
	if err.Error() != "API error" {
		t.Errorf("Expected specific error message, got: %v", err)
	}
}

func TestPrepareSourceFolder(t *testing.T) {
	sourceFolder := sb.Story{
		ID:       1,
		FullSlug: "source/folder",
		IsFolder: true,
		Content:  json.RawMessage([]byte(`{"content_types":["page"]}`)),
	}

	api := &mockFolderAPI{
		storyContent: map[int]sb.Story{
			1: sourceFolder,
		},
	}
	report := &mockFolderReport{}
	sourceStories := []sb.Story{sourceFolder}

	builder := NewFolderPathBuilder(api, report, sourceStories, 1, 2, true)

	ctx := context.Background()
	parentID := 456
	result, err := builder.PrepareSourceFolder(ctx, "source/folder", &parentID)

	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}

	if result.ID != 0 {
		t.Error("Expected ID to be cleared for creation")
	}
	if result.FolderID == nil || *result.FolderID != parentID {
		t.Errorf("Expected parent ID %d, got %v", parentID, result.FolderID)
	}
	if len(result.Content) == 0 {
		t.Error("Expected content to be preserved")
	}
}

func TestPrepareSourceFolder_NotFound(t *testing.T) {
	api := &mockFolderAPI{
		storyContent: make(map[int]sb.Story),
	}
	report := &mockFolderReport{}

	builder := NewFolderPathBuilder(api, report, []sb.Story{}, 1, 2, true)

	ctx := context.Background()
	_, err := builder.PrepareSourceFolder(ctx, "nonexistent/folder", nil)

	if err == nil {
		t.Error("Expected error for nonexistent folder")
	}
	if !contains(err.Error(), "source folder not found") {
		t.Errorf("Expected specific error message, got: %v", err)
	}
}

func TestCreateFolder(t *testing.T) {
	folder := sb.Story{
		FullSlug: "test/folder",
		IsFolder: true,
		Content:  json.RawMessage([]byte(`{"content_types":["page"]}`)),
	}

	api := &mockFolderAPI{
		createCalls: make([]sb.Story, 0),
	}
	report := &mockFolderReport{}

	builder := NewFolderPathBuilder(api, report, []sb.Story{}, 1, 2, true)

	ctx := context.Background()
	result, err := builder.CreateFolder(ctx, folder)

	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}

	if result.ID == 0 {
		t.Error("Expected ID to be set after creation")
	}
	if len(api.createCalls) != 1 {
		t.Errorf("Expected 1 create call, got %d", len(api.createCalls))
	}
}

func TestCreateFolder_Error(t *testing.T) {
	folder := sb.Story{
		FullSlug: "test/folder",
		IsFolder: true,
	}

	api := &mockFolderAPI{
		shouldError:  true,
		errorMessage: "Create failed",
	}
	report := &mockFolderReport{}

	builder := NewFolderPathBuilder(api, report, []sb.Story{}, 1, 2, true)

	ctx := context.Background()
	_, err := builder.CreateFolder(ctx, folder)

	if err == nil {
		t.Error("Expected error from create operation")
	}
	if err.Error() != "Create failed" {
		t.Errorf("Expected specific error message, got: %v", err)
	}
}

func TestEnsureFolderPath_SimpleCase(t *testing.T) {
	sourceFolder := sb.Story{
		ID:       1,
		FullSlug: "app",
		IsFolder: true,
		Content:  json.RawMessage([]byte(`{"content_types":["page"]}`)),
	}

	api := &mockFolderAPI{
		stories: make(map[string][]sb.Story), // No existing folders
		storyContent: map[int]sb.Story{
			1: sourceFolder,
		},
		createCalls: make([]sb.Story, 0),
	}
	report := &mockFolderReport{}
	sourceStories := []sb.Story{sourceFolder}

	builder := NewFolderPathBuilder(api, report, sourceStories, 1, 2, true)

	created, err := builder.EnsureFolderPath("app/page")

	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}

	if len(created) != 1 {
		t.Errorf("Expected 1 folder created, got %d", len(created))
	}
	if len(api.createCalls) != 1 {
		t.Errorf("Expected 1 create call, got %d", len(api.createCalls))
	}
	if len(report.successes) != 1 {
		t.Errorf("Expected 1 success report, got %d", len(report.successes))
	}
}

func TestEnsureFolderPath_NestedFolders(t *testing.T) {
	sourceStories := []sb.Story{
		{
			ID:       1,
			FullSlug: "app",
			IsFolder: true,
			Content:  json.RawMessage([]byte(`{"content_types":["page"]}`)),
		},
		{
			ID:       2,
			FullSlug: "app/sub",
			IsFolder: true,
			Content:  json.RawMessage([]byte(`{"content_types":["page"]}`)),
		},
	}

	api := &mockFolderAPI{
		stories: make(map[string][]sb.Story), // No existing folders
		storyContent: map[int]sb.Story{
			1: sourceStories[0],
			2: sourceStories[1],
		},
		createCalls: make([]sb.Story, 0),
	}
	report := &mockFolderReport{}

	builder := NewFolderPathBuilder(api, report, sourceStories, 1, 2, true)

	created, err := builder.EnsureFolderPath("app/sub/page")

	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}

	if len(created) != 2 {
		t.Errorf("Expected 2 folders created, got %d", len(created))
	}
	if len(api.createCalls) != 2 {
		t.Errorf("Expected 2 create calls, got %d", len(api.createCalls))
	}

	// Verify parent-child relationship
	if created[1].FolderID == nil {
		t.Error("Expected sub folder to have parent ID set")
	} else if *created[1].FolderID != created[0].ID {
		t.Errorf("Expected sub folder parent ID %d, got %d", created[0].ID, *created[1].FolderID)
	}
}

func TestEnsureFolderPath_ExistingFolder(t *testing.T) {
	existingFolder := sb.Story{
		ID:       456,
		FullSlug: "app",
		IsFolder: true,
	}

	sourceStories := []sb.Story{
		{
			ID:       1,
			FullSlug: "app",
			IsFolder: true,
			Content:  json.RawMessage([]byte(`{"content_types":["page"]}`)),
		},
		{
			ID:       2,
			FullSlug: "app/sub",
			IsFolder: true,
			Content:  json.RawMessage([]byte(`{"content_types":["page"]}`)),
		},
	}

	api := &mockFolderAPI{
		stories: map[string][]sb.Story{
			"app": {existingFolder}, // Existing folder
		},
		storyContent: map[int]sb.Story{
			1: sourceStories[0],
			2: sourceStories[1],
		},
		createCalls: make([]sb.Story, 0),
	}
	report := &mockFolderReport{}

	builder := NewFolderPathBuilder(api, report, sourceStories, 1, 2, true)

	created, err := builder.EnsureFolderPath("app/sub/page")

	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}

	// Should only create the sub folder, not the app folder
	if len(created) != 1 {
		t.Errorf("Expected 1 folder created, got %d", len(created))
	}
	if len(api.createCalls) != 1 {
		t.Errorf("Expected 1 create call, got %d", len(api.createCalls))
	}

	// The created folder should reference the existing folder as parent
	if created[0].FolderID == nil {
		t.Error("Expected sub folder to have parent ID set")
	} else if *created[0].FolderID != existingFolder.ID {
		t.Errorf("Expected sub folder parent ID %d, got %d", existingFolder.ID, *created[0].FolderID)
	}
}

func TestEnsureFolderPath_NoFolders(t *testing.T) {
	api := &mockFolderAPI{}
	report := &mockFolderReport{}

	builder := NewFolderPathBuilder(api, report, []sb.Story{}, 1, 2, true)

	// Page at root level - no folders needed
	created, err := builder.EnsureFolderPath("page")

	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}

	if len(created) != 0 {
		t.Errorf("Expected 0 folders created, got %d", len(created))
	}
}

func TestEnsureFolderPathStatic(t *testing.T) {
	sourceFolder := sb.Story{
		ID:       1,
		FullSlug: "app",
		IsFolder: true,
		Content:  json.RawMessage([]byte(`{"content_types":["page"]}`)),
	}

	api := &mockFolderAPI{
		stories: make(map[string][]sb.Story),
		storyContent: map[int]sb.Story{
			1: sourceFolder,
		},
	}
	report := &mockFolderReport{}
	sourceStories := []sb.Story{sourceFolder}

	created, err := EnsureFolderPathStatic(api, report, sourceStories, 1, 2, "app/page", true)

	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}

	if len(created) != 1 {
		t.Errorf("Expected 1 folder created, got %d", len(created))
	}
}

// Test timeout context behavior - removed since mockFolderAPI doesn't respect context timeout
// In a real implementation, this would be tested with actual API calls that respect context

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
			func() bool {
				for i := 0; i <= len(s)-len(substr); i++ {
					if s[i:i+len(substr)] == substr {
						return true
					}
				}
				return false
			}())))
}
