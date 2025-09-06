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
        // Components mode selected – placeholder until components scan is implemented
        m.currentMode = modeComponents
        // For now, stay on picker with a status message.
        // Next iterations will trigger scanComponentsCmd and navigate to Components list.
        m.statusMsg = "Components-Modus ausgewählt – Implementierung folgt in den nächsten Schritten."
        return m, nil
    case "esc", "b":
        // Back to space select
        m.state = stateSpaceSelect
        m.statusMsg = "Zurück zur Space-Auswahl."
        return m, nil
    }
    return m, nil
}

