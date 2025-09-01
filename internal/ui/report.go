package ui

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"time"

	"storyblok-sync/internal/sb"
)

// ReportEntry captures the result of a single sync item with comprehensive details.
type ReportEntry struct {
	Slug        string    `json:"slug"`
	Status      string    `json:"status"`              // success|warning|failure
	Operation   string    `json:"operation,omitempty"` // create|update|skip
	Error       string    `json:"error,omitempty"`
	Warning     string    `json:"warning,omitempty"`
	Duration    int64     `json:"duration_ms,omitempty"`  // Duration in milliseconds
	Story       *sb.Story `json:"source_story,omitempty"` // Complete source story for errors/warnings
	TargetStory *sb.Story `json:"target_story,omitempty"` // Target story if created/updated
	// Rate limit related counters (deltas captured per item)
	RateLimit429 int `json:"rate_limit_429,omitempty"`
}

// Report collects all entries and provides comprehensive sync reporting.
type Report struct {
	StartTime   time.Time     `json:"start_time"`
	EndTime     time.Time     `json:"end_time,omitempty"`
	Duration    int64         `json:"total_duration_ms,omitempty"`
	SourceSpace string        `json:"source_space,omitempty"`
	TargetSpace string        `json:"target_space,omitempty"`
	Entries     []ReportEntry `json:"entries"`
	Summary     ReportSummary `json:"summary"`
}

// ReportSummary provides aggregate statistics
type ReportSummary struct {
	Total   int `json:"total"`
	Success int `json:"success"`
	Warning int `json:"warning"`
	Failure int `json:"failure"`
	Created int `json:"created"`
	Updated int `json:"updated"`
	Skipped int `json:"skipped"`
}

// NewReport creates a new report with initial metadata
func NewReport(sourceSpace, targetSpace string) *Report {
	return &Report{
		StartTime:   time.Now(),
		SourceSpace: sourceSpace,
		TargetSpace: targetSpace,
		Entries:     make([]ReportEntry, 0),
	}
}

// Add adds an entry to the report
func (r *Report) Add(e ReportEntry) {
	r.Entries = append(r.Entries, e)
}

// AddSuccess adds a successful sync entry
func (r *Report) AddSuccess(slug, operation string, duration int64, targetStory *sb.Story) {
	r.Add(ReportEntry{
		Slug:        slug,
		Status:      "success",
		Operation:   operation,
		Duration:    duration,
		TargetStory: targetStory,
	})
}

// AddWarning adds a warning entry with complete source story
func (r *Report) AddWarning(slug, operation, warning string, duration int64, sourceStory, targetStory *sb.Story) {
	r.Add(ReportEntry{
		Slug:        slug,
		Status:      "warning",
		Operation:   operation,
		Warning:     warning,
		Duration:    duration,
		Story:       sourceStory,
		TargetStory: targetStory,
	})
}

// AddError adds an error entry with complete source story
func (r *Report) AddError(slug, operation, error string, duration int64, sourceStory *sb.Story) {
	r.Add(ReportEntry{
		Slug:      slug,
		Status:    "failure",
		Operation: operation,
		Error:     error,
		Duration:  duration,
		Story:     sourceStory,
	})
}

// Finalize calculates final statistics and duration
func (r *Report) Finalize() {
	r.EndTime = time.Now()
	r.Duration = r.EndTime.Sub(r.StartTime).Milliseconds()
	r.calculateSummary()
}

// calculateSummary computes the report summary statistics
func (r *Report) calculateSummary() {
	summary := ReportSummary{}

	for _, e := range r.Entries {
		summary.Total++

		switch e.Status {
		case "success":
			summary.Success++
		case "warning":
			summary.Warning++
		case "failure":
			summary.Failure++
		}

		switch e.Operation {
		case "create":
			summary.Created++
		case "update":
			summary.Updated++
		case "skip":
			summary.Skipped++
		}
	}

	r.Summary = summary
}

// Counts returns the count of success, warning, and failure entries (for backward compatibility)
func (r *Report) Counts() (success, warning, failure int) {
	for _, e := range r.Entries {
		switch e.Status {
		case "success":
			success++
		case "warning":
			warning++
		case "failure":
			failure++
		}
	}
	return
}

// GetDisplaySummary returns a German summary string for UI display
func (r *Report) GetDisplaySummary() string {
	r.calculateSummary()
	return fmt.Sprintf("%d Erfolge, %d Warnungen, %d Fehler",
		r.Summary.Success, r.Summary.Warning, r.Summary.Failure)
}

// Save writes the comprehensive report to a JSON file in the current directory.
// It also performs cleanup of old report files to prevent disk space accumulation.
func (r *Report) Save() error {
	r.Finalize()

	if len(r.Entries) == 0 {
		return nil
	}

	// Clean up old report files before creating a new one
	if err := r.cleanupOldReports(); err != nil {
		// Log error but don't fail the save operation
		log.Printf("Warning: failed to cleanup old reports: %v", err)
	}

	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}

	filename := fmt.Sprintf("sync-report-%s.json", time.Now().Format("20060102-150405"))
	return os.WriteFile(filename, data, 0o644)
}

// cleanupOldReports removes old report files, keeping only the most recent 10 files
func (r *Report) cleanupOldReports() error {
	files, err := filepath.Glob("sync-report-*.json")
	if err != nil {
		return fmt.Errorf("failed to find report files: %w", err)
	}

	// Keep only the most recent 10 reports
	if len(files) <= 10 {
		return nil
	}

	// Sort files by name (which includes timestamp, so this sorts by date)
	sort.Strings(files)

	// Remove the oldest files, keeping the last 10
	filesToRemove := files[:len(files)-10]
	for _, file := range filesToRemove {
		if err := os.Remove(file); err != nil {
			log.Printf("Warning: failed to remove old report file %s: %v", file, err)
		}
	}

	if len(filesToRemove) > 0 {
		log.Printf("Cleaned up %d old report files", len(filesToRemove))
	}

	return nil
}
