package ui

import (
	"testing"

	"storyblok-sync/internal/sb"
)

func TestHandleBrowseTreeNavigation(t *testing.T) {
	m := createTestModel()
	m.addTestStories()

	tests := []struct {
		name              string
		key               string
		initialCursor     int
		initialCollapsed  map[int]bool
		expectedCollapsed map[int]bool
		expectedCursor    int
	}{
		{
			name:              "l expands folder",
			key:               "l",
			initialCursor:     1, // on folder
			initialCollapsed:  map[int]bool{2: true},
			expectedCollapsed: map[int]bool{2: false},
			expectedCursor:    1,
		},
		{
			name:              "l on story does nothing",
			key:               "l",
			initialCursor:     0, // on story
			initialCollapsed:  map[int]bool{2: true},
			expectedCollapsed: map[int]bool{2: true},
			expectedCursor:    0,
		},
		{
			name:              "h collapses folder",
			key:               "h",
			initialCursor:     1, // on folder
			initialCollapsed:  map[int]bool{2: false},
			expectedCollapsed: map[int]bool{2: true},
			expectedCursor:    1,
		},
		{
			name:              "h on story navigates to parent",
			key:               "h",
			initialCursor:     2, // on child story
			initialCollapsed:  map[int]bool{2: false},
			expectedCollapsed: map[int]bool{2: true},
			expectedCursor:    1, // should move to parent folder
		},
		{
			name:              "H collapses all folders",
			key:               "H",
			initialCursor:     0,
			initialCollapsed:  map[int]bool{2: false, 3: false},
			expectedCollapsed: map[int]bool{2: true, 3: true},
			expectedCursor:    0,
		},
		{
			name:              "L expands all folders",
			key:               "L",
			initialCursor:     0,
			initialCollapsed:  map[int]bool{2: true, 3: true},
			expectedCollapsed: map[int]bool{2: false, 3: false},
			expectedCursor:    0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testModel := m
			testModel.selection.listIndex = tt.initialCursor
			testModel.folderCollapsed = make(map[int]bool)
			for k, v := range tt.initialCollapsed {
				testModel.folderCollapsed[k] = v
			}

			result, _ := testModel.handleBrowseTreeNavigation(tt.key)

			for folderID, expectedCollapsed := range tt.expectedCollapsed {
				if result.folderCollapsed[folderID] != expectedCollapsed {
					t.Errorf("Expected folder %d collapsed=%v, got %v", folderID, expectedCollapsed, result.folderCollapsed[folderID])
				}
			}

			if tt.expectedCursor >= 0 && result.selection.listIndex != tt.expectedCursor {
				t.Errorf("Expected cursor at %d, got %d", tt.expectedCursor, result.selection.listIndex)
			}
		})
	}
}

func TestHandleBrowseTreeNavigationEmptyList(t *testing.T) {
	m := createTestModel()
	// No stories added - empty list

	keys := []string{"l", "h", "H", "L"}
	for _, key := range keys {
		t.Run("key "+key+" with empty list", func(t *testing.T) {
			result, _ := m.handleBrowseTreeNavigation(key)
			// Should not crash and should return unchanged model
			if result.selection.listIndex != m.selection.listIndex {
				t.Error("Cursor should not change with empty list")
			}
		})
	}
}

func TestHandleBrowseCursorMovement(t *testing.T) {
	m := createTestModel()
	m.addTestStories()

	tests := []struct {
		name           string
		key            string
		initialCursor  int
		expectedCursor int
	}{
		{
			name:           "j moves cursor down",
			key:            "j",
			initialCursor:  0,
			expectedCursor: 1,
		},
		{
			name:           "down moves cursor down",
			key:            "down",
			initialCursor:  0,
			expectedCursor: 1,
		},
		{
			name:           "j at last item stays at last",
			key:            "j",
			initialCursor:  2, // last item
			expectedCursor: 2,
		},
		{
			name:           "k moves cursor up",
			key:            "k",
			initialCursor:  2,
			expectedCursor: 1,
		},
		{
			name:           "up moves cursor up",
			key:            "up",
			initialCursor:  2,
			expectedCursor: 1,
		},
		{
			name:           "k at first item stays at first",
			key:            "k",
			initialCursor:  0,
			expectedCursor: 0,
		},
		{
			name:           "ctrl+d pages down",
			key:            "ctrl+d",
			initialCursor:  0,
			expectedCursor: 2, // should jump to last item in small list
		},
		{
			name:           "pgdown pages down",
			key:            "pgdown",
			initialCursor:  0,
			expectedCursor: 2,
		},
		{
			name:           "ctrl+u pages up",
			key:            "ctrl+u",
			initialCursor:  2,
			expectedCursor: 0,
		},
		{
			name:           "pgup pages up",
			key:            "pgup",
			initialCursor:  2,
			expectedCursor: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testModel := m
			testModel.selection.listIndex = tt.initialCursor
			testModel.viewport.Height = 10 // Set viewport height for paging

			result, _ := testModel.handleBrowseCursorMovement(tt.key)

			if result.selection.listIndex != tt.expectedCursor {
				t.Errorf("Expected cursor at %d, got %d", tt.expectedCursor, result.selection.listIndex)
			}
		})
	}
}

func TestHandleBrowseCursorMovementEmptyList(t *testing.T) {
	m := createTestModel()
	// No stories added - empty list

	keys := []string{"j", "k", "down", "up", "ctrl+d", "ctrl+u", "pgdown", "pgup"}
	for _, key := range keys {
		t.Run("key "+key+" with empty list", func(t *testing.T) {
			result, _ := m.handleBrowseCursorMovement(key)
			// Should not crash and should return unchanged model
			if result.selection.listIndex != m.selection.listIndex {
				t.Error("Cursor should not change with empty list")
			}
		})
	}
}

func TestHandleBrowseActions(t *testing.T) {
	m := createTestModel()
	m.addTestStories()

	tests := []struct {
		name          string
		key           string
		selections    map[string]bool
		expectedState state
		expectError   bool
	}{
		{
			name:          "r triggers rescan",
			key:           "r",
			expectedState: stateScanning,
		},
		{
			name:        "s with no selections shows error",
			key:         "s",
			selections:  map[string]bool{},
			expectError: true,
		},
		{
			name:          "s with selections starts preflight",
			key:           "s",
			selections:    map[string]bool{"story-1": true},
			expectedState: statePreflight,
		},
		{
			name:          "unknown key does nothing",
			key:           "x",
			expectedState: stateBrowseList,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testModel := m
			testModel.selection.selected = tt.selections

			result, cmd := testModel.handleBrowseActions(tt.key)

			if tt.expectError {
				if result.statusMsg == "" {
					t.Error("Expected error message for no selections")
				}
				return
			}

			if tt.expectedState != stateBrowseList {
				if result.state != tt.expectedState {
					t.Errorf("Expected state %v, got %v", tt.expectedState, result.state)
				}
			}

			if tt.key == "r" && cmd == nil {
				t.Error("Expected scan command for rescan")
			}
		})
	}
}

// Add more test stories to the helper
func (m *Model) addExtendedTestStories() {
	m.storiesSource = []sb.Story{
		{ID: 1, Name: "Story 1", FullSlug: "story-1", IsFolder: false},
		{ID: 2, Name: "Folder 1", FullSlug: "folder-1", IsFolder: true},
		{ID: 3, Name: "Story 2", FullSlug: "folder-1/story-2", IsFolder: false, FolderID: &[]int{2}[0]},
		{ID: 4, Name: "Story 3", FullSlug: "story-3", IsFolder: false},
		{ID: 5, Name: "Folder 2", FullSlug: "folder-2", IsFolder: true},
		{ID: 6, Name: "Story 4", FullSlug: "folder-2/story-4", IsFolder: false, FolderID: &[]int{5}[0]},
		{ID: 7, Name: "Story 5", FullSlug: "story-5", IsFolder: false},
		{ID: 8, Name: "Story 6", FullSlug: "story-6", IsFolder: false},
		{ID: 9, Name: "Story 7", FullSlug: "story-7", IsFolder: false},
		{ID: 10, Name: "Story 8", FullSlug: "story-8", IsFolder: false},
	}
	m.rebuildStoryIndex()
	m.refreshVisible()
}

func TestHandleBrowseCursorMovementPaging(t *testing.T) {
	m := createTestModel()
	m.addExtendedTestStories()
	m.viewport.Height = 3 // Small viewport for testing paging

	// Test page down from beginning
	m.selection.listIndex = 0
	result, _ := m.handleBrowseCursorMovement("ctrl+d")
	if result.selection.listIndex != 3 { // 0 + 3 (viewport height)
		t.Errorf("Expected cursor at 3 after page down, got %d", result.selection.listIndex)
	}

	// Test page down near end (should clamp to last item)
	m.selection.listIndex = 8
	result, _ = m.handleBrowseCursorMovement("ctrl+d")
	expectedLast := len(m.storiesSource) - 1
	if result.selection.listIndex != expectedLast {
		t.Errorf("Expected cursor at %d (last item) after page down, got %d", expectedLast, result.selection.listIndex)
	}

	// Test page up from end
	m.selection.listIndex = 9
	result, _ = m.handleBrowseCursorMovement("ctrl+u")
	if result.selection.listIndex != 6 { // 9 - 3 (viewport height)
		t.Errorf("Expected cursor at 6 after page up, got %d", result.selection.listIndex)
	}

	// Test page up near beginning (should clamp to 0)
	m.selection.listIndex = 1
	result, _ = m.handleBrowseCursorMovement("ctrl+u")
	if result.selection.listIndex != 0 {
		t.Errorf("Expected cursor at 0 after page up, got %d", result.selection.listIndex)
	}
}
