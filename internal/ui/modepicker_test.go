package ui

import (
	tea "github.com/charmbracelet/bubbletea"
	"testing"
)

func TestModePickerNavigation(t *testing.T) {
	m := InitialModel()
	m.state = stateModePicker
	// move down to Components and select
	var cmd tea.Cmd
	m, cmd = m.handleModePickerKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if m.modePickerIndex != 1 {
		t.Fatalf("expected index 1, got %d", m.modePickerIndex)
	}
	m, cmd = m.handleModePickerKey(tea.KeyMsg{Type: tea.KeyEnter})
	if m.state != stateScanning {
		t.Fatalf("expected stateScanning, got %v", m.state)
	}
	if m.currentMode != modeComponents {
		t.Fatalf("expected modeComponents")
	}
	if cmd == nil {
		t.Fatalf("expected a scan command")
	}

	// go back and choose stories
	m.state = stateModePicker
	m.modePickerIndex = 0
	m, cmd = m.handleModePickerKey(tea.KeyMsg{Type: tea.KeyEnter})
	if m.state != stateScanning {
		t.Fatalf("expected stateScanning for stories, got %v", m.state)
	}
	if m.currentMode != modeStories {
		t.Fatalf("expected modeStories")
	}
	if cmd == nil {
		t.Fatalf("expected a scan command for stories")
	}
}
