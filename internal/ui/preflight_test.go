package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"storyblok-sync/internal/sb"
)

func TestStartPreflightDetectsCollisions(t *testing.T) {
	st1 := sb.Story{ID: 1, Name: "one", Slug: "one", FullSlug: "one"}
	st2 := sb.Story{ID: 2, Name: "two", Slug: "two", FullSlug: "two"}
	tgt := sb.Story{ID: 3, Name: "one", Slug: "one", FullSlug: "one"}
	m := InitialModel()
	m.storiesSource = []sb.Story{st1, st2}
	m.storiesTarget = []sb.Story{tgt}
	m.rebuildStoryIndex()
	m.applyFilter()
	if m.selection.selected == nil {
		m.selection.selected = make(map[string]bool)
	}
	m.selection.selected[st1.FullSlug] = true

	m.startPreflight()
	if m.state != statePreflight {
		t.Fatalf("expected statePreflight, got %v", m.state)
	}
	if len(m.preflight.items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(m.preflight.items))
	}
	if !m.preflight.items[0].Collision {
		t.Fatalf("expected collision for first item")
	}
}

func TestPreflightSkipToggleAndGlobal(t *testing.T) {
	st := sb.Story{ID: 1, Name: "one", Slug: "one", FullSlug: "one"}
	tgt := sb.Story{ID: 2, Name: "one", Slug: "one", FullSlug: "one"}
	m := InitialModel()
	m.storiesSource = []sb.Story{st}
	m.storiesTarget = []sb.Story{tgt}
	m.rebuildStoryIndex()
	m.applyFilter()
	if m.selection.selected == nil {
		m.selection.selected = make(map[string]bool)
	}
	m.selection.selected[st.FullSlug] = true
	m.startPreflight()

	m, _ = m.handlePreflightKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	if !m.preflight.items[0].Skip {
		t.Fatalf("expected item skipped after x")
	}
	m.preflight.items[0].Skip = false
	m, _ = m.handlePreflightKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'X'}})
	if !m.preflight.items[0].Skip {
		t.Fatalf("expected item skipped after X")
	}
	m, _ = m.handlePreflightKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if m.state != stateBrowseList {
		t.Fatalf("expected return to browse list on q")
	}
}
