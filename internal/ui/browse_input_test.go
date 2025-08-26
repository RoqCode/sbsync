package ui

import (
	"testing"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"storyblok-sync/internal/sb"
)

// createKeyMsg creates a KeyMsg from a string representation
func createKeyMsg(key string) tea.KeyMsg {
	switch key {
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEscape}
	case "tab":
		return tea.KeyMsg{Type: tea.KeyTab}
	case "ctrl+c":
		return tea.KeyMsg{Type: tea.KeyCtrlC}
	case "up":
		return tea.KeyMsg{Type: tea.KeyUp}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	default:
		// Handle single character keys
		if len(key) == 1 {
			return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{rune(key[0])}}
		}
		// For unhandled multi-character keys, treat as runes
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)}
	}
}

func TestHandlePrefixFilterInput(t *testing.T) {
	m := createTestModel()
	m.filter.prefixing = true
	m.filter.prefixInput.Focus()
	m.filter.prefixInput.SetValue("test-prefix")

	tests := []struct {
		name           string
		key            string
		expectedPrefix string
		expectedActive bool
	}{
		{
			name:           "ESC with empty value clears prefix",
			key:            "esc",
			expectedPrefix: "",
			expectedActive: false,
		},
		{
			name:           "ESC with value keeps trimmed value",
			key:            "esc",
			expectedPrefix: "test-prefix",
			expectedActive: false,
		},
		{
			name:           "Enter sets prefix from input",
			key:            "enter",
			expectedPrefix: "test-prefix",
			expectedActive: false,
		},
		{
			name:           "Ctrl+C quits",
			key:            "ctrl+c",
			expectedPrefix: "test-prefix",
			expectedActive: true, // should remain active since we quit
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testModel := m
			testModel.filter.prefixing = true
			testModel.filter.prefixInput.Focus()

			if tt.name == "ESC with empty value clears prefix" {
				testModel.filter.prefixInput.SetValue("  ") // whitespace only
			} else {
				testModel.filter.prefixInput.SetValue("test-prefix")
			}

			msg := createKeyMsg(tt.key)

			result, cmd := testModel.handlePrefixFilterInput(msg)

			if tt.key == "ctrl+c" || tt.key == "q" {
				if cmd == nil {
					t.Error("Expected quit command but got nil")
				}
				return
			}

			if result.filter.prefixing != tt.expectedActive {
				t.Errorf("Expected prefixing=%v, got %v", tt.expectedActive, result.filter.prefixing)
			}

			if result.filter.prefix != tt.expectedPrefix {
				t.Errorf("Expected prefix='%s', got '%s'", tt.expectedPrefix, result.filter.prefix)
			}
		})
	}
}

func TestHandleSearchInput(t *testing.T) {
	m := createTestModel()
	m.search.searching = true
	m.search.searchInput.Focus()
	m.addTestStories()

	tests := []struct {
		name           string
		key            string
		initialValue   string
		expectedQuery  string
		expectedActive bool
		shouldExpand   bool
		shouldCollapse bool
	}{
		{
			name:           "ESC with empty value closes search",
			key:            "esc",
			initialValue:   "",
			expectedQuery:  "",
			expectedActive: false,
			shouldCollapse: true,
		},
		{
			name:           "ESC with value clears search but keeps active",
			key:            "esc",
			initialValue:   "test",
			expectedQuery:  "",
			expectedActive: true,
			shouldCollapse: true,
		},
		{
			name:           "Enter sets query and deactivates",
			key:            "enter",
			initialValue:   "test query",
			expectedQuery:  "test query",
			expectedActive: false,
		},
		{
			name:           "Ctrl+C quits",
			key:            "ctrl+c",
			initialValue:   "test",
			expectedQuery:  "test", // query unchanged
			expectedActive: true,   // should remain active since we quit
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testModel := m
			testModel.search.searching = true
			testModel.search.searchInput.Focus()
			testModel.search.searchInput.SetValue(tt.initialValue)
			testModel.search.query = tt.initialValue

			msg := createKeyMsg(tt.key)

			result, cmd := testModel.handleSearchInput(msg)

			if tt.key == "ctrl+c" || tt.key == "q" {
				if cmd == nil {
					t.Error("Expected quit command but got nil")
				}
				return
			}

			if result.search.searching != tt.expectedActive {
				t.Errorf("Expected searching=%v, got %v", tt.expectedActive, result.search.searching)
			}

			if result.search.query != tt.expectedQuery {
				t.Errorf("Expected query='%s', got '%s'", tt.expectedQuery, result.search.query)
			}
		})
	}
}

func TestHandleSearchInputLiveUpdate(t *testing.T) {
	m := createTestModel()
	m.search.searching = true
	m.search.searchInput.Focus()
	m.addTestStories()

	// Setup initial state with expanded folders
	m.folderCollapsed = map[int]bool{1: false}

	// Simulate typing by updating the text input directly
	m.search.searchInput.SetValue("test")
	m.search.query = ""

	// Create a text input update message to simulate typing
	updatedInput := textinput.New()
	updatedInput.SetValue("test-new")

	// This simulates the case where the user types and the input value changes
	msg := createKeyMsg("a") // any regular key

	// Set the search input to the new value to simulate the update
	m.search.searchInput.SetValue("test-new")

	result, _ := m.handleSearchInput(msg)

	// When search query changes from empty to non-empty, folders should expand
	// When search query changes from non-empty to empty, folders should collapse
	// The live update logic should detect the change and update the query
	if result.search.query == "" {
		t.Error("Expected query to be updated in live search")
	}
}

func TestHandleBrowseSearchAndFilterControls(t *testing.T) {
	m := createTestModel()

	tests := []struct {
		name               string
		key                string
		initialSearching   bool
		initialPrefixing   bool
		expectedSearching  bool
		expectedPrefixing  bool
		expectedQueryEmpty bool
	}{
		{
			name:              "f toggles search on",
			key:               "f",
			initialSearching:  false,
			expectedSearching: true,
		},
		{
			name:              "f toggles search off",
			key:               "f",
			initialSearching:  true,
			expectedSearching: false,
		},
		{
			name:               "F clears search",
			key:                "F",
			initialSearching:   true,
			expectedSearching:  true,
			expectedQueryEmpty: true,
		},
		{
			name:              "p toggles prefix filter on",
			key:               "p",
			initialPrefixing:  false,
			expectedPrefixing: true,
		},
		{
			name:              "P toggles prefix filter on",
			key:               "P",
			initialPrefixing:  false,
			expectedPrefixing: true,
		},
		{
			name:              "p toggles prefix filter off",
			key:               "p",
			initialPrefixing:  true,
			expectedPrefixing: false,
		},
		{
			name:               "c clears all filters and selections",
			key:                "c",
			expectedSearching:  false,
			expectedPrefixing:  false,
			expectedQueryEmpty: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testModel := m
			testModel.search.searching = tt.initialSearching
			testModel.filter.prefixing = tt.initialPrefixing
			testModel.search.query = "test-query"
			testModel.filter.prefix = "test-prefix"
			testModel.selection.selected = map[string]bool{"test": true}

			result, _ := testModel.handleBrowseSearchAndFilterControls(tt.key)

			if result.search.searching != tt.expectedSearching {
				t.Errorf("Expected searching=%v, got %v", tt.expectedSearching, result.search.searching)
			}

			if result.filter.prefixing != tt.expectedPrefixing {
				t.Errorf("Expected prefixing=%v, got %v", tt.expectedPrefixing, result.filter.prefixing)
			}

			if tt.expectedQueryEmpty {
				if result.search.query != "" {
					t.Errorf("Expected empty query, got '%s'", result.search.query)
				}
				if result.filter.prefix != "" {
					t.Errorf("Expected empty prefix, got '%s'", result.filter.prefix)
				}
			}

			if tt.key == "c" {
				if len(result.selection.selected) != 0 {
					t.Error("Expected selections to be cleared")
				}
			}
		})
	}
}

// Helper function to create a test model
func createTestModel() Model {
	m := InitialModel()
	m.state = stateBrowseList
	return m
}

// Helper function to add test stories
func (m *Model) addTestStories() {
	m.storiesSource = []sb.Story{
		{ID: 1, Name: "Story 1", FullSlug: "story-1", IsFolder: false},
		{ID: 2, Name: "Folder 1", FullSlug: "folder-1", IsFolder: true},
		{ID: 3, Name: "Story 2", FullSlug: "folder-1/story-2", IsFolder: false, FolderID: &[]int{2}[0]},
	}
	m.rebuildStoryIndex()
	m.refreshVisible()
}
