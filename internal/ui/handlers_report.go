package ui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"storyblok-sync/internal/sb"
)

func (m Model) handleReportKey(key string) (Model, tea.Cmd) {
	switch key {
	case "enter", "b":
		// Go back to scan screen to allow starting a new sync
		m.state = stateScanning
		m.statusMsg = "Returning to scan screen for new sync…"
		return m, m.scanStoriesCmd()
	case "r":
		// Retry failures - rebuild preflight with only failed items
		if m.report.Summary.Failure > 0 {
			failedItems := m.getFailedItemsForRetry()
			if len(failedItems) > 0 {
				m.preflight.items = failedItems
				m.preflight.listIndex = 0
				m.state = statePreflight
				m.statusMsg = fmt.Sprintf("Retry: %d failed items ready for sync", len(failedItems))
				return m, nil
			}
		}
		// If no failures or couldn't build retry list, just stay on report
		m.statusMsg = "No failures to retry"
		return m, nil
	}
	return m, nil
}

// getFailedItemsForRetry creates preflight items from failed report entries
func (m Model) getFailedItemsForRetry() []PreflightItem {
	var failedItems []PreflightItem

	// Build a map of source stories by slug for quick lookup
	sourceMap := make(map[string]sb.Story)
	for _, story := range m.storiesSource {
		sourceMap[story.FullSlug] = story
	}

	// Build target stories map for collision detection
	targetMap := make(map[string]bool)
	for _, story := range m.storiesTarget {
		targetMap[story.FullSlug] = true
	}

	// Create preflight items for each failed entry
	for _, entry := range m.report.Entries {
		if entry.Status == "failure" {
			if sourceStory, exists := sourceMap[entry.Slug]; exists {
				item := PreflightItem{
					Story:     sourceStory,
					Collision: targetMap[entry.Slug],
					Skip:      false,
					Selected:  true, // Auto-select failed items for retry
					Run:       RunPending,
				}
				item.RecalcState()
				failedItems = append(failedItems, item)
			}
		}
	}

	return failedItems
}
