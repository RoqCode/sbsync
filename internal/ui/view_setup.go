package ui

import (
	"fmt"
	"strings"
)

func (m Model) viewWelcome() string {
	title := titleStyle.Render("🚀 Storyblok Sync")
	subtitle := subtitleStyle.Render("Synchronisiere Stories zwischen Spaces")

	var statusLines []string
	if m.cfg.Token != "" {
		statusLines = append(statusLines, okStyle.Render("✓ Token vorhanden"))
	} else {
		statusLines = append(statusLines, warnStyle.Render("⚠ Kein Token gefunden (~/.sbrc oder SB_TOKEN)"))
	}

	if m.hasSBRC {
		statusLines = append(statusLines, subtleStyle.Render(fmt.Sprintf("📁 Konfiguration: %s", m.sbrcPath)))
	}

	if m.statusMsg != "" {
		statusLines = append(statusLines, subtleStyle.Render(m.statusMsg))
	}

	content := fmt.Sprintf("%s\n%s\n\n%s",
		title,
		subtitle,
		strings.Join(statusLines, "\n"))

	boxContent := welcomeBoxStyle.Render(content)
	help := renderFooter("", "⌨️  Enter: weiter  •  q: beenden")

	return centeredStyle.Width(m.width).Render(boxContent) + "\n\n" +
		centeredStyle.Width(m.width).Render(help)
}

func (m Model) viewTokenPrompt() string {
	title := titleStyle.Render("🔑 Token Eingabe")
	prompt := subtitleStyle.Render("Bitte gib deinen Storyblok Management API Token ein")

	var errorMsg string
	if m.validateErr != nil {
		errorMsg = "\n" + errorStyle.Render("❌ "+m.validateErr.Error())
	}

	content := fmt.Sprintf("%s\n%s\n\n%s%s",
		title,
		prompt,
		m.ti.View(),
		errorMsg)

	boxContent := welcomeBoxStyle.Render(content)
	help := renderFooter("", "⌨️  Enter: bestätigen  •  Esc: zurück")

	return centeredStyle.Width(m.width).Render(boxContent) + "\n\n" +
		centeredStyle.Width(m.width).Render(help)
}

func (m Model) viewValidating() string {
	title := titleStyle.Render("⏳ Validierung läuft")
	content := fmt.Sprintf("%s\n\n%s %s",
		title,
		m.spinner.View(),
		subtitleStyle.Render("Validiere Token..."))

	boxContent := welcomeBoxStyle.Render(content)
	help := renderFooter("", "⌨️  q: abbrechen")

	return centeredStyle.Width(m.width).Render(boxContent) + "\n\n" +
		centeredStyle.Width(m.width).Render(help)
}

func (m Model) viewSpaceSelect() string {
	var header string
	if m.selectingSource {
		header = listHeaderStyle.Render("🎯 Wähle Source Space")
	} else {
		header = listHeaderStyle.Render("🎯 Wähle Target Space")
		if m.sourceSpace != nil {
			sourceInfo := subtleStyle.Render(fmt.Sprintf("✅ Source: %s (ID: %d)", m.sourceSpace.Name, m.sourceSpace.ID))
			header += "\n" + sourceInfo + "\n"
		}
	}

	var content strings.Builder
	visible := m.selectableSpaces()
	if len(visible) == 0 {
		content.WriteString(warnStyle.Render("❌ Keine Spaces gefunden"))
	} else {
		for i, sp := range visible {
			var line string
			spaceInfo := fmt.Sprintf("%s (ID: %d)", sp.Name, sp.ID)

			if i == m.selectedIndex {
				line = spaceSelectedStyle.Render("▶ " + spaceInfo)
			} else {
				line = spaceItemStyle.Render("  " + spaceInfo)
			}
			content.WriteString(line + "\n")
		}
	}

	// Create footer that sits at the bottom
	footer := renderFooter("", "⌨️  ↑↓/j/k: navigieren  •  Enter: auswählen  •  q: beenden")

	// Calculate available height for content (total height - header - footer - margins)
	contentHeight := m.height - 4 // rough estimate for header and footer space
	contentStr := content.String()

	// If content is shorter than available space, add padding
	lines := strings.Split(strings.TrimRight(contentStr, "\n"), "\n")
	if len(lines) < contentHeight-2 {
		for len(lines) < contentHeight-2 {
			lines = append(lines, "")
		}
		contentStr = strings.Join(lines, "\n")
	}

	return header + "\n" + contentStr + "\n" + footer
}

func (m Model) viewScanning() string {
	header := listHeaderStyle.Render("🔄 Scanne Stories")

	src := "(none)"
	tgt := "(none)"
	if m.sourceSpace != nil {
		src = fmt.Sprintf("%s (ID: %d)", m.sourceSpace.Name, m.sourceSpace.ID)
	}
	if m.targetSpace != nil {
		tgt = fmt.Sprintf("%s (ID: %d)", m.targetSpace.Name, m.targetSpace.ID)
	}

	content := fmt.Sprintf("%s %s\n\n", m.spinner.View(), subtitleStyle.Render("Lade Stories aus beiden Spaces..."))
	content += fmt.Sprintf("📂 Source: %s\n", okStyle.Render(src))
	content += fmt.Sprintf("📂 Target: %s\n", okStyle.Render(tgt))

	footer := renderFooter("", "⌨️  q: beenden")

	// Add padding to push footer to bottom
	contentHeight := m.height - 6 // space for header, content, and footer
	lines := strings.Split(strings.TrimRight(content, "\n"), "\n")
	for len(lines) < contentHeight {
		lines = append(lines, "")
	}
	paddedContent := strings.Join(lines, "\n")

	return header + "\n\n" + paddedContent + "\n" + footer
}

// viewCopyAsNew renders a full-screen step to choose a new slug and options.
func (m Model) viewCopyAsNew() string {
	// Current item context
	var item PreflightItem
	if m.copy.itemIdx >= 0 && m.copy.itemIdx < len(m.preflight.items) {
		item = m.preflight.items[m.copy.itemIdx]
	}

	title := titleStyle.Render("🍴 Kopie als neu anlegen")
	subtitle := subtitleStyle.Render("Kollision lösen: wähle neuen Slug und Optionen")

	var lines []string
	// Preview lines
	// Name preview (append (copy) if chosen)
	displayName := item.Story.Name
	if displayName == "" {
		displayName = item.Story.Slug
	}
	if m.copy.appendCopyToName && displayName != "" && !strings.HasSuffix(displayName, " (copy)") {
		displayName += " (copy)"
	}
	lines = append(lines, "Name: "+okStyle.Render(displayName))

	// Slug preview (parent + selected slug)
	selected := strings.TrimSpace(m.copy.input.Value())
	var fullSelected string
	if m.copy.parent != "" {
		fullSelected = m.copy.parent + "/" + selected
	} else {
		fullSelected = selected
	}
	lines = append(lines, "Slug: "+okStyle.Render(fullSelected))
	lines = append(lines, "")
	lines = append(lines, "Vorschläge:")
	// Presets list
	for i, p := range m.copy.presets {
		marker := "  "
		if i == m.copy.selectedPreset {
			marker = "> "
		}
		lines = append(lines, spaceItemStyle.Render(marker+p))
	}
	lines = append(lines, "")
	lines = append(lines, "Neuer Slug:")
	lines = append(lines, m.copy.input.View())
	lines = append(lines, "")
	// Checkbox: append (copy) to name
	chk := checkboxOff
	if m.copy.appendCopyToName {
		chk = checkboxOn
	}
	lines = append(lines, chk+" "+"Namen um \" (copy)\" ergänzen")
	if m.copy.errorMsg != "" {
		lines = append(lines, "")
		lines = append(lines, errorStyle.Render("❌ "+m.copy.errorMsg))
	}

	content := title + "\n" + subtitle + "\n\n" + strings.Join(lines, "\n")
	box := welcomeBoxStyle.Render(content)
	help := renderFooter("", "⌨️  ↑↓: Preset wählen  •  Tab: Feld wechseln  •  Space: Checkbox  •  Enter: übernehmen  •  Esc: zurück")
	return centeredStyle.Width(m.width).Render(box) + "\n\n" + centeredStyle.Width(m.width).Render(help)
}
