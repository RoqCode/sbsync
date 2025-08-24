package ui

import (
	"errors"
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
