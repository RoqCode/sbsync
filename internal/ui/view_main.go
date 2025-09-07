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
	case stateBrowseList, statePreflight, stateSync, stateReport:
		stateHeader := m.renderStateHeader()
		content := m.renderViewportContent()
		return lipgloss.JoinVertical(lipgloss.Left, header, stateHeader, content, footer)
	case stateCompList:
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
		case stateModePicker:
			b.WriteString(m.viewModePicker())
		case stateCopyAsNew:
			b.WriteString(m.viewCopyAsNew())
		case stateFolderFork:
			b.WriteString(m.viewFolderFork())
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
	case stateCompList:
		return m.renderCompBrowseFooter()
	case statePreflight:
		return m.renderPreflightFooter()
	case stateSync:
		return m.renderSyncFooter()
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
	case stateCompList:
		return m.renderCompBrowseHeader()
	case statePreflight:
		return m.renderPreflightHeader()
	case stateSync:
		return m.renderSyncHeader()
	case stateReport:
		return m.renderReportHeader()
	case stateCopyAsNew:
		return listHeaderStyle.Render("🍴 Copy as new – Kollision lösen")
	case stateFolderFork:
		return listHeaderStyle.Render("🍴 Ordner-Fork – Ordnerbaum kopieren")
	default:
		return ""
	}
}

func (m *Model) updateViewportContent() {
	switch m.state {
	case stateBrowseList:
		m.updateBrowseViewport()
	case stateCompList:
		m.updateCompBrowseViewport()
	case statePreflight:
		m.updatePreflightViewport()
	case stateSync:
		m.updateSyncViewport()
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
