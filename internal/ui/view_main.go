package ui

import (
	"strings"
)

func (m Model) View() string {
	if m.state == stateQuit {
		return ""
	}
	var b strings.Builder
	m.renderTitle(&b)
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
	case stateBrowseList:
		b.WriteString(m.viewBrowseList())
	case statePreflight:
		b.WriteString(m.viewPreflight())
	case stateReport:
		b.WriteString(m.viewReport())
	}
	m.renderFooter(&b)
	return b.String()
}

func (m Model) renderTitle(b *strings.Builder) {
	b.WriteString(titleStyle.Render("Storyblok Sync (TUI)"))
	b.WriteString("\n")
	b.WriteString(dividerStyle.Render(strings.Repeat("─", max(10, m.width-2))))
	b.WriteString("\n\n")
}

func (m Model) renderFooter(b *strings.Builder) {
	b.WriteString("\n")
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
