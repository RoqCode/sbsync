package ui

import (
	"fmt"
	"strings"
)

func (m Model) viewWelcome() string {
	var b strings.Builder
	b.WriteString("Willkommen! Diese App hilft dir, Stories zwischen Spaces zu synchronisieren.\n\n")
	if m.cfg.Token != "" {
		b.WriteString(okStyle.Render("✓ Token vorhanden") + "\n")
	} else {
		b.WriteString(warnStyle.Render("! Kein Token gefunden (~/.sbrc oder SB_TOKEN)") + "\n")
	}
	if m.hasSBRC {
		b.WriteString(subtleStyle.Render(fmt.Sprintf("Konfiguration gefunden: %s", m.sbrcPath)) + "\n")
	}
	b.WriteString(subtleStyle.Render(m.statusMsg) + "\n\n")
	b.WriteString(helpStyle.Render("Tasten: Enter weiter  |  q beenden"))
	return b.String()
}

func (m Model) viewTokenPrompt() string {
	var b strings.Builder
	b.WriteString("Bitte gib deinen Storyblok Token ein:\n\n")
	b.WriteString(m.ti.View() + "\n\n")
	if m.validateErr != nil {
		b.WriteString(warnStyle.Render(m.validateErr.Error()) + "\n\n")
	}
	b.WriteString(helpStyle.Render("Enter bestätigen  |  Esc zurück"))
	return b.String()
}

func (m Model) viewValidating() string {
	var b strings.Builder
	b.WriteString(m.spinner.View() + " Validiere Token…\n\n")
	b.WriteString(helpStyle.Render("q abbrechen"))
	return b.String()
}

func (m Model) viewSpaceSelect() string {
	var b strings.Builder
	if m.selectingSource {
		b.WriteString("Spaces (wähle **Source**):\n\n")
	} else {
		b.WriteString("Spaces (wähle **Target**):\n\n")
		if m.sourceSpace != nil {
			b.WriteString(subtleStyle.Render(fmt.Sprintf("Source: %s (%d)", m.sourceSpace.Name, m.sourceSpace.ID)) + "\n\n")
		}
	}
	if len(m.spaces) == 0 {
		b.WriteString(warnStyle.Render("Keine Spaces gefunden.") + "\n")
	} else {
		for i, sp := range m.spaces {
			cursor := "  "
			if i == m.selectedIndex {
				cursor = "> "
			}
			line := fmt.Sprintf("%s%s (id %d)", cursor, sp.Name, sp.ID)
			if i == m.selectedIndex {
				line = focusStyle.Render(line)
			}
			b.WriteString(line + "\n")
		}
	}
	b.WriteString("\n" + helpStyle.Render("j/k bewegen  |  Enter wählen  |  q beenden"))
	return b.String()
}

func (m Model) viewScanning() string {
	var b strings.Builder
	src := "(none)"
	tgt := "(none)"
	if m.sourceSpace != nil {
		src = fmt.Sprintf("%s (%d)", m.sourceSpace.Name, m.sourceSpace.ID)
	}
	if m.targetSpace != nil {
		tgt = fmt.Sprintf("%s (%d)", m.targetSpace.Name, m.targetSpace.ID)
	}
	b.WriteString(m.spinner.View() + " Scanning…\n\n")
	b.WriteString(fmt.Sprintf("Source: %s\nTarget: %s\n\n", src, tgt))
	b.WriteString(subtleStyle.Render("Hole Stories (flach)…") + "\n\n")
	b.WriteString(helpStyle.Render("q beenden"))
	return b.String()
}
