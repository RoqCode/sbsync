package ui

import (
	"fmt"
	"strings"
)

func (m Model) renderCompSyncHeader() string {
	var b strings.Builder
	total := len(m.compPre.items)
	completed, running, cancelled, pending := 0, 0, 0, 0
	for _, it := range m.compPre.items {
		switch it.Run {
		case RunDone:
			completed++
		case RunRunning:
			running++
		case RunCancelled:
			cancelled++
		default:
			pending++
		}
	}
	parts := []string{fmt.Sprintf("%d✓", completed)}
	if running > 0 {
		parts = append(parts, fmt.Sprintf("%d◯", running))
	}
	if cancelled > 0 {
		parts = append(parts, fmt.Sprintf("%d✗", cancelled))
	}
	if pending > 0 {
		parts = append(parts, fmt.Sprintf("%d◦", pending))
	}
	b.WriteString(listHeaderStyle.Render(fmt.Sprintf("Components Sync (%s von %d)", strings.Join(parts, " "), total)))
	b.WriteString("\n")
	// Current item
	for _, it := range m.compPre.items {
		if it.Run == RunRunning {
			b.WriteString(warnStyle.Render(fmt.Sprintf("Läuft: %s", it.Source.Name)))
			break
		}
	}
	b.WriteString("\n")
	b.WriteString(m.renderStatsPanel())
	return b.String()
}

func (m Model) renderCompSyncFooter() string {
	left := ""
	if m.syncing {
		left = m.spinner.View() + " Synchronisiere..."
	}
	help := "ctrl+c: Abbrechen"
	line := help
	if left != "" {
		line = subtleStyle.Render(left) + "  |  " + help
	}
	return renderFooter("", line)
}

func (m *Model) updateCompSyncViewport() {
	// Build a compact progress list for components
	total := len(m.compPre.items)
	completed, cancelled := 0, 0
	for _, it := range m.compPre.items {
		if it.Run == RunDone {
			completed++
		} else if it.Run == RunCancelled {
			cancelled++
		}
	}
	m.viewport.SetContent(m.renderProgressBar(completed, cancelled, total) + "\n\n" + m.renderCompItemsList())
}

func (m Model) renderCompItemsList() string {
	var b strings.Builder
	// Display up to viewport height minus header room
	maxDisplay := m.viewport.Height - 3
	if maxDisplay < 5 {
		maxDisplay = 5
	}
	startIdx := 0
	if len(m.compPre.items) > maxDisplay {
		// anchor at first running or pending
		anchor := 0
		for i, it := range m.compPre.items {
			if it.Run == RunRunning || it.Run == RunPending {
				anchor = i
				break
			}
		}
		startIdx = anchor - maxDisplay/2
		if startIdx < 0 {
			startIdx = 0
		}
		if startIdx+maxDisplay > len(m.compPre.items) {
			startIdx = len(m.compPre.items) - maxDisplay
		}
	}
	endIdx := startIdx + maxDisplay
	if endIdx > len(m.compPre.items) {
		endIdx = len(m.compPre.items)
	}
	for i := startIdx; i < endIdx; i++ {
		it := m.compPre.items[i]
		status, color := m.getItemStatusDisplay(it.Run)
		action := it.State
		if it.CopyAsNew {
			action = "create"
		}
		line := fmt.Sprintf("%s %s (%s)", color.Render(status), it.Source.Name, subtleStyle.Render(action))
		if it.Issue != "" {
			line += " " + warnStyle.Render("["+it.Issue+"]")
		}
		b.WriteString(line + "\n")
	}
	if len(m.compPre.items) > (endIdx - startIdx) {
		b.WriteString("\n")
		b.WriteString(subtleStyle.Render(fmt.Sprintf("... zeige %d-%d von %d Items", startIdx+1, endIdx, len(m.compPre.items))))
	}
	return b.String()
}
