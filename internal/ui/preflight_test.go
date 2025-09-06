package ui

import (
	"fmt"
	"strings"
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
	if m.preflight.items[0].State != StateUpdate {
		t.Fatalf("expected state update, got %v", m.preflight.items[0].State)
	}
}

func TestPreflightMarksUnselectedFoldersSkipped(t *testing.T) {
	parent := sb.Story{ID: 1, Name: "app", Slug: "app", FullSlug: "app", IsFolder: true}
	child := sb.Story{ID: 2, Name: "child", Slug: "child", FullSlug: "app/child", FolderID: &parent.ID}
	tgt := sb.Story{ID: 3, Name: "app", Slug: "app", FullSlug: "app"}
	m := InitialModel()
	m.storiesSource = []sb.Story{parent, child}
	m.storiesTarget = []sb.Story{tgt}
	m.rebuildStoryIndex()
	m.applyFilter()
	if m.selection.selected == nil {
		m.selection.selected = make(map[string]bool)
	}
	m.selection.selected[child.FullSlug] = true

	m.startPreflight()

	if len(m.preflight.items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(m.preflight.items))
	}
	var folderItem *PreflightItem
	for i := range m.preflight.items {
		if m.preflight.items[i].Story.ID == parent.ID {
			folderItem = &m.preflight.items[i]
		}
	}
	if folderItem == nil {
		t.Fatalf("folder not found in preflight items")
	}
	if !folderItem.Collision {
		t.Fatalf("expected collision for unselected folder")
	}
	if !folderItem.Skip {
		t.Fatalf("expected unselected folder to be skipped")
	}
	if folderItem.State != StateSkip {
		t.Fatalf("expected state skip for unselected folder, got %v", folderItem.State)
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
	if m.preflight.items[0].State != StateSkip {
		t.Fatalf("expected state skip after x, got %v", m.preflight.items[0].State)
	}
	m.preflight.items[0].Skip = false
	m.preflight.items[0].State = StateUpdate
	m, _ = m.handlePreflightKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'X'}})
	if !m.preflight.items[0].Skip {
		t.Fatalf("expected item skipped after X")
	}
	if m.preflight.items[0].State != StateSkip {
		t.Fatalf("expected state skip after X, got %v", m.preflight.items[0].State)
	}
	m, _ = m.handlePreflightKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	if m.selection.selected[st.FullSlug] {
		t.Fatalf("expected selection removed after c")
	}
	if len(m.preflight.items) != 0 {
		t.Fatalf("expected preflight list cleared after c, got %d", len(m.preflight.items))
	}
	m, _ = m.handlePreflightKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if m.state != stateBrowseList {
		t.Fatalf("expected return to browse list on q")
	}
}

func TestDisplayPreflightItemDimsSlug(t *testing.T) {
	it := PreflightItem{Story: sb.Story{ID: 1, Name: "one", Slug: "one", FullSlug: "one"}}
	s := displayPreflightItem(it)
	expected := fmt.Sprintf("%s one  (one)", symbolStory)
	if s != expected {
		t.Fatalf("unexpected render for selected item: %q", s)
	}
	it.Selected = false
	s = displayPreflightItem(it)
	expected = fmt.Sprintf("%s %s  %s", symbolStory, subtleStyle.Render("one"), subtleStyle.Render("(one)"))
	if s != expected {
		t.Fatalf("expected dimmed slug and name when unselected: %q", s)
	}
	it.Selected = true
	it.Skip = true
	s = displayPreflightItem(it)
	if s != expected {
		t.Fatalf("expected dimmed slug and name when skipped: %q", s)
	}
}

func TestViewPreflightShowsStateCell(t *testing.T) {
	st := sb.Story{ID: 1, Name: "one", Slug: "one", FullSlug: "one"}
	m := InitialModel()
	m.storiesSource = []sb.Story{st}
	m.rebuildStoryIndex()
	m.applyFilter()
	if m.selection.selected == nil {
		m.selection.selected = make(map[string]bool)
	}
	m.selection.selected[st.FullSlug] = true
	m.startPreflight()

	// Use the new viewport-based render
	m.updatePreflightViewport()
	out := m.renderPreflightHeader() + "\n" + m.renderViewportContent() + "\n" + m.renderPreflightFooter()
	if !strings.Contains(out, stateStyles[StateCreate].Render(stateLabel(StateCreate))) {
		t.Fatalf("expected create state cell")
	}

	m.preflight.items[0].Skip = true
	recalcState(&m.preflight.items[0])
	m.updatePreflightViewport()
	out = m.renderPreflightHeader() + "\n" + m.renderViewportContent() + "\n" + m.renderPreflightFooter()
	if !strings.Contains(out, stateStyles[StateSkip].Render(stateLabel(StateSkip))) {
		t.Fatalf("expected skip state cell")
	}
}

func TestDefaultPublishModes(t *testing.T) {
	// One published source, one draft source
	stPub := sb.Story{ID: 1, Name: "one", Slug: "one", FullSlug: "one", Published: true}
	stDr := sb.Story{ID: 2, Name: "two", Slug: "two", FullSlug: "two", Published: false}
	m := InitialModel()
	m.storiesSource = []sb.Story{stPub, stDr}
	m.rebuildStoryIndex()
	if m.selection.selected == nil {
		m.selection.selected = make(map[string]bool)
	}
	m.selection.selected[stPub.FullSlug] = true
	m.selection.selected[stDr.FullSlug] = true
	m.startPreflight()
	if mode := m.getPublishMode(stPub.FullSlug); mode != PublishModePublish {
		t.Fatalf("expected published source default to Publish, got %s", mode)
	}
	if mode := m.getPublishMode(stDr.FullSlug); mode != PublishModeDraft {
		t.Fatalf("expected draft source default to Draft, got %s", mode)
	}
}

func TestPublishToggleCycleAndBadges(t *testing.T) {
	// Existing published target makes Publish&Changes valid
	st := sb.Story{ID: 1, Name: "one", Slug: "one", FullSlug: "one", Published: true}
	tgt := sb.Story{ID: 9, Name: "one", Slug: "one", FullSlug: "one", Published: true}
	m := InitialModel()
	m.storiesSource = []sb.Story{st}
	m.storiesTarget = []sb.Story{tgt}
	m.rebuildStoryIndex()
	if m.selection.selected == nil {
		m.selection.selected = make(map[string]bool)
	}
	m.selection.selected[st.FullSlug] = true
	m.startPreflight()

	// Focus first item and cycle p twice. Default for published source is Publish,
	// so after first 'p' we expect Publish&Changes, then back to Draft.
	m.preflight.listIndex = 0
	m, _ = m.handlePreflightKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	if mode := m.getPublishMode(st.FullSlug); mode != PublishModePublishChanges {
		t.Fatalf("expected mode PublishChanges after first p, got %s", mode)
	}
	// Render should contain [Pub+∆] after first toggle
	m.updatePreflightViewport()
	out := m.renderPreflightHeader() + "\n" + m.renderViewportContent()
	if !strings.Contains(out, "[Pub+∆]") {
		t.Fatalf("expected [Pub+∆] badge in preflight render")
	}
	// Second toggle to Draft
	m, _ = m.handlePreflightKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	if mode := m.getPublishMode(st.FullSlug); mode != PublishModeDraft {
		t.Fatalf("expected mode Draft after second p, got %s", mode)
	}
}

func TestPropagationP_OnFolderAndStory(t *testing.T) {
	parent := sb.Story{ID: 1, Name: "app", Slug: "app", FullSlug: "app", IsFolder: true}
	child1 := sb.Story{ID: 2, Name: "a", Slug: "a", FullSlug: "app/a", FolderID: &parent.ID}
	child2 := sb.Story{ID: 3, Name: "b", Slug: "b", FullSlug: "app/b", FolderID: &parent.ID}
	sibFolder := sb.Story{ID: 4, Name: "sf", Slug: "sf", FullSlug: "app/sf", IsFolder: true, FolderID: &parent.ID}
	sibStory := sb.Story{ID: 5, Name: "c", Slug: "c", FullSlug: "app/sf/c", FolderID: &sibFolder.ID}
	m := InitialModel()
	m.storiesSource = []sb.Story{parent, child1, child2, sibFolder, sibStory}
	m.rebuildStoryIndex()
	if m.selection.selected == nil {
		m.selection.selected = make(map[string]bool)
	}
	// Select all
	for _, s := range m.storiesSource {
		m.selection.selected[s.FullSlug] = true
	}
	m.startPreflight()

	// Set folder line index
	var folderListIdx int
	for i, vi := range m.preflight.visibleIdx {
		if m.preflight.items[vi].Story.ID == parent.ID {
			folderListIdx = i
			break
		}
	}
	m.preflight.listIndex = folderListIdx
	// Cycle mode on folder's first child to Publish
	m.setPublishMode(child1.FullSlug, PublishModePublish)
	// Propagate with P on folder: should set children and subtree
	m, _ = m.handlePreflightKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'P'}})
	if m.getPublishMode(child2.FullSlug) != PublishModePublish {
		t.Fatalf("expected child2 to inherit Publish from folder P")
	}
	if m.getPublishMode(sibStory.FullSlug) != PublishModePublish {
		t.Fatalf("expected sibStory under sibFolder to inherit Publish from folder P")
	}

	// Now focus on child1 and set to Draft; P should apply to siblings + sibling folder subtree
	// Find child1 visible index
	var childListIdx int
	for i, vi := range m.preflight.visibleIdx {
		if m.preflight.items[vi].Story.ID == child1.ID {
			childListIdx = i
			break
		}
	}
	m.preflight.listIndex = childListIdx
	m.setPublishMode(child1.FullSlug, PublishModeDraft)
	m, _ = m.handlePreflightKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'P'}})
	if m.getPublishMode(child2.FullSlug) != PublishModeDraft {
		t.Fatalf("expected child2 to inherit Draft from story P")
	}
	if m.getPublishMode(sibStory.FullSlug) != PublishModeDraft {
		t.Fatalf("expected sibStory to inherit Draft from story P")
	}
}

func TestOptimizePreflightDedupesFolders(t *testing.T) {
	folder := sb.Story{ID: 1, Name: "app", Slug: "app", FullSlug: "app", IsFolder: true}
	m := InitialModel()
	m.preflight.items = []PreflightItem{
		{Story: folder, Selected: true},
		{Story: folder, Selected: true},
	}
	m.optimizePreflight()
	if len(m.preflight.items) != 1 {
		t.Fatalf("expected 1 item after dedupe, got %d", len(m.preflight.items))
	}
}

func TestOptimizePreflight_FullFolderSelection(t *testing.T) {
	parent := sb.Story{ID: 1, Name: "app", Slug: "app", FullSlug: "app", IsFolder: true}
	child1 := sb.Story{ID: 2, Name: "one", Slug: "one", FullSlug: "app/one", FolderID: &parent.ID}
	child2 := sb.Story{ID: 3, Name: "two", Slug: "two", FullSlug: "app/two", FolderID: &parent.ID}
	m := InitialModel()
	m.storiesSource = []sb.Story{parent, child1, child2}
	m.rebuildStoryIndex()
	m.applyFilter()
	if m.selection.selected == nil {
		m.selection.selected = make(map[string]bool)
	}
	m.selection.selected[parent.FullSlug] = true
	m.selection.selected[child1.FullSlug] = true
	m.selection.selected[child2.FullSlug] = true
	m.startPreflight()
	m.optimizePreflight()
	if len(m.preflight.items) != 3 {
		t.Fatalf("expected 3 items after optimization, got %d", len(m.preflight.items))
	}
}

func TestEnterStartsSequentialWhenFoldersPresent(t *testing.T) {
	// Preflight contains a folder and a story; pressing enter should start only one worker (sequential)
	m := InitialModel()
	folder := sb.Story{ID: 1, Name: "app", Slug: "app", FullSlug: "app", IsFolder: true}
	story := sb.Story{ID: 2, Name: "p", Slug: "p", FullSlug: "app/p"}
	m.preflight.items = []PreflightItem{
		{Story: folder, Selected: true, Run: RunPending},
		{Story: story, Selected: true, Run: RunPending},
	}

	// simulate token so API client gets created
	m.cfg.Token = "test-token"
	// Press enter to begin sync
	m, _ = m.handlePreflightKey(tea.KeyMsg{Type: tea.KeyEnter})
	if m.state != stateSync {
		t.Fatalf("expected stateSync after enter")
	}
	// Exactly one item should be marked running due to sequential folder phase
	running := 0
	for _, it := range m.preflight.items {
		if it.Run == RunRunning {
			running++
		}
	}
	if running != 1 {
		t.Fatalf("expected exactly 1 item running at start when folders present, got %d", running)
	}
}

func TestCopyAsNewFlowStoryAddsForkBadge(t *testing.T) {
	// Set up a single collision story
	st := sb.Story{ID: 1, Name: "one", Slug: "one", FullSlug: "one"}
	tgt := sb.Story{ID: 9, Name: "one", Slug: "one", FullSlug: "one"}
	m := InitialModel()
	m.storiesSource = []sb.Story{st}
	m.storiesTarget = []sb.Story{tgt}
	m.rebuildStoryIndex()
	if m.selection.selected == nil {
		m.selection.selected = make(map[string]bool)
	}
	m.selection.selected[st.FullSlug] = true
	m.startPreflight()

	// Open copy-as-new view with 'f'
	m, _ = m.handlePreflightKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}})
	if m.state != stateCopyAsNew {
		t.Fatalf("expected copy-as-new state after f")
	}
	// Toggle name suffix and confirm
	m, _ = m.handleCopyAsNewKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	m, _ = m.handleCopyAsNewKey(tea.KeyMsg{Type: tea.KeyEnter})

	if m.state != statePreflight {
		t.Fatalf("expected return to preflight after confirm")
	}
	if !m.preflight.items[0].CopyAsNew {
		t.Fatalf("expected item marked CopyAsNew")
	}
	if m.preflight.items[0].Story.Published {
		t.Fatalf("expected fork to be draft")
	}
	if m.preflight.items[0].Story.UUID != "" {
		t.Fatalf("expected fork to clear UUID")
	}
	if !strings.HasSuffix(m.preflight.items[0].Story.Name, " (copy)") {
		t.Fatalf("expected name to include (copy)")
	}
	// Render and check badge
	m.updatePreflightViewport()
	out := m.renderPreflightHeader() + "\n" + m.renderViewportContent()
	if !strings.Contains(out, "[Fork]") {
		t.Fatalf("expected Fork badge in preflight render")
	}
}

func TestQuickForkMovesCursorAndMutatesItem(t *testing.T) {
	// Two collision stories
	st1 := sb.Story{ID: 1, Name: "one", Slug: "one", FullSlug: "one"}
	st2 := sb.Story{ID: 2, Name: "two", Slug: "two", FullSlug: "two"}
	tgt1 := sb.Story{ID: 9, Name: "one", Slug: "one", FullSlug: "one"}
	tgt2 := sb.Story{ID: 10, Name: "two", Slug: "two", FullSlug: "two"}
	m := InitialModel()
	m.storiesSource = []sb.Story{st1, st2}
	m.storiesTarget = []sb.Story{tgt1, tgt2}
	m.rebuildStoryIndex()
	if m.selection.selected == nil {
		m.selection.selected = make(map[string]bool)
	}
	m.selection.selected[st1.FullSlug] = true
	m.selection.selected[st2.FullSlug] = true
	m.startPreflight()

	// Quick fork first item with 'F'
	m.preflight.listIndex = 0
	m, _ = m.handlePreflightKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'F'}})
	if !m.preflight.items[0].CopyAsNew {
		t.Fatalf("expected first item CopyAsNew after quick fork")
	}
	if !strings.HasSuffix(m.preflight.items[0].Story.Slug, "-copy") {
		t.Fatalf("expected slug to have -copy suffix, got %s", m.preflight.items[0].Story.Slug)
	}
	if !strings.HasSuffix(m.preflight.items[0].Story.Name, " (copy)") {
		t.Fatalf("expected name to include (copy)")
	}
	if m.preflight.items[0].Story.Published {
		t.Fatalf("expected draft after quick fork")
	}
	// Cursor moved down by one
	if m.preflight.listIndex != 1 {
		t.Fatalf("expected cursor to move down by one, got %d", m.preflight.listIndex)
	}
}

func TestApplyFolderForkRebasesSubtree(t *testing.T) {
	// Set up a folder with children
	parent := sb.Story{ID: 1, Name: "app", Slug: "app", FullSlug: "app", IsFolder: true}
	child1 := sb.Story{ID: 2, Name: "page1", Slug: "page1", FullSlug: "app/page1", FolderID: &parent.ID}
	child2 := sb.Story{ID: 3, Name: "page2", Slug: "page2", FullSlug: "app/page2", FolderID: &parent.ID}
	subfolder := sb.Story{ID: 4, Name: "nested", Slug: "nested", FullSlug: "app/nested", IsFolder: true, FolderID: &parent.ID}
	subchild := sb.Story{ID: 5, Name: "deep", Slug: "deep", FullSlug: "app/nested/deep", FolderID: &subfolder.ID}

	m := InitialModel()
	m.storiesSource = []sb.Story{parent, child1, child2, subfolder, subchild}
	m.storiesTarget = []sb.Story{} // No existing stories
	m.rebuildStoryIndex()
	if m.selection.selected == nil {
		m.selection.selected = make(map[string]bool)
	}
	// Select all items
	m.selection.selected[parent.FullSlug] = true
	m.selection.selected[child1.FullSlug] = true
	m.selection.selected[child2.FullSlug] = true
	m.selection.selected[subfolder.FullSlug] = true
	m.selection.selected[subchild.FullSlug] = true
	m.startPreflight()

	// Find the parent folder index
	var folderIdx int
	for i, item := range m.preflight.items {
		if item.Story.ID == parent.ID {
			folderIdx = i
			break
		}
	}

	// Apply folder fork
	m.applyFolderFork(folderIdx, "app-copy", true, false)

	// Verify all items are marked as CopyAsNew
	for _, item := range m.preflight.items {
		if !item.CopyAsNew {
			t.Fatalf("expected all items to be marked CopyAsNew")
		}
	}

	// Verify folder structure is preserved with new root
	expectedPaths := map[int]string{
		parent.ID:    "app-copy",
		child1.ID:    "app-copy/page1",
		child2.ID:    "app-copy/page2",
		subfolder.ID: "app-copy/nested",
		subchild.ID:  "app-copy/nested/deep",
	}

	for _, item := range m.preflight.items {
		expected, exists := expectedPaths[item.Story.ID]
		if !exists {
			continue
		}
		if item.Story.FullSlug != expected {
			t.Fatalf("expected %s to have FullSlug %s, got %s", item.Story.Name, expected, item.Story.FullSlug)
		}
		if item.Story.Published {
			t.Fatalf("expected %s to be draft after fork", item.Story.Name)
		}
		if item.Story.UUID != "" {
			t.Fatalf("expected %s to have empty UUID after fork", item.Story.Name)
		}
	}

	// Verify folder names have (copy) suffix
	for _, item := range m.preflight.items {
		if item.Story.IsFolder && !strings.HasSuffix(item.Story.Name, " (copy)") {
			t.Fatalf("expected folder %s to have (copy) suffix", item.Story.Name)
		}
	}
}

func TestApplyFolderForkEnforcesUniqueness(t *testing.T) {
	// Set up folder with existing target story
	parent := sb.Story{ID: 1, Name: "app", Slug: "app", FullSlug: "app", IsFolder: true}
	child := sb.Story{ID: 2, Name: "page", Slug: "page", FullSlug: "app/page", FolderID: &parent.ID}
	existing := sb.Story{ID: 3, Name: "app-copy", Slug: "app-copy", FullSlug: "app-copy"}

	m := InitialModel()
	m.storiesSource = []sb.Story{parent, child}
	m.storiesTarget = []sb.Story{existing}
	m.rebuildStoryIndex()
	if m.selection.selected == nil {
		m.selection.selected = make(map[string]bool)
	}
	m.selection.selected[parent.FullSlug] = true
	m.selection.selected[child.FullSlug] = true
	m.startPreflight()

	// Find the parent folder index
	var folderIdx int
	for i, item := range m.preflight.items {
		if item.Story.ID == parent.ID {
			folderIdx = i
			break
		}
	}

	// Apply folder fork with conflicting slug
	m.applyFolderFork(folderIdx, "app-copy", false, false)

	// Verify uniqueness is enforced
	for _, item := range m.preflight.items {
		if item.Story.ID == parent.ID {
			if item.Story.Slug == "app-copy" {
				t.Fatalf("expected slug to be made unique, got %s", item.Story.Slug)
			}
			if !strings.HasPrefix(item.Story.Slug, "app-copy") {
				t.Fatalf("expected slug to start with app-copy, got %s", item.Story.Slug)
			}
			break
		}
	}
}

func TestFolderForkKeyHandlerOpensFullScreenView(t *testing.T) {
	// Set up a folder
	parent := sb.Story{ID: 1, Name: "app", Slug: "app", FullSlug: "app", IsFolder: true}
	child := sb.Story{ID: 2, Name: "page", Slug: "page", FullSlug: "app/page", FolderID: &parent.ID}

	m := InitialModel()
	m.storiesSource = []sb.Story{parent, child}
	m.storiesTarget = []sb.Story{}
	m.rebuildStoryIndex()
	if m.selection.selected == nil {
		m.selection.selected = make(map[string]bool)
	}
	m.selection.selected[parent.FullSlug] = true
	m.selection.selected[child.FullSlug] = true
	m.startPreflight()

	// Find the parent folder index in visible list
	var folderListIdx int
	for i, visibleIdx := range m.preflight.visibleIdx {
		if m.preflight.items[visibleIdx].Story.ID == parent.ID {
			folderListIdx = i
			break
		}
	}
	m.preflight.listIndex = folderListIdx

	// Press 'f' to open folder fork view
	m, _ = m.handlePreflightKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}})

	if m.state != stateFolderFork {
		t.Fatalf("expected stateFolderFork after f key, got %v", m.state)
	}
	if m.folder.itemIdx < 0 {
		t.Fatalf("expected folder itemIdx to be set")
	}
	if m.folder.baseSlug != "app" {
		t.Fatalf("expected baseSlug to be 'app', got %s", m.folder.baseSlug)
	}
	if !m.folder.appendCopyToFolderName {
		t.Fatalf("expected appendCopyToFolderName to be true by default")
	}
	if m.folder.appendCopyToChildStoryNames {
		t.Fatalf("expected appendCopyToChildStoryNames to be false by default")
	}
}

func TestFolderQuickForkAppliesImmediately(t *testing.T) {
	// Set up a folder with children
	parent := sb.Story{ID: 1, Name: "app", Slug: "app", FullSlug: "app", IsFolder: true}
	child := sb.Story{ID: 2, Name: "page", Slug: "page", FullSlug: "app/page", FolderID: &parent.ID}

	m := InitialModel()
	m.storiesSource = []sb.Story{parent, child}
	m.storiesTarget = []sb.Story{}
	m.rebuildStoryIndex()
	if m.selection.selected == nil {
		m.selection.selected = make(map[string]bool)
	}
	m.selection.selected[parent.FullSlug] = true
	m.selection.selected[child.FullSlug] = true
	m.startPreflight()

	// Find the parent folder index in visible list
	var folderListIdx int
	for i, visibleIdx := range m.preflight.visibleIdx {
		if m.preflight.items[visibleIdx].Story.ID == parent.ID {
			folderListIdx = i
			break
		}
	}
	m.preflight.listIndex = folderListIdx

	// Press 'F' for quick fork
	m, _ = m.handlePreflightKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'F'}})

	if m.state != statePreflight {
		t.Fatalf("expected return to statePreflight after quick fork, got %v", m.state)
	}

	// Verify folder fork was applied
	found := false
	for _, item := range m.preflight.items {
		if item.Story.ID == parent.ID {
			if !item.CopyAsNew {
				t.Fatalf("expected folder to be marked CopyAsNew after quick fork")
			}
			if !strings.HasSuffix(item.Story.Slug, "-copy") {
				t.Fatalf("expected folder slug to have -copy suffix, got %s", item.Story.Slug)
			}
			if !strings.HasSuffix(item.Story.Name, " (copy)") {
				t.Fatalf("expected folder name to have (copy) suffix")
			}
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("folder not found in preflight items")
	}

	// Verify cursor moved down
	if m.preflight.listIndex != folderListIdx+1 {
		t.Fatalf("expected cursor to move down by one, got %d", m.preflight.listIndex)
	}
}

func TestFolderForkKeyHandlerIgnoresNonFolders(t *testing.T) {
	// Set up a story (not folder)
	story := sb.Story{ID: 1, Name: "page", Slug: "page", FullSlug: "page"}

	m := InitialModel()
	m.storiesSource = []sb.Story{story}
	m.storiesTarget = []sb.Story{}
	m.rebuildStoryIndex()
	if m.selection.selected == nil {
		m.selection.selected = make(map[string]bool)
	}
	m.selection.selected[story.FullSlug] = true
	m.startPreflight()

	// Press 'f' on story
	m, _ = m.handlePreflightKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}})

	// Should not open folder fork view for stories
	if m.state == stateFolderFork {
		t.Fatalf("expected not to open folder fork view for story")
	}
}

func TestFolderForkKeyHandlerIgnoresUnselectedItems(t *testing.T) {
	// Set up a folder but don't select it
	parent := sb.Story{ID: 1, Name: "app", Slug: "app", FullSlug: "app", IsFolder: true}
	child := sb.Story{ID: 2, Name: "page", Slug: "page", FullSlug: "app/page", FolderID: &parent.ID}

	m := InitialModel()
	m.storiesSource = []sb.Story{parent, child}
	m.storiesTarget = []sb.Story{}
	m.rebuildStoryIndex()
	if m.selection.selected == nil {
		m.selection.selected = make(map[string]bool)
	}
	// Only select child, not parent
	m.selection.selected[child.FullSlug] = true
	m.startPreflight()

	// Find the parent folder index in visible list (should be skipped)
	var folderListIdx int
	for i, visibleIdx := range m.preflight.visibleIdx {
		if m.preflight.items[visibleIdx].Story.ID == parent.ID {
			folderListIdx = i
			break
		}
	}
	m.preflight.listIndex = folderListIdx

	// Press 'f' on unselected folder
	m, _ = m.handlePreflightKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}})

	// Should not open folder fork view for unselected folders
	if m.state == stateFolderFork {
		t.Fatalf("expected not to open folder fork view for unselected folder")
	}
}

func TestFolderForkKeyHandlerNavigation(t *testing.T) {
	// Set up folder fork state
	parent := sb.Story{ID: 1, Name: "app", Slug: "app", FullSlug: "app", IsFolder: true}
	m := InitialModel()
	m.storiesSource = []sb.Story{parent}
	m.storiesTarget = []sb.Story{}
	m.rebuildStoryIndex()
	if m.selection.selected == nil {
		m.selection.selected = make(map[string]bool)
	}
	m.selection.selected[parent.FullSlug] = true
	m.startPreflight()

	// Open folder fork view
	m, _ = m.handlePreflightKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}})
	if m.state != stateFolderFork {
		t.Fatalf("expected stateFolderFork")
	}

	// Test preset navigation
	initialPreset := m.folder.selectedPreset
	m, _ = m.handleFolderForkKey(tea.KeyMsg{Type: tea.KeyDown})
	if m.folder.selectedPreset != initialPreset+1 {
		t.Fatalf("expected preset to increment on down arrow")
	}

	m, _ = m.handleFolderForkKey(tea.KeyMsg{Type: tea.KeyUp})
	if m.folder.selectedPreset != initialPreset {
		t.Fatalf("expected preset to decrement on up arrow")
	}
}

func TestFolderForkKeyHandlerCheckboxToggle(t *testing.T) {
	// Set up folder fork state
	parent := sb.Story{ID: 1, Name: "app", Slug: "app", FullSlug: "app", IsFolder: true}
	m := InitialModel()
	m.storiesSource = []sb.Story{parent}
	m.storiesTarget = []sb.Story{}
	m.rebuildStoryIndex()
	if m.selection.selected == nil {
		m.selection.selected = make(map[string]bool)
	}
	m.selection.selected[parent.FullSlug] = true
	m.startPreflight()

	// Open folder fork view
	m, _ = m.handlePreflightKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}})

	// Test checkbox cycling with space
	// Initial state: folder copy ON, child copy OFF
	if !m.folder.appendCopyToFolderName || m.folder.appendCopyToChildStoryNames {
		t.Fatalf("unexpected initial checkbox state")
	}

	// First space: both ON
	m, _ = m.handleFolderForkKey(tea.KeyMsg{Type: tea.KeySpace})
	if !m.folder.appendCopyToFolderName || !m.folder.appendCopyToChildStoryNames {
		t.Fatalf("expected both checkboxes ON after first space")
	}

	// Second space: both OFF
	m, _ = m.handleFolderForkKey(tea.KeyMsg{Type: tea.KeySpace})
	if m.folder.appendCopyToFolderName || m.folder.appendCopyToChildStoryNames {
		t.Fatalf("expected both checkboxes OFF after second space")
	}

	// Third space: folder copy ON, child copy OFF (cycle back)
	m, _ = m.handleFolderForkKey(tea.KeyMsg{Type: tea.KeySpace})
	if !m.folder.appendCopyToFolderName || m.folder.appendCopyToChildStoryNames {
		t.Fatalf("expected folder copy ON, child copy OFF after third space")
	}
}

func TestFolderForkKeyHandlerInputValidation(t *testing.T) {
	// Set up folder fork state
	parent := sb.Story{ID: 1, Name: "app", Slug: "app", FullSlug: "app", IsFolder: true}
	m := InitialModel()
	m.storiesSource = []sb.Story{parent}
	m.storiesTarget = []sb.Story{}
	m.rebuildStoryIndex()
	if m.selection.selected == nil {
		m.selection.selected = make(map[string]bool)
	}
	m.selection.selected[parent.FullSlug] = true
	m.startPreflight()

	// Open folder fork view
	m, _ = m.handlePreflightKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}})

	// Test empty input validation - empty input gets normalized to "copy" and applied
	m.folder.input.SetValue("")
	m, _ = m.handleFolderForkKey(tea.KeyMsg{Type: tea.KeyEnter})
	if m.state != statePreflight {
		t.Fatalf("expected return to preflight state after empty input (normalized to 'copy')")
	}

	// Test whitespace-only input - whitespace gets normalized to "copy" and applied
	m.folder.input.SetValue("   ")
	m, _ = m.handleFolderForkKey(tea.KeyMsg{Type: tea.KeyEnter})
	if m.state != statePreflight {
		t.Fatalf("expected return to preflight state after whitespace input (normalized to 'copy')")
	}
}

func TestFolderForkKeyHandlerUniquenessCheck(t *testing.T) {
	// Set up folder fork state with existing target
	parent := sb.Story{ID: 1, Name: "app", Slug: "app", FullSlug: "app", IsFolder: true}
	existing := sb.Story{ID: 2, Name: "app-copy", Slug: "app-copy", FullSlug: "app-copy"}
	m := InitialModel()
	m.storiesSource = []sb.Story{parent}
	m.storiesTarget = []sb.Story{existing}
	m.rebuildStoryIndex()
	if m.selection.selected == nil {
		m.selection.selected = make(map[string]bool)
	}
	m.selection.selected[parent.FullSlug] = true
	m.startPreflight()

	// Open folder fork view
	m, _ = m.handlePreflightKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}})

	// Test conflicting slug
	m.folder.input.SetValue("app-copy")
	m, _ = m.handleFolderForkKey(tea.KeyMsg{Type: tea.KeyEnter})
	if m.state != stateFolderFork {
		t.Fatalf("expected to stay in folder fork state with conflicting slug")
	}
	if m.folder.errorMsg == "" {
		t.Fatalf("expected error message for conflicting slug")
	}
	// Input should be updated with unique suggestion
	if m.folder.input.Value() == "app-copy" {
		t.Fatalf("expected input to be updated with unique suggestion")
	}
}

func TestFolderForkKeyHandlerSuccessfulApply(t *testing.T) {
	// Set up folder fork state
	parent := sb.Story{ID: 1, Name: "app", Slug: "app", FullSlug: "app", IsFolder: true}
	child := sb.Story{ID: 2, Name: "page", Slug: "page", FullSlug: "app/page", FolderID: &parent.ID}
	m := InitialModel()
	m.storiesSource = []sb.Story{parent, child}
	m.storiesTarget = []sb.Story{}
	m.rebuildStoryIndex()
	if m.selection.selected == nil {
		m.selection.selected = make(map[string]bool)
	}
	m.selection.selected[parent.FullSlug] = true
	m.selection.selected[child.FullSlug] = true
	m.startPreflight()

	// Open folder fork view
	m, _ = m.handlePreflightKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}})

	// Apply with valid input
	m.folder.input.SetValue("app-new")
	m, _ = m.handleFolderForkKey(tea.KeyMsg{Type: tea.KeyEnter})

	if m.state != statePreflight {
		t.Fatalf("expected return to preflight state after successful apply")
	}

	// Verify folder fork was applied
	found := false
	for _, item := range m.preflight.items {
		if item.Story.ID == parent.ID {
			if !item.CopyAsNew {
				t.Fatalf("expected folder to be marked CopyAsNew")
			}
			if item.Story.Slug != "app-new" {
				t.Fatalf("expected folder slug to be 'app-new', got %s", item.Story.Slug)
			}
			if item.Story.FullSlug != "app-new" {
				t.Fatalf("expected folder FullSlug to be 'app-new', got %s", item.Story.FullSlug)
			}
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("folder not found in preflight items")
	}
}

func TestFolderForkKeyHandlerEscape(t *testing.T) {
	// Set up folder fork state
	parent := sb.Story{ID: 1, Name: "app", Slug: "app", FullSlug: "app", IsFolder: true}
	m := InitialModel()
	m.storiesSource = []sb.Story{parent}
	m.storiesTarget = []sb.Story{}
	m.rebuildStoryIndex()
	if m.selection.selected == nil {
		m.selection.selected = make(map[string]bool)
	}
	m.selection.selected[parent.FullSlug] = true
	m.startPreflight()

	// Open folder fork view
	m, _ = m.handlePreflightKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}})

	// Test escape key
	m, _ = m.handleFolderForkKey(tea.KeyMsg{Type: tea.KeyEscape})
	if m.state != statePreflight {
		t.Fatalf("expected return to preflight state on escape")
	}

	// Test 'q' key
	m, _ = m.handlePreflightKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}})
	m, _ = m.handleFolderForkKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if m.state != statePreflight {
		t.Fatalf("expected return to preflight state on 'q'")
	}
}

func TestViewFolderForkRendersCorrectly(t *testing.T) {
	// Set up folder fork state
	parent := sb.Story{ID: 1, Name: "My App", Slug: "app", FullSlug: "app", IsFolder: true}
	child1 := sb.Story{ID: 2, Name: "Page 1", Slug: "page1", FullSlug: "app/page1", FolderID: &parent.ID}
	child2 := sb.Story{ID: 3, Name: "Page 2", Slug: "page2", FullSlug: "app/page2", FolderID: &parent.ID}
	m := InitialModel()
	m.storiesSource = []sb.Story{parent, child1, child2}
	m.storiesTarget = []sb.Story{}
	m.rebuildStoryIndex()
	if m.selection.selected == nil {
		m.selection.selected = make(map[string]bool)
	}
	m.selection.selected[parent.FullSlug] = true
	m.selection.selected[child1.FullSlug] = true
	m.selection.selected[child2.FullSlug] = true
	m.startPreflight()

	// Open folder fork view
	m, _ = m.handlePreflightKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}})

	// Set some test values
	m.folder.input.SetValue("app-copy")
	m.folder.appendCopyToFolderName = true
	m.folder.appendCopyToChildStoryNames = false

	// Render the view
	view := m.viewFolderFork()

	// Check for key elements in the rendered view
	if !strings.Contains(view, "🍴 Ordner-Fork vorbereiten") {
		t.Fatalf("expected title in folder fork view")
	}
	if !strings.Contains(view, "Ordnerbaum unter neuem Slug kopieren") {
		t.Fatalf("expected subtitle in folder fork view")
	}
	if !strings.Contains(view, "My App") {
		t.Fatalf("expected folder name in view")
	}
	if !strings.Contains(view, "app-copy") {
		t.Fatalf("expected new slug in view")
	}
	if !strings.Contains(view, "My App (copy)") {
		t.Fatalf("expected folder name with copy suffix in view")
	}
}

func TestViewFolderForkShowsSubtreeCounts(t *testing.T) {
	// Set up folder fork state with mixed content
	parent := sb.Story{ID: 1, Name: "app", Slug: "app", FullSlug: "app", IsFolder: true}
	story1 := sb.Story{ID: 2, Name: "page1", Slug: "page1", FullSlug: "app/page1", FolderID: &parent.ID}
	story2 := sb.Story{ID: 3, Name: "page2", Slug: "page2", FullSlug: "app/page2", FolderID: &parent.ID}
	subfolder := sb.Story{ID: 4, Name: "nested", Slug: "nested", FullSlug: "app/nested", IsFolder: true, FolderID: &parent.ID}
	substory := sb.Story{ID: 5, Name: "deep", Slug: "deep", FullSlug: "app/nested/deep", FolderID: &subfolder.ID}
	m := InitialModel()
	m.storiesSource = []sb.Story{parent, story1, story2, subfolder, substory}
	m.storiesTarget = []sb.Story{}
	m.rebuildStoryIndex()
	if m.selection.selected == nil {
		m.selection.selected = make(map[string]bool)
	}
	// Select all items
	m.selection.selected[parent.FullSlug] = true
	m.selection.selected[story1.FullSlug] = true
	m.selection.selected[story2.FullSlug] = true
	m.selection.selected[subfolder.FullSlug] = true
	m.selection.selected[substory.FullSlug] = true
	m.startPreflight()

	// Open folder fork view
	m, _ = m.handlePreflightKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}})

	// Render the view
	view := m.viewFolderFork()

	// Check for subtree counts (3 stories, 2 folders including parent)
	if !strings.Contains(view, "3 Stories") {
		t.Fatalf("expected '3 Stories' in view, got: %s", view)
	}
	if !strings.Contains(view, "2 Ordner") {
		t.Fatalf("expected '2 Ordner' in view, got: %s", view)
	}
}

func TestFolderForkWithTranslatedSlugs(t *testing.T) {
	// Set up folder with translated slugs
	parent := sb.Story{
		ID: 1, Name: "app", Slug: "app", FullSlug: "app", IsFolder: true,
		TranslatedSlugs: []sb.TranslatedSlug{
			{Lang: "de", Path: "de/app"},
			{Lang: "en", Path: "en/app"},
		},
	}
	child := sb.Story{
		ID: 2, Name: "page", Slug: "page", FullSlug: "app/page", FolderID: &parent.ID,
		TranslatedSlugs: []sb.TranslatedSlug{
			{Lang: "de", Path: "de/app/page"},
			{Lang: "en", Path: "en/app/page"},
		},
	}
	m := InitialModel()
	m.storiesSource = []sb.Story{parent, child}
	m.storiesTarget = []sb.Story{}
	m.rebuildStoryIndex()
	if m.selection.selected == nil {
		m.selection.selected = make(map[string]bool)
	}
	m.selection.selected[parent.FullSlug] = true
	m.selection.selected[child.FullSlug] = true
	m.startPreflight()

	// Find the parent folder index
	var folderIdx int
	for i, item := range m.preflight.items {
		if item.Story.ID == parent.ID {
			folderIdx = i
			break
		}
	}

	// Apply folder fork
	m.applyFolderFork(folderIdx, "app-copy", false, false)

	// Verify translated slugs are updated
	for _, item := range m.preflight.items {
		if item.Story.ID == parent.ID {
			if len(item.Story.TranslatedSlugs) != 2 {
				t.Fatalf("expected 2 translated slugs for parent")
			}
			for _, ts := range item.Story.TranslatedSlugs {
				if !strings.HasSuffix(ts.Path, "app-copy") {
					t.Fatalf("expected translated path to end with 'app-copy', got %s", ts.Path)
				}
				if ts.ID != nil {
					t.Fatalf("expected translated slug ID to be cleared")
				}
			}
		}
		if item.Story.ID == child.ID {
			if len(item.Story.TranslatedSlugs) != 2 {
				t.Fatalf("expected 2 translated slugs for child")
			}
			for _, ts := range item.Story.TranslatedSlugs {
				// The child's translated path should have the new slug (last segment only)
				// The child slug should be "page" (unchanged) after fork
				if !strings.HasSuffix(ts.Path, "page") {
					t.Fatalf("expected translated path to end with 'page', got %s (child slug: %s)", ts.Path, item.Story.Slug)
				}
				if ts.ID != nil {
					t.Fatalf("expected translated slug ID to be cleared")
				}
			}
		}
	}
}
