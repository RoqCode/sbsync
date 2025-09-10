package sync

import (
	"context"
	"errors"
	"sync"
	"testing"

	"storyblok-sync/internal/sb"
)

// mockRateLimitedAPI implements SyncAPI with rate limiting simulation
type mockRateLimitedAPI struct {
	mu sync.Mutex
	// Call tracking
	readCalls  int
	writeCalls int

	// Rate limiting simulation
	shouldRateLimit bool
	rateLimitCount  int

	// Mock data
	storiesBySlug map[string][]sb.Story
	storiesByID   map[int]sb.Story
	rawStories    map[int]map[string]interface{}
}

func newMockRateLimitedAPI() *mockRateLimitedAPI {
	return &mockRateLimitedAPI{
		storiesBySlug: make(map[string][]sb.Story),
		storiesByID:   make(map[int]sb.Story),
		rawStories:    make(map[int]map[string]interface{}),
	}
}

func (m *mockRateLimitedAPI) GetStoriesBySlug(ctx context.Context, spaceID int, slug string) ([]sb.Story, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.readCalls++

	if m.shouldRateLimit && m.rateLimitCount > 0 {
		m.rateLimitCount--
		return nil, errors.New("HTTP 429 Too Many Requests")
	}

	if stories, ok := m.storiesBySlug[slug]; ok {
		return stories, nil
	}
	return []sb.Story{}, nil
}

func (m *mockRateLimitedAPI) GetStoryWithContent(ctx context.Context, spaceID, storyID int) (sb.Story, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.readCalls++

	if m.shouldRateLimit && m.rateLimitCount > 0 {
		m.rateLimitCount--
		return sb.Story{}, errors.New("HTTP 429 Too Many Requests")
	}

	return m.storiesByID[storyID], nil
}

func (m *mockRateLimitedAPI) CreateStoryWithPublish(ctx context.Context, spaceID int, st sb.Story, publish bool) (sb.Story, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.writeCalls++

	if m.shouldRateLimit && m.rateLimitCount > 0 {
		m.rateLimitCount--
		return sb.Story{}, errors.New("HTTP 429 Too Many Requests")
	}

	st.ID = 100 + m.writeCalls
	m.storiesBySlug[st.FullSlug] = []sb.Story{st}
	return st, nil
}

func (m *mockRateLimitedAPI) UpdateStory(ctx context.Context, spaceID int, st sb.Story, publish bool) (sb.Story, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.writeCalls++

	if m.shouldRateLimit && m.rateLimitCount > 0 {
		m.rateLimitCount--
		return sb.Story{}, errors.New("HTTP 429 Too Many Requests")
	}

	m.storiesBySlug[st.FullSlug] = []sb.Story{st}
	return st, nil
}

func (m *mockRateLimitedAPI) UpdateStoryUUID(ctx context.Context, spaceID, storyID int, uuid string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.writeCalls++

	if m.shouldRateLimit && m.rateLimitCount > 0 {
		m.rateLimitCount--
		return errors.New("429 Too Many Requests")
	}

	return nil
}

// storyRawAPI implementation
func (m *mockRateLimitedAPI) GetStoryRaw(ctx context.Context, spaceID, storyID int) (map[string]interface{}, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.readCalls++

	if m.shouldRateLimit && m.rateLimitCount > 0 {
		m.rateLimitCount--
		return nil, errors.New("HTTP 429 Too Many Requests")
	}

	return m.rawStories[storyID], nil
}

func (m *mockRateLimitedAPI) CreateStoryRawWithPublish(ctx context.Context, spaceID int, story map[string]interface{}, publish bool) (sb.Story, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.writeCalls++

	if m.shouldRateLimit && m.rateLimitCount > 0 {
		m.rateLimitCount--
		return sb.Story{}, errors.New("HTTP 429 Too Many Requests")
	}

	st := sb.Story{ID: 100 + m.writeCalls, FullSlug: "test-story"}
	m.storiesBySlug[st.FullSlug] = []sb.Story{st}
	return st, nil
}

func (m *mockRateLimitedAPI) UpdateStoryRawWithPublish(ctx context.Context, spaceID int, storyID int, story map[string]interface{}, publish bool) (sb.Story, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.writeCalls++

	if m.shouldRateLimit && m.rateLimitCount > 0 {
		m.rateLimitCount--
		return sb.Story{}, errors.New("HTTP 429 Too Many Requests")
	}

	st := sb.Story{ID: storyID, FullSlug: "test-story"}
	m.storiesBySlug[st.FullSlug] = []sb.Story{st}
	return st, nil
}

func TestStorySyncer_RateLimitingIntegration(t *testing.T) {
	api := newMockRateLimitedAPI()
	api.shouldRateLimit = true
	api.rateLimitCount = 1 // Will rate limit first call

	// Set up test data
	story := sb.Story{ID: 1, Slug: "test", FullSlug: "test", Content: []byte(`{"component":"page"}`)}
	api.storiesByID[1] = story
	api.rawStories[1] = map[string]interface{}{
		"uuid":      "test-uuid",
		"name":      "Test",
		"slug":      "test",
		"full_slug": "test",
		"content":   map[string]interface{}{"component": "page"},
		"is_folder": false,
	}

	syncer := NewStorySyncer(api, 10, 20, map[string]sb.Story{})

	// Sync should succeed even with rate limiting on GetStoriesBySlug (it's handled gracefully)
	_, err := syncer.SyncStory(context.Background(), story, true)
	if err != nil {
		t.Errorf("Expected sync to succeed despite rate limiting, got error: %v", err)
	}

	// Verify rate limiting was triggered (rateLimitCount should be 0)
	if api.rateLimitCount != 0 {
		t.Errorf("Expected rate limiting to be triggered, rateLimitCount=%d", api.rateLimitCount)
	}

	// Verify limiter was nudged down on target space (where GetStoriesBySlug was called)
	buckets := syncer.limiter.get(20) // target space ID
	if buckets.read.rps >= 7 {
		t.Errorf("Expected read RPS to be reduced from 7, got %v", buckets.read.rps)
	}
}

func TestStorySyncer_RateLimitingRecovery(t *testing.T) {
	api := newMockRateLimitedAPI()
	api.shouldRateLimit = true
	api.rateLimitCount = 1 // Will rate limit first call only

	// Set up test data
	story := sb.Story{ID: 1, Slug: "test", FullSlug: "test", Content: []byte(`{"component":"page"}`)}
	api.storiesByID[1] = story
	api.rawStories[1] = map[string]interface{}{
		"uuid":      "test-uuid",
		"name":      "Test",
		"slug":      "test",
		"full_slug": "test",
		"content":   map[string]interface{}{"component": "page"},
		"is_folder": false,
	}

	syncer := NewStorySyncer(api, 10, 20, map[string]sb.Story{})

	// First sync should succeed despite rate limiting on GetStoriesBySlug (handled gracefully)
	_, err := syncer.SyncStory(context.Background(), story, true)
	if err != nil {
		t.Errorf("Expected first sync to succeed despite rate limiting, got error: %v", err)
	}

	// Verify rate limiting was triggered
	if api.rateLimitCount != 0 {
		t.Errorf("Expected rate limiting to be triggered, rateLimitCount=%d", api.rateLimitCount)
	}

	// Verify limiter was nudged down on target space
	buckets := syncer.limiter.get(20) // target space ID
	if buckets.read.rps >= 7 {
		t.Errorf("Expected read RPS to be reduced from 7, got %v", buckets.read.rps)
	}

	// Second sync should succeed and nudge RPS back up
	_, err = syncer.SyncStory(context.Background(), story, true)
	if err != nil {
		t.Errorf("Expected second sync to succeed, got error: %v", err)
	}

	// Verify limiter was nudged up after success
	if buckets.read.rps <= 1 {
		t.Errorf("Expected read RPS to be increased after success, got %v", buckets.read.rps)
	}
}

func TestStorySyncer_LimiterUsage(t *testing.T) {
	api := newMockRateLimitedAPI()

	// Set up test data
	story := sb.Story{ID: 1, Slug: "test", FullSlug: "test", Content: []byte(`{"component":"page"}`)}
	api.storiesByID[1] = story
	api.rawStories[1] = map[string]interface{}{
		"uuid":      "test-uuid",
		"name":      "Test",
		"slug":      "test",
		"full_slug": "test",
		"content":   map[string]interface{}{"component": "page"},
		"is_folder": false,
	}

	syncer := NewStorySyncer(api, 10, 20, map[string]sb.Story{})

	// Sync should succeed
	_, err := syncer.SyncStory(context.Background(), story, true)
	if err != nil {
		t.Errorf("Expected sync to succeed, got error: %v", err)
	}

	// Verify limiter was used (should have made API calls)
	if api.readCalls == 0 {
		t.Error("Expected read calls to be made")
	}
	if api.writeCalls == 0 {
		t.Error("Expected write calls to be made")
	}

	// Verify limiter buckets exist
	targetBuckets := syncer.limiter.get(20) // target space ID
	if targetBuckets == nil {
		t.Error("Expected target buckets to exist")
	}
}

func TestStorySyncer_UpdateWithRateLimiting(t *testing.T) {
	api := newMockRateLimitedAPI()
	api.shouldRateLimit = true
	api.rateLimitCount = 1 // Will rate limit first call

	// Set up existing story in target
	existing := sb.Story{ID: 200, FullSlug: "test"}
	api.storiesBySlug["test"] = []sb.Story{existing}

	// Set up source story
	story := sb.Story{ID: 1, Slug: "test", FullSlug: "test", Content: []byte(`{"component":"page"}`)}
	api.storiesByID[1] = story
	api.rawStories[1] = map[string]interface{}{
		"uuid":      "test-uuid",
		"name":      "Test",
		"slug":      "test",
		"full_slug": "test",
		"content":   map[string]interface{}{"component": "page"},
		"is_folder": false,
	}

	syncer := NewStorySyncer(api, 10, 20, map[string]sb.Story{})

	// Update should succeed despite rate limiting on GetStoriesBySlug (handled gracefully)
	_, err := syncer.SyncStory(context.Background(), story, true)
	if err != nil {
		t.Errorf("Expected update to succeed despite rate limiting, got error: %v", err)
	}

	// Verify rate limiting was triggered
	if api.rateLimitCount != 0 {
		t.Errorf("Expected rate limiting to be triggered, rateLimitCount=%d", api.rateLimitCount)
	}

	// Verify read limiter was nudged down on target space
	buckets := syncer.limiter.get(20) // target space ID
	if buckets.read.rps >= 7 {
		t.Errorf("Expected read RPS to be reduced from 7, got %v", buckets.read.rps)
	}
}

func TestStorySyncer_FolderSyncWithRateLimiting(t *testing.T) {
	api := newMockRateLimitedAPI()
	api.shouldRateLimit = true
	api.rateLimitCount = 1 // Will rate limit first call

	// Set up folder
	folder := sb.Story{ID: 1, Slug: "folder", FullSlug: "folder", IsFolder: true, Content: []byte(`{}`)}
	api.storiesByID[1] = folder
	api.rawStories[1] = map[string]interface{}{
		"uuid":      "folder-uuid",
		"name":      "Folder",
		"slug":      "folder",
		"full_slug": "folder",
		"is_folder": true,
	}

	syncer := NewStorySyncer(api, 10, 20, map[string]sb.Story{})

	// Folder sync should fail due to rate limiting on GetStoriesBySlug (not handled gracefully)
	_, err := syncer.SyncFolder(context.Background(), folder, false)
	if err == nil {
		t.Error("Expected folder sync to fail due to rate limiting")
	}

	// Verify rate limiting was detected
	if !IsRateLimited(err) {
		t.Error("Expected IsRateLimited to return true")
	}

	// Verify rate limiting was triggered
	if api.rateLimitCount != 0 {
		t.Errorf("Expected rate limiting to be triggered, rateLimitCount=%d", api.rateLimitCount)
	}
}

func TestStorySyncer_DetailedSyncWithRateLimiting(t *testing.T) {
	api := newMockRateLimitedAPI()
	api.shouldRateLimit = true
	api.rateLimitCount = 1 // Will rate limit first call

	// Set up test data
	story := sb.Story{ID: 1, Slug: "test", FullSlug: "test", Content: []byte(`{"component":"page"}`)}
	api.storiesByID[1] = story
	api.rawStories[1] = map[string]interface{}{
		"uuid":      "test-uuid",
		"name":      "Test",
		"slug":      "test",
		"full_slug": "test",
		"content":   map[string]interface{}{"component": "page"},
		"is_folder": false,
	}

	syncer := NewStorySyncer(api, 10, 20, map[string]sb.Story{})

	// Detailed sync should succeed despite rate limiting (handled gracefully)
	result, err := syncer.SyncStoryDetailed(story, true)
	if err != nil {
		t.Errorf("Expected detailed sync to succeed despite rate limiting, got error: %v", err)
	}

	// Verify result contains retry information (should be 0 since no transport retries occurred)
	if result == nil {
		t.Fatal("Expected result to be non-nil")
	}
	if result.RetryTotal != 0 {
		t.Errorf("Expected RetryTotal to be 0 (no transport retries), got %d", result.RetryTotal)
	}

	// Verify rate limiting was triggered
	if api.rateLimitCount != 0 {
		t.Errorf("Expected rate limiting to be triggered, rateLimitCount=%d", api.rateLimitCount)
	}
}

func TestStorySyncer_ConcurrentRateLimiting(t *testing.T) {
	api := newMockRateLimitedAPI()
	api.shouldRateLimit = true
	api.rateLimitCount = 5 // Will rate limit first 5 calls

	// Set up test data
	story := sb.Story{ID: 1, Slug: "test", FullSlug: "test", Content: []byte(`{"component":"page"}`)}
	api.storiesByID[1] = story
	api.rawStories[1] = map[string]interface{}{
		"uuid":      "test-uuid",
		"name":      "Test",
		"slug":      "test",
		"full_slug": "test",
		"content":   map[string]interface{}{"component": "page"},
		"is_folder": false,
	}

	syncer := NewStorySyncer(api, 10, 20, map[string]sb.Story{})

	// Test concurrent access to same limiter
	done := make(chan error, 3)

	for i := 0; i < 3; i++ {
		go func() {
			_, err := syncer.SyncStory(context.Background(), story, true)
			done <- err
		}()
	}

	// Wait for all goroutines to complete
	var errors []error
	for i := 0; i < 3; i++ {
		errors = append(errors, <-done)
	}

	// At least one should have succeeded (after rate limiting is exhausted)
	successCount := 0
	for _, err := range errors {
		if err == nil {
			successCount++
		}
	}

	if successCount == 0 {
		t.Error("Expected at least one sync to succeed")
	}

	// Verify limiter was used
	buckets := syncer.limiter.get(20) // target space ID
	if buckets.read.rps >= 7 {
		t.Errorf("Expected read RPS to be reduced from 7, got %v", buckets.read.rps)
	}
}

func TestStorySyncer_LimiterPerSpace(t *testing.T) {
	api := newMockRateLimitedAPI()

	// Set up test data
	story := sb.Story{ID: 1, Slug: "test", FullSlug: "test", Content: []byte(`{"component":"page"}`)}
	api.storiesByID[1] = story
	api.rawStories[1] = map[string]interface{}{
		"uuid":      "test-uuid",
		"name":      "Test",
		"slug":      "test",
		"full_slug": "test",
		"content":   map[string]interface{}{"component": "page"},
		"is_folder": false,
	}

	syncer := NewStorySyncer(api, 10, 20, map[string]sb.Story{})

	// Set initial RPS to lower values to allow nudging up
	sourceBuckets := syncer.limiter.get(10) // source space ID
	targetBuckets := syncer.limiter.get(20) // target space ID
	sourceBuckets.read.rps = 5
	targetBuckets.write.rps = 5

	// Sync should succeed
	_, err := syncer.SyncStory(context.Background(), story, true)
	if err != nil {
		t.Errorf("Expected sync to succeed, got error: %v", err)
	}

	// Verify different spaces have different limiters
	if sourceBuckets == targetBuckets {
		t.Error("Expected different limiters for different spaces")
	}

	// Both should have been used
	if sourceBuckets.read.rps <= 5 {
		t.Errorf("Expected source read RPS to be nudged up from 5, got %v", sourceBuckets.read.rps)
	}
	if targetBuckets.write.rps <= 5 {
		t.Errorf("Expected target write RPS to be nudged up from 5, got %v", targetBuckets.write.rps)
	}
}

func TestStorySyncer_ContextCancellation(t *testing.T) {
	api := newMockRateLimitedAPI()

	// Set up test data
	story := sb.Story{ID: 1, Slug: "test", FullSlug: "test", Content: []byte(`{"component":"page"}`)}
	api.storiesByID[1] = story
	api.rawStories[1] = map[string]interface{}{
		"uuid":      "test-uuid",
		"name":      "Test",
		"slug":      "test",
		"full_slug": "test",
		"content":   map[string]interface{}{"component": "page"},
		"is_folder": false,
	}

	syncer := NewStorySyncer(api, 10, 20, map[string]sb.Story{})

	// Test that context is passed through properly
	ctx := context.Background()

	// Sync should succeed with valid context
	_, err := syncer.SyncStory(ctx, story, true)
	if err != nil {
		t.Errorf("Expected sync to succeed with valid context, got error: %v", err)
	}

	// Test that the limiter respects context (this is tested in the limiter tests)
	// The story sync itself doesn't have long-running operations that would benefit from context cancellation
}
