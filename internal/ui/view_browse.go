package ui

import (
	"fmt"
	"storyblok-sync/internal/sb"
	"strings"

	"github.com/charmbracelet/lipgloss"
	tree "github.com/charmbracelet/lipgloss/tree"
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
		b.WriteString(subtleStyle.Render("Prefix:" + m.filter.prefix))
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

	// Render tree lines
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
			content = cursorLineStyle.Width(m.width - 4).Render(content)
		} else {
			content = lipgloss.NewStyle().Width(m.width - 4).Render(content)
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

	b.WriteString(strings.Join(lines, "\n"))
	return b.String()
}

func (m Model) renderBrowseFooter() string {
	var b strings.Builder

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
	b.WriteString(subtleStyle.Render(fmt.Sprintf("Total: %d | Markiert: %d%s", total, checked, suffix)) + "\n")

	// Help text
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
