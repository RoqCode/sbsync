package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"storyblok-sync/internal/sb"
)

// Helper to find preflight item by story ID
func findPreflightByID(items []PreflightItem, id int) *PreflightItem {
	for i := range items {
		if items[i].Story.ID == id {
			return &items[i]
		}
	}
	return nil
}

func TestPreflightKeyF_OpensFolderForkForFolder(t *testing.T) {
	// Source: a folder that collides in target
	folder := sb.Story{ID: 1, Name: "app", Slug: "app", FullSlug: "app", IsFolder: true}
	// Target already has the same folder -> collision
	tgt := sb.Story{ID: 99, Name: "app", Slug: "app", FullSlug: "app", IsFolder: true}

	m := InitialModel()
	m.storiesSource = []sb.Story{folder}
	m.storiesTarget = []sb.Story{tgt}
	m.rebuildStoryIndex()
	m.applyFilter()
	if m.selection.selected == nil {
		m.selection.selected = make(map[string]bool)
	}
	// Select the folder itself
	m.selection.selected[folder.FullSlug] = true

	m.startPreflight()
	if len(m.preflight.items) != 1 {
		t.Fatalf("expected 1 preflight item, got %d", len(m.preflight.items))
	}
	// Press 'f' to open folder fork
	m, _ = m.handlePreflightKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}})
	if m.state != stateFolderFork {
		t.Fatalf("expected stateFolderFork after 'f' on folder, got %v", m.state)
	}
	if m.folder.parent != "" {
		t.Fatalf("expected parent '' for root folder, got %q", m.folder.parent)
	}
	if m.folder.baseSlug != "app" {
		t.Fatalf("expected baseSlug 'app', got %q", m.folder.baseSlug)
	}
	if m.folder.input.Value() == "" || !strings.Contains(m.folder.input.Value(), "app") {
		t.Fatalf("expected input prefilled with suggestion containing 'app', got %q", m.folder.input.Value())
	}
	if !m.folder.appendCopyToFolderName || m.folder.appendCopyToChildStoryNames {
		t.Fatalf("expected default checkboxes: folderName=true, childNames=false")
	}
}

func TestFolderForkFullScreen_AppliesSubtreeRebase(t *testing.T) {
	// Build a small subtree: app (folder) -> page (story), blog (folder) -> post (story)
	app := sb.Story{ID: 1, Name: "App", Slug: "app", FullSlug: "app", IsFolder: true, TranslatedSlugs: []sb.TranslatedSlug{{Lang: "de", Path: "de/app"}, {Lang: "en", Path: "en/app"}}}
	appID := app.ID
	page := sb.Story{ID: 2, Name: "Page", Slug: "page", FullSlug: "app/page", FolderID: &appID, TranslatedSlugs: []sb.TranslatedSlug{{Lang: "de", Path: "de/app/page"}}}
	blog := sb.Story{ID: 3, Name: "Blog", Slug: "blog", FullSlug: "app/blog", IsFolder: true, FolderID: &appID}
	blogID := blog.ID
	post := sb.Story{ID: 4, Name: "Post", Slug: "post", FullSlug: "app/blog/post", FolderID: &blogID, TranslatedSlugs: []sb.TranslatedSlug{{Lang: "en", Path: "en/app/blog/post"}}}

	// Target contains the original app folder to cause a collision
	tgt := sb.Story{ID: 99, Name: "App", Slug: "app", FullSlug: "app", IsFolder: true}

	m := InitialModel()
	m.storiesSource = []sb.Story{app, blog, page, post}
	m.storiesTarget = []sb.Story{tgt}
	m.rebuildStoryIndex()
	m.applyFilter()
	if m.selection.selected == nil {
		m.selection.selected = make(map[string]bool)
	}
	// Select full subtree
	m.selection.selected[app.FullSlug] = true
	m.selection.selected[page.FullSlug] = true
	m.selection.selected[blog.FullSlug] = true
	m.selection.selected[post.FullSlug] = true

	m.startPreflight()
	// Open folder fork view
	m, _ = m.handlePreflightKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}})
	if m.state != stateFolderFork {
		t.Fatalf("expected stateFolderFork after 'f'")
	}

	// Confirm with Enter (accept suggested slug, defaults to app-copy)
	suggested := m.folder.input.Value()
	if suggested == "" {
		t.Fatalf("expected a suggested slug prefilled")
	}
	m, _ = m.handleFolderForkKey(tea.KeyMsg{Type: tea.KeyEnter})

	if m.state != statePreflight {
		t.Fatalf("expected return to statePreflight after confirm")
	}

	// Find items by ID and assert mutations
	itApp := findPreflightByID(m.preflight.items, app.ID)
	itPage := findPreflightByID(m.preflight.items, page.ID)
	itBlog := findPreflightByID(m.preflight.items, blog.ID)
	itPost := findPreflightByID(m.preflight.items, post.ID)
	if itApp == nil || itPage == nil || itBlog == nil || itPost == nil {
		t.Fatalf("expected all subtree items in preflight")
	}

	// Top folder
	if !itApp.CopyAsNew {
		t.Fatalf("expected top folder marked CopyAsNew")
	}
	if itApp.Story.Published {
		t.Fatalf("expected top folder draft after fork")
	}
	if itApp.Story.UUID != "" {
		t.Fatalf("expected top folder UUID cleared")
	}
	if !strings.HasSuffix(itApp.Story.Name, " (copy)") {
		t.Fatalf("expected top folder name suffixed with (copy)")
	}
	if !strings.HasSuffix(itApp.Story.Slug, "-copy") {
		t.Fatalf("expected top folder slug with -copy suffix, got %s", itApp.Story.Slug)
	}
	// Translated slugs last segment replaced
	for _, ts := range itApp.Story.TranslatedSlugs {
		if ts.ID != nil {
			t.Fatalf("expected translated slug IDs cleared for folder")
		}
		if !strings.HasSuffix(ts.Path, itApp.Story.Slug) {
			t.Fatalf("expected translated path to end with new slug, got %s", ts.Path)
		}
	}

	// Child story under new root
	if !itPage.CopyAsNew || itPage.Story.Published || itPage.Story.UUID != "" {
		t.Fatalf("expected child story forked and draft with cleared UUID")
	}
	if !strings.HasPrefix(itPage.Story.FullSlug, itApp.Story.FullSlug+"/") {
		t.Fatalf("expected child story rebased under new folder, got %s", itPage.Story.FullSlug)
	}
	// Name unchanged by default for child stories
	if itPage.Story.Name != "Page" {
		t.Fatalf("expected child story name unchanged by default, got %s", itPage.Story.Name)
	}
	for _, ts := range itPage.Story.TranslatedSlugs {
		if ts.ID != nil {
			t.Fatalf("expected translated slug IDs cleared for child story")
		}
		if !strings.HasSuffix(ts.Path, itPage.Story.Slug) {
			t.Fatalf("expected translated path to end with child slug, got %s", ts.Path)
		}
	}

	// Sub-folder Blog should have name suffixed and be under the new root
	if !strings.HasPrefix(itBlog.Story.FullSlug, itApp.Story.FullSlug+"/") {
		t.Fatalf("expected blog folder rebased under new root")
	}
	if !strings.HasSuffix(itBlog.Story.Name, " (copy)") {
		t.Fatalf("expected blog folder name suffixed with (copy)")
	}

	// Post story moved under blog at new root
	if !strings.HasPrefix(itPost.Story.FullSlug, itBlog.Story.FullSlug+"/") {
		t.Fatalf("expected post under blog at new root")
	}
}

func TestFolderQuickForkMovesCursorAndMutatesSubtree(t *testing.T) {
	app := sb.Story{ID: 1, Name: "App", Slug: "app", FullSlug: "app", IsFolder: true}
	appID := app.ID
	page := sb.Story{ID: 2, Name: "Page", Slug: "page", FullSlug: "app/page", FolderID: &appID}

	tgt := sb.Story{ID: 99, Name: "App", Slug: "app", FullSlug: "app", IsFolder: true}

	m := InitialModel()
	m.storiesSource = []sb.Story{app, page}
	m.storiesTarget = []sb.Story{tgt}
	m.rebuildStoryIndex()
	m.applyFilter()
	if m.selection.selected == nil {
		m.selection.selected = make(map[string]bool)
	}
	// Select folder and child
	m.selection.selected[app.FullSlug] = true
	m.selection.selected[page.FullSlug] = true

	m.startPreflight()

	// Quick fork folder with 'F'
	m.preflight.listIndex = 0
	m, _ = m.handlePreflightKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'F'}})

	// Top item should be forked and draft
	itApp := findPreflightByID(m.preflight.items, app.ID)
	if itApp == nil || !itApp.CopyAsNew {
		t.Fatalf("expected folder CopyAsNew after quick fork")
	}
	if !strings.HasSuffix(itApp.Story.Slug, "-copy") {
		t.Fatalf("expected folder slug to have -copy suffix")
	}
	if !strings.HasSuffix(itApp.Story.Name, " (copy)") {
		t.Fatalf("expected folder name suffixed with (copy)")
	}
	if itApp.Story.Published {
		t.Fatalf("expected folder draft after quick fork")
	}

	// Cursor moved down by one
	if m.preflight.listIndex != 1 {
		t.Fatalf("expected cursor to move down by one, got %d", m.preflight.listIndex)
	}
}
