package ui

import (
	"storyblok-sync/internal/sb"
	"testing"
)

func TestVisibleCompLines_SortAndCutoff(t *testing.T) {
	m := InitialModel()
	m.currentMode = modeComponents
	m.componentGroupsSource = []sb.ComponentGroup{{UUID: "g1", Name: "Alpha"}}
	m.componentsSource = []sb.Component{
		{ID: 1, Name: "B", ComponentGroupUUID: "g1", CreatedAt: "2025-09-01", UpdatedAt: "2025-09-02"},
		{ID: 2, Name: "A", ComponentGroupUUID: "g1", CreatedAt: "2025-09-03", UpdatedAt: "2025-09-04"},
	}
	// default sort updated desc
	m.comp.sortKey = compSortUpdated
	m.comp.sortAsc = false
	lines := m.visibleCompLines()
	if len(lines) < 2 {
		t.Fatalf("expected 2 items, got %d", len(lines))
	}
	// With desc updated, item A (2025-09-04) should appear before B
	if !(contains(lines[0], " A") && contains(lines[1], " B")) {
		t.Fatalf("expected A before B in lines: %v", lines)
	}
	// Apply cutoff after 2025-09-03: should show only A
	m.comp.dateCutoff = parseTime("2025-09-03")
	lines = m.visibleCompLines()
	if len(lines) != 1 || !contains(lines[0], " A") {
		t.Fatalf("expected only A after cutoff, got %v", lines)
	}
}

func contains(s, sub string) bool   { return len(s) >= len(sub) && (stringIndex(s, sub) >= 0) }
func stringIndex(s, sub string) int { return indexOf(s, sub) }
func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
