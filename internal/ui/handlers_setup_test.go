package ui

import (
	"testing"

	"storyblok-sync/internal/config"
	"storyblok-sync/internal/sb"
)

func TestHandleWelcomeKeyVariants(t *testing.T) {
	tests := []struct {
		name          string
		key           string
		hasToken      bool
		expectedState state
	}{
		{
			name:          "enter with no token goes to token prompt",
			key:           "enter",
			hasToken:      false,
			expectedState: stateTokenPrompt,
		},
		{
			name:          "enter with token goes to validating",
			key:           "enter",
			hasToken:      true,
			expectedState: stateValidating,
		},
		{
			name:          "other keys do nothing",
			key:           "x",
			hasToken:      false,
			expectedState: stateWelcome,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := InitialModel()
			if tt.hasToken {
				m.cfg.Token = "test-token"
			} else {
				m.cfg.Token = ""
			}

			result, cmd := m.handleWelcomeKey(tt.key)

			if result.state != tt.expectedState {
				t.Errorf("Expected state %v, got %v", tt.expectedState, result.state)
			}

			if tt.key == "enter" && tt.hasToken && cmd == nil {
				t.Error("Expected validation command when entering with token")
			}
		})
	}
}

func TestHandleTokenPromptKey(t *testing.T) {
	m := InitialModel()
	m.state = stateTokenPrompt
	m.ti.Focus()

	tests := []struct {
		name          string
		key           string
		inputValue    string
		expectedState state
		expectsCmd    bool
	}{
		{
			name:          "esc returns to welcome",
			key:           "esc",
			inputValue:    "test-token",
			expectedState: stateWelcome,
			expectsCmd:    false,
		},
		{
			name:          "enter with empty token shows error",
			key:           "enter",
			inputValue:    "",
			expectedState: stateTokenPrompt,
			expectsCmd:    false,
		},
		{
			name:          "enter with whitespace token shows error",
			key:           "enter",
			inputValue:    "   ",
			expectedState: stateTokenPrompt,
			expectsCmd:    false,
		},
		{
			name:          "enter with valid token validates",
			key:           "enter",
			inputValue:    "valid-token",
			expectedState: stateValidating,
			expectsCmd:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testModel := m
			testModel.ti.SetValue(tt.inputValue)

			msg := createKeyMsg(tt.key)

			result, cmd := testModel.handleTokenPromptKey(msg)

			if result.state != tt.expectedState {
				t.Errorf("Expected state %v, got %v", tt.expectedState, result.state)
			}

			if tt.expectsCmd && cmd == nil {
				t.Error("Expected command but got nil")
			}

			if !tt.expectsCmd && tt.key != "esc" && cmd != nil {
				t.Error("Did not expect command")
			}

			if tt.key == "enter" && tt.inputValue != "" && len(tt.inputValue) > 0 && tt.inputValue != "   " {
				if result.cfg.Token != "valid-token" {
					t.Errorf("Expected token to be set to 'valid-token', got '%s'", result.cfg.Token)
				}
			}
		})
	}
}

func TestHandleValidatingKey(t *testing.T) {
	m := InitialModel()
	m.state = stateValidating

	// Validating state should not respond to any keys
	keys := []string{"enter", "esc", "q", "x", "ctrl+c"}
	for _, key := range keys {
		t.Run("key "+key, func(t *testing.T) {
			result, cmd := m.handleValidatingKey(key)

			if result.state != stateValidating {
				t.Error("State should remain stateValidating")
			}

			if cmd != nil {
				t.Error("No commands should be issued during validation")
			}
		})
	}
}

func TestHandleSpaceSelectKey(t *testing.T) {
	m := InitialModel()
	m.state = stateSpaceSelect
	m.spaces = []sb.Space{
		{ID: 1, Name: "Space 1"},
		{ID: 2, Name: "Space 2"},
		{ID: 3, Name: "Space 3"},
	}

	tests := []struct {
		name            string
		key             string
		initialIndex    int
		selectingSource bool
		expectedIndex   int
		expectedState   state
		expectSourceSet bool
		expectTargetSet bool
		expectScanStart bool
	}{
		{
			name:            "j moves down",
			key:             "j",
			initialIndex:    0,
			selectingSource: true,
			expectedIndex:   1,
			expectedState:   stateSpaceSelect,
		},
		{
			name:            "down moves down",
			key:             "down",
			initialIndex:    0,
			selectingSource: true,
			expectedIndex:   1,
			expectedState:   stateSpaceSelect,
		},
		{
			name:            "j at last index stays at last",
			key:             "j",
			initialIndex:    2,
			selectingSource: true,
			expectedIndex:   2,
			expectedState:   stateSpaceSelect,
		},
		{
			name:            "k moves up",
			key:             "k",
			initialIndex:    2,
			selectingSource: true,
			expectedIndex:   1,
			expectedState:   stateSpaceSelect,
		},
		{
			name:            "up moves up",
			key:             "up",
			initialIndex:    2,
			selectingSource: true,
			expectedIndex:   1,
			expectedState:   stateSpaceSelect,
		},
		{
			name:            "k at first index stays at first",
			key:             "k",
			initialIndex:    0,
			selectingSource: true,
			expectedIndex:   0,
			expectedState:   stateSpaceSelect,
		},
		{
			name:            "enter on source selection sets source",
			key:             "enter",
			initialIndex:    1,
			selectingSource: true,
			expectedIndex:   0, // resets for target selection
			expectedState:   stateSpaceSelect,
			expectSourceSet: true,
		},
		{
			name:            "enter on target selection starts scan",
			key:             "enter",
			initialIndex:    2,
			selectingSource: false,
			expectedIndex:   2,
			expectedState:   stateScanning,
			expectTargetSet: true,
			expectScanStart: true,
		},
		{
			name:            "unknown key does nothing",
			key:             "x",
			initialIndex:    1,
			selectingSource: true,
			expectedIndex:   1,
			expectedState:   stateSpaceSelect,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testModel := m
			testModel.selectedIndex = tt.initialIndex
			testModel.selectingSource = tt.selectingSource
			testModel.sourceSpace = nil
			testModel.targetSpace = nil

			result, cmd := testModel.handleSpaceSelectKey(tt.key)

			if result.selectedIndex != tt.expectedIndex {
				t.Errorf("Expected index %d, got %d", tt.expectedIndex, result.selectedIndex)
			}

			if result.state != tt.expectedState {
				t.Errorf("Expected state %v, got %v", tt.expectedState, result.state)
			}

			if tt.expectSourceSet {
				if result.sourceSpace == nil {
					t.Error("Expected source space to be set")
				} else if result.sourceSpace.ID != m.spaces[tt.initialIndex].ID {
					t.Errorf("Expected source space ID %d, got %d", m.spaces[tt.initialIndex].ID, result.sourceSpace.ID)
				}
				if result.selectingSource {
					t.Error("Expected selectingSource to be false after setting source")
				}
			}

			if tt.expectTargetSet {
				if result.targetSpace == nil {
					t.Error("Expected target space to be set")
				} else if result.targetSpace.ID != m.spaces[tt.initialIndex].ID {
					t.Errorf("Expected target space ID %d, got %d", m.spaces[tt.initialIndex].ID, result.targetSpace.ID)
				}
			}

			if tt.expectScanStart && cmd == nil {
				t.Error("Expected scan command to be issued")
			}
		})
	}
}

func TestHandleSpaceSelectKeyEmptySpaces(t *testing.T) {
	m := InitialModel()
	m.state = stateSpaceSelect
	m.spaces = []sb.Space{} // empty list

	result, cmd := m.handleSpaceSelectKey("enter")

	// Should not crash with empty spaces list
	if result.state != stateSpaceSelect {
		t.Error("State should remain stateSpaceSelect with empty spaces")
	}

	if cmd != nil {
		t.Error("Should not issue commands with empty spaces")
	}
}

func TestHandleScanningKey(t *testing.T) {
	m := InitialModel()
	m.state = stateScanning

	// Scanning state should not respond to any keys (placeholder implementation)
	keys := []string{"enter", "esc", "q", "x", "ctrl+c"}
	for _, key := range keys {
		t.Run("key "+key, func(t *testing.T) {
			result, cmd := m.handleScanningKey(key)

			if result.state != stateScanning {
				t.Error("State should remain stateScanning")
			}

			if cmd != nil {
				t.Error("No commands should be issued during scanning")
			}
		})
	}
}

// Test helper for creating model with config
func createTestModelWithToken(token string) Model {
	m := InitialModel()
	m.cfg = config.Config{
		Token: token,
		Path:  "/test/.sbrc",
	}
	return m
}

func TestWelcomeStateFlow(t *testing.T) {
	// Test the complete flow from welcome to validation
	m := createTestModelWithToken("test-token")

	// Start at welcome
	if m.state != stateWelcome {
		t.Error("Should start at welcome state")
	}

	// Press enter with token
	result, cmd := m.handleWelcomeKey("enter")

	if result.state != stateValidating {
		t.Error("Should move to validating state")
	}

	if cmd == nil {
		t.Error("Should issue validation command")
	}
}

func TestTokenPromptStateFlow(t *testing.T) {
	// Test the complete flow from token prompt to validation
	m := InitialModel()
	m.state = stateTokenPrompt
	m.ti.SetValue("new-token")

	msg := createKeyMsg("enter")

	result, cmd := m.handleTokenPromptKey(msg)

	if result.state != stateValidating {
		t.Error("Should move to validating state")
	}

	if result.cfg.Token != "new-token" {
		t.Error("Should set token from input")
	}

	if cmd == nil {
		t.Error("Should issue validation command")
	}
}
