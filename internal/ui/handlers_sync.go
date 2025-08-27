package ui

import (
	"context"
	tea "github.com/charmbracelet/bubbletea"
	"log"
	"storyblok-sync/internal/sb"
)

func (m Model) handleSyncKey(key string) (tea.Model, tea.Cmd) {
	// During sync, most keys are ignored to prevent user interference
	// ctrl+c is handled globally in update_main.go
	switch key {
	case "r":
		// Resume: if not currently syncing, pick up from first pending item
		if !m.syncing && m.state == stateSync {
			log.Printf("RESUME(SYNC): attempting resume; items=%d", len(m.preflight.items))
			// Count run states for diagnostics
			var cPending, cRunning, cDone, cCancelled int
			for _, it := range m.preflight.items {
				switch it.Run {
				case RunPending:
					cPending++
				case RunRunning:
					cRunning++
				case RunDone:
					cDone++
				case RunCancelled:
					cCancelled++
				}
			}
			log.Printf("RESUME(SYNC): counts pending=%d running=%d done=%d cancelled=%d", cPending, cRunning, cDone, cCancelled)
			next := -1
			for i, it := range m.preflight.items {
				if it.Run == RunPending {
					next = i
					break
				}
			}
			if next >= 0 {
				log.Printf("RESUME(SYNC): next pending index=%d (prev syncIndex=%d)", next, m.syncIndex)
				m.syncing = true
				m.syncIndex = next
				// New context for resumed run
				if m.api == nil {
					log.Printf("RESUME(SYNC): api client is nil, creating new with token present=%t", m.cfg.Token != "")
					m.api = sb.New(m.cfg.Token)
				}
				if m.sourceSpace == nil {
					log.Printf("RESUME(SYNC): sourceSpace is nil")
				} else {
					log.Printf("RESUME(SYNC): sourceSpace=%s(%d)", m.sourceSpace.Name, m.sourceSpace.ID)
				}
				if m.targetSpace == nil {
					log.Printf("RESUME(SYNC): targetSpace is nil")
				} else {
					log.Printf("RESUME(SYNC): targetSpace=%s(%d)", m.targetSpace.Name, m.targetSpace.ID)
				}
				m.syncContext, m.syncCancel = context.WithCancel(context.Background())
				log.Printf("RESUME(SYNC): created new context and starting next item")
				return m, tea.Batch(m.spinner.Tick, m.runNextItem())
			}
			log.Printf("RESUME(SYNC): no pending items found; nothing to resume")
		}
	}
	return m, nil
}
