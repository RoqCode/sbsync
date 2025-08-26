package sync

import (
	"context"
	"errors"
	"testing"

	"storyblok-sync/internal/sb"
)

// Mock for content manager testing
type mockContentAPI struct {
	storyContent map[int]sb.Story // storyID -> story with content
	callCount    int              // Track number of API calls
	shouldError  bool
	errorMessage string
}

func (m *mockContentAPI) GetStoriesBySlug(ctx context.Context, spaceID int, slug string) ([]sb.Story, error) {
	return []sb.Story{}, nil
}

func (m *mockContentAPI) GetStoryWithContent(ctx context.Context, spaceID, storyID int) (sb.Story, error) {
	m.callCount++
	if m.shouldError {
		return sb.Story{}, errors.New(m.errorMessage)
	}
	if story, ok := m.storyContent[storyID]; ok {
		return story, nil
	}
	return sb.Story{}, errors.New("not found")
}

func (m *mockContentAPI) CreateStoryWithPublish(ctx context.Context, spaceID int, st sb.Story, publish bool) (sb.Story, error) {
	return st, nil
}

func (m *mockContentAPI) UpdateStory(ctx context.Context, spaceID int, st sb.Story, publish bool) (sb.Story, error) {
	return st, nil
}

func (m *mockContentAPI) UpdateStoryUUID(ctx context.Context, spaceID, storyID int, uuid string) error {
	return nil
}

func TestNewContentManager(t *testing.T) {
	api := &mockContentAPI{}
	cm := NewContentManager(api, 1)
	
	if cm == nil {
		t.Fatal("Expected content manager to be created")
	}
	
	if cm.spaceID != 1 {
		t.Errorf("Expected space ID 1, got %d", cm.spaceID)
	}
	
	if cm.maxSize != 500 {
		t.Errorf("Expected max size 500, got %d", cm.maxSize)
	}
	
	if cm.cache == nil {
		t.Error("Expected cache to be initialized")
	}
}

func TestEnsureContent_CacheHit(t *testing.T) {
	api := &mockContentAPI{
		storyContent: map[int]sb.Story{
			1: {
				ID: 1,
				Slug: "test",
				FullSlug: "test",
				Content: map[string]interface{}{"component": "page"},
			},
		},
	}
	
	cm := NewContentManager(api, 1)
	ctx := context.Background()
	
	story := sb.Story{ID: 1, Slug: "test", FullSlug: "test"}
	
	// First call should hit API
	result1, err1 := cm.EnsureContent(ctx, story)
	if err1 != nil {
		t.Fatalf("Expected success on first call, got error: %v", err1)
	}
	
	if api.callCount != 1 {
		t.Errorf("Expected 1 API call after first request, got %d", api.callCount)
	}
	
	// Second call should hit cache
	result2, err2 := cm.EnsureContent(ctx, story)
	if err2 != nil {
		t.Fatalf("Expected success on cached call, got error: %v", err2)
	}
	
	if api.callCount != 1 {
		t.Errorf("Expected still 1 API call after cached request, got %d", api.callCount)
	}
	
	// Results should be identical
	if result1.ID != result2.ID || result1.FullSlug != result2.FullSlug {
		t.Error("Expected identical results from cache hit")
	}
	
	if result1.Content == nil || result2.Content == nil {
		t.Error("Expected content to be preserved")
	}
}

func TestEnsureContent_AlreadyHasContent(t *testing.T) {
	api := &mockContentAPI{}
	cm := NewContentManager(api, 1)
	ctx := context.Background()
	
	// Story already has content
	story := sb.Story{
		ID: 1,
		Slug: "test",
		FullSlug: "test",
		Content: map[string]interface{}{"component": "existing"},
	}
	
	result, err := cm.EnsureContent(ctx, story)
	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}
	
	if api.callCount != 0 {
		t.Errorf("Expected 0 API calls when story already has content, got %d", api.callCount)
	}
	
	if result.Content == nil {
		t.Error("Expected content to be preserved")
	}
	
	component := result.Content["component"]
	if component != "existing" {
		t.Errorf("Expected existing content to be preserved, got %v", component)
	}
}

func TestEnsureContent_APIError(t *testing.T) {
	api := &mockContentAPI{
		shouldError:  true,
		errorMessage: "API failure",
	}
	
	cm := NewContentManager(api, 1)
	ctx := context.Background()
	
	story := sb.Story{ID: 1, Slug: "test", FullSlug: "test"}
	
	_, err := cm.EnsureContent(ctx, story)
	if err == nil {
		t.Error("Expected error when API fails")
	}
	
	if err.Error() != "API failure" {
		t.Errorf("Expected specific error message, got: %v", err)
	}
}

func TestAddToCache_Basic(t *testing.T) {
	api := &mockContentAPI{}
	cm := NewContentManager(api, 1)
	
	story := sb.Story{
		ID: 1,
		Slug: "test",
		FullSlug: "test",
		Content: map[string]interface{}{"component": "page"},
	}
	
	cm.addToCache(story)
	
	// Check if story was cached
	cached, exists := cm.cache[1]
	if !exists {
		t.Error("Expected story to be cached")
	}
	
	if cached.ID != 1 || cached.FullSlug != "test" {
		t.Error("Expected cached story to match original")
	}
	
	if cm.hitCount != 0 {
		t.Errorf("Expected hit count to be 0, got %d", cm.hitCount)
	}
}

func TestAddToCache_EvictionStrategy(t *testing.T) {
	api := &mockContentAPI{}
	cm := NewContentManager(api, 1)
	cm.maxSize = 2 // Set small cache size for testing
	
	// Add three stories to trigger eviction
	stories := []sb.Story{
		{ID: 1, FullSlug: "story1", Content: map[string]interface{}{"component": "page"}},
		{ID: 2, FullSlug: "story2", Content: map[string]interface{}{"component": "page"}},
		{ID: 3, FullSlug: "story3", Content: map[string]interface{}{"component": "page"}},
	}
	
	for _, story := range stories {
		cm.addToCache(story)
	}
	
	// Cache should not exceed max size
	if len(cm.cache) > cm.maxSize {
		t.Errorf("Expected cache size <= %d, got %d", cm.maxSize, len(cm.cache))
	}
	
	// Should contain the most recently added stories
	if _, exists := cm.cache[3]; !exists {
		t.Error("Expected most recent story (ID 3) to be in cache")
	}
	
	if _, exists := cm.cache[2]; !exists {
		t.Error("Expected recent story (ID 2) to be in cache")
	}
	
	// First story should have been evicted
	if _, exists := cm.cache[1]; exists {
		t.Error("Expected oldest story (ID 1) to be evicted")
	}
}

func TestCacheStats(t *testing.T) {
	api := &mockContentAPI{
		storyContent: map[int]sb.Story{
			1: {
				ID: 1,
				Content: map[string]interface{}{"component": "page"},
			},
		},
	}
	
	cm := NewContentManager(api, 1)
	ctx := context.Background()
	
	story := sb.Story{ID: 1, FullSlug: "test"}
	
	// First call - cache miss
	_, err := cm.EnsureContent(ctx, story)
	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}
	
	if cm.hitCount != 0 {
		t.Errorf("Expected 0 hits after cache miss, got %d", cm.hitCount)
	}
	
	// Second call - cache hit
	_, err = cm.EnsureContent(ctx, story)
	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}
	
	if cm.hitCount != 1 {
		t.Errorf("Expected 1 hit after cache hit, got %d", cm.hitCount)
	}
	
	// Third call - another cache hit
	_, err = cm.EnsureContent(ctx, story)
	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}
	
	if cm.hitCount != 2 {
		t.Errorf("Expected 2 hits after second cache hit, got %d", cm.hitCount)
	}
}

func TestEnsureContent_EmptyCache(t *testing.T) {
	api := &mockContentAPI{
		storyContent: map[int]sb.Story{
			1: {
				ID: 1,
				Slug: "test",
				FullSlug: "test",
				Content: map[string]interface{}{"component": "page"},
			},
		},
	}
	
	cm := NewContentManager(api, 1)
	ctx := context.Background()
	
	// Verify cache is initially empty
	if len(cm.cache) != 0 {
		t.Errorf("Expected empty cache initially, got %d items", len(cm.cache))
	}
	
	story := sb.Story{ID: 1, Slug: "test", FullSlug: "test"}
	
	result, err := cm.EnsureContent(ctx, story)
	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}
	
	// Verify content was fetched and cached
	if result.Content == nil {
		t.Error("Expected content to be fetched")
	}
	
	if len(cm.cache) != 1 {
		t.Errorf("Expected 1 item in cache after fetch, got %d", len(cm.cache))
	}
}

func TestEnsureContent_ConcurrentAccess(t *testing.T) {
	api := &mockContentAPI{
		storyContent: map[int]sb.Story{
			1: {ID: 1, Content: map[string]interface{}{"component": "page"}},
			2: {ID: 2, Content: map[string]interface{}{"component": "article"}},
			3: {ID: 3, Content: map[string]interface{}{"component": "blog"}},
		},
	}
	
	cm := NewContentManager(api, 1)
	ctx := context.Background()
	
	// Simulate concurrent access to different stories
	stories := []sb.Story{
		{ID: 1, FullSlug: "story1"},
		{ID: 2, FullSlug: "story2"},
		{ID: 3, FullSlug: "story3"},
	}
	
	// Process stories concurrently (simulated by sequential calls)
	for _, story := range stories {
		result, err := cm.EnsureContent(ctx, story)
		if err != nil {
			t.Fatalf("Expected success for story %d, got error: %v", story.ID, err)
		}
		if result.Content == nil {
			t.Errorf("Expected content for story %d", story.ID)
		}
	}
	
	// All stories should be cached
	if len(cm.cache) != 3 {
		t.Errorf("Expected 3 items in cache, got %d", len(cm.cache))
	}
}