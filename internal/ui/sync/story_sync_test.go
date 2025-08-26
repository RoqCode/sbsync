package sync

import (
    "context"
    "encoding/json"
    "errors"
    "testing"

    "storyblok-sync/internal/sb"
)

// Extended mock for story sync testing
type mockStorySyncAPI struct {
	stories                map[string][]sb.Story // slug -> stories
	storyContent           map[int]sb.Story      // storyID -> story with content
	createCalls            []sb.Story            // Track create calls
	updateCalls            []sb.Story            // Track update calls
	uuidUpdates            map[int]string        // storyID -> newUUID
	shouldError            bool
	errorMessage           string
	returnExistingOnUpdate bool // return existing story in UpdateStory to simulate API behavior
}

func (m *mockStorySyncAPI) GetStoriesBySlug(ctx context.Context, spaceID int, slug string) ([]sb.Story, error) {
	if m.shouldError {
		return nil, errors.New(m.errorMessage)
	}
	if stories, ok := m.stories[slug]; ok {
		return stories, nil
	}
	return []sb.Story{}, nil
}

func (m *mockStorySyncAPI) GetStoryWithContent(ctx context.Context, spaceID, storyID int) (sb.Story, error) {
	if m.shouldError {
		return sb.Story{}, errors.New(m.errorMessage)
	}
	if story, ok := m.storyContent[storyID]; ok {
		return story, nil
	}
	return sb.Story{}, errors.New("not found")
}

func (m *mockStorySyncAPI) CreateStoryWithPublish(ctx context.Context, spaceID int, st sb.Story, publish bool) (sb.Story, error) {
	if m.shouldError {
		return sb.Story{}, errors.New(m.errorMessage)
	}
	st.ID = len(m.createCalls) + 100 // Give it a unique ID
	m.createCalls = append(m.createCalls, st)
	return st, nil
}

func (m *mockStorySyncAPI) UpdateStory(ctx context.Context, spaceID int, st sb.Story, publish bool) (sb.Story, error) {
	if m.shouldError {
		return sb.Story{}, errors.New(m.errorMessage)
	}
	m.updateCalls = append(m.updateCalls, st)
	if m.returnExistingOnUpdate {
		if stories, ok := m.stories[st.Slug]; ok && len(stories) > 0 {
			return stories[0], nil
		}
	}
	return st, nil
}

func (m *mockStorySyncAPI) UpdateStoryUUID(ctx context.Context, spaceID, storyID int, uuid string) error {
	if m.shouldError {
		return errors.New(m.errorMessage)
	}
	if m.uuidUpdates == nil {
		m.uuidUpdates = make(map[int]string)
	}
	m.uuidUpdates[storyID] = uuid
	return nil
}

func TestNewStorySyncer(t *testing.T) {
	api := &mockStorySyncAPI{}
	syncer := NewStorySyncer(api, 1, 2)

	if syncer == nil {
		t.Fatal("Expected story syncer to be created")
	}
	if syncer.sourceSpaceID != 1 {
		t.Errorf("Expected source space ID 1, got %d", syncer.sourceSpaceID)
	}
	if syncer.targetSpaceID != 2 {
		t.Errorf("Expected target space ID 2, got %d", syncer.targetSpaceID)
	}
}

func TestSyncStory_CreateNew(t *testing.T) {
	api := &mockStorySyncAPI{
		stories: make(map[string][]sb.Story),
		storyContent: map[int]sb.Story{
            1: {
                ID:       1,
                Slug:     "test",
                FullSlug: "test",
                Content:  json.RawMessage([]byte(`{"component":"page"}`)),
            },
		},
	}

	syncer := NewStorySyncer(api, 1, 2)

	story := sb.Story{
		ID:       1,
		Slug:     "test",
		FullSlug: "test",
	}

	ctx := context.Background()
	result, err := syncer.SyncStory(ctx, story, true)

	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}

	if len(api.createCalls) != 1 {
		t.Errorf("Expected 1 create call, got %d", len(api.createCalls))
	}

	if len(api.updateCalls) != 0 {
		t.Errorf("Expected 0 update calls, got %d", len(api.updateCalls))
	}

	if result.ID == 0 {
		t.Error("Expected result to have ID set")
	}
}

func TestSyncStory_UpdateExisting(t *testing.T) {
    existingStory := sb.Story{
		ID:       123,
		Slug:     "test",
		FullSlug: "test",
        Content:  json.RawMessage([]byte(`{"component":"page"}`)),
	}

	api := &mockStorySyncAPI{
		stories: map[string][]sb.Story{
			"test": {existingStory},
		},
		storyContent: map[int]sb.Story{
            1: {
                ID:       1,
                Slug:     "test",
                FullSlug: "test",
                Content:  json.RawMessage([]byte(`{"component":"page"}`)),
                UUID:     "source-uuid",
            },
		},
	}

	syncer := NewStorySyncer(api, 1, 2)

	story := sb.Story{
		ID:       1,
		Slug:     "test",
		FullSlug: "test",
		UUID:     "source-uuid",
	}

	ctx := context.Background()
	result, err := syncer.SyncStory(ctx, story, true)

	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}

	if len(api.createCalls) != 0 {
		t.Errorf("Expected 0 create calls, got %d", len(api.createCalls))
	}

	if len(api.updateCalls) != 1 {
		t.Errorf("Expected 1 update call, got %d", len(api.updateCalls))
	}

	if result.ID != existingStory.ID {
		t.Errorf("Expected result ID %d, got %d", existingStory.ID, result.ID)
	}
}

func TestSyncStory_UUIDUpdate(t *testing.T) {
    existingStory := sb.Story{
		ID:       123,
		Slug:     "test",
		FullSlug: "test",
		UUID:     "existing-uuid",
        Content:  json.RawMessage([]byte(`{"component":"page"}`)),
	}

	api := &mockStorySyncAPI{
		stories: map[string][]sb.Story{
			"test": {existingStory},
		},
		storyContent: map[int]sb.Story{
            1: {
                ID:       1,
                Slug:     "test",
                FullSlug: "test",
                Content:  json.RawMessage([]byte(`{"component":"page"}`)),
                UUID:     "source-uuid",
            },
		},
		uuidUpdates:            make(map[int]string), // Initialize the map
		returnExistingOnUpdate: true,
	}

	syncer := NewStorySyncer(api, 1, 2)

	story := sb.Story{
		ID:       1,
		Slug:     "test",
		FullSlug: "test",
		UUID:     "source-uuid",
	}

	ctx := context.Background()
	_, err := syncer.SyncStory(ctx, story, true)

	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}

	// The UUID update should be tracked by the mock
	if len(api.uuidUpdates) != 1 {
		t.Errorf("Expected 1 UUID update, got %d", len(api.uuidUpdates))
	}

	if uuid, exists := api.uuidUpdates[123]; !exists || uuid != "source-uuid" {
		t.Errorf("Expected UUID update for story 123 to 'source-uuid', got %s", uuid)
	}
}

func TestSyncFolder_CreateNew(t *testing.T) {
	api := &mockStorySyncAPI{
		stories: make(map[string][]sb.Story),
		storyContent: map[int]sb.Story{
            1: {
                ID:       1,
                Slug:     "folder",
                FullSlug: "folder",
                IsFolder: true,
                Content:  json.RawMessage([]byte(`{"content_types":["page"]}`)),
            },
		},
	}

	syncer := NewStorySyncer(api, 1, 2)

	folder := sb.Story{
		ID:       1,
		Slug:     "folder",
		FullSlug: "folder",
		IsFolder: true,
	}

	ctx := context.Background()
	result, err := syncer.SyncFolder(ctx, folder, true)

	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}

	if len(api.createCalls) != 1 {
		t.Errorf("Expected 1 create call, got %d", len(api.createCalls))
	}

	if !api.createCalls[0].IsFolder {
		t.Error("Expected created item to be marked as folder")
	}

	if result.ID == 0 {
		t.Error("Expected result to have ID set")
	}
}

func TestSyncFolder_UpdateExisting(t *testing.T) {
    existingFolder := sb.Story{
		ID:       123,
		Slug:     "folder",
		FullSlug: "folder",
		IsFolder: true,
        Content:  json.RawMessage([]byte(`{"content_types":["page"]}`)),
	}

	api := &mockStorySyncAPI{
		stories: map[string][]sb.Story{
			"folder": {existingFolder},
		},
		storyContent: map[int]sb.Story{
            1: {
                ID:       1,
                Slug:     "folder",
                FullSlug: "folder",
                IsFolder: true,
                Content:  json.RawMessage([]byte(`{"content_types":["page"]}`)),
            },
		},
	}

	syncer := NewStorySyncer(api, 1, 2)

	folder := sb.Story{
		ID:       1,
		Slug:     "folder",
		FullSlug: "folder",
		IsFolder: true,
	}

	ctx := context.Background()
	result, err := syncer.SyncFolder(ctx, folder, true)

	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}

	if len(api.updateCalls) != 1 {
		t.Errorf("Expected 1 update call, got %d", len(api.updateCalls))
	}

	if result.ID != existingFolder.ID {
		t.Errorf("Expected result ID %d, got %d", existingFolder.ID, result.ID)
	}
}

func TestSyncStoryDetailed_OperationDetection(t *testing.T) {
	// Test create operation detection
	api := &mockStorySyncAPI{
		stories: make(map[string][]sb.Story),
		storyContent: map[int]sb.Story{
            1: {
                ID:       1,
                Slug:     "test",
                FullSlug: "test",
                Content:  json.RawMessage([]byte(`{"component":"page"}`)),
            },
		},
	}

	syncer := NewStorySyncer(api, 1, 2)

	story := sb.Story{
		ID:       1,
		Slug:     "test",
		FullSlug: "test",
	}

	result, err := syncer.SyncStoryDetailed(story, true)

	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}

	if result.Operation != OperationCreate {
		t.Errorf("Expected operation %s, got %s", OperationCreate, result.Operation)
	}

	if result.TargetStory == nil {
		t.Error("Expected target story to be set")
	}

	if result.Warning != "" {
		t.Errorf("Expected no warning, got: %s", result.Warning)
	}
}

func TestResolveParentFolder_ExistingParent(t *testing.T) {
	parentFolder := sb.Story{
		ID:       456,
		Slug:     "parent",
		FullSlug: "parent",
		IsFolder: true,
	}

	api := &mockStorySyncAPI{
		stories: map[string][]sb.Story{
			"parent": {parentFolder},
		},
	}

	syncer := NewStorySyncer(api, 1, 2)

	story := sb.Story{
		ID:       1,
		Slug:     "child",
		FullSlug: "parent/child",
		FolderID: &[]int{123}[0], // Original parent ID from source
	}

	ctx := context.Background()
	result := syncer.resolveParentFolder(ctx, story)

	if result.FolderID == nil {
		t.Fatal("Expected FolderID to be set")
	}

	if *result.FolderID != 456 {
		t.Errorf("Expected FolderID to be updated to 456, got %d", *result.FolderID)
	}
}

func TestResolveParentFolder_MissingParent(t *testing.T) {
	api := &mockStorySyncAPI{
		stories: make(map[string][]sb.Story), // No parent folder
	}

	syncer := NewStorySyncer(api, 1, 2)

	story := sb.Story{
		ID:       1,
		Slug:     "child",
		FullSlug: "missing/child",
		FolderID: &[]int{123}[0], // Original parent ID from source
	}

	ctx := context.Background()
	result := syncer.resolveParentFolder(ctx, story)

	if result.FolderID != nil {
		t.Errorf("Expected FolderID to be cleared when parent not found, got %v", *result.FolderID)
	}
}

func TestSyncStory_APIError(t *testing.T) {
	api := &mockStorySyncAPI{
		shouldError:  true,
		errorMessage: "API error",
	}

	syncer := NewStorySyncer(api, 1, 2)

	story := sb.Story{
		ID:       1,
		Slug:     "test",
		FullSlug: "test",
	}

	ctx := context.Background()
	_, err := syncer.SyncStory(ctx, story, true)

	if err == nil {
		t.Error("Expected error when API fails")
	}

	if err.Error() != "API error" {
		t.Errorf("Expected specific error message, got: %v", err)
	}
}
