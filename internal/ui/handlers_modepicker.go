package ui

import (
	tea "github.com/charmbracelet/bubbletea"
)

// handleModePickerKey processes key input on the Sync Mode picker screen.
func (m Model) handleModePickerKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	key := msg.String()
	switch key {
	case "j", "down":
		if m.modePickerIndex < 1 { // 0: Stories, 1: Components
			m.modePickerIndex++
		}
	case "k", "up":
		if m.modePickerIndex > 0 {
			m.modePickerIndex--
		}
    case "enter":
        if m.modePickerIndex == 0 {
            m.currentMode = modeStories
            m.state = stateScanning
            m.statusMsg = "Scanne Stories…"
            return m, tea.Batch(m.spinner.Tick, m.scanStoriesCmd())
        }
        // Components mode selected – kick off components scan
        m.currentMode = modeComponents
        m.state = stateScanning
        m.statusMsg = "Scanne Components…"
        return m, tea.Batch(m.spinner.Tick, m.scanComponentsCmd())
	case "esc", "b":
		// Back to space select
		m.state = stateSpaceSelect
		m.statusMsg = "Zurück zur Space-Auswahl."
		return m, nil
	}
	return m, nil
}
