package ui

import (
    "encoding/json"
    "testing"

    "storyblok-sync/internal/sb"
)

func TestReportCreationAndSaving(t *testing.T) {
	// Create a new report
	report := NewReport("Source Space (123)", "Target Space (456)")

	// Add various types of entries
    sourceStory := &sb.Story{
		ID:       1,
		Name:     "Test Story",
		Slug:     "test-story",
		FullSlug: "folder/test-story",
        Content:  json.RawMessage([]byte(`{"title":"Test Title"}`)),
		UUID:     "test-uuid-123",
	}

    targetStory := &sb.Story{
		ID:       2,
		Name:     "Test Story",
		Slug:     "test-story",
		FullSlug: "folder/test-story",
        Content:  json.RawMessage([]byte(`{"title":"Test Title"}`)),
		UUID:     "test-uuid-123",
	}

	// Add success entry
	report.AddSuccess("folder/test-story", "create", 1500, targetStory)

	// Add warning entry
	report.AddWarning("folder/warning-story", "update", "UUID update failed", 2000, sourceStory, targetStory)

	// Add error entry
	report.AddError("folder/error-story", "create", "API connection failed", 500, sourceStory)

	// Test display summary
	summary := report.GetDisplaySummary()
	expected := "1 Erfolge, 1 Warnungen, 1 Fehler"
	if summary != expected {
		t.Errorf("Expected display summary '%s', got '%s'", expected, summary)
	}

	// Test finalization
	report.Finalize()

	// Check summary calculations
	if report.Summary.Total != 3 {
		t.Errorf("Expected total 3, got %d", report.Summary.Total)
	}
	if report.Summary.Success != 1 {
		t.Errorf("Expected success 1, got %d", report.Summary.Success)
	}
	if report.Summary.Warning != 1 {
		t.Errorf("Expected warning 1, got %d", report.Summary.Warning)
	}
	if report.Summary.Failure != 1 {
		t.Errorf("Expected failure 1, got %d", report.Summary.Failure)
	}
	if report.Summary.Created != 2 { // create + create (error)
		t.Errorf("Expected created 2, got %d", report.Summary.Created)
	}
	if report.Summary.Updated != 1 {
		t.Errorf("Expected updated 1, got %d", report.Summary.Updated)
	}

	// Save report
	err := report.Save()
	if err != nil {
		t.Errorf("Failed to save report: %v", err)
	}

	// Clean up - remove the generated file
	// We can't predict the exact filename due to timestamp, so we'll skip cleanup in test
}

func TestReportJSONFormat(t *testing.T) {
	report := NewReport("Source (123)", "Target (456)")

    sourceStory := &sb.Story{
		ID:       1,
		Name:     "Test Story",
		FullSlug: "test-story",
        Content:  json.RawMessage([]byte(`{"component":"story","title":"Test"}`)),
		UUID:     "story-uuid-123",
	}

	// Add error with complete source story
	report.AddError("test-story", "create", "Network timeout", 3000, sourceStory)

	report.Finalize()

	// Marshal to JSON to test the format
	jsonData, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal report to JSON: %v", err)
	}

	// Parse back to verify structure
	var parsed map[string]interface{}
	err = json.Unmarshal(jsonData, &parsed)
	if err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	// Verify key fields are present
	if parsed["source_space"] != "Source (123)" {
		t.Error("Source space not properly set")
	}
	if parsed["target_space"] != "Target (456)" {
		t.Error("Target space not properly set")
	}

	entries, ok := parsed["entries"].([]interface{})
	if !ok || len(entries) != 1 {
		t.Fatalf("Expected 1 entry, got %v", entries)
	}

	entry := entries[0].(map[string]interface{})

	// Verify the error entry has complete source story
	if entry["slug"] != "test-story" {
		t.Error("Slug not properly set")
	}
	if entry["status"] != "failure" {
		t.Error("Status not properly set")
	}
	if entry["error"] != "Network timeout" {
		t.Error("Error not properly set")
	}

	// Most importantly - verify the complete source story is included
	sourceStoryData, ok := entry["source_story"].(map[string]interface{})
	if !ok {
		t.Fatal("Source story not included in error entry")
	}

	if sourceStoryData["uuid"] != "story-uuid-123" {
		t.Error("Source story UUID not preserved")
	}

	// Verify content is included
	content, ok := sourceStoryData["content"].(map[string]interface{})
	if !ok {
		t.Fatal("Source story content not included")
	}

	if content["title"] != "Test" {
		t.Error("Source story content not preserved")
	}
}

func TestBackwardCompatibility(t *testing.T) {
	report := NewReport("Test Source", "Test Target")

	// Add entries using new methods
	report.AddSuccess("story1", "create", 1000, nil)
	report.AddWarning("story2", "update", "minor issue", 1500, nil, nil)
	report.AddError("story3", "create", "major issue", 2000, nil)

	// Test old Counts method still works
	success, warning, failure := report.Counts()

	if success != 1 {
		t.Errorf("Expected 1 success, got %d", success)
	}
	if warning != 1 {
		t.Errorf("Expected 1 warning, got %d", warning)
	}
	if failure != 1 {
		t.Errorf("Expected 1 failure, got %d", failure)
	}
}
