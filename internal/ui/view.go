package ui

import (
	"fmt"
	"storyblok-sync/internal/sb"
	"strings"

	"github.com/charmbracelet/lipgloss"
	tree "github.com/charmbracelet/lipgloss/tree"
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
			// Sammle sichtbare Stories
			stories := make([]sb.Story, total)
			for i := 0; i < total; i++ {
				stories[i] = m.itemAt(i)
			}

			// Erzeuge Tree-Struktur
			tr := tree.New()
			nodes := make(map[int]*tree.Tree, len(stories))
			for _, st := range stories {
				node := tree.Root(displayStory(st))
				nodes[st.ID] = node
			}

			for _, st := range stories {
				node := nodes[st.ID]
				if st.FolderID != nil {
					if parent, ok := nodes[*st.FolderID]; ok {
						parent.Child(node)
						continue
					}
				}
				tr.Child(node)
			}

			// Begrenze Ausgabe auf sichtbaren Bereich
			lines := strings.Split(tr.String(), "\n")
			if len(lines) > 0 && lines[len(lines)-1] == "" {
				lines = lines[:len(lines)-1]
			}

			for i, st := range stories {
				if i >= len(lines) {
					break
				}

				content := lines[i]
				if i == m.selection.listIndex {
					content = cursorLineStyle.Width(m.width - 2).Render(content)
				} else {
					content = lipgloss.NewStyle().Width(m.width - 2).Render(content)
				}

				cursorCell := " "
				if i == m.selection.listIndex {
					cursorCell = cursorBarStyle.Render(" ")
				}

				markCell := " "
				if m.selection.selected[st.FullSlug] {
					markCell = markBarStyle.Render(" ")
				} else if st.IsFolder {
					if m.hasSelectedDirectChild(st.FullSlug) {
						markCell = markNestedStyle.Render(":")
					} else if m.hasSelectedDescendant(st.FullSlug) {
						markCell = markNestedStyle.Render("·")
					}
				}

				lines[i] = cursorCell + markCell + content
			}

			start := m.selection.listOffset
			if start > len(lines) {
				start = len(lines)
			}
			end := start + m.selection.listViewport
			if end > len(lines) {
				end = len(lines)
			}
			b.WriteString(strings.Join(lines[start:end], "\n"))
			b.WriteString("\n")

			shown := end - start

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
	b.WriteString(helpStyle.Render("j/k bewegen  |  h/l falten  |  H alles zu  |  L alles auf  |  space Story markieren  |  r rescan  |  s preflight  |  q beenden") + "\n")
	b.WriteString(helpStyle.Render("p Prefix  |  P Prefix löschen  |  f suchen |  F Suche löschen  |  c Filter löschen  |  Enter schließen  |  Esc löschen/zurück"))
	return b.String()
}

func (m Model) viewPreflight() string {
	var b strings.Builder
	total := len(m.preflight.items)
	collisions := 0
	for _, it := range m.preflight.items {
		if it.Collision {
			collisions++
		}
	}
	b.WriteString(fmt.Sprintf("Preflight – %d Items  |  Kollisionen: %d\n\n", total, collisions))
	if total == 0 {
		b.WriteString(warnStyle.Render("Keine Stories markiert.") + "\n")
	} else {
		tr := tree.New()
		nodes := make(map[int]*tree.Tree, len(m.preflight.items))
		for _, it := range m.preflight.items {
			node := tree.Root(displayPreflightItem(it))
			nodes[it.Story.ID] = node
		}
		for _, it := range m.preflight.items {
			node := nodes[it.Story.ID]
			if it.Story.FolderID != nil {
				if parent, ok := nodes[*it.Story.FolderID]; ok {
					parent.Child(node)
					continue
				}
			}
			tr.Child(node)
		}
		lines := strings.Split(tr.String(), "\n")
		if len(lines) > 0 && lines[len(lines)-1] == "" {
			lines = lines[:len(lines)-1]
		}
		for i, it := range m.preflight.items {
			if i >= len(lines) {
				break
			}
			content := lines[i]
			if it.Collision {
				content = collisionSign + " " + content
			} else {
				content = "  " + content
			}
			lineStyle := lipgloss.NewStyle().Width(m.width - 2)
			if i == m.preflight.listIndex {
				lineStyle = cursorLineStyle.Copy().Width(m.width - 2)
			}
			if it.State == StateSkip {
				lineStyle = lineStyle.Faint(true)
			}
			content = lineStyle.Render(content)
			cursorCell := " "
			if i == m.preflight.listIndex {
				cursorCell = cursorBarStyle.Render(" ")
			}
			stateCell := " "
			switch it.Run {
			case RunRunning:
				stateCell = m.spinner.View()
			case RunDone:
				stateCell = stateDoneStyle.Render(string(it.State))
			case RunCancelled:
				stateCell = lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Background(lipgloss.Color("0")).Bold(true).Render("X")
			default:
				if it.State != "" {
					if st, ok := stateStyles[it.State]; ok {
						stateCell = st.Render(string(it.State))
					} else {
						stateCell = string(it.State)
					}
				}
			}
			lines[i] = cursorCell + stateCell + content
		}
		start := m.preflight.listOffset
		if start > len(lines) {
			start = len(lines)
		}
		end := start + m.preflight.listViewport
		if end > len(lines) {
			end = len(lines)
		}
		b.WriteString(strings.Join(lines[start:end], "\n"))
		b.WriteString("\n")
	}
	b.WriteString("\n")
	if m.syncing {
		b.WriteString(renderProgress(m.syncIndex, len(m.preflight.items), m.width-2))
		b.WriteString("\n\n")
	}
	if m.syncing {
		b.WriteString(helpStyle.Render("Syncing... | Ctrl+C to cancel"))
	} else {
		b.WriteString(helpStyle.Render("j/k bewegen  |  x skip  |  X alle skippen  |  c Skips entfernen  |  Enter OK  |  esc/q zurück"))
	}
	return b.String()
}

func displayPreflightItem(it PreflightItem) string {
	name := it.Story.Name
	if name == "" {
		name = it.Story.Slug
	}
	slug := "(" + it.Story.FullSlug + ")"
	if !it.Selected || it.State == StateSkip {
		name = subtleStyle.Render(name)
		slug = subtleStyle.Render(slug)
	}
	sym := storyTypeSymbol(it.Story)
	return fmt.Sprintf("%s %s  %s", sym, name, slug)
}

func renderProgress(done, total, width int) string {
	if total <= 0 {
		return ""
	}
	if width <= 0 {
		width = 20
	}
	filled := int(float64(done) / float64(total) * float64(width))
	if filled > width {
		filled = width
	}
	return "[" + strings.Repeat("#", filled) + strings.Repeat("-", width-filled) + fmt.Sprintf("] %d/%d", done, total)
}

func displayStory(st sb.Story) string {
	name := st.Name
	if name == "" {
		name = st.Slug
	}
	sym := storyTypeSymbol(st)
	return fmt.Sprintf("%s %s  (%s)", sym, name, st.FullSlug)
}

func storyTypeSymbol(st sb.Story) string {
	switch {
	case st.IsFolder:
		return symbolFolder
	case st.IsStartpage:
		return symbolRoot
	default:
		return symbolStory
	}
}

func (m Model) viewReport() string {
	var b strings.Builder

	// Header with sync summary
	totalDuration := float64(m.report.Duration) / 1000.0
	b.WriteString(fmt.Sprintf("Sync Report – %s\n", m.report.GetDisplaySummary()))
	b.WriteString(fmt.Sprintf("Duration: %.2fs  |  Source: %s  |  Target: %s\n\n",
		totalDuration, m.report.SourceSpace, m.report.TargetSpace))

	// Statistics section with colored boxes
	successStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Bold(true)
	warningStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("11")).Bold(true)
	errorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Bold(true)

	statsBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("240")).
		Padding(1, 2).
		Margin(0, 1)

	// Create stats content
	var stats strings.Builder
	stats.WriteString(successStyle.Render(fmt.Sprintf("✓ %d Success", m.report.Summary.Success)))
	if m.report.Summary.Warning > 0 {
		stats.WriteString("  " + warningStyle.Render(fmt.Sprintf("⚠ %d Warnings", m.report.Summary.Warning)))
	}
	if m.report.Summary.Failure > 0 {
		stats.WriteString("  " + errorStyle.Render(fmt.Sprintf("✗ %d Failures", m.report.Summary.Failure)))
	}
	stats.WriteString(fmt.Sprintf("\n%d Created  |  %d Updated  |  %d Skipped",
		m.report.Summary.Created, m.report.Summary.Updated, m.report.Summary.Skipped))

	b.WriteString(statsBox.Render(stats.String()))
	b.WriteString("\n\n")

	if len(m.report.Entries) == 0 {
		b.WriteString(subtleStyle.Render("No entries in report.") + "\n")
	} else {
		// Group entries by status for better organization
		var successes, warnings, failures []ReportEntry
		for _, entry := range m.report.Entries {
			switch entry.Status {
			case "success":
				successes = append(successes, entry)
			case "warning":
				warnings = append(warnings, entry)
			case "failure":
				failures = append(failures, entry)
			}
		}

		// Show failures first (most important)
		if len(failures) > 0 {
			b.WriteString(errorStyle.Render("⚠ FAILURES") + "\n")
			for _, entry := range failures {
				duration := fmt.Sprintf("%dms", entry.Duration)
				b.WriteString(fmt.Sprintf("  %s %s (%s) %s - %s\n",
					symbolStory, entry.Slug, entry.Operation, duration, entry.Error))
			}
			b.WriteString("\n")
		}

		// Show warnings next
		if len(warnings) > 0 {
			b.WriteString(warningStyle.Render("⚠ WARNINGS") + "\n")
			for _, entry := range warnings {
				duration := fmt.Sprintf("%dms", entry.Duration)
				b.WriteString(fmt.Sprintf("  %s %s (%s) %s - %s\n",
					symbolStory, entry.Slug, entry.Operation, duration, entry.Warning))
			}
			b.WriteString("\n")
		}

		// Show recent successes (limit to avoid clutter)
		if len(successes) > 0 {
			b.WriteString(successStyle.Render("✓ SUCCESSES") + " ")
			if len(successes) > 10 {
				b.WriteString(subtleStyle.Render(fmt.Sprintf("(showing last 10 of %d)", len(successes))))
			}
			b.WriteString("\n")

			start := len(successes) - 10
			if start < 0 {
				start = 0
			}
			for i := start; i < len(successes); i++ {
				entry := successes[i]
				duration := fmt.Sprintf("%dms", entry.Duration)
				symbol := symbolStory
				if entry.TargetStory != nil && entry.TargetStory.IsFolder {
					symbol = symbolFolder
				}
				b.WriteString(fmt.Sprintf("  %s %s (%s) %s\n",
					symbol, entry.Slug, entry.Operation, duration))
			}
		}
	}

	b.WriteString("\n")

	// Footer with actions
	if m.report.Summary.Failure > 0 {
		b.WriteString(helpStyle.Render("r retry failures  |  enter back to scan  |  q exit"))
	} else {
		b.WriteString(helpStyle.Render("enter back to scan  |  q exit"))
	}

	return b.String()
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
