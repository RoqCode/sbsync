package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// removed unused viewReport (superseded by renderReport* functions)

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
	// Average stats panel for the whole sync run
	avg := m.renderReportAverages()
	if avg != "" {
		b.WriteString("\n")
		b.WriteString(avg)
	}
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

		// Show all successes; rely on viewport for scrolling
		if len(successes) > 0 {
			b.WriteString(successStyle.Render("✓ SUCCESSES") + "\n")
			for i := 0; i < len(successes); i++ {
				entry := successes[i]
				duration := fmt.Sprintf("%dms", entry.Duration)
				symbol := symbolStory
				if entry.TargetStory != nil && entry.TargetStory.IsFolder {
					symbol = symbolFolder
				}
				extra := fmt.Sprintf("  · rl:%d", entry.RateLimit429)
				b.WriteString(fmt.Sprintf("  %s %s (%s) %s%s\n",
					symbol, entry.Slug, entry.Operation, duration, extra))
			}
		}
	}

	return b.String()
}

func (m Model) renderReportFooter() string {
	var helpText string
	if m.report.Summary.Failure > 0 {
		helpText = "j/k scroll  |  pgup/pgdown blättern  |  r retry failures  |  enter/b back to scan  |  q exit"
	} else {
		helpText = "j/k scroll  |  pgup/pgdown blättern  |  enter/b back to scan  |  q exit"
	}
	return renderFooter("", helpText)
}

// renderReportAverages shows average Req/s, Read/s, Write/s, Succ/s, Warn%, Err% for the whole sync run
func (m Model) renderReportAverages() string {
	// Need non-zero duration
	durMs := m.report.Duration
	if durMs <= 0 {
		return ""
	}
	durSec := float64(durMs) / 1000.0

	// Successes per second: count successes and warnings (non-failures)
	succCount := float64(m.report.Summary.Success + m.report.Summary.Warning)
	succPerS := 0.0
	if durSec > 0 {
		succPerS = succCount / durSec
	}

	// Transport metrics (requests, read/write, status buckets)
	var reqPerS, readPerS, writePerS, warnPct, errPct float64
	if m.api != nil {
		snap := m.api.MetricsSnapshot()
		total := float64(snap.TotalRequests)
		if durSec > 0 {
			reqPerS = total / durSec
			readPerS = float64(snap.ReadRequests) / durSec
			writePerS = float64(snap.WriteRequests) / durSec
		}
		if total > 0 {
			warnPct = float64(snap.Status429) / total * 100.0
			errPct = float64(snap.Status5xx) / total * 100.0
		}
	}

	line := whiteTextStyle.Render(
		fmt.Sprintf(
			"Durchschnitt – Req/s: %.1f  Read/s: %.1f  Write/s: %.1f  Succ/s: %.1f  Warn: %.1f%%  Err: %.1f%%",
			reqPerS, readPerS, writePerS, succPerS, warnPct, errPct,
		),
	)
	return line
}
