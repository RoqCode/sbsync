package ui

import (
	"storyblok-sync/internal/sb"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
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

func TestMarkingFolderMarksDescendants(t *testing.T) {
	folderID := 1
	child1ID := 2
	child2ID := 3
	folder := sb.Story{ID: folderID, Name: "folder", Slug: "folder", FullSlug: "folder", IsFolder: true}
	folderPtr := folderID
	child1 := sb.Story{ID: child1ID, Name: "c1", Slug: "c1", FullSlug: "folder/c1", FolderID: &folderPtr}
	child2 := sb.Story{ID: child2ID, Name: "c2", Slug: "c2", FullSlug: "folder/c2", FolderID: &folderPtr}

	m := InitialModel()
	m.storiesSource = []sb.Story{folder, child1, child2}
	m.rebuildStoryIndex()
	m.applyFilter()

	m, _ = m.handleBrowseListKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})

	if !m.selection.selected[folder.FullSlug] || !m.selection.selected[child1.FullSlug] || !m.selection.selected[child2.FullSlug] {
		t.Fatalf("expected folder and children selected")
	}

	m, _ = m.handleBrowseListKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})

	if m.selection.selected[folder.FullSlug] || m.selection.selected[child1.FullSlug] || m.selection.selected[child2.FullSlug] {
		t.Fatalf("expected folder and children unselected after toggle")
	}
}

func TestPartialFolderMarkingIndicator(t *testing.T) {
	folderID := 1
	childID := 2
	folder := sb.Story{ID: folderID, Name: "folder", Slug: "folder", FullSlug: "folder", IsFolder: true}
	folderPtr := folderID
	child := sb.Story{ID: childID, Name: "child", Slug: "child", FullSlug: "folder/child", FolderID: &folderPtr}

	m := InitialModel()
	m.storiesSource = []sb.Story{folder, child}
	m.rebuildStoryIndex()
	m.applyFilter()
	if m.selection.selected == nil {
		m.selection.selected = make(map[string]bool)
	}
	m.selection.selected[child.FullSlug] = true
	m.refreshVisible()
	m.selection.listViewport = 10

	out := m.viewBrowseList()
	if !strings.Contains(out, markNestedStyle.Render(":")) {
		t.Fatalf("expected ':' marker for folder with selected child")
	}
}

func TestNestedFolderMarkingIndicator(t *testing.T) {
	rootID := 1
	subID := 2
	childID := 3
	root := sb.Story{ID: rootID, Name: "root", Slug: "root", FullSlug: "root", IsFolder: true}
	rootPtr := rootID
	sub := sb.Story{ID: subID, Name: "sub", Slug: "sub", FullSlug: "root/sub", IsFolder: true, FolderID: &rootPtr}
	subPtr := subID
	child := sb.Story{ID: childID, Name: "child", Slug: "child", FullSlug: "root/sub/child", FolderID: &subPtr}

	m := InitialModel()
	m.storiesSource = []sb.Story{root, sub, child}
	m.rebuildStoryIndex()
	m.applyFilter()
	if m.selection.selected == nil {
		m.selection.selected = make(map[string]bool)
	}
	m.selection.selected[child.FullSlug] = true
	m.refreshVisible()
	m.selection.listViewport = 10

	out := m.viewBrowseList()
	if !strings.Contains(out, markNestedStyle.Render("·")) {
		t.Fatalf("expected '·' marker for folder with selected descendant")
	}
}

func TestMarkMovesCursorDown(t *testing.T) {
	st1 := sb.Story{ID: 1, Name: "one", Slug: "one", FullSlug: "one"}
	st2 := sb.Story{ID: 2, Name: "two", Slug: "two", FullSlug: "two"}

	m := InitialModel()
	m.storiesSource = []sb.Story{st1, st2}
	m.rebuildStoryIndex()
	m.applyFilter()

	if m.selection.listIndex != 0 {
		t.Fatalf("expected cursor at 0, got %d", m.selection.listIndex)
	}

	m, _ = m.handleBrowseListKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})

	if !m.selection.selected[st1.FullSlug] {
		t.Fatalf("expected first story selected")
	}
	if m.selection.listIndex != 1 {
		t.Fatalf("expected cursor moved to 1, got %d", m.selection.listIndex)
	}
}

func TestBuildSyncPlanDetectsCollisions(t *testing.T) {
	st1 := sb.Story{ID: 1, Name: "one", Slug: "one", FullSlug: "one"}
	st2 := sb.Story{ID: 2, Name: "two", Slug: "two", FullSlug: "two"}
	tgt := sb.Story{ID: 3, Name: "two", Slug: "two", FullSlug: "two"}

	m := InitialModel()
	m.storiesSource = []sb.Story{st1, st2}
	m.storiesTarget = []sb.Story{tgt}
	m.selection.selected = map[string]bool{st1.FullSlug: true, st2.FullSlug: true}

	plan := m.buildSyncPlan()
	if len(plan.Items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(plan.Items))
	}
	coll := 0
	for _, it := range plan.Items {
		if it.Collision {
			coll++
			if it.Story.FullSlug != st2.FullSlug {
				t.Fatalf("collision flagged wrong item")
			}
		}
	}
	if coll != 1 {
		t.Fatalf("expected 1 collision, got %d", coll)
	}
}

func TestPreflightKeyHandlers(t *testing.T) {
	st1 := sb.Story{ID: 1, Slug: "one", FullSlug: "one"}
	st2 := sb.Story{ID: 2, Slug: "two", FullSlug: "two"}
	m := InitialModel()
	m.preflight.plan = SyncPlan{Items: []SyncPlanItem{{Story: st1}, {Story: st2, Collision: true}}}
	m.state = statePreflight

	// mark all collisions
	m, _ = m.handlePreflightKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	if !m.preflight.plan.Items[1].Skip {
		t.Fatalf("expected second item skipped")
	}
	if m.preflight.plan.Items[0].Skip {
		t.Fatalf("expected first item unaffected")
	}

	// toggle skip on first item
	m.preflight.listIndex = 0
	m, _ = m.handlePreflightKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	if !m.preflight.plan.Items[0].Skip {
		t.Fatalf("expected first item toggled to skip")
	}

	// quit back to browse list
	m, _ = m.handlePreflightKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if m.state != stateBrowseList {
		t.Fatalf("expected stateBrowseList, got %v", m.state)
	}
}
