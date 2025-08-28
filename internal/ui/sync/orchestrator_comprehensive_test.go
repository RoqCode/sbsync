package sync

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"storyblok-sync/internal/sb"
)

// mockOrchestratorSyncItem implements SyncItem for testing
type mockOrchestratorSyncItem struct {
	story    sb.Story
	isFolder bool
}

func (m *mockOrchestratorSyncItem) GetStory() sb.Story {
	return m.story
}

func (m *mockOrchestratorSyncItem) IsFolder() bool {
	return m.isFolder
}

// mockOrchestratorReport implements ReportInterface for testing
type mockOrchestratorReport struct {
	successes []string
	warnings  []string
	errors    []string
}

func (m *mockOrchestratorReport) AddSuccess(slug, operation string, duration int64, story *sb.Story) {
	m.successes = append(m.successes, slug+":"+operation)
}

func (m *mockOrchestratorReport) AddWarning(slug, operation, warning string, duration int64, sourceStory, targetStory *sb.Story) {
	m.warnings = append(m.warnings, slug+":"+operation+":"+warning)
}

func (m *mockOrchestratorReport) AddError(slug, operation string, duration int64, sourceStory *sb.Story, err error) {
	m.errors = append(m.errors, slug+":"+operation+":"+err.Error())
}

func TestOrchestratorCreation(t *testing.T) {
	api := &mockStorySyncAPI{}
	report := &mockOrchestratorReport{}
	sourceSpace := &sb.Space{ID: 1, Name: "Source"}
	targetSpace := &sb.Space{ID: 2, Name: "Target"}

	orchestrator := NewSyncOrchestrator(api, report, sourceSpace, targetSpace)

	if orchestrator == nil {
		t.Fatal("Expected sync orchestrator to be created")
	}
	if orchestrator.sourceSpace.ID != 1 {
		t.Errorf("Expected source space ID 1, got %d", orchestrator.sourceSpace.ID)
	}
	if orchestrator.targetSpace.ID != 2 {
		t.Errorf("Expected target space ID 2, got %d", orchestrator.targetSpace.ID)
	}
	if orchestrator.contentMgr == nil {
		t.Error("Expected content manager to be initialized")
	}
}

func TestOrchestratorRunSyncItem_Story(t *testing.T) {
	story := sb.Story{
		ID:       1,
		FullSlug: "test/story",
		IsFolder: false,
		Content:  json.RawMessage([]byte(`{"component":"page"}`)),
	}

	api := &mockStorySyncAPI{
		stories: make(map[string][]sb.Story),
		storyContent: map[int]sb.Story{
			1: story,
		},
	}
	report := &mockOrchestratorReport{}
	sourceSpace := &sb.Space{ID: 1}
	targetSpace := &sb.Space{ID: 2}

	orchestrator := NewSyncOrchestrator(api, report, sourceSpace, targetSpace)

	item := &mockOrchestratorSyncItem{story: story, isFolder: false}

	ctx := context.Background()
	cmd := orchestrator.RunSyncItem(ctx, 0, item)

	// Execute the command
	msg := cmd()

	result, ok := msg.(SyncResultMsg)
	if !ok {
		t.Fatalf("Expected SyncResultMsg, got %T", msg)
	}

	if result.Index != 0 {
		t.Errorf("Expected index 0, got %d", result.Index)
	}
	if result.Err != nil {
		t.Errorf("Expected no error, got %v", result.Err)
	}
	if result.Result == nil {
		t.Fatal("Expected result to be set")
	}
	if result.Result.Operation != OperationCreate {
		t.Errorf("Expected operation %s, got %s", OperationCreate, result.Result.Operation)
	}
	if result.Duration < 0 {
		t.Error("Expected non-negative duration")
	}
}

func TestOrchestratorRunSyncItem_Folder(t *testing.T) {
	folder := sb.Story{
		ID:       1,
		FullSlug: "test/folder",
		IsFolder: true,
		Content:  json.RawMessage([]byte(`{"content_types":["page"]}`)),
	}

	api := &mockStorySyncAPI{
		stories: make(map[string][]sb.Story),
		storyContent: map[int]sb.Story{
			1: folder,
		},
	}
	report := &mockOrchestratorReport{}
	sourceSpace := &sb.Space{ID: 1}
	targetSpace := &sb.Space{ID: 2}

	orchestrator := NewSyncOrchestrator(api, report, sourceSpace, targetSpace)

	item := &mockOrchestratorSyncItem{story: folder, isFolder: true}

	ctx := context.Background()
	cmd := orchestrator.RunSyncItem(ctx, 1, item)

	msg := cmd()

	result, ok := msg.(SyncResultMsg)
	if !ok {
		t.Fatalf("Expected SyncResultMsg, got %T", msg)
	}

	if result.Index != 1 {
		t.Errorf("Expected index 1, got %d", result.Index)
	}
	if result.Err != nil {
		t.Errorf("Expected no error, got %v", result.Err)
	}
	if result.Result == nil {
		t.Fatal("Expected result to be set")
	}
	if result.Result.Operation != OperationCreate {
		t.Errorf("Expected operation %s, got %s", OperationCreate, result.Result.Operation)
	}
}

// Removed: no starts-with execution mode; prefix is a filter only.

func TestOrchestratorRunSyncItem_Cancelled(t *testing.T) {
	story := sb.Story{ID: 1, FullSlug: "test"}

	api := &mockStorySyncAPI{}
	report := &mockOrchestratorReport{}
	sourceSpace := &sb.Space{ID: 1}
	targetSpace := &sb.Space{ID: 2}

	orchestrator := NewSyncOrchestrator(api, report, sourceSpace, targetSpace)

	item := &mockOrchestratorSyncItem{story: story, isFolder: false}

	// Create a cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	cmd := orchestrator.RunSyncItem(ctx, 3, item)

	msg := cmd()

	result, ok := msg.(SyncResultMsg)
	if !ok {
		t.Fatalf("Expected SyncResultMsg, got %T", msg)
	}

	if result.Index != 3 {
		t.Errorf("Expected index 3, got %d", result.Index)
	}
	if !result.Cancelled {
		t.Error("Expected cancelled to be true")
	}
}

func TestOrchestratorRunSyncItem_Error(t *testing.T) {
	story := sb.Story{
		ID:       1,
		FullSlug: "test/story",
		IsFolder: false,
	}

	api := &mockStorySyncAPI{
		shouldError:  true,
		errorMessage: "sync failed",
	}
	report := &mockOrchestratorReport{}
	sourceSpace := &sb.Space{ID: 1}
	targetSpace := &sb.Space{ID: 2}

	orchestrator := NewSyncOrchestrator(api, report, sourceSpace, targetSpace)

	item := &mockOrchestratorSyncItem{story: story, isFolder: false}

	ctx := context.Background()
	cmd := orchestrator.RunSyncItem(ctx, 4, item)

	msg := cmd()

	result, ok := msg.(SyncResultMsg)
	if !ok {
		t.Fatalf("Expected SyncResultMsg, got %T", msg)
	}

	if result.Index != 4 {
		t.Errorf("Expected index 4, got %d", result.Index)
	}
	if result.Err == nil {
		t.Error("Expected error")
	}
	if result.Err.Error() != "sync failed" {
		t.Errorf("Expected specific error message, got: %v", result.Err)
	}
}

func TestOrchestratorRunSyncItem_WithWarning(t *testing.T) {
	story := sb.Story{
		ID:       1,
		FullSlug: "test/story",
		IsFolder: false,
		Content:  json.RawMessage([]byte(`{"component":"page"}`)),
	}

	// Create existing story to trigger update with warning
	existingStory := sb.Story{
		ID:       123,
		FullSlug: "test/story",
		UUID:     "different-uuid",
	}

	api := &mockStorySyncAPI{
		stories: map[string][]sb.Story{
			"test/story": {existingStory},
		},
		storyContent: map[int]sb.Story{
			1: story,
		},
	}
	report := &mockOrchestratorReport{}
	sourceSpace := &sb.Space{ID: 1}
	targetSpace := &sb.Space{ID: 2}

	orchestrator := NewSyncOrchestrator(api, report, sourceSpace, targetSpace)

	item := &mockOrchestratorSyncItem{story: story, isFolder: false}

	ctx := context.Background()
	cmd := orchestrator.RunSyncItem(ctx, 5, item)

	msg := cmd()

	result, ok := msg.(SyncResultMsg)
	if !ok {
		t.Fatalf("Expected SyncResultMsg, got %T", msg)
	}

	if result.Err != nil {
		t.Errorf("Expected no error, got %v", result.Err)
	}
	if result.Result == nil {
		t.Fatal("Expected result to be set")
	}
	if result.Result.Operation != OperationUpdate {
		t.Errorf("Expected operation %s, got %s", OperationUpdate, result.Result.Operation)
	}
}

func TestOrchestratorSyncWithRetry_RateLimited(t *testing.T) {
	api := &mockStorySyncAPI{}
	report := &mockOrchestratorReport{}
	sourceSpace := &sb.Space{ID: 1}
	targetSpace := &sb.Space{ID: 2}

	orchestrator := NewSyncOrchestrator(api, report, sourceSpace, targetSpace)

	callCount := 0
	operation := func() error {
		callCount++
		if callCount < 2 {
			return errors.New("rate limited")
		}
		return nil
	}

	startTime := time.Now()
	err := orchestrator.SyncWithRetry(operation)
	duration := time.Since(startTime)

	if err != nil {
		t.Errorf("Expected success after retry, got error: %v", err)
	}
	if callCount != 2 {
		t.Errorf("Expected 2 calls, got %d", callCount)
	}
	// Should have some delay due to rate limiting
	if duration < 500*time.Millisecond {
		t.Errorf("Expected some delay due to rate limiting, got %v", duration)
	}
}

func TestOrchestratorCalculateRetryDelay(t *testing.T) {
	api := &mockStorySyncAPI{}
	report := &mockOrchestratorReport{}
	sourceSpace := &sb.Space{ID: 1}
	targetSpace := &sb.Space{ID: 2}

	orchestrator := NewSyncOrchestrator(api, report, sourceSpace, targetSpace)

	tests := []struct {
		name     string
		err      error
		attempt  int
		expected time.Duration
	}{
		{
			name:     "rate limited error attempt 0",
			err:      errors.New("rate limited"),
			attempt:  0,
			expected: 1 * time.Second,
		},
		{
			name:     "rate limited error attempt 2",
			err:      errors.New("rate limited"),
			attempt:  2,
			expected: 3 * time.Second,
		},
		{
			name:     "general error",
			err:      errors.New("general error"),
			attempt:  1,
			expected: 500 * time.Millisecond,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := orchestrator.calculateRetryDelay(test.err, test.attempt)
			if result != test.expected {
				t.Errorf("Expected delay %v, got %v", test.expected, result)
			}
		})
	}
}

func TestOrchestratorShouldPublish(t *testing.T) {
	api := &mockStorySyncAPI{}
	report := &mockOrchestratorReport{}
	sourceSpace := &sb.Space{ID: 1}

	tests := []struct {
		name        string
		targetSpace *sb.Space
		expected    bool
	}{
		{
			name:        "nil target space",
			targetSpace: nil,
			expected:    true,
		},
		{
			name:        "plan level 999",
			targetSpace: &sb.Space{ID: 2, PlanLevel: 999},
			expected:    false,
		},
		{
			name:        "plan level 1",
			targetSpace: &sb.Space{ID: 2, PlanLevel: 1},
			expected:    true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			orchestrator := NewSyncOrchestrator(api, report, sourceSpace, test.targetSpace)
			result := orchestrator.ShouldPublish()
			if result != test.expected {
				t.Errorf("Expected %t, got %t", test.expected, result)
			}
		})
	}
}

// Removed: no starts-with execution mode.

func TestOrchestratorSyncFolderDetailed(t *testing.T) {
	folder := sb.Story{
		ID:       1,
		FullSlug: "test/folder",
		IsFolder: true,
		Content:  json.RawMessage([]byte(`{"content_types":["page"]}`)),
	}

	api := &mockStorySyncAPI{
		stories: make(map[string][]sb.Story),
		storyContent: map[int]sb.Story{
			1: folder,
		},
	}
	report := &mockOrchestratorReport{}
	sourceSpace := &sb.Space{ID: 1}
	targetSpace := &sb.Space{ID: 2}

	orchestrator := NewSyncOrchestrator(api, report, sourceSpace, targetSpace)

	result, err := orchestrator.SyncFolderDetailed(folder)

	if err != nil {
		t.Errorf("Expected success, got error: %v", err)
	}
	if result == nil {
		t.Fatal("Expected result to be returned")
	}
	if result.Operation != OperationCreate {
		t.Errorf("Expected operation %s, got %s", OperationCreate, result.Operation)
	}
}

func TestOrchestratorSyncStoryDetailed(t *testing.T) {
	story := sb.Story{
		ID:       1,
		FullSlug: "test/story",
		IsFolder: false,
		Content:  json.RawMessage([]byte(`{"component":"page"}`)),
	}

	api := &mockStorySyncAPI{
		stories: make(map[string][]sb.Story),
		storyContent: map[int]sb.Story{
			1: story,
		},
	}
	report := &mockOrchestratorReport{}
	sourceSpace := &sb.Space{ID: 1}
	targetSpace := &sb.Space{ID: 2}

	orchestrator := NewSyncOrchestrator(api, report, sourceSpace, targetSpace)

	result, err := orchestrator.SyncStoryDetailed(story)

	if err != nil {
		t.Errorf("Expected success, got error: %v", err)
	}
	if result == nil {
		t.Fatal("Expected result to be returned")
	}
	if result.Operation != OperationCreate {
		t.Errorf("Expected operation %s, got %s", OperationCreate, result.Operation)
	}
}

// Test concurrent sync operations
func TestOrchestratorRunSyncItem_Concurrent(t *testing.T) {
	stories := []sb.Story{
		{ID: 1, FullSlug: "story1", Content: json.RawMessage([]byte(`{"component":"page"}`))},
		{ID: 2, FullSlug: "story2", Content: json.RawMessage([]byte(`{"component":"page"}`))},
		{ID: 3, FullSlug: "story3", Content: json.RawMessage([]byte(`{"component":"page"}`))},
	}

	api := &mockStorySyncAPI{
		stories: make(map[string][]sb.Story),
		storyContent: map[int]sb.Story{
			1: stories[0],
			2: stories[1],
			3: stories[2],
		},
	}
	report := &mockOrchestratorReport{}
	sourceSpace := &sb.Space{ID: 1}
	targetSpace := &sb.Space{ID: 2}

	orchestrator := NewSyncOrchestrator(api, report, sourceSpace, targetSpace)

	ctx := context.Background()
	results := make([]tea.Msg, 3)
	done := make(chan int, 3)

	// Start multiple operations concurrently
	for i := 0; i < 3; i++ {
		go func(index int) {
			item := &mockOrchestratorSyncItem{story: stories[index], isFolder: false}
			cmd := orchestrator.RunSyncItem(ctx, index, item)
			results[index] = cmd()
			done <- index
		}(i)
	}

	// Wait for all to complete
	for i := 0; i < 3; i++ {
		<-done
	}

	// Verify all completed successfully
	for i, msg := range results {
		result, ok := msg.(SyncResultMsg)
		if !ok {
			t.Fatalf("Expected SyncResultMsg at index %d, got %T", i, msg)
		}
		if result.Index != i {
			t.Errorf("Expected index %d, got %d", i, result.Index)
		}
		if result.Err != nil {
			t.Errorf("Expected no error at index %d, got %v", i, result.Err)
		}
	}
}
