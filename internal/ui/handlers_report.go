package ui

import (
	"context"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"storyblok-sync/internal/sb"
)

func (m Model) handleReportKey(key string) (Model, tea.Cmd) {
	switch key {
	// Scrolling in report view (like other lists)
	case "j", "down":
		m.viewport.SetYOffset(m.viewport.YOffset + 1)
		return m, nil
	case "k", "up":
		off := m.viewport.YOffset - 1
		if off < 0 {
			off = 0
		}
		m.viewport.SetYOffset(off)
		return m, nil
	case "ctrl+d", "pgdown":
		jump := m.viewport.Height
		if jump <= 0 {
			jump = 10
		}
		m.viewport.SetYOffset(m.viewport.YOffset + jump)
		return m, nil
	case "ctrl+u", "pgup":
		jump := m.viewport.Height
		if jump <= 0 {
			jump = 10
		}
		off := m.viewport.YOffset - jump
		if off < 0 {
			off = 0
		}
		m.viewport.SetYOffset(off)
		return m, nil
	case "enter", "b":
		// Go back to scan screen to allow starting a new sync
		m.state = stateModePicker
		m.statusMsg = "Zurück zur Modus-Auswahl…"
		return m, nil
	case "m":
		m.state = stateModePicker
		m.statusMsg = "Zurück zur Modus-Auswahl…"
		return m, nil
	case "r":
		// Resume any pending work first; else retry failures if any
		next := -1
		for i, it := range m.preflight.items {
			if it.Run == RunPending {
				next = i
				break
			}
		}
		if next >= 0 {
			m.state = stateSync
			m.syncing = true
			m.syncIndex = next
			// Ensure API client is available
			if m.api == nil {
				m.api = sb.New(m.cfg.Token)
			}
			m.syncContext, m.syncCancel = context.WithCancel(context.Background())
			m.statusMsg = "Resuming sync…"
			return m, tea.Batch(m.spinner.Tick, m.runNextItem())
		}
		// No pending items; fall back to retry failures pathway below
		// Retry failures - rebuild preflight with only failed items
		if m.report.Summary.Failure > 0 {
			failedItems := m.getFailedItemsForRetry()
			if len(failedItems) > 0 {
				// Replace preflight with only failed items and rebuild visibility
				m.preflight = PreflightState{items: failedItems, listIndex: 0}
				m.refreshPreflightVisible()
				m.state = statePreflight
				m.statusMsg = fmt.Sprintf("Retry: %d failed items ready for sync", len(failedItems))
				m.updateViewportContent()
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
				recalcState(&item)
				failedItems = append(failedItems, item)
			}
		}
	}

	return failedItems
}
