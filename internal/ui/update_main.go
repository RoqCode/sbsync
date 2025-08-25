package ui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
)

// ---------- Update ----------
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.KeyMsg:
		key := msg.String()
		if m.state == statePreflight {
			return m.handlePreflightKey(msg)
		}

		// global shortcuts
		if key == "ctrl+c" {
			// If we're syncing, cancel the sync operations
			if m.syncing && m.syncCancel != nil {
				m.syncCancel()
				m.statusMsg = "Sync cancelled by user (Ctrl+C)"
				return m, nil
			}
			return m, tea.Quit
		}
		if key == "q" {
			return m, tea.Quit
		}

		switch m.state {
		case stateWelcome:
			return m.handleWelcomeKey(key)
		case stateTokenPrompt:
			return m.handleTokenPromptKey(msg)
		case stateValidating:
			return m.handleValidatingKey(key)
		case stateSpaceSelect:
			return m.handleSpaceSelectKey(key)
		case stateScanning:
			return m.handleScanningKey(key)
		case stateBrowseList:
			return m.handleBrowseListKey(msg)
		case stateReport:
			return m.handleReportKey(key)
		}

	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		// grobe Reserve für Header, Divider, Titel, Footer/Hilfe
		const chrome = 12
		vp := m.height - chrome
		if vp < 3 {
			vp = 3
		}
		m.selection.listViewport = vp
		m.preflight.listViewport = vp

	case validateMsg:
		if msg.err != nil {
			m.validateErr = msg.err
			m.statusMsg = "Validierung fehlgeschlagen: " + msg.err.Error()
			m.state = stateTokenPrompt
			return m, nil
		}
		m.spaces = msg.spaces
		m.statusMsg = fmt.Sprintf("Token ok. %d Spaces gefunden.", len(m.spaces))
		// check if we have spaces configured and validate if their ids are in m.spaces
		if m.cfg.SourceSpace != "" && m.cfg.TargetSpace != "" {
			sourceSpace, sourceIdIsOk := containsSpaceID(m.spaces, m.cfg.SourceSpace)
			targetSpace, targetIdIsOk := containsSpaceID(m.spaces, m.cfg.TargetSpace)

			if sourceIdIsOk && targetIdIsOk {
				m.sourceSpace = &sourceSpace
				m.targetSpace = &targetSpace
				m.statusMsg = fmt.Sprintf("Target gesetzt: %s (%d). Scanne jetzt Stories…", sourceSpace.Name, sourceSpace.ID)
				m.state = stateScanning
				return m, tea.Batch(m.spinner.Tick, m.scanStoriesCmd())
			}
		}
		m.state = stateSpaceSelect
		m.selectingSource = true
		m.selectedIndex = 0
		return m, nil

	case scanMsg:
		if msg.err != nil {
			m.statusMsg = "Scan-Fehler: " + msg.err.Error()
			m.state = stateSpaceSelect // zurück; du kannst auch einen Fehler-Screen bauen
			return m, nil
		}
		m.storiesSource = msg.src
		m.storiesTarget = msg.tgt
		m.selection.listIndex, m.selection.listOffset = 0, 0
		m.rebuildStoryIndex()
		m.applyFilter()
		if m.selection.selected == nil {
			m.selection.selected = make(map[string]bool)
		} else {
			// optional: Selektion leeren, da sich die Liste geändert hat
			clear(m.selection.selected)
		}
		m.statusMsg = fmt.Sprintf("Scan ok. Source: %d Stories, Target: %d Stories.", len(m.storiesSource), len(m.storiesTarget))
		m.state = stateBrowseList
		return m, nil

	case spinner.TickMsg:
		if m.state == stateValidating || m.state == stateScanning || (m.state == statePreflight && m.syncing) {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
		return m, nil

	case syncResultMsg:
		if msg.index < len(m.preflight.items) {
			if msg.cancelled {
				m.preflight.items[msg.index].Run = RunCancelled
				it := m.preflight.items[msg.index]
				m.report.AddError(it.Story.FullSlug, "cancelled", "Sync cancelled by user", 0, &it.Story)

				// Mark all remaining items as cancelled
				for i := msg.index + 1; i < len(m.preflight.items); i++ {
					if m.preflight.items[i].Run == RunPending || m.preflight.items[i].Run == RunRunning {
						m.preflight.items[i].Run = RunCancelled
						it := m.preflight.items[i]
						m.report.AddError(it.Story.FullSlug, "cancelled", "Sync cancelled by user", 0, &it.Story)
					}
				}

				m.syncing = false
				m.syncCancel = nil
				m.syncContext = nil
				m.statusMsg = "Sync cancelled - press 'r' to generate report"
				return m, nil
			}

			m.preflight.items[msg.index].Run = RunDone
			it := m.preflight.items[msg.index]

			if msg.err != nil {
				// Add error to report with complete source story
				m.report.AddError(it.Story.FullSlug, "sync", msg.err.Error(), msg.duration, &it.Story)
			} else if msg.result != nil {
				// Add successful sync to report
				if msg.result.warning != "" {
					// Success with warning
					m.report.AddWarning(it.Story.FullSlug, msg.result.operation, msg.result.warning, msg.duration, &it.Story, msg.result.targetStory)
				} else {
					// Pure success
					m.report.AddSuccess(it.Story.FullSlug, msg.result.operation, msg.duration, msg.result.targetStory)
				}
			} else {
				// Fallback for unexpected case
				m.report.AddSuccess(it.Story.FullSlug, "unknown", msg.duration, nil)
			}
		}

		done := 0
		cancelled := 0
		for _, it := range m.preflight.items {
			if it.Run == RunDone {
				done++
			} else if it.Run == RunCancelled {
				cancelled++
			}
		}
		m.syncIndex = done

		// Continue only if we haven't finished all items and haven't been cancelled
		if done+cancelled < len(m.preflight.items) && cancelled == 0 {
			return m, m.runNextItem()
		}

		m.syncing = false
		if cancelled > 0 {
			m.statusMsg = fmt.Sprintf("Sync cancelled - %d completed, %d cancelled", done, cancelled)
		} else {
			m.statusMsg = m.report.GetDisplaySummary()
		}
		_ = m.report.Save()

		// Transition to report screen to show detailed results
		m.state = stateReport
		return m, nil
	}

	return m, nil
}
