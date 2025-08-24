package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"storyblok-sync/internal/sb"
)

// TestHandleWelcomeKey verifies state transitions from the welcome screen.
func TestHandleWelcomeKey(t *testing.T) {
	m := Model{state: stateWelcome}

	// Without token the handler should switch to the token prompt.
	m.cfg.Token = ""
	var cmd tea.Cmd
	m, cmd = m.handleWelcomeKey("enter")
	if m.state != stateTokenPrompt {
		t.Fatalf("expected stateTokenPrompt, got %v", m.state)
	}
	if cmd != nil {
		t.Fatalf("expected nil cmd")
	}

	// With a token we expect validation to start.
	m = Model{state: stateWelcome}
	m.cfg.Token = "abc"
	m, cmd = m.handleWelcomeKey("enter")
	if m.state != stateValidating {
		t.Fatalf("expected stateValidating, got %v", m.state)
	}
	if cmd == nil {
		t.Fatalf("expected non-nil cmd")
	}
}

// TestUpdateGlobalQuit ensures that global quit keys are handled before state handlers.
func TestUpdateGlobalQuit(t *testing.T) {
	m := Model{state: stateWelcome}
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd == nil {
		t.Fatalf("expected quit cmd")
	}
	if msg := cmd(); msg == nil {
		t.Fatalf("expected quit msg")
	} else if _, ok := msg.(tea.QuitMsg); !ok {
		t.Fatalf("expected tea.QuitMsg, got %T", msg)
	}
}

// TestTreeFolding ensures that folders can be expanded and collapsed via key bindings.
func TestTreeFolding(t *testing.T) {
	folderID := 1
	stories := []sb.Story{
		{ID: folderID, Name: "folder", FullSlug: "folder", IsFolder: true},
		{ID: 2, Name: "child", FullSlug: "folder/child", FolderID: &folderID},
	}
	m := Model{}
	m.storiesSource = stories
	m.applyFilter()

	if l := m.itemsLen(); l != 1 {
		t.Fatalf("expected 1 visible item, got %d", l)
	}

	m.expandAll()
	if l := m.itemsLen(); l != 2 {
		t.Fatalf("expected 2 items after expandAll, got %d", l)
	}

	m.selection.listIndex = 1
	m, _ = m.handleBrowseListKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	if l := m.itemsLen(); l != 1 {
		t.Fatalf("expected 1 item after collapsing parent, got %d", l)
	}
	if m.selection.listIndex != 0 {
		t.Fatalf("expected cursor at parent, got %d", m.selection.listIndex)
	}
}
