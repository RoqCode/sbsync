package sync

import (
	"context"
	"errors"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"storyblok-sync/internal/sb"
)

func TestNewSyncOperations(t *testing.T) {
	api := &mockFolderAPI{}
	sourceSpace := &sb.Space{ID: 1, Name: "Source"}
	targetSpace := &sb.Space{ID: 2, Name: "Target"}

	ops := NewSyncOperations(api, sourceSpace, targetSpace)

	if ops == nil {
		t.Fatal("Expected sync operations to be created")
	}
	if ops.sourceSpace.ID != 1 {
		t.Errorf("Expected source space ID 1, got %d", ops.sourceSpace.ID)
	}
	if ops.targetSpace.ID != 2 {
		t.Errorf("Expected target space ID 2, got %d", ops.targetSpace.ID)
	}
	if ops.contentMgr == nil {
		t.Error("Expected content manager to be initialized")
	}
}

func TestRunSyncItem_Success(t *testing.T) {
	api := &mockFolderAPI{}
	sourceSpace := &sb.Space{ID: 1}
	targetSpace := &sb.Space{ID: 2}

	ops := NewSyncOperations(api, sourceSpace, targetSpace)

	ctx := context.Background()
	cmd := ops.RunSyncItem(ctx, 0, "test item")

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
	if result.Duration < 0 {
		t.Error("Expected non-negative duration")
	}
	if result.Cancelled {
		t.Error("Expected not cancelled")
	}
}

func TestRunSyncItem_Cancelled(t *testing.T) {
	api := &mockFolderAPI{}
	sourceSpace := &sb.Space{ID: 1}
	targetSpace := &sb.Space{ID: 2}

	ops := NewSyncOperations(api, sourceSpace, targetSpace)

	// Create a cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	cmd := ops.RunSyncItem(ctx, 5, "test item")

	// Execute the command
	msg := cmd()

	result, ok := msg.(SyncResultMsg)
	if !ok {
		t.Fatalf("Expected SyncResultMsg, got %T", msg)
	}

	if result.Index != 5 {
		t.Errorf("Expected index 5, got %d", result.Index)
	}
	if !result.Cancelled {
		t.Error("Expected cancelled to be true")
	}
}

func TestSyncWithRetry_Success(t *testing.T) {
	api := &mockFolderAPI{}
	sourceSpace := &sb.Space{ID: 1}
	targetSpace := &sb.Space{ID: 2}

	ops := NewSyncOperations(api, sourceSpace, targetSpace)

	callCount := 0
	operation := func() error {
		callCount++
		return nil // Success on first try
	}

	err := ops.SyncWithRetry(operation)

	if err != nil {
		t.Errorf("Expected success, got error: %v", err)
	}
	if callCount != 1 {
		t.Errorf("Expected 1 call, got %d", callCount)
	}
}

func TestSyncWithRetry_EventualSuccess(t *testing.T) {
	api := &mockFolderAPI{}
	sourceSpace := &sb.Space{ID: 1}
	targetSpace := &sb.Space{ID: 2}

	ops := NewSyncOperations(api, sourceSpace, targetSpace)

	callCount := 0
	operation := func() error {
		callCount++
		if callCount < 3 {
			return errors.New("temporary error")
		}
		return nil // Success on third try
	}

	err := ops.SyncWithRetry(operation)

	if err != nil {
		t.Errorf("Expected success, got error: %v", err)
	}
	if callCount != 3 {
		t.Errorf("Expected 3 calls, got %d", callCount)
	}
}

func TestSyncWithRetry_AllAttemptsFail(t *testing.T) {
	api := &mockFolderAPI{}
	sourceSpace := &sb.Space{ID: 1}
	targetSpace := &sb.Space{ID: 2}

	ops := NewSyncOperations(api, sourceSpace, targetSpace)

	callCount := 0
	operation := func() error {
		callCount++
		return errors.New("persistent error")
	}

	err := ops.SyncWithRetry(operation)

	if err == nil {
		t.Error("Expected error after all retries")
	}
	if err.Error() != "persistent error" {
		t.Errorf("Expected last error to be returned, got: %v", err)
	}
	if callCount != 3 {
		t.Errorf("Expected 3 calls, got %d", callCount)
	}
}

func TestSyncWithRetry_RateLimited(t *testing.T) {
	api := &mockFolderAPI{}
	sourceSpace := &sb.Space{ID: 1}
	targetSpace := &sb.Space{ID: 2}

	ops := NewSyncOperations(api, sourceSpace, targetSpace)

	callCount := 0
	operation := func() error {
		callCount++
		if callCount < 2 {
			return errors.New("rate limited") // This should match IsRateLimited
		}
		return nil
	}

	startTime := time.Now()
	err := ops.SyncWithRetry(operation)
	duration := time.Since(startTime)

	if err != nil {
		t.Errorf("Expected success, got error: %v", err)
	}
	if callCount != 2 {
		t.Errorf("Expected 2 calls, got %d", callCount)
	}
	// Should have some delay due to rate limiting
	if duration < 500*time.Millisecond {
		t.Errorf("Expected some delay due to rate limiting, got %v", duration)
	}
}

func TestCalculateRetryDelay(t *testing.T) {
	api := &mockFolderAPI{}
	sourceSpace := &sb.Space{ID: 1}
	targetSpace := &sb.Space{ID: 2}

	ops := NewSyncOperations(api, sourceSpace, targetSpace)

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
			name:     "rate limited error attempt 1",
			err:      errors.New("rate limited"),
			attempt:  1,
			expected: 2 * time.Second,
		},
		{
			name:     "general error",
			err:      errors.New("general error"),
			attempt:  0,
			expected: 500 * time.Millisecond,
		},
		{
			name:     "general error attempt 2",
			err:      errors.New("general error"),
			attempt:  2,
			expected: 500 * time.Millisecond,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := ops.calculateRetryDelay(test.err, test.attempt)
			if result != test.expected {
				t.Errorf("Expected delay %v, got %v", test.expected, result)
			}
		})
	}
}

func TestShouldPublish(t *testing.T) {
	api := &mockFolderAPI{}
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
		{
			name:        "plan level 0",
			targetSpace: &sb.Space{ID: 2, PlanLevel: 0},
			expected:    true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ops := NewSyncOperations(api, sourceSpace, test.targetSpace)
			result := ops.ShouldPublish()
			if result != test.expected {
				t.Errorf("Expected %t, got %t", test.expected, result)
			}
		})
	}
}

// Test concurrent operations don't interfere
func TestRunSyncItem_Concurrent(t *testing.T) {
	api := &mockFolderAPI{}
	sourceSpace := &sb.Space{ID: 1}
	targetSpace := &sb.Space{ID: 2}

	ops := NewSyncOperations(api, sourceSpace, targetSpace)

	ctx := context.Background()

	// Start multiple operations concurrently
	results := make([]tea.Msg, 3)
	done := make(chan int, 3)

	for i := 0; i < 3; i++ {
		go func(index int) {
			cmd := ops.RunSyncItem(ctx, index, "test item")
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

// Test that long operations can be cancelled
func TestRunSyncItem_LongOperation(t *testing.T) {
	api := &mockFolderAPI{}
	sourceSpace := &sb.Space{ID: 1}
	targetSpace := &sb.Space{ID: 2}

	ops := NewSyncOperations(api, sourceSpace, targetSpace)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	// This should complete quickly since the operation is lightweight
	cmd := ops.RunSyncItem(ctx, 0, "test item")
	msg := cmd()

	result, ok := msg.(SyncResultMsg)
	if !ok {
		t.Fatalf("Expected SyncResultMsg, got %T", msg)
	}

	// Should not be cancelled since operation is quick
	if result.Cancelled {
		t.Error("Expected operation to complete, not be cancelled")
	}
}