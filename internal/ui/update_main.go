package ui

import (
	"fmt"
	"time"

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
			// In sync state, treat Ctrl+C as pause: cancel context if present and prevent new scheduling
			if m.state == stateSync {
				if m.syncCancel != nil {
					m.syncCancel()
					// Keep syncContext set to the cancelled context so in-flight commands see it
				}
				m.paused = true
				m.statusMsg = "Sync cancelled by user (Ctrl+C) – press 'r' to resume or 'q' to quit"
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
        // Header height: default 3 (title + divider + 1-line state header)
        headerHeight := 3
        if m.state == stateSync {
            // Empirically account for:
            // - progress line with style margin
            // - current item line
            // - stats line
            // - multi-row RPS graph
            // This totals 6 + graphHeight lines for the state header portion.
            headerHeight = 6
            if m.showRpsGraph && m.rpsGraphHeight > 0 {
                headerHeight += m.rpsGraphHeight
            }
        }
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

    case statsTickMsg:
        if m.state != stateSync {
            return m, nil
        }
        now := time.Now()
        if m.api != nil {
            snap := m.api.MetricsSnapshot()
            if m.lastSnapTime.IsZero() {
                // initialize
                m.lastSnapTime = now
                m.lastSnap = snap
                // store first point
                m.reqTimes = append(m.reqTimes, now)
                m.reqTotals = append(m.reqTotals, snap.TotalRequests)
            } else {
                dt := now.Sub(m.lastSnapTime).Seconds()
                var rpsInst float64
                if dt > 0 {
                    delta := float64(snap.TotalRequests - m.lastSnap.TotalRequests)
                    rpsInst = delta / dt
                }
                // append sample + cumulative total
                m.reqTimes = append(m.reqTimes, now)
                m.reqSamples = append(m.reqSamples, rpsInst)
                m.reqTotals = append(m.reqTotals, snap.TotalRequests)
                // prune window
                cutoff := now.Add(-m.reqWindow)
                j := 0
                for j < len(m.reqTimes) && m.reqTimes[j].Before(cutoff) {
                    j++
                }
                if j > 0 {
                    m.reqTimes = append([]time.Time(nil), m.reqTimes[j:]...)
                    m.reqSamples = append([]float64(nil), m.reqSamples[j:]...)
                    m.reqTotals = append([]int64(nil), m.reqTotals[j:]...)
                }
                // window-based RPS using first and last totals
                if len(m.reqTimes) >= 2 {
                    elapsed := now.Sub(m.reqTimes[0]).Seconds()
                    if elapsed > 0 {
                        totalDelta := float64(m.reqTotals[len(m.reqTotals)-1] - m.reqTotals[0])
                        m.rpsCurrent = totalDelta / elapsed
                    } else {
                        m.rpsCurrent = rpsInst
                    }
                } else {
                    m.rpsCurrent = rpsInst
                }
                // update last snapshot
                m.lastSnapTime = now
                m.lastSnap = snap
            }
        }
        return m, m.statsTick()

	case syncResultMsg:
		// Prefer counters reported by result; fallback to client metrics delta
		rate429Delta := 0
		if msg.Result != nil {
			rate429Delta = msg.Result.Retry429
		} else if m.api != nil {
			after := m.api.MetricsSnapshot()
			if before, ok := m.syncStartMetrics[msg.Index]; ok {
				rate429Delta = int(after.Status429 - before.Status429)
				delete(m.syncStartMetrics, msg.Index)
			}
		}

		if msg.Index < len(m.preflight.items) {
			if msg.Cancelled {
				m.preflight.items[msg.Index].Run = RunCancelled
				it := m.preflight.items[msg.Index]
				m.report.Add(ReportEntry{Slug: it.Story.FullSlug, Status: "failure", Operation: "cancelled", Error: "Sync cancelled by user", Duration: 0, Story: &it.Story, RateLimit429: rate429Delta})
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
				m.report.Add(ReportEntry{Slug: it.Story.FullSlug, Status: "failure", Operation: "sync", Error: msg.Err.Error(), Duration: msg.Duration, Story: &it.Story, RateLimit429: rate429Delta})
				// Set inline issue message
				m.preflight.items[msg.Index].Issue = msg.Err.Error()
			} else if msg.Result != nil {
				// Add successful sync to report
				if msg.Result.Warning != "" {
					// Success with warning
					m.report.Add(ReportEntry{Slug: it.Story.FullSlug, Status: "warning", Operation: msg.Result.Operation, Warning: msg.Result.Warning, Duration: msg.Duration, Story: &it.Story, TargetStory: msg.Result.TargetStory, RateLimit429: rate429Delta})
					// Set inline issue message
					m.preflight.items[msg.Index].Issue = msg.Result.Warning
				} else {
					// Pure success
					m.report.Add(ReportEntry{Slug: it.Story.FullSlug, Status: "success", Operation: msg.Result.Operation, Duration: msg.Duration, TargetStory: msg.Result.TargetStory, RateLimit429: rate429Delta})
					// Keep target index fresh: if a folder was created/updated, update m.storiesTarget
					if msg.Result.TargetStory != nil && msg.Result.TargetStory.IsFolder {
						updated := false
						for i := range m.storiesTarget {
							if m.storiesTarget[i].FullSlug == msg.Result.TargetStory.FullSlug {
								m.storiesTarget[i] = *msg.Result.TargetStory
								updated = true
								break
							}
						}
						if !updated {
							m.storiesTarget = append(m.storiesTarget, *msg.Result.TargetStory)
						}
					}
				}
			} else {
				// Fallback for unexpected case
				m.report.Add(ReportEntry{Slug: it.Story.FullSlug, Status: "success", Operation: "unknown", Duration: msg.Duration, RateLimit429: rate429Delta})
			}
		}

        // no-op: throughput by requests/sec is sampled via metrics snapshots

		// Recompute aggregate counts
		done := 0
		cancelled := 0
		pending := 0
		running := 0
		hasPendingFolders := false
		for _, it := range m.preflight.items {
			switch it.Run {
			case RunDone:
				done++
			case RunCancelled:
				cancelled++
			case RunPending:
				pending++
				if it.Story.IsFolder {
					hasPendingFolders = true
				}
			case RunRunning:
				running++
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

		// Maintain a worker pool. During folder phase, allow only 1; afterwards, allow up to 6.
		allowed := 6
		if hasPendingFolders {
			allowed = 1
		}
		m.maxWorkers = allowed
		if pending > 0 && !m.paused {
			toStart := allowed - running
			if toStart > pending {
				toStart = pending
			}
			if toStart > 0 {
				cmds := make([]tea.Cmd, 0, toStart)
				for i := 0; i < toStart; i++ {
					cmds = append(cmds, m.runNextItem())
				}
				return m, tea.Batch(cmds...)
			}
		}
		// If paused or no pending but still running workers, wait for them to finish
		if running > 0 {
			return m, nil
		}

		// If paused and nothing running, do not auto-finish into report; stay in sync state
		if m.paused {
			m.syncing = false
			m.statusMsg = "Sync paused – press 'r' to resume or 'q' to quit"
			return m, nil
		}

		// All work done
		m.syncing = false
		m.paused = false
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

// countLines returns the number of visual lines in a string by counting newlines.
// Returns 0 for empty strings.
// (no dynamic header/footer counting; use fixed base + known extras for stability)

// --- Stats tick (Phase 4) ---
type statsTickMsg struct{}

func (m Model) statsTick() tea.Cmd {
	return tea.Tick(500*time.Millisecond, func(time.Time) tea.Msg { return statsTickMsg{} })
}
