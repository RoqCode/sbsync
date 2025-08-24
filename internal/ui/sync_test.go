package ui

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"storyblok-sync/internal/sb"
)

func TestSyncRetryLogic(t *testing.T) {
	m := InitialModel()

	attempts := 0
	operation := func() error {
		attempts++
		if attempts < 3 {
			return errors.New("temporary error")
		}
		return nil // succeed on third attempt
	}

	err := m.syncWithRetry(operation)
	if err != nil {
		t.Errorf("Expected operation to succeed after retries, got error: %v", err)
	}

	if attempts != 3 {
		t.Errorf("Expected 3 attempts, got %d", attempts)
	}
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
		{ID: 1, Name: "foo", Slug: "foo", FullSlug: "foo", IsFolder: true, ContentType: "page"},
		{ID: 2, Name: "bar", Slug: "bar", FullSlug: "foo/bar", IsFolder: true, FolderID: &[]int{1}[0], ContentType: "page"},
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
	} else if foo.ContentType != "page" {
		t.Errorf("expected folder 'foo' to keep content type 'page'")
	}
	if bar, ok := api.target["foo/bar"]; !ok {
		t.Errorf("expected folder 'foo/bar' to be created")
	} else {
		parent := api.target["foo"]
		if bar.FolderID == nil || *bar.FolderID != parent.ID {
			t.Errorf("expected 'foo/bar' to reference parent 'foo'")
		}
		if bar.ContentType != "page" {
			t.Errorf("expected folder 'foo/bar' to keep content type 'page'")
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

func TestShouldPublishByPlanLevel(t *testing.T) {
	m := InitialModel()
	m.targetSpace = &sb.Space{PlanLevel: 999}
	if m.shouldPublish() {
		t.Error("expected publish to be false for plan level 999")
	}
	m.targetSpace.PlanLevel = 100
	if !m.shouldPublish() {
		t.Error("expected publish to be true for plan level 100")
	}
}
