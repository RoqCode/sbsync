package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func (m Model) viewReport() string {
	var b strings.Builder

	// Header with sync summary
	totalDuration := float64(m.report.Duration) / 1000.0
	b.WriteString(fmt.Sprintf("Sync Report – %s\n", m.report.GetDisplaySummary()))
	b.WriteString(fmt.Sprintf("Duration: %.2fs  |  Source: %s  |  Target: %s\n\n",
		totalDuration, m.report.SourceSpace, m.report.TargetSpace))

	// Statistics section with colored boxes
	successStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Bold(true)
	warningStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("11")).Bold(true)
	errorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Bold(true)

	statsBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("240")).
		Padding(1, 2).
		Margin(0, 1)

	// Create stats content
	var stats strings.Builder
	stats.WriteString(successStyle.Render(fmt.Sprintf("✓ %d Success", m.report.Summary.Success)))
	if m.report.Summary.Warning > 0 {
		stats.WriteString("  " + warningStyle.Render(fmt.Sprintf("⚠ %d Warnings", m.report.Summary.Warning)))
	}
	if m.report.Summary.Failure > 0 {
		stats.WriteString("  " + errorStyle.Render(fmt.Sprintf("✗ %d Failures", m.report.Summary.Failure)))
	}
	stats.WriteString(fmt.Sprintf("\n%d Created  |  %d Updated  |  %d Skipped",
		m.report.Summary.Created, m.report.Summary.Updated, m.report.Summary.Skipped))

	b.WriteString(statsBox.Render(stats.String()))
	b.WriteString("\n\n")

	if len(m.report.Entries) == 0 {
		b.WriteString(subtleStyle.Render("No entries in report.") + "\n")
	} else {
		// Group entries by status for better organization
		var successes, warnings, failures []ReportEntry
		for _, entry := range m.report.Entries {
			switch entry.Status {
			case "success":
				successes = append(successes, entry)
			case "warning":
				warnings = append(warnings, entry)
			case "failure":
				failures = append(failures, entry)
			}
		}

		// Show failures first (most important)
		if len(failures) > 0 {
			b.WriteString(errorStyle.Render("⚠ FAILURES") + "\n")
			for _, entry := range failures {
				duration := fmt.Sprintf("%dms", entry.Duration)
				b.WriteString(fmt.Sprintf("  %s %s (%s) %s - %s\n",
					symbolStory, entry.Slug, entry.Operation, duration, entry.Error))
			}
			b.WriteString("\n")
		}

		// Show warnings next
		if len(warnings) > 0 {
			b.WriteString(warningStyle.Render("⚠ WARNINGS") + "\n")
			for _, entry := range warnings {
				duration := fmt.Sprintf("%dms", entry.Duration)
				b.WriteString(fmt.Sprintf("  %s %s (%s) %s - %s\n",
					symbolStory, entry.Slug, entry.Operation, duration, entry.Warning))
			}
			b.WriteString("\n")
		}

		// Show recent successes (limit to avoid clutter)
		if len(successes) > 0 {
			b.WriteString(successStyle.Render("✓ SUCCESSES") + " ")
			if len(successes) > 10 {
				b.WriteString(subtleStyle.Render(fmt.Sprintf("(showing last 10 of %d)", len(successes))))
			}
			b.WriteString("\n")

			start := len(successes) - 10
			if start < 0 {
				start = 0
			}
			for i := start; i < len(successes); i++ {
				entry := successes[i]
				duration := fmt.Sprintf("%dms", entry.Duration)
				symbol := symbolStory
				if entry.TargetStory != nil && entry.TargetStory.IsFolder {
					symbol = symbolFolder
				}
				b.WriteString(fmt.Sprintf("  %s %s (%s) %s\n",
					symbol, entry.Slug, entry.Operation, duration))
			}
		}
	}

	b.WriteString("\n")

	// Footer with actions
	if m.report.Summary.Failure > 0 {
		b.WriteString(helpStyle.Render("r retry failures  |  enter back to scan  |  q exit"))
	} else {
		b.WriteString(helpStyle.Render("enter back to scan  |  q exit"))
	}

	return b.String()
}

func (m *Model) updateReportViewport() {
	content := m.renderReportContent()
	m.viewport.SetContent(content)
}

func (m Model) renderReportHeader() string {
	totalDuration := float64(m.report.Duration) / 1000.0
	return fmt.Sprintf("Sync Report – %s | Duration: %.2fs | Source: %s | Target: %s",
		m.report.GetDisplaySummary(), totalDuration, m.report.SourceSpace, m.report.TargetSpace)
}

func (m Model) renderReportContent() string {
	var b strings.Builder

	// Statistics section with colored boxes
	successStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Bold(true)
	warningStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("11")).Bold(true)
	errorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Bold(true)

	statsBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("240")).
		Padding(1, 2).
		Margin(0, 1)

	// Create stats content
	var stats strings.Builder
	stats.WriteString(successStyle.Render(fmt.Sprintf("✓ %d Success", m.report.Summary.Success)))
	if m.report.Summary.Warning > 0 {
		stats.WriteString("  " + warningStyle.Render(fmt.Sprintf("⚠ %d Warnings", m.report.Summary.Warning)))
	}
	if m.report.Summary.Failure > 0 {
		stats.WriteString("  " + errorStyle.Render(fmt.Sprintf("✗ %d Failures", m.report.Summary.Failure)))
	}
	stats.WriteString(fmt.Sprintf("\n%d Created  |  %d Updated  |  %d Skipped",
		m.report.Summary.Created, m.report.Summary.Updated, m.report.Summary.Skipped))

	b.WriteString(statsBox.Render(stats.String()))
	b.WriteString("\n\n")

	if len(m.report.Entries) == 0 {
		b.WriteString(subtleStyle.Render("No entries in report.") + "\n")
	} else {
		// Group entries by status for better organization
		var successes, warnings, failures []ReportEntry
		for _, entry := range m.report.Entries {
			switch entry.Status {
			case "success":
				successes = append(successes, entry)
			case "warning":
				warnings = append(warnings, entry)
			case "failure":
				failures = append(failures, entry)
			}
		}

		// Show failures first (most important)
		if len(failures) > 0 {
			b.WriteString(errorStyle.Render("⚠ FAILURES") + "\n")
			for _, entry := range failures {
				duration := fmt.Sprintf("%dms", entry.Duration)
				b.WriteString(fmt.Sprintf("  %s %s (%s) %s - %s\n",
					symbolStory, entry.Slug, entry.Operation, duration, entry.Error))
			}
			b.WriteString("\n")
		}

		// Show warnings next
		if len(warnings) > 0 {
			b.WriteString(warningStyle.Render("⚠ WARNINGS") + "\n")
			for _, entry := range warnings {
				duration := fmt.Sprintf("%dms", entry.Duration)
				b.WriteString(fmt.Sprintf("  %s %s (%s) %s - %s\n",
					symbolStory, entry.Slug, entry.Operation, duration, entry.Warning))
			}
			b.WriteString("\n")
		}

		// Show recent successes (limit to avoid clutter)
		if len(successes) > 0 {
			b.WriteString(successStyle.Render("✓ SUCCESSES") + " ")
			if len(successes) > 10 {
				b.WriteString(subtleStyle.Render(fmt.Sprintf("(showing last 10 of %d)", len(successes))))
			}
			b.WriteString("\n")

			start := len(successes) - 10
			if start < 0 {
				start = 0
			}
			for i := start; i < len(successes); i++ {
				entry := successes[i]
				duration := fmt.Sprintf("%dms", entry.Duration)
				symbol := symbolStory
				if entry.TargetStory != nil && entry.TargetStory.IsFolder {
					symbol = symbolFolder
				}
				b.WriteString(fmt.Sprintf("  %s %s (%s) %s\n",
					symbol, entry.Slug, entry.Operation, duration))
			}
		}
	}

	return b.String()
}

func (m Model) renderReportFooter() string {
	var helpText string
	if m.report.Summary.Failure > 0 {
		helpText = "r retry failures  |  enter back to scan  |  q exit"
	} else {
		helpText = "enter back to scan  |  q exit"
	}
	return renderFooter("", helpText)
}
