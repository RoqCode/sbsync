package ui

import (
	"encoding/json"
	"os"
	"time"
)

// ReportEntry captures the result of a single sync item.
type ReportEntry struct {
	Slug   string `json:"slug"`
	Status string `json:"status"` // success|warning|failure
	Error  string `json:"error,omitempty"`
}

// Report collects all entries and can persist them.
type Report struct {
	Entries []ReportEntry `json:"entries"`
}

func (r *Report) Add(e ReportEntry) {
	r.Entries = append(r.Entries, e)
}

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

// Save writes the report to a JSON file in the current directory.
func (r *Report) Save() error {
	if len(r.Entries) == 0 {
		return nil
	}
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	name := "sync-report-" + time.Now().Format("20060102-150405") + ".json"
	return os.WriteFile(name, data, 0o644)
}
