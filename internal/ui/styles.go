package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// --- UI Styles ---
var (
	titleStyle       = lipgloss.NewStyle().Bold(true).Underline(true)
	subtleStyle      = lipgloss.NewStyle().Faint(true)
	okStyle          = lipgloss.NewStyle().Bold(true)
	warnStyle        = lipgloss.NewStyle().Bold(true)
	helpStyle        = lipgloss.NewStyle().Faint(true)
	dividerStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	focusStyle       = lipgloss.NewStyle().Bold(true)
	cursorLineStyle  = lipgloss.NewStyle().Background(lipgloss.Color("#2A2B3D"))
	cursorBarStyle   = lipgloss.NewStyle().Background(lipgloss.Color("#FFAB78"))
	markBarStyle     = lipgloss.NewStyle().Background(lipgloss.Color("#3AC4BA"))
	markNestedStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#3AC4BA"))
	collisionSign    = lipgloss.NewStyle().Foreground(lipgloss.Color("11")).Render("!")
	stateCreateStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	stateUpdateStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("11"))
	stateSkipStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	stateDoneStyle   = lipgloss.NewStyle().Background(lipgloss.Color("10")).Foreground(lipgloss.Color("0")).Bold(true)

	// markers for different story types (colored squares)
	symbolStory  = fgSymbol("#8942E1", "S")
	symbolFolder = fgSymbol("#3AC4BA", "F")
	symbolRoot   = fgSymbol("214", "R")
)

var stateStyles = map[SyncState]lipgloss.Style{
	StateCreate: stateCreateStyle,
	StateUpdate: stateUpdateStyle,
	StateSkip:   stateSkipStyle,
}

func fgSymbol(col, ch string) string {
	s := lipgloss.NewStyle().Foreground(lipgloss.Color(col)).Render(ch)
	const reset = "\x1b[0m"
	return strings.TrimSuffix(s, reset) + "\x1b[39m"
}
