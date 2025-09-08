package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"storyblok-sync/internal/sb"
)

// local helper to produce tea.KeyMsg
func compKey(key string) tea.KeyMsg {
	switch key {
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEscape}
	case "up":
		return tea.KeyMsg{Type: tea.KeyUp}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	default:
		if len(key) == 1 {
			return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{rune(key[0])}}
		}
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)}
	}
}

func TestCompSearchInput_ApplyAndClear(t *testing.T) {
	m := InitialModel()
	m.state = stateCompList
	m.componentsSource = []sb.Component{
		{ID: 1, Name: "Alpha"},
		{ID: 2, Name: "Beta"},
	}

	// Enter search input mode
	m, _ = m.handleCompListKey(compKey("f"))
	if m.comp.inputMode != "search" {
		t.Fatalf("expected inputMode=search, got %q", m.comp.inputMode)
	}
	// Type and apply
	m.comp.search.input.SetValue("al")
	m, _ = m.handleCompListKey(compKey("enter"))
	if m.comp.inputMode != "" {
		t.Fatalf("expected inputMode cleared, got %q", m.comp.inputMode)
	}
	if m.comp.search.query != "al" {
		t.Fatalf("expected query 'al', got %q", m.comp.search.query)
	}
	lines := m.visibleCompLines()
	if len(lines) != 1 || indexOf(lines[0], " Alpha") < 0 {
		t.Fatalf("expected only Alpha after search, got %v", lines)
	}
	// Clear with 'F'
	m, _ = m.handleCompListKey(compKey("F"))
	if m.comp.search.query != "" {
		t.Fatalf("expected cleared query, got %q", m.comp.search.query)
	}
	lines = m.visibleCompLines()
	if len(lines) < 2 {
		t.Fatalf("expected 2 lines after clearing, got %d", len(lines))
	}
}

func TestCompSearchInput_EscKeepsPrevious(t *testing.T) {
	m := InitialModel()
	m.state = stateCompList
	m.componentsSource = []sb.Component{{ID: 1, Name: "A"}, {ID: 2, Name: "B"}}
	// Set previous query
	m.comp.search.query = "B"
	// Enter input, change value, then ESC should keep previous query
	m, _ = m.handleCompListKey(compKey("f"))
	m.comp.search.input.SetValue("A")
	m, _ = m.handleCompListKey(compKey("esc"))
	if m.comp.inputMode != "" {
		t.Fatalf("expected inputMode cleared on esc")
	}
	if m.comp.search.query != "B" {
		t.Fatalf("expected query to remain 'B', got %q", m.comp.search.query)
	}
}
