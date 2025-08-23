package ui

import (
	"fmt"
	"strings"

	"storyblok-sync/internal/sb"
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
	b.WriteString("Validiere Token…\n\n")
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
	b.WriteString("Scanning…\n\n")
	b.WriteString(fmt.Sprintf("Source: %s\nTarget: %s\n\n", src, tgt))
	b.WriteString(subtleStyle.Render("Hole Stories (flach)…") + "\n\n")
	b.WriteString(helpStyle.Render("q beenden"))
	return b.String()
}

func (m Model) viewBrowseList() string {
	var b strings.Builder
	srcCount := len(m.storiesSource)
	tgtCount := len(m.storiesTarget)
	b.WriteString(fmt.Sprintf("Browse (Source Stories) – %d Items  |  Target: %d\n\n", srcCount, tgtCount))
	if srcCount == 0 {
		b.WriteString(warnStyle.Render("Keine Stories im Source gefunden.") + "\n")
	} else {
		// sichtbaren Bereich bestimmen
		total := m.itemsLen()
		start := m.selection.listOffset
		end := min(start+m.selection.listViewport, total)

		// Suchleiste (falls aktiv oder Query gesetzt)
		label := "Suche: "
		if m.search.searching {
			b.WriteString(label + m.search.searchInput.View() + "  |  ")
		} else {
			b.WriteString(label + m.search.query + "  |  ")
		}

		if m.filter.prefixing {
			b.WriteString("Prefix: " + m.filter.prefixInput.View() + "\n\n")
		} else {
			b.WriteString(subtleStyle.Render("Prefix:"+m.filter.prefix) + "\n\n")
		}

		if total == 0 {
			b.WriteString(warnStyle.Render("Keine Stories gefunden (Filter aktiv?).") + "\n")
		} else {
			for i := start; i < end; i++ {
				st := m.itemAt(i)
				cursor := "  "
				if i == m.selection.listIndex {
					cursor = "▶ "
				}

				mark := "[ ]"
				if m.selection.selected[st.FullSlug] {
					mark = "[x]"
				}

				line := fmt.Sprintf("%s%s  %s", cursor, mark, displayStory(st))
				if i == m.selection.listIndex {
					line = selStyle.Render(line)
				}
				b.WriteString(line + "\n")
			}
		}

		shown := 0
		if end > start {
			shown = end - start
		}

		// Anzeige: Filterstatus + Range
		suffix := ""
		if m.search.filteredIdx != nil {
			suffix = fmt.Sprintf("  |  gefiltert: %d", total)
		}
		b.WriteString("\n")
		b.WriteString(subtleStyle.Render(
			fmt.Sprintf("Zeilen %d–%d von %d (sichtbar: %d)%s",
				start+1, end, total, shown, suffix),
		))
		b.WriteString("\n")
	}
	// Footer / Hilfe
	checked := 0
	for _, v := range m.selection.selected {
		if v {
			checked++
		}
	}
	b.WriteString("\n")
	b.WriteString(subtleStyle.Render(fmt.Sprintf("Markiert: %d", checked)) + "\n")
	b.WriteString(helpStyle.Render("j/k bewegen  |  space Story markieren  |  r rescan  |  s preflight  |  q beenden") + "\n")
	b.WriteString(helpStyle.Render("p Prefix  |  P Prefix löschen  |  f suchen |  F Suche löschen  |  c Filter löschen  |  Enter schließen  |  Esc löschen/zurück"))
	return b.String()
}

func displayStory(st sb.Story) string {
	name := st.Name
	if name == "" {
		name = st.Slug
	}
	return fmt.Sprintf("%s  (%s)", name, st.FullSlug)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
