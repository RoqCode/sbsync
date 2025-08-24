package ui

import (
	"encoding/json"
	"os"
)

type ReportStatus string

const (
	ReportSuccess ReportStatus = "success"
	ReportFailure ReportStatus = "failure"
	ReportWarning ReportStatus = "warning"
)

type ReportEntry struct {
	Slug   string       `json:"slug"`
	Status ReportStatus `json:"status"`
	Error  string       `json:"error,omitempty"`
}

type Report struct {
	Entries []ReportEntry `json:"entries"`
}

func (r *Report) AddSuccess(slug string) {
	r.Entries = append(r.Entries, ReportEntry{Slug: slug, Status: ReportSuccess})
}

func (r *Report) AddFailure(slug string, err error) {
	r.Entries = append(r.Entries, ReportEntry{Slug: slug, Status: ReportFailure, Error: err.Error()})
}

func (r *Report) Counts() (successes, warnings, failures int) {
	for _, e := range r.Entries {
		switch e.Status {
		case ReportSuccess:
			successes++
		case ReportWarning:
			warnings++
		case ReportFailure:
			failures++
		}
	}
	return
}

func (r Report) Dump(path string) error {
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
