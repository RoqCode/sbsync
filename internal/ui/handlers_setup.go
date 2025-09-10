package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"storyblok-sync/internal/infra/logx"
)

// ---------- Setup Screen Handlers ----------

func (m Model) handleWelcomeKey(key string) (Model, tea.Cmd) {
	switch key {
	case "enter":
		if m.cfg.Token == "" {
			m.state = stateTokenPrompt
			m.statusMsg = "Bitte gib deinen Token ein."
			return m, nil
		}
		m.state = stateValidating
		m.statusMsg = "Validiere Token…"
		return m, tea.Batch(m.spinner.Tick, m.validateTokenCmd())
	}
	return m, nil
}

func (m Model) handleTokenPromptKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	key := msg.String()
	switch key {
	case "esc":
		m.state = stateWelcome
		m.statusMsg = "Zurück zum Welcome."
		return m, nil
	case "enter":
		m.cfg.Token = strings.TrimSpace(m.ti.Value())
		if m.cfg.Token == "" {
			m.statusMsg = "Token leer."
			return m, nil
		}
		// Register newly entered token for log redaction
		logx.RegisterSecret(m.cfg.Token)
		m.state = stateValidating
		m.statusMsg = "Validiere Token…"
		return m, tea.Batch(m.spinner.Tick, m.validateTokenCmd())
	default:
		var cmd tea.Cmd
		m.ti, cmd = m.ti.Update(msg)
		return m, cmd
	}
}

func (m Model) handleValidatingKey(key string) (Model, tea.Cmd) {
	return m, nil
}

func (m Model) handleSpaceSelectKey(key string) (Model, tea.Cmd) {
	// Work off the selectable list (filters out source during target pick)
	visible := m.selectableSpaces()

	switch key {
	case "j", "down":
		if m.selectedIndex < len(visible)-1 {
			m.selectedIndex++
		}
	case "k", "up":
		if m.selectedIndex > 0 {
			m.selectedIndex--
		}
	case "enter":
		if len(visible) == 0 {
			return m, nil
		}
		chosen := visible[m.selectedIndex]
		if m.selectingSource {
			// Source wählen; Target-Auswahl vorbereiten
			m.sourceSpace = &chosen
			m.selectingSource = false
			m.statusMsg = fmt.Sprintf("Source gesetzt: %s (%d). Wähle jetzt Target.", chosen.Name, chosen.ID)
			// Reset index, as the visible list will change (source removed)
			m.selectedIndex = 0
		} else {
			m.targetSpace = &chosen
			m.statusMsg = fmt.Sprintf("Target gesetzt: %s (%d). Wähle Sync-Modus…", chosen.Name, chosen.ID)
			m.state = stateModePicker
			m.modePickerIndex = 0
			return m, nil
		}
	}
	return m, nil
}

func (m Model) handleScanningKey(key string) (Model, tea.Cmd) {
	// Platzhalter – später starten wir hier den echten Scan und wechseln nach BrowseList.
	return m, nil
}
