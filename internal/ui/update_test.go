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

func TestFolderFoldingNavigation(t *testing.T) {
	folderID := 1
	childID := 2
	folder := sb.Story{ID: folderID, Name: "folder", Slug: "folder", FullSlug: "folder", IsFolder: true}
	folderIDPtr := folderID
	child := sb.Story{ID: childID, Name: "child", Slug: "child", FullSlug: "folder/child", FolderID: &folderIDPtr}

	m := InitialModel()
	m.storiesSource = []sb.Story{folder, child}
	m.rebuildStoryIndex()
	m.applyFilter()

	if got := m.itemsLen(); got != 1 {
		t.Fatalf("expected 1 item, got %d", got)
	}

	m, _ = m.handleBrowseListKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	if got := m.itemsLen(); got != 2 {
		t.Fatalf("expected 2 items after expand, got %d", got)
	}

	m.selection.listIndex = 1
	m, _ = m.handleBrowseListKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	if got := m.itemsLen(); got != 1 {
		t.Fatalf("expected 1 item after collapse, got %d", got)
	}
	if m.selection.listIndex != 0 {
		t.Fatalf("expected cursor on parent, got %d", m.selection.listIndex)
	}
}

func TestSearchExpandsAndCollapses(t *testing.T) {
	folderID := 1
	childID := 2
	folder := sb.Story{ID: folderID, Name: "folder", Slug: "folder", FullSlug: "folder", IsFolder: true}
	folderIDPtr := folderID
	child := sb.Story{ID: childID, Name: "child", Slug: "child", FullSlug: "folder/child", FolderID: &folderIDPtr}

	m := InitialModel()
	m.storiesSource = []sb.Story{folder, child}
	m.rebuildStoryIndex()
	m.applyFilter()

	if !m.folderCollapsed[folderID] {
		t.Fatalf("expected folder collapsed initially")
	}

	m.search.searching = true
	m.search.searchInput.Focus()
	m, _ = m.handleBrowseListKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	if m.folderCollapsed[folderID] {
		t.Fatalf("expected folder expanded after typing")
	}

	m.search.searching = false
	m.search.query = "a"
	m, _ = m.handleBrowseListKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'F'}})
	if !m.folderCollapsed[folderID] {
		t.Fatalf("expected folder collapsed after clearing search")
	}
}
