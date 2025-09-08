package ui

import (
	"fmt"
	"sort"
	"storyblok-sync/internal/sb"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// updateCompBrowseViewport renders the components list into the viewport
func (m *Model) updateCompBrowseViewport() {
	content := m.renderCompBrowseContent()
	m.viewport.SetContent(content)
}

func (m Model) renderCompBrowseHeader() string {
	srcCount := len(m.componentsSource)
	tgtCount := len(m.componentsTarget)
	var b strings.Builder
	b.WriteString(fmt.Sprintf("Browse (Components) – %d Items  |  Target: %d\n", srcCount, tgtCount))
	// Search and filters summary
	b.WriteString("Suche: ")
	if m.comp.inputMode == "search" {
		b.WriteString(m.comp.search.input.View())
	} else {
		b.WriteString(m.comp.search.query)
	}
	b.WriteString("  |  Sort: ")
	switch m.comp.sortKey {
	case compSortUpdated:
		b.WriteString("updated")
	case compSortCreated:
		b.WriteString("created")
	default:
		b.WriteString("name")
	}
	if m.comp.sortAsc {
		b.WriteString("↑")
	} else {
		b.WriteString("↓")
	}
	if m.comp.inputMode == "date" {
		b.WriteString("  |  Cutoff: ")
		b.WriteString(m.comp.dateInput.View())
	} else {
		if !m.comp.dateCutoff.IsZero() {
			b.WriteString("  |  Cutoff: " + m.comp.dateCutoff.Format("2006-01-02"))
		}
	}
	if m.comp.group != "" {
		b.WriteString("  |  Group: " + m.comp.group)
	}
	return b.String()
}

func (m Model) renderCompBrowseContent() string {
	lines := m.visibleCompLines()
	return strings.Join(lines, "\n")
}

type compLine struct {
	content  string // rendered content (without cursor/mark cells)
	name     string // component name
	selected bool
}

// visibleCompModel builds a flat list of components (no grouping)
func (m Model) visibleCompModel() []compLine {
	// Build group name map (for optional filtering)
	groupName := make(map[string]string)
	for _, g := range m.componentGroupsSource {
		groupName[g.UUID] = g.Name
	}

	// Filter and collect
	items := make([]sb.Component, 0, len(m.componentsSource))
	for _, c := range m.componentsSource {
		if m.comp.search.query != "" && !strings.Contains(strings.ToLower(c.Name), strings.ToLower(m.comp.search.query)) {
			continue
		}
		if !m.comp.dateCutoff.IsZero() {
			if mostRecentTime(c).Before(m.comp.dateCutoff) {
				continue
			}
		}
		if m.comp.group != "" {
			if gn := groupName[c.ComponentGroupUUID]; gn != m.comp.group {
				continue
			}
		}
		items = append(items, c)
	}
	// Sort
	sort.Slice(items, func(i, j int) bool {
		a, b := items[i], items[j]
		var less bool
		switch m.comp.sortKey {
		case compSortCreated:
			less = parseTime(a.CreatedAt).Before(parseTime(b.CreatedAt))
		case compSortName:
			less = strings.ToLower(a.Name) < strings.ToLower(b.Name)
		default:
			less = parseTime(a.UpdatedAt).Before(parseTime(b.UpdatedAt))
		}
		if m.comp.sortAsc {
			return less
		}
		return !less
	})
	// Build lines
	lines := make([]compLine, 0, len(items))
	for _, c := range items {
		// Show date label aligned with current sort key
		var dateLabel string
		var dt time.Time
		switch m.comp.sortKey {
		case compSortCreated:
			dateLabel = "created at:"
			dt = parseTime(c.CreatedAt)
			if dt.IsZero() {
				dt = parseTime(c.UpdatedAt)
			}
		default: // compSortUpdated or name
			dateLabel = "updated at:"
			dt = parseTime(c.UpdatedAt)
			if dt.IsZero() {
				dt = parseTime(c.CreatedAt)
			}
		}
		datePart := ""
		if !dt.IsZero() {
			datePart = " " + subtleStyle.Render(dateLabel+" "+dt.Format("2006-01-02"))
		}
		item := fmt.Sprintf("%s %s%s", symbolComp, c.Name, datePart)
		// Reserve cells: cursor(1) + state(1) + spacer(1) => content width = m.width-5
		content := lipgloss.NewStyle().Width(m.width - 5).Render(item)
		lines = append(lines, compLine{content: content, name: c.Name, selected: m.comp.selected[c.Name]})
	}
	if len(lines) == 0 {
		lines = append(lines, compLine{content: warnStyle.Render("Keine Components gefunden (Filter aktiv?).")})
	}
	return lines
}

// visibleCompLines returns only the rendered lines
func (m Model) visibleCompLines() []string {
	model := m.visibleCompModel()
	out := make([]string, len(model))
	for i := range model {
		cursorCell := " "
		if i == m.comp.listIndex {
			cursorCell = cursorBarStyle.Render(" ")
		}
		stateCell := " "
		if model[i].selected {
			stateCell = markBarStyle.Render(" ")
		}
		spacer := " "
		out[i] = cursorCell + stateCell + spacer + model[i].content
	}
	return out
}

func parseTime(s string) time.Time {
	// Try RFC3339, fall back to date only
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t
	}
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return t
	}
	return time.Time{}
}

func mostRecentTime(c sb.Component) time.Time {
	tu := parseTime(c.UpdatedAt)
	tc := parseTime(c.CreatedAt)
	if tu.After(tc) {
		return tu
	}
	return tc
}

// ensureCompCursorVisible keeps the cursor within viewport when moving
func (m *Model) ensureCompCursorVisible() {
	lines := m.visibleCompLines()
	n := len(lines)
	if n == 0 {
		m.comp.listIndex = 0
		return
	}
	if m.comp.listIndex < 0 {
		m.comp.listIndex = 0
	}
	if m.comp.listIndex > n-1 {
		m.comp.listIndex = n - 1
	}

	cursor := m.comp.listIndex
	top := m.viewport.YOffset
	height := m.viewport.Height
	if cursor < top {
		m.viewport.SetYOffset(cursor)
		return
	}
	if cursor >= top+height-1 {
		m.viewport.SetYOffset(cursor - height + 1)
		return
	}
}
