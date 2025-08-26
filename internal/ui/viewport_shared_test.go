package ui

import (
	"strings"
	"testing"
)

func TestEnsureCursorInViewportScrollsUp(t *testing.T) {
	m := InitialModel()
	m.viewport.Height = 5 // margin becomes 1 for small heights
	m.viewport.SetContent(strings.Repeat("x\n", 100))
	m.viewport.SetYOffset(10)

	// With margin 1 and top=10, cursor at 8 should scroll up to 7
	m.ensureCursorInViewport(8)
	if m.viewport.YOffset != 7 {
		t.Fatalf("expected YOffset 7, got %d", m.viewport.YOffset)
	}
}

func TestEnsureCursorInViewportScrollsDown(t *testing.T) {
	m := InitialModel()
	m.viewport.Height = 5 // margin becomes 1
	m.viewport.SetContent(strings.Repeat("x\n", 100))
	m.viewport.SetYOffset(0)

	// With margin 1 and height 5, cursor at 4 should scroll down to 1
	m.ensureCursorInViewport(4)
	if m.viewport.YOffset != 1 {
		t.Fatalf("expected YOffset 1, got %d", m.viewport.YOffset)
	}
}
