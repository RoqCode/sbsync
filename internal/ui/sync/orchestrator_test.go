package sync

import (
	"context"
	"errors"
	"testing"
	"time"

	"storyblok-sync/internal/sb"
)

// Mock implementations for testing
type mockSyncAPI struct {
	getStoriesFunc        func(ctx context.Context, spaceID int, slug string) ([]sb.Story, error)
	getStoryContentFunc   func(ctx context.Context, spaceID, storyID int) (sb.Story, error)
	createStoryFunc       func(ctx context.Context, spaceID int, st sb.Story, publish bool) (sb.Story, error)
	updateStoryFunc       func(ctx context.Context, spaceID int, st sb.Story, publish bool) (sb.Story, error)
	updateStoryUUIDFunc   func(ctx context.Context, spaceID, storyID int, uuid string) error
}

func (m *mockSyncAPI) GetStoriesBySlug(ctx context.Context, spaceID int, slug string) ([]sb.Story, error) {
	if m.getStoriesFunc != nil {
		return m.getStoriesFunc(ctx, spaceID, slug)
	}
	return []sb.Story{}, nil
}

func (m *mockSyncAPI) GetStoryWithContent(ctx context.Context, spaceID, storyID int) (sb.Story, error) {
	if m.getStoryContentFunc != nil {
		return m.getStoryContentFunc(ctx, spaceID, storyID)
	}
	return sb.Story{}, nil
}

func (m *mockSyncAPI) CreateStoryWithPublish(ctx context.Context, spaceID int, st sb.Story, publish bool) (sb.Story, error) {
	if m.createStoryFunc != nil {
		return m.createStoryFunc(ctx, spaceID, st, publish)
	}
	st.ID = 123
	return st, nil
}

func (m *mockSyncAPI) UpdateStory(ctx context.Context, spaceID int, st sb.Story, publish bool) (sb.Story, error) {
	if m.updateStoryFunc != nil {
		return m.updateStoryFunc(ctx, spaceID, st, publish)
	}
	return st, nil
}

func (m *mockSyncAPI) UpdateStoryUUID(ctx context.Context, spaceID, storyID int, uuid string) error {
	if m.updateStoryUUIDFunc != nil {
		return m.updateStoryUUIDFunc(ctx, spaceID, storyID, uuid)
	}
	return nil
}

type mockReport struct {
	successCalls []string
	warningCalls []string
	errorCalls   []string
}

func (m *mockReport) AddSuccess(slug, operation string, duration int64, story *sb.Story) {
	m.successCalls = append(m.successCalls, slug+":"+operation)
}

func (m *mockReport) AddWarning(slug, operation, warning string, duration int64, sourceStory, targetStory *sb.Story) {
	m.warningCalls = append(m.warningCalls, slug+":"+operation+":"+warning)
}

func (m *mockReport) AddError(slug, operation string, duration int64, sourceStory *sb.Story, err error) {
	m.errorCalls = append(m.errorCalls, slug+":"+operation+":"+err.Error())
}

type mockSyncItem struct {
	story      sb.Story
	startsWith bool
	isFolder   bool
}

func (m *mockSyncItem) GetStory() sb.Story   { return m.story }
func (m *mockSyncItem) IsStartsWith() bool   { return m.startsWith }
func (m *mockSyncItem) IsFolder() bool       { return m.isFolder }

func TestNewSyncOrchestrator(t *testing.T) {
	api := &mockSyncAPI{}
	report := &mockReport{}
	sourceSpace := &sb.Space{ID: 1}
	targetSpace := &sb.Space{ID: 2}

	orchestrator := NewSyncOrchestrator(api, report, sourceSpace, targetSpace)

	if orchestrator == nil {
		t.Fatal("Expected orchestrator to be created")
	}
	if orchestrator.sourceSpace.ID != 1 {
		t.Errorf("Expected source space ID 1, got %d", orchestrator.sourceSpace.ID)
	}
	if orchestrator.targetSpace.ID != 2 {
		t.Errorf("Expected target space ID 2, got %d", orchestrator.targetSpace.ID)
	}
}

func TestSyncWithRetry_Success(t *testing.T) {
	api := &mockSyncAPI{}
	report := &mockReport{}
	sourceSpace := &sb.Space{ID: 1}
	targetSpace := &sb.Space{ID: 2}

	orchestrator := NewSyncOrchestrator(api, report, sourceSpace, targetSpace)

	attempts := 0
	operation := func() error {
		attempts++
		if attempts < 2 {
			return errors.New("temporary error")
		}
		return nil // succeed on second attempt
	}

	err := orchestrator.SyncWithRetry(operation)
	if err != nil {
		t.Errorf("Expected operation to succeed after retries, got error: %v", err)
	}

	if attempts != 2 {
		t.Errorf("Expected 2 attempts, got %d", attempts)
	}
}

func TestSyncWithRetry_MaxRetriesExceeded(t *testing.T) {
	api := &mockSyncAPI{}
	report := &mockReport{}
	sourceSpace := &sb.Space{ID: 1}
	targetSpace := &sb.Space{ID: 2}

	orchestrator := NewSyncOrchestrator(api, report, sourceSpace, targetSpace)

	attempts := 0
	expectedErr := errors.New("persistent error")
	operation := func() error {
		attempts++
		return expectedErr
	}

	err := orchestrator.SyncWithRetry(operation)
	if err != expectedErr {
		t.Errorf("Expected persistent error to be returned, got: %v", err)
	}

	if attempts != 3 {
		t.Errorf("Expected 3 attempts, got %d", attempts)
	}
}

func TestSyncWithRetry_RateLimited(t *testing.T) {
	api := &mockSyncAPI{}
	report := &mockReport{}
	sourceSpace := &sb.Space{ID: 1}
	targetSpace := &sb.Space{ID: 2}

	orchestrator := NewSyncOrchestrator(api, report, sourceSpace, targetSpace)

	attempts := 0
	rateLimitErr := errors.New("429 Too Many Requests")
	operation := func() error {
		attempts++
		if attempts < 3 {
			return rateLimitErr
		}
		return nil // succeed on third attempt
	}

	start := time.Now()
	err := orchestrator.SyncWithRetry(operation)
	duration := time.Since(start)

	if err != nil {
		t.Errorf("Expected operation to succeed after rate limit retries, got error: %v", err)
	}

	if attempts != 3 {
		t.Errorf("Expected 3 attempts, got %d", attempts)
	}

	// Should have delays for rate limiting (at least 1 second total)
	if duration < time.Millisecond*500 {
		t.Errorf("Expected delays for rate limiting, but operation completed too quickly: %v", duration)
	}
}

func TestCalculateRetryDelay(t *testing.T) {
	api := &mockSyncAPI{}
	report := &mockReport{}
	sourceSpace := &sb.Space{ID: 1}
	targetSpace := &sb.Space{ID: 2}

	orchestrator := NewSyncOrchestrator(api, report, sourceSpace, targetSpace)

	// Test rate limited error
	rateLimitErr := errors.New("429 Too Many Requests")
	delay := orchestrator.calculateRetryDelay(rateLimitErr, 1)
	if delay != 2*time.Second {
		t.Errorf("Expected 2s delay for rate limit on attempt 1, got %v", delay)
	}

	// Test regular error
	regularErr := errors.New("regular error")
	delay = orchestrator.calculateRetryDelay(regularErr, 1)
	if delay != 500*time.Millisecond {
		t.Errorf("Expected 500ms delay for regular error, got %v", delay)
	}
}

func TestShouldPublish(t *testing.T) {
	api := &mockSyncAPI{}
	report := &mockReport{}
	sourceSpace := &sb.Space{ID: 1}

	// Test with nil target space (should default to true)
	orchestrator := NewSyncOrchestrator(api, report, sourceSpace, nil)
	if !orchestrator.ShouldPublish() {
		t.Error("Expected ShouldPublish to return true for nil target space")
	}

	// Test with dev plan level 999 (should return false)
	targetSpace := &sb.Space{ID: 2, PlanLevel: 999}
	orchestrator = NewSyncOrchestrator(api, report, sourceSpace, targetSpace)
	if orchestrator.ShouldPublish() {
		t.Error("Expected ShouldPublish to return false for plan level 999")
	}

	// Test with regular plan level (should return true)
	targetSpace.PlanLevel = 1
	orchestrator = NewSyncOrchestrator(api, report, sourceSpace, targetSpace)
	if !orchestrator.ShouldPublish() {
		t.Error("Expected ShouldPublish to return true for plan level 1")
	}
}

func TestRunSyncItem_ContextCancellation(t *testing.T) {
	api := &mockSyncAPI{}
	report := &mockReport{}
	sourceSpace := &sb.Space{ID: 1}
	targetSpace := &sb.Space{ID: 2}

	orchestrator := NewSyncOrchestrator(api, report, sourceSpace, targetSpace)

	// Create cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	item := &mockSyncItem{
		story: sb.Story{FullSlug: "test-story"},
	}

	cmd := orchestrator.RunSyncItem(ctx, 0, item)
	
	// Execute the command
	msg := cmd()
	
	// Should return a cancelled result
	result, ok := msg.(SyncResultMsg)
	if !ok {
		t.Fatalf("Expected SyncResultMsg, got %T", msg)
	}
	
	if !result.Cancelled {
		t.Error("Expected cancelled result for cancelled context")
	}
	
	if result.Index != 0 {
		t.Errorf("Expected index 0, got %d", result.Index)
	}
}

// TestIsRateLimited is tested in api_adapters_test.go to avoid duplication