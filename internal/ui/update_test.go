package ui

import (
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
