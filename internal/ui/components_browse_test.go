package ui

import (
    "testing"
    "storyblok-sync/internal/sb"
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
    if len(lines) < 3 { t.Fatalf("expected group + 2 items") }
    if lines[1] == lines[2] {
        t.Fatalf("lines identical; invalid test setup")
    }
    // With desc updated, item with UpdatedAt=2025-09-04 (A) should appear before B
    if got := containsInOrder(lines, " A", " B"); !got {
        t.Fatalf("expected A before B in lines: %v", lines)
    }
    // Apply cutoff after 2025-09-03: should show only A
    m.comp.dateCutoff = parseTime("2025-09-03")
    lines = m.visibleCompLines()
    // Expect group + 1 item
    cnt := 0
    for _, l := range lines { if l != "" && l[0] != '!' { cnt++ } }
    if len(lines) < 2 { t.Fatalf("expected at least 2 lines, got %v", lines) }
}

func containsInOrder(lines []string, a, b string) bool {
    ai, bi := -1, -1
    for i, l := range lines {
        if ai == -1 && contains(l, a) { ai = i }
        if bi == -1 && contains(l, b) { bi = i }
    }
    return ai >=0 && bi >=0 && ai < bi
}

func contains(s, sub string) bool { return len(s) >= len(sub) && (stringIndex(s, sub) >= 0) }
func stringIndex(s, sub string) int { return indexOf(s, sub) }
func indexOf(s, sub string) int {
    for i := 0; i+len(sub) <= len(s); i++ {
        if s[i:i+len(sub)] == sub { return i }
    }
    return -1
}

