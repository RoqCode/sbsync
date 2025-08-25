package ui

import (
	"fmt"
	"strings"
)

func (m Model) renderSyncHeader() string {
	var b strings.Builder

	// Progress information
	total := len(m.plan.Items)
	completed := m.syncIndex

	progressText := fmt.Sprintf("Synchronisierung läuft... (%d/%d)", completed, total)
	b.WriteString(listHeaderStyle.Render(progressText))
	b.WriteString("\n")

	// Current item being processed
	if m.syncIndex < len(m.plan.Items) {
		currentItem := m.plan.Items[m.syncIndex]
		itemText := fmt.Sprintf("Aktuell: %s (%s)", currentItem.Story.Name, currentItem.State)
		b.WriteString(subtleStyle.Render(itemText))
	}

	b.WriteString("\n")
	return b.String()
}

func (m Model) renderSyncFooter() string {
	statusLine := ""
	if m.syncing {
		statusLine = m.spinner.View() + " Synchronisiere..."
	}

	return renderFooter(statusLine, "ctrl+c: Abbrechen")
}

func (m *Model) updateSyncViewport() {
	var content strings.Builder

	// Show sync progress
	total := len(m.plan.Items)
	completed := m.syncIndex

	// Progress bar
	progressBar := m.renderProgressBar(completed, total)
	content.WriteString(progressBar)
	content.WriteString("\n\n")

	// Show recent sync operations (last 10-15 items)
	startIdx := 0
	if completed > 15 {
		startIdx = completed - 15
	}

	for i := startIdx; i < completed && i < len(m.plan.Items); i++ {
		item := m.plan.Items[i]
		status := "✓"
		color := okStyle

		// Use different status based on item state
		line := fmt.Sprintf("%s %s (%s)",
			color.Render(status),
			item.Story.Name,
			subtleStyle.Render(string(item.State)))
		content.WriteString(line)
		content.WriteString("\n")
	}

	// Show current item if sync is ongoing
	if m.syncing && m.syncIndex < len(m.plan.Items) {
		currentItem := m.plan.Items[m.syncIndex]
		line := fmt.Sprintf("%s %s (%s)",
			warnStyle.Render("◯"),
			currentItem.Story.Name,
			subtleStyle.Render(string(currentItem.State)))
		content.WriteString(line)
		content.WriteString("\n")
	}

	m.viewport.SetContent(content.String())
}

func (m Model) renderProgressBar(completed, total int) string {
	if total == 0 {
		return "Kein Fortschritt verfügbar"
	}

	percentage := float64(completed) / float64(total) * 100
	barWidth := 50
	filledWidth := int(float64(barWidth) * float64(completed) / float64(total))

	var bar strings.Builder
	bar.WriteString("[")

	// Filled portion
	for i := 0; i < filledWidth; i++ {
		bar.WriteString("█")
	}

	// Empty portion
	for i := filledWidth; i < barWidth; i++ {
		bar.WriteString("░")
	}

	bar.WriteString("]")

	progressText := fmt.Sprintf("%s %.1f%% (%d/%d)",
		bar.String(), percentage, completed, total)

	return focusStyle.Render(progressText)
}
