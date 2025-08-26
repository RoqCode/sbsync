package ui

// This file now contains only the functions that have NOT been extracted to other files.
// All the main browse handling logic has been moved to specialized files:
//
// - browse_input.go: Input handling for search and filter modes
// - browse_navigation.go: Tree navigation and cursor movement 
// - browse_selection.go: Item selection logic
// - browse_viewport.go: Viewport scrolling and cursor positioning
// - browse_filtering.go: Search and filter application logic
// - browse_tree.go: Tree operations (expand/collapse, utilities)
// - handlers_browse_main.go: Main orchestrating function

// The main handleBrowseListKey function is now in handlers_browse_main.go
// and delegates to the specialized handlers above.