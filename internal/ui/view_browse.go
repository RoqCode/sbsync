package ui

import (
	"fmt"
	"storyblok-sync/internal/sb"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func (m *Model) updateBrowseViewport() {
	content := m.renderBrowseContent()
	m.viewport.SetContent(content)
}

func (m Model) renderBrowseHeader() string {
	srcCount := len(m.storiesSource)
	tgtCount := len(m.storiesTarget)

	var b strings.Builder
	b.WriteString(fmt.Sprintf("Browse (Source Stories) – %d Items  |  Target: %d\n", srcCount, tgtCount))

	// Search and filter status
	label := "Suche: "
	if m.search.searching {
		b.WriteString(label + m.search.searchInput.View() + "  |  ")
	} else {
		b.WriteString(label + m.search.query + "  |  ")
	}

	if m.filter.prefixing {
		b.WriteString("Prefix: " + m.filter.prefixInput.View())
	} else {
		if m.filter.prefix != "" {
			b.WriteString("Prefix:" + m.filter.prefix)
		} else {
			b.WriteString(subtleStyle.Render("Prefix:"))
		}
	}

	return b.String()
}

func (m Model) renderBrowseContent() string {
	var b strings.Builder
	srcCount := len(m.storiesSource)

	if srcCount == 0 {
		b.WriteString(warnStyle.Render("Keine Stories im Source gefunden.") + "\n")
		return b.String()
	}

	total := m.itemsLen()
	if total == 0 {
		b.WriteString(warnStyle.Render("Keine Stories gefunden (Filter aktiv?).") + "\n")
		return b.String()
	}

	// Sammle sichtbare Stories (shared helper)
	stories, _ := m.visibleOrderBrowse()

	// Erzeuge Tree-Struktur (shared helper)
	lines := generateTreeLinesFromStories(stories)

	for i, st := range stories {
		if i >= len(lines) {
			break
		}

		content := lines[i]
		if i == m.selection.listIndex {
			content = cursorLineStyle.Width(m.width - 4).Render(content)
		} else {
			content = lipgloss.NewStyle().Width(m.width - 4).Render(content)
		}

		cursorCell := " "
		if i == m.selection.listIndex {
			cursorCell = cursorBarStyle.Render(" ")
		}

		// Preflight has a stateCell; for consistency, browse also reserves a state cell.
		// Here we render selection mark into that cell and keep same width budget.
		stateCell := " "
		if m.selection.selected[st.FullSlug] {
			stateCell = markBarStyle.Render(" ")
		} else if st.IsFolder {
			if m.hasSelectedDirectChild(st.FullSlug) {
				stateCell = markNestedStyle.Render(":")
			} else if m.hasSelectedDescendant(st.FullSlug) {
				stateCell = markNestedStyle.Render("·")
			}
		}

		lines[i] = cursorCell + stateCell + content
	}

	b.WriteString(strings.Join(lines, "\n"))
	return b.String()
}

func (m Model) renderBrowseFooter() string {
	total := m.itemsLen()
	checked := 0
	for _, v := range m.selection.selected {
		if v {
			checked++
		}
	}

	// Status info
	suffix := ""
	if m.search.filteredIdx != nil {
		suffix = fmt.Sprintf("  |  gefiltert: %d", total)
	}
	statusLine := fmt.Sprintf("Total: %d | Markiert: %d%s", total, checked, suffix)

	return renderFooter(
		statusLine,
		"j/k bewegen  |  h/l falten  |  H alles zu  |  L alles auf  |  space Story markieren  |  r rescan  |  s preflight  |  m Modus  |  q beenden",
		"p Prefix  |  P Prefix löschen  |  f suchen |  F Suche löschen  |  c Filter löschen  |  Enter schließen  |  Esc löschen/zurück",
	)
}

// Components footer mirrors browse footer but with components-specific help
func (m Model) renderCompBrowseFooter() string {
	total := len(m.componentsSource)
	checked := 0
	for _, v := range m.comp.selected {
		if v {
			checked++
		}
	}
	statusLine := fmt.Sprintf("Total: %d | Markiert: %d", total, checked)
	return renderFooter(
		statusLine,
		"j/k bewegen  |  pgup/pgdown blättern  |  space markieren  |  s preflight  |  t sort  |  o Richtung  |  d Cutoff  |  m Modus  |  q beenden",
		"f suchen (ein/aus)",
	)
}

// Components Preflight header/footer
func (m Model) renderCompPreflightHeader() string {
    total := len(m.compPre.items)
    coll := countCompCollisions(m.compPre.items)
    forced := "Aus"
    if m.compPre.forceUpdateAll {
        forced = "An"
    }
    return listHeaderStyle.Render(
        fmt.Sprintf("Preflight (Components) – %d Items  |  Kollisionen: %d  |  Force-Update: %s", total, coll, forced),
    )
}

func (m Model) renderCompPreflightFooter() string {
	// Count states
	cCreate, cUpdate, cSkip := 0, 0, 0
	for _, it := range m.compPre.items {
		switch it.State {
		case StateCreate:
			cCreate++
		case StateUpdate:
			cUpdate++
		case StateSkip:
			cSkip++
		}
	}
	status := fmt.Sprintf("create:%d update:%d skip:%d", cCreate, cUpdate, cSkip)
    return renderFooter(status,
        "j/k bewegen  |  space Skip/Apply  |  f Fork (umbenennen)  |  u Force-Update (Presets)  |  Enter Anwenden  |  b/Esc zurück",
        "Enter beendet Umbenennen | Esc abbrechen",
    )
}

func displayStory(st sb.Story) string {
	name := st.Name
	if name == "" {
		name = st.Slug
	}
	sym := storyTypeSymbol(st)
	// Simple source-state badge (hint only)
	badge := "[Draft]"
	if st.Published {
		badge = "[Pub]"
	}
	return fmt.Sprintf("%s %s %s  (%s)", sym, name, helpStyle.Render(badge), st.FullSlug)
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
