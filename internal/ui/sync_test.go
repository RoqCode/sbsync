package ui

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"storyblok-sync/internal/sb"
)

// TestSyncRetryLogic has been moved to test the extracted sync module
// The retry logic is now handled by sync.SyncOrchestrator.SyncWithRetry
func TestSyncRetryLogic(t *testing.T) {
	// This test has been replaced by tests in the sync module
	// The syncWithRetry functionality is now in sync.SyncOrchestrator
	t.Skip("Retry logic has been moved to extracted sync module")
}

func TestTranslatedSlugsProcessing(t *testing.T) {
	sourceStory := sb.Story{
		ID:       1,
		Name:     "test",
		FullSlug: "test",
		TranslatedSlugs: []sb.TranslatedSlug{
			{Lang: "en", Name: "test", Path: "test"},
			{Lang: "de", Name: "test-de", Path: "test-de"},
		},
	}

	existingStory := sb.Story{
		ID:       2,
		Name:     "test",
		FullSlug: "test",
		TranslatedSlugs: []sb.TranslatedSlug{
			{ID: &[]int{100}[0], Lang: "en", Name: "test", Path: "test"},
			{ID: &[]int{101}[0], Lang: "de", Name: "test-de", Path: "test-de"},
		},
	}

	m := InitialModel()
	result := m.processTranslatedSlugs(sourceStory, []sb.Story{existingStory})

	// Should have no TranslatedSlugs but TranslatedSlugsAttributes instead
	if len(result.TranslatedSlugs) != 0 {
		t.Error("TranslatedSlugs should be cleared")
	}

	if len(result.TranslatedSlugsAttributes) != 2 {
		t.Fatalf("expected 2 translated slug attributes, got %d", len(result.TranslatedSlugsAttributes))
	}

	// Check that IDs were preserved from existing story
	for _, attr := range result.TranslatedSlugsAttributes {
		if attr.Lang == "en" && (attr.ID == nil || *attr.ID != 100) {
			t.Error("English translated slug ID should be preserved as 100")
		}
		if attr.Lang == "de" && (attr.ID == nil || *attr.ID != 101) {
			t.Error("German translated slug ID should be preserved as 101")
		}
	}
}

func TestParentSlugFunction(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"app/sub/page", "app/sub"},
		{"app/page", "app"},
		{"page", ""},
		{"", ""},
	}

	for _, test := range tests {
		result := parentSlug(test.input)
		if result != test.expected {
			t.Errorf("parentSlug(%q) = %q, expected %q", test.input, result, test.expected)
		}
	}
}

type mockAPI struct {
	source       map[int]sb.Story
	target       map[string]sb.Story
	nextID       int
	publishCalls []bool
}

func (m *mockAPI) GetStoriesBySlug(ctx context.Context, spaceID int, slug string) ([]sb.Story, error) {
	if st, ok := m.target[slug]; ok {
		return []sb.Story{st}, nil
	}
	return nil, nil
}

func (m *mockAPI) GetStoryWithContent(ctx context.Context, spaceID, storyID int) (sb.Story, error) {
	if st, ok := m.source[storyID]; ok {
		return st, nil
	}
	return sb.Story{}, fmt.Errorf("not found")
}

func (m *mockAPI) CreateStoryWithPublish(ctx context.Context, spaceID int, st sb.Story, publish bool) (sb.Story, error) {
	m.publishCalls = append(m.publishCalls, publish)
	m.nextID++
	st.ID = m.nextID
	m.target[st.FullSlug] = st
	return st, nil
}

func TestEnsureFolderPathCreatesFolders(t *testing.T) {
	srcFolders := []sb.Story{
		{ID: 1, Name: "foo", Slug: "foo", FullSlug: "foo", IsFolder: true, Content: json.RawMessage([]byte(`{"content_types":["page"],"lock_subfolders_content_types":false}`))},
		{ID: 2, Name: "bar", Slug: "bar", FullSlug: "foo/bar", IsFolder: true, FolderID: &[]int{1}[0], Content: json.RawMessage([]byte(`{"content_types":["page"],"lock_subfolders_content_types":false}`))},
	}
	api := &mockAPI{
		source: map[int]sb.Story{
			1: srcFolders[0],
			2: srcFolders[1],
		},
		target: make(map[string]sb.Story),
	}
	report := Report{}

	created, err := ensureFolderPathImpl(api, &report, srcFolders, 1, 2, "foo/bar/baz", true)
	if err != nil {
		t.Fatalf("ensureFolderPathImpl returned error: %v", err)
	}
	if len(created) != 2 {
		t.Fatalf("expected 2 folders created, got %d", len(created))
	}
	if foo, ok := api.target["foo"]; !ok {
		t.Errorf("expected folder 'foo' to be created")
	} else {
		var tmp map[string]interface{}
		_ = json.Unmarshal(foo.Content, &tmp)
		v, ok := tmp["content_types"]
		if !ok {
			t.Errorf("expected folder 'foo' to keep content type 'page'")
		} else {
			arr, _ := v.([]interface{})
			if len(arr) != 1 || arr[0] != "page" {
				t.Errorf("expected folder 'foo' to keep content type 'page'")
			}
		}
	}
	if bar, ok := api.target["foo/bar"]; !ok {
		t.Errorf("expected folder 'foo/bar' to be created")
	} else {
		parent := api.target["foo"]
		if bar.FolderID == nil || *bar.FolderID != parent.ID {
			t.Errorf("expected 'foo/bar' to reference parent 'foo'")
		}
		var tmp map[string]interface{}
		_ = json.Unmarshal(bar.Content, &tmp)
		v, ok := tmp["content_types"]
		if !ok {
			t.Errorf("expected folder 'foo/bar' to keep content type 'page'")
		} else {
			arr, _ := v.([]interface{})
			if len(arr) != 1 || arr[0] != "page" {
				t.Errorf("expected folder 'foo/bar' to keep content type 'page'")
			}
		}
	}
	if len(report.Entries) != 2 {
		t.Fatalf("expected 2 report entries, got %d", len(report.Entries))
	}
	if report.Entries[0].Operation != "create" {
		t.Errorf("expected operation 'create', got %s", report.Entries[0].Operation)
	}
	if len(api.publishCalls) != 2 || !api.publishCalls[0] || !api.publishCalls[1] {
		t.Errorf("expected publish flag true for created folders")
	}
}

type publishLimitCreateMock struct {
	calls []bool
}

func (m *publishLimitCreateMock) CreateStoryWithPublish(ctx context.Context, spaceID int, st sb.Story, publish bool) (sb.Story, error) {
	m.calls = append(m.calls, publish)
	if len(m.calls) == 1 {
		return sb.Story{}, errors.New("This space is in the development mode. Publishing is limited to 3 publishes per day. Please upgrade the space.")
	}
	st.ID = 1
	return st, nil
}

func TestCreateStoryWithPublishRetryDevMode(t *testing.T) {
	// This test has been moved to the extracted sync module
	// The retry logic is now in sync.APIAdapter.CreateStoryWithPublishRetry
	t.Skip("Publish retry logic has been moved to extracted sync module")
}

type publishLimitUpdateMock struct {
	calls []bool
}

func (m *publishLimitUpdateMock) UpdateStory(ctx context.Context, spaceID int, st sb.Story, publish bool) (sb.Story, error) {
	m.calls = append(m.calls, publish)
	if len(m.calls) == 1 {
		return sb.Story{}, errors.New("This space is in the development mode. Publishing is limited to 3 publishes per day. Please upgrade the space.")
	}
	return st, nil
}

func TestUpdateStoryWithPublishRetryDevMode(t *testing.T) {
	// This test has been moved to the extracted sync module
	// The retry logic is now in sync.APIAdapter.UpdateStoryWithPublishRetry
	t.Skip("Publish retry logic has been moved to extracted sync module")
}

func TestShouldPublishChecksPlanLevel(t *testing.T) {
	m := InitialModel()
	if !m.shouldPublish() {
		t.Errorf("expected default shouldPublish to be true")
	}

	m.targetSpace = &sb.Space{PlanLevel: 999}
	if m.shouldPublish() {
		t.Errorf("expected shouldPublish false for plan level 999")
	}

	m.targetSpace.PlanLevel = 1
	if !m.shouldPublish() {
		t.Errorf("expected shouldPublish true for plan level 1")
	}
}
