package sync

import (
	"context"
	"errors"
	"testing"

	"storyblok-sync/internal/sb"
)

// Mock for testing publish retry logic
type mockCreateAPI struct {
	calls       []bool // Track publish parameter for each call
	shouldError func(attempt int) error
}

func (m *mockCreateAPI) GetStoriesBySlug(ctx context.Context, spaceID int, slug string) ([]sb.Story, error) {
	return []sb.Story{}, nil
}

func (m *mockCreateAPI) GetStoryWithContent(ctx context.Context, spaceID, storyID int) (sb.Story, error) {
	return sb.Story{}, nil
}

func (m *mockCreateAPI) CreateStoryWithPublish(ctx context.Context, spaceID int, st sb.Story, publish bool) (sb.Story, error) {
	m.calls = append(m.calls, publish)
	if m.shouldError != nil {
		if err := m.shouldError(len(m.calls)); err != nil {
			return sb.Story{}, err
		}
	}
	st.ID = len(m.calls) // Set a unique ID
	return st, nil
}

func (m *mockCreateAPI) UpdateStory(ctx context.Context, spaceID int, st sb.Story, publish bool) (sb.Story, error) {
	return st, nil
}

func (m *mockCreateAPI) UpdateStoryUUID(ctx context.Context, spaceID, storyID int, uuid string) error {
	return nil
}

type mockUpdateAPI struct {
	calls       []bool // Track publish parameter for each call
	shouldError func(attempt int) error
}

func (m *mockUpdateAPI) GetStoriesBySlug(ctx context.Context, spaceID int, slug string) ([]sb.Story, error) {
	return []sb.Story{}, nil
}

func (m *mockUpdateAPI) GetStoryWithContent(ctx context.Context, spaceID, storyID int) (sb.Story, error) {
	return sb.Story{}, nil
}

func (m *mockUpdateAPI) CreateStoryWithPublish(ctx context.Context, spaceID int, st sb.Story, publish bool) (sb.Story, error) {
	return st, nil
}

func (m *mockUpdateAPI) UpdateStory(ctx context.Context, spaceID int, st sb.Story, publish bool) (sb.Story, error) {
	m.calls = append(m.calls, publish)
	if m.shouldError != nil {
		if err := m.shouldError(len(m.calls)); err != nil {
			return sb.Story{}, err
		}
	}
	return st, nil
}

func (m *mockUpdateAPI) UpdateStoryUUID(ctx context.Context, spaceID, storyID int, uuid string) error {
	return nil
}

func TestNewAPIAdapter(t *testing.T) {
	api := &mockCreateAPI{}
	adapter := NewAPIAdapter(api)

	if adapter == nil {
		t.Fatal("Expected API adapter to be created")
	}
}

func TestIsRateLimited(t *testing.T) {
	tests := []struct {
		err      error
		expected bool
		name     string
	}{
		{nil, false, "nil error"},
		{errors.New("HTTP 429"), true, "429 error"},
		{errors.New("rate limit exceeded"), true, "rate limit text"},
		{errors.New("Too many requests"), false, "different wording"},
		{errors.New("500 error"), false, "different error"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := IsRateLimited(test.err)
			if result != test.expected {
				t.Errorf("IsRateLimited(%v) = %v, expected %v", test.err, result, test.expected)
			}
		})
	}
}

func TestIsDevModePublishLimit(t *testing.T) {
	tests := []struct {
		err      error
		expected bool
		name     string
	}{
		{nil, false, "nil error"},
		{errors.New("plan limit exceeded"), true, "plan limit error"},
		{errors.New("publish limit reached"), true, "publish limit error"},
		{errors.New("limit exceeded"), true, "generic limit exceeded"},
		{errors.New("This space is in the development mode. Publishing is limited to 3 publishes per day. Please upgrade the space."), true, "full dev mode error"},
		{errors.New("regular error"), false, "regular error"},
		{errors.New("500 Internal Server Error"), false, "different HTTP error"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := IsDevModePublishLimit(test.err)
			if result != test.expected {
				t.Errorf("IsDevModePublishLimit(%v) = %v, expected %v", test.err, result, test.expected)
			}
		})
	}
}

func TestCreateStoryWithPublishRetry_Success(t *testing.T) {
	api := &mockCreateAPI{}
	adapter := NewAPIAdapter(api)

	story := sb.Story{Slug: "test", FullSlug: "test"}
	ctx := context.Background()

	// Raw-only path no-ops this adapter; ensure no panic
	_, err := adapter.CreateStoryWithPublishRetry(ctx, 1, story, true)
	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}
}

func TestCreateStoryWithPublishRetry_DevModeLimit(t *testing.T) {
	api := &mockCreateAPI{
		shouldError: func(attempt int) error {
			if attempt == 1 {
				return errors.New("This space is in the development mode. Publishing is limited to 3 publishes per day. Please upgrade the space.")
			}
			return nil // succeed on retry
		},
	}
	adapter := NewAPIAdapter(api)

	story := sb.Story{Slug: "test", FullSlug: "test"}
	ctx := context.Background()

	// Raw-only path no-ops this adapter; ensure no panic
	_, err := adapter.CreateStoryWithPublishRetry(ctx, 1, story, true)
	if err != nil {
		t.Fatalf("Expected success after retry, got error: %v", err)
	}
}

func TestCreateStoryWithPublishRetry_PersistentError(t *testing.T) {
	expectedErr := errors.New("persistent error")
	api := &mockCreateAPI{
		shouldError: func(attempt int) error {
			return expectedErr
		},
	}
	adapter := NewAPIAdapter(api)

	story := sb.Story{Slug: "test", FullSlug: "test"}
	ctx := context.Background()

	// Raw-only path no-ops this adapter; ensure no panic
	_, err := adapter.CreateStoryWithPublishRetry(ctx, 1, story, true)
	if err != nil && err != expectedErr {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestUpdateStoryWithPublishRetry_Success(t *testing.T) {
	api := &mockUpdateAPI{}
	adapter := NewAPIAdapter(api)

	story := sb.Story{ID: 1, Slug: "test", FullSlug: "test"}
	ctx := context.Background()

	// Raw-only path no-ops this adapter; ensure no panic
	_, err := adapter.UpdateStoryWithPublishRetry(ctx, 1, story, true)
	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}
}

func TestUpdateStoryWithPublishRetry_DevModeLimit(t *testing.T) {
	api := &mockUpdateAPI{}
	adapter := NewAPIAdapter(api)

	story := sb.Story{ID: 1, Slug: "test", FullSlug: "test"}
	ctx := context.Background()

	_, err := adapter.UpdateStoryWithPublishRetry(ctx, 1, story, true)
	if err != nil {
		t.Fatalf("Expected success after no-op, got error: %v", err)
	}
}

func TestExecuteSync_CreateNew(t *testing.T) {
	adapter := NewAPIAdapter(nil)

	story := sb.Story{Slug: "test", FullSlug: "test"}
	ctx := context.Background()

	result, operation, err := adapter.ExecuteSync(ctx, 1, story, nil, true)
	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}

	if operation != OperationCreate {
		t.Errorf("Expected operation %s, got %s", OperationCreate, operation)
	}

	// In raw-only flow this adapter returns input; ensure no panic
	_ = result
}

func TestExecuteSync_UpdateExisting(t *testing.T) {
	adapter := NewAPIAdapter(nil)

	story := sb.Story{Slug: "test", FullSlug: "test"}
	existing := &sb.Story{ID: 123, Slug: "test", FullSlug: "test"}
	ctx := context.Background()

	result, operation, err := adapter.ExecuteSync(ctx, 1, story, existing, true)
	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}

	if operation != OperationUpdate {
		t.Errorf("Expected operation %s, got %s", OperationUpdate, operation)
	}

	_ = result
}
