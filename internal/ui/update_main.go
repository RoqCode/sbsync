package ui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"

	"storyblok-sync/internal/config"
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
			// If we're syncing, cancel the sync operations but don't quit
			if m.syncing && m.syncCancel != nil {
				m.syncCancel()
				m.statusMsg = "Sync cancelled by user (Ctrl+C) – press 'r' to resume"
				return m, nil
			}
			// If not syncing, quit the application
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
		case stateSync:
			return m.handleSyncKey(key)
		case stateReport:
			return m.handleReportKey(key)
		}

	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height

		// Update viewport dimensions
		headerHeight := 3 // title + divider + state header
		footerHeight := 3 // help text lines
		viewportHeight := msg.Height - headerHeight - footerHeight
		if viewportHeight < 5 {
			viewportHeight = 5
		}
		m.viewport.Width = msg.Width
		m.viewport.Height = viewportHeight

		// BubbleTea viewport handles all scrolling now

		// Update viewport content after resize
		m.updateViewportContent()

	case validateMsg:
		if msg.err != nil {
			m.validateErr = msg.err
			m.statusMsg = "Validierung fehlgeschlagen: " + msg.err.Error()
			m.state = stateTokenPrompt
			return m, nil
		}
		m.spaces = msg.spaces

		// Save token to .sbrc file after successful validation
		if err := config.Save(m.cfg.Path, m.cfg); err != nil {
			m.statusMsg = "Token validiert, aber Speichern fehlgeschlagen: " + err.Error()
		} else {
			m.statusMsg = fmt.Sprintf("Token gespeichert. %d Spaces gefunden.", len(m.spaces))
		}
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
		m.selection.listIndex = 0
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
		m.updateViewportContent()
		return m, nil

	case spinner.TickMsg:
		if m.state == stateValidating || m.state == stateScanning || m.state == stateSync {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
		return m, nil

	case syncResultMsg:
		if msg.Index < len(m.preflight.items) {
			if msg.Cancelled {
				m.preflight.items[msg.Index].Run = RunCancelled
				it := m.preflight.items[msg.Index]
				m.report.AddError(it.Story.FullSlug, "cancelled", "Sync cancelled by user", 0, &it.Story)
				// Set inline issue for cancelled item
				m.preflight.items[msg.Index].Issue = "Sync cancelled by user"

				// Do NOT cancel remaining items; leave them pending to allow resume
				m.syncing = false
				m.syncCancel = nil
				m.syncContext = nil
				m.statusMsg = "Sync cancelled – press 'r' to resume or 'q' to quit"
				return m, nil
			}

			m.preflight.items[msg.Index].Run = RunDone
			it := m.preflight.items[msg.Index]

			// Clear any previous issue by default
			m.preflight.items[msg.Index].Issue = ""
			if msg.Err != nil {
				// Add error to report with complete source story
				m.report.AddError(it.Story.FullSlug, "sync", msg.Err.Error(), msg.Duration, &it.Story)
				// Set inline issue message
				m.preflight.items[msg.Index].Issue = msg.Err.Error()
			} else if msg.Result != nil {
				// Add successful sync to report
				if msg.Result.Warning != "" {
					// Success with warning
					m.report.AddWarning(it.Story.FullSlug, msg.Result.Operation, msg.Result.Warning, msg.Duration, &it.Story, msg.Result.TargetStory)
					// Set inline issue message
					m.preflight.items[msg.Index].Issue = msg.Result.Warning
				} else {
					// Pure success
					m.report.AddSuccess(it.Story.FullSlug, msg.Result.Operation, msg.Duration, msg.Result.TargetStory)
				}
			} else {
				// Fallback for unexpected case
				m.report.AddSuccess(it.Story.FullSlug, "unknown", msg.Duration, nil)
			}
		}

		done := 0
		cancelled := 0
		pending := 0
		for _, it := range m.preflight.items {
			switch it.Run {
			case RunDone:
				done++
			case RunCancelled:
				cancelled++
			case RunPending:
				pending++
			}
		}
		// Keep syncIndex at next pending position if available
		if pending > 0 {
			for i, it := range m.preflight.items {
				if it.Run == RunPending {
					m.syncIndex = i
					break
				}
			}
		} else {
			m.syncIndex = done
		}

		// Update viewport content to show progress in real-time
		if m.state == stateSync {
			m.updateViewportContent()
		}

		// Continue if there are still pending items
		if pending > 0 {
			return m, m.runNextItem()
		}

		m.syncing = false
		m.state = stateReport
		if cancelled > 0 {
			m.statusMsg = fmt.Sprintf("Sync cancelled - %d completed, %d cancelled", done, cancelled)
		} else {
			m.statusMsg = m.report.GetDisplaySummary()
		}
		_ = m.report.Save()

		// Update viewport content for report view
		m.updateViewportContent()
		return m, nil
	}

	return m, nil
}
