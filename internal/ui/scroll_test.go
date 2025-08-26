package ui

import (
    "strings"
    "testing"

    "storyblok-sync/internal/sb"
)

func makeStories(n int) []sb.Story {
	stories := make([]sb.Story, n)
	for i := 0; i < n; i++ {
		stories[i] = sb.Story{ID: i + 1, Name: "s", Slug: "s", FullSlug: "s"}
	}
	return stories
}

func TestBrowseEnsureCursorVisibleScrollsDownAndUp(t *testing.T) {
	m := InitialModel()
	m.storiesSource = makeStories(10)
	m.rebuildStoryIndex()
	m.applyFilter()

    m.viewport.Height = 5
    m.viewport.SetContent(strings.Repeat("x\n", 100))
    m.viewport.SetYOffset(0)

	// Move cursor to bottom edge -> should scroll down by margin logic (to 1)
	m.selection.listIndex = 4
	m.ensureCursorVisible()
	if m.viewport.YOffset != 1 {
		t.Fatalf("expected YOffset 1 after scrolling down, got %d", m.viewport.YOffset)
	}

	// Now set a high offset and move cursor above top margin -> scroll up
    m.viewport.SetYOffset(10)
	m.selection.listIndex = 8
	m.ensureCursorVisible()
	if m.viewport.YOffset != 7 {
		t.Fatalf("expected YOffset 7 after scrolling up, got %d", m.viewport.YOffset)
	}
}

func TestPreflightEnsureCursorVisibleScrollsDownAndUp(t *testing.T) {
	m := InitialModel()
	// Build 10 simple preflight items with root stories
	items := make([]PreflightItem, 10)
	for i := 0; i < 10; i++ {
		items[i] = PreflightItem{Story: sb.Story{ID: i + 1, Name: "s", Slug: "s", FullSlug: "s"}}
	}
	m.preflight.items = items
	m.refreshPreflightVisible()

    m.viewport.Height = 5
    m.viewport.SetContent(strings.Repeat("x\n", 100))
    m.viewport.SetYOffset(0)

	// Move cursor to bottom edge -> should scroll down by margin logic (to 1)
	m.preflight.listIndex = 4
	m.ensurePreflightCursorVisible()
	if m.viewport.YOffset != 1 {
		t.Fatalf("expected YOffset 1 after scrolling down, got %d", m.viewport.YOffset)
	}

	// Now set a high offset and move cursor above top margin -> scroll up
    m.viewport.SetYOffset(10)
	m.preflight.listIndex = 8
	m.ensurePreflightCursorVisible()
	if m.viewport.YOffset != 7 {
		t.Fatalf("expected YOffset 7 after scrolling up, got %d", m.viewport.YOffset)
	}
}
