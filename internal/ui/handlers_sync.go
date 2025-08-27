package ui

import (
	"context"
	tea "github.com/charmbracelet/bubbletea"
)

func (m Model) handleSyncKey(key string) (tea.Model, tea.Cmd) {
	// During sync, most keys are ignored to prevent user interference
	// ctrl+c is handled globally in update_main.go
	switch key {
	case "r":
		// Resume: if not currently syncing, pick up from first pending item
		if !m.syncing && m.state == stateSync {
			next := -1
			for i, it := range m.preflight.items {
				if it.Run == RunPending {
					next = i
					break
				}
			}
			if next >= 0 {
				m.syncing = true
				m.syncIndex = next
				// New context for resumed run
				m.syncContext, m.syncCancel = context.WithCancel(context.Background())
				return m, tea.Batch(m.spinner.Tick, m.runNextItem())
			}
		}
	}
	return m, nil
}
