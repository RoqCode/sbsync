package ui

import (
	"fmt"
	"storyblok-sync/internal/sb"
	"strings"

	"github.com/charmbracelet/lipgloss"
	tree "github.com/charmbracelet/lipgloss/tree"
)

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
