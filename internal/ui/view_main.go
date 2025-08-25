package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func (m Model) View() string {
	if m.state == stateQuit {
		return ""
	}

	header := m.renderHeader()
	footer := m.renderFooter()

	// States that use viewport
	switch m.state {
	case stateBrowseList, statePreflight, stateReport:
		stateHeader := m.renderStateHeader()
		content := m.renderViewportContent()
		return lipgloss.JoinVertical(lipgloss.Left, header, stateHeader, content, footer)
	default:
		// States that don't use viewport (full-screen content)
		var b strings.Builder
		switch m.state {
		case stateWelcome:
			b.WriteString(m.viewWelcome())
		case stateTokenPrompt:
			b.WriteString(m.viewTokenPrompt())
		case stateValidating:
			b.WriteString(m.viewValidating())
		case stateSpaceSelect:
			b.WriteString(m.viewSpaceSelect())
		case stateScanning:
			b.WriteString(m.viewScanning())
		}
		return lipgloss.JoinVertical(lipgloss.Left, header, b.String(), footer)
	}
}

func (m Model) renderHeader() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Storyblok Sync (TUI)"))
	b.WriteString("\n")
	b.WriteString(dividerStyle.Render(strings.Repeat("─", max(10, m.width-2))))
	b.WriteString("\n")
	return b.String()
}

func (m Model) renderFooter() string {
	switch m.state {
	case stateBrowseList:
		return m.renderBrowseFooter()
	case statePreflight:
		return m.renderPreflightFooter()
	case stateReport:
		return m.renderReportFooter()
	default:
		return ""
	}
}

func (m Model) renderStateHeader() string {
	switch m.state {
	case stateBrowseList:
		return m.renderBrowseHeader()
	case statePreflight:
		return m.renderPreflightHeader()
	case stateReport:
		return m.renderReportHeader()
	default:
		return ""
	}
}

func (m *Model) updateViewportContent() {
	switch m.state {
	case stateBrowseList:
		m.updateBrowseViewport()
	case statePreflight:
		m.updatePreflightViewport()
	case stateReport:
		m.updateReportViewport()
	}
}

func (m Model) renderViewportContent() string {
	return m.viewport.View()
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
