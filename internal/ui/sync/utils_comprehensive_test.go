package sync

import (
	"encoding/json"
	"log"
	"os"
	"strings"
	"testing"

	"storyblok-sync/internal/sb"
)

func TestGetContentKeys(t *testing.T) {
	tests := []struct {
		name     string
		content  json.RawMessage
		expected []string
	}{
		{
			name:     "empty content",
			content:  json.RawMessage([]byte{}),
			expected: nil,
		},
		{
			name:     "nil content",
			content:  nil,
			expected: nil,
		},
		{
			name:     "invalid json",
			content:  json.RawMessage([]byte("invalid")),
			expected: nil,
		},
		{
			name:     "simple object",
			content:  json.RawMessage([]byte(`{"component":"page","title":"Test"}`)),
			expected: []string{"component", "title"}, // Note: order may vary in maps
		},
		{
			name:     "nested object",
			content:  json.RawMessage([]byte(`{"component":"page","body":[{"text":"hello"}]}`)),
			expected: []string{"component", "body"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := GetContentKeys(test.content)

			if test.expected == nil {
				if result != nil {
					t.Errorf("Expected nil, got %v", result)
				}
				return
			}

			if len(result) != len(test.expected) {
				t.Errorf("Expected %d keys, got %d", len(test.expected), len(result))
				return
			}

			// Since map iteration order is not guaranteed, check that all expected keys are present
			resultMap := make(map[string]bool)
			for _, key := range result {
				resultMap[key] = true
			}

			for _, expectedKey := range test.expected {
				if !resultMap[expectedKey] {
					t.Errorf("Expected key %s not found in result", expectedKey)
				}
			}
		})
	}
}

func TestGetContentField(t *testing.T) {
	content := json.RawMessage([]byte(`{"component":"page","title":"Test","count":42,"active":true,"data":{"nested":"value"}}`))

	tests := []struct {
		key      string
		expected interface{}
		found    bool
	}{
		{"component", "page", true},
		{"title", "Test", true},
		{"count", float64(42), true}, // JSON numbers are parsed as float64
		{"active", true, true},
		{"data", map[string]interface{}{"nested": "value"}, true},
		{"nonexistent", nil, false},
	}

	for _, test := range tests {
		t.Run("field_"+test.key, func(t *testing.T) {
			result, found := GetContentField(content, test.key)

			if found != test.found {
				t.Errorf("Expected found=%t, got found=%t", test.found, found)
			}

			if test.found {
				switch expected := test.expected.(type) {
				case map[string]interface{}:
					resultMap, ok := result.(map[string]interface{})
					if !ok {
						t.Errorf("Expected map, got %T", result)
						return
					}
					if len(resultMap) != len(expected) {
						t.Errorf("Map length mismatch: expected %d, got %d", len(expected), len(resultMap))
					}
					for k, v := range expected {
						if resultMap[k] != v {
							t.Errorf("Map key %s: expected %v, got %v", k, v, resultMap[k])
						}
					}
				default:
					if result != expected {
						t.Errorf("Expected %v (%T), got %v (%T)", expected, expected, result, result)
					}
				}
			}
		})
	}

	// Test edge cases
	t.Run("empty_content", func(t *testing.T) {
		result, found := GetContentField(json.RawMessage([]byte{}), "test")
		if found {
			t.Error("Expected not found for empty content")
		}
		if result != nil {
			t.Errorf("Expected nil result, got %v", result)
		}
	})

	t.Run("invalid_json", func(t *testing.T) {
		result, found := GetContentField(json.RawMessage([]byte("invalid")), "test")
		if found {
			t.Error("Expected not found for invalid JSON")
		}
		if result != nil {
			t.Errorf("Expected nil result, got %v", result)
		}
	})
}

// logCapture helps capture log output for testing
type logCapture struct {
	logs []string
}

func (lc *logCapture) Write(p []byte) (n int, err error) {
	lc.logs = append(lc.logs, string(p))
	return len(p), nil
}

func (lc *logCapture) getLogs() string {
	return strings.Join(lc.logs, "")
}

func TestLogError(t *testing.T) {
	// Capture log output
	capture := &logCapture{}
	log.SetOutput(capture)
	defer log.SetOutput(os.Stderr) // Restore original output

	// Test with nil story
	LogError("create", "test/story", &testError{message: "test error"}, nil)
	logs := capture.getLogs()
	if !strings.Contains(logs, "ERROR: create failed for test/story: test error") {
		t.Error("Expected error log not found")
	}

	// Reset capture
	capture.logs = nil

	// Test with full story
	story := &sb.Story{
		ID:       123,
		UUID:     "test-uuid",
		Name:     "Test Story",
		FullSlug: "test/story",
		IsFolder: false,
		Published: true,
		FolderID: &[]int{456}[0],
		TagList:  []string{"tag1", "tag2"},
		TranslatedSlugs: []sb.TranslatedSlug{
			{Lang: "en", Name: "Test Story", Path: "test/story"},
			{Lang: "de", Name: "Test Geschichte", Path: "test/geschichte"},
		},
		Content: json.RawMessage([]byte(`{"component":"page","title":"Test"}`)),
	}

	LogError("update", "test/story", &testError{message: "update failed"}, story)
	logs = capture.getLogs()

	expectedParts := []string{
		"ERROR: update failed for test/story: update failed",
		"ERROR CONTEXT for test/story:",
		"Story ID: 123",
		"Story UUID: test-uuid",
		"Story Name: Test Story",
		"Full Slug: test/story",
		"Is Folder: false",
		"Published: true",
		"Parent ID: 456",
		"Tags: [tag1 tag2]",
		"Translated Slugs: 2 entries",
		"en: Test Story (test/story)",
		"de: Test Geschichte (test/geschichte)",
		"Content Keys:",
		"Component Type: page",
		"Full Story JSON:",
	}

	for _, part := range expectedParts {
		if !strings.Contains(logs, part) {
			t.Errorf("Expected log part not found: %s", part)
		}
	}
}

func TestLogWarning(t *testing.T) {
	capture := &logCapture{}
	log.SetOutput(capture)
	defer log.SetOutput(os.Stderr)

	story := &sb.Story{
		ID:       123,
		UUID:     "test-uuid",
		FullSlug: "test/story",
		FolderID: &[]int{456}[0],
	}

	LogWarning("create", "test/story", "test warning", story)
	logs := capture.getLogs()

	expectedParts := []string{
		"WARNING: create for test/story: test warning",
		"WARNING CONTEXT for test/story:",
		"Story ID: 123 (UUID: test-uuid)",
		"Full Slug: test/story",
		"Parent ID: 456",
	}

	for _, part := range expectedParts {
		if !strings.Contains(logs, part) {
			t.Errorf("Expected log part not found: %s", part)
		}
	}
}

func TestLogSuccess(t *testing.T) {
	capture := &logCapture{}
	log.SetOutput(capture)
	defer log.SetOutput(os.Stderr)

	story := &sb.Story{
		ID:        123,
		UUID:      "test-uuid",
		FullSlug:  "test/story",
		FolderID:  &[]int{456}[0],
		Published: true,
	}

	LogSuccess("create", "test/story", 1500, story)
	logs := capture.getLogs()

	expectedParts := []string{
		"SUCCESS: create completed for test/story in 1500ms",
		"SUCCESS CONTEXT for test/story:",
		"Created/Updated Story ID: 123 (UUID: test-uuid)",
		"Parent ID: 456",
		"Published: true",
	}

	for _, part := range expectedParts {
		if !strings.Contains(logs, part) {
			t.Errorf("Expected log part not found: %s", part)
		}
	}
}

func TestLogExtendedErrorContext(t *testing.T) {
	capture := &logCapture{}
	log.SetOutput(capture)
	defer log.SetOutput(os.Stderr)

	tests := []struct {
		error    string
		expected string
	}{
		{"HTTP status 401", "Authentication/Authorization Error"},
		{"HTTP status 403", "Authentication/Authorization Error"},
		{"HTTP status 404", "Resource Not Found"},
		{"HTTP status 429", "Rate Limited"},
		{"HTTP status 500", "Server Error"},
		{"HTTP status 502", "Server Error"},
		{"HTTP status 503", "Server Error"},
		{"connection timeout", "Timeout Error"},
		{"general error", ""}, // Should not match any pattern
	}

	for _, test := range tests {
		t.Run(test.error, func(t *testing.T) {
			capture.logs = nil // Reset logs
			logExtendedErrorContext(&testError{message: test.error})
			logs := capture.getLogs()

			if test.expected == "" {
				// Should only have the basic error, no additional context
				if strings.Contains(logs, "Error Details") || strings.Contains(logs, "Timeout Error") {
					t.Errorf("Unexpected additional context for generic error: %s", logs)
				}
			} else {
				if !strings.Contains(logs, test.expected) {
					t.Errorf("Expected error context '%s' not found in logs: %s", test.expected, logs)
				}
			}
		})
	}

	// Test with nil error
	capture.logs = nil
	logExtendedErrorContext(nil)
	if len(capture.logs) > 0 {
		t.Error("Expected no logs for nil error")
	}
}

// testError is a simple error implementation for testing
type testError struct {
	message string
}

func (e *testError) Error() string {
	return e.message
}