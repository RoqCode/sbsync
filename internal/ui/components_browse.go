package ui

import (
    "fmt"
    "sort"
    "strings"
    "time"
    "storyblok-sync/internal/sb"

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
    if m.comp.search.searching {
        b.WriteString(m.comp.search.query)
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
    if m.comp.sortAsc { b.WriteString("↑") } else { b.WriteString("↓") }
    if !m.comp.dateCutoff.IsZero() {
        b.WriteString("  |  Cutoff: "+ m.comp.dateCutoff.Format("2006-01-02"))
    }
    if m.comp.group != "" {
        b.WriteString("  |  Group: "+ m.comp.group)
    }
    return b.String()
}

func (m Model) renderCompBrowseContent() string {
    lines := m.visibleCompLines()
    return strings.Join(lines, "\n")
}

// visibleCompLines builds grouped lines according to filters and sorting
func (m Model) visibleCompLines() []string {
    // Build group name map by UUID
    groupName := make(map[string]string)
    for _, g := range m.componentGroupsSource {
        groupName[g.UUID] = g.Name
    }
    // Build groups -> comps
    groups := make(map[string][]sb.Component)
    for _, c := range m.componentsSource {
        if m.comp.search.query != "" && !strings.Contains(strings.ToLower(c.Name), strings.ToLower(m.comp.search.query)) {
            continue
        }
        // date cutoff
        if !m.comp.dateCutoff.IsZero() {
            ts := mostRecentTime(c)
            if ts.Before(m.comp.dateCutoff) {
                continue
            }
        }
        // group filter
        gname := groupName[c.ComponentGroupUUID]
        if gname == "" { gname = "(no group)" }
        if m.comp.group != "" && m.comp.group != gname {
            continue
        }
        groups[gname] = append(groups[gname], c)
    }
    // sort group names alpha
    var order []string
    for name := range groups { order = append(order, name) }
    sort.Strings(order)
    // sort comps within group by current sort
    for _, k := range order {
        sort.Slice(groups[k], func(i, j int) bool {
            a, b := groups[k][i], groups[k][j]
            var less bool
            switch m.comp.sortKey {
            case compSortCreated:
                ta := parseTime(a.CreatedAt)
                tb := parseTime(b.CreatedAt)
                less = ta.Before(tb)
            case compSortName:
                less = strings.ToLower(a.Name) < strings.ToLower(b.Name)
            default:
                ta := parseTime(a.UpdatedAt)
                tb := parseTime(b.UpdatedAt)
                less = ta.Before(tb)
            }
            if m.comp.sortAsc { return less }
            return !less
        })
    }
    // Build lines
    var lines []string
    for _, g := range order {
        // group header line
        header := fmt.Sprintf("%s %s", symbolFolder, g)
        if m.comp.collapsed[g] {
            lines = append(lines, lipgloss.NewStyle().Bold(true).Render(header))
            continue
        }
        lines = append(lines, lipgloss.NewStyle().Bold(true).Render(header))
        for _, c := range groups[g] {
            mark := " "
            if m.comp.selected[c.Name] { mark = markBarStyle.Render(" ") }
            item := fmt.Sprintf("%s %s", symbolComp, c.Name)
            lines = append(lines, mark+" "+lipgloss.NewStyle().Width(m.width-4).Render(item))
        }
    }
    if len(lines) == 0 {
        lines = append(lines, warnStyle.Render("Keine Components gefunden (Filter aktiv?)."))
    }
    // Highlight cursor line if within bounds
    if m.comp.listIndex >= 0 && m.comp.listIndex < len(lines) {
        lines[m.comp.listIndex] = cursorBarStyle.Render(" ") + lines[m.comp.listIndex][1:]
    }
    return lines
}

func parseTime(s string) time.Time {
    // Try RFC3339, fall back to date only
    if t, err := time.Parse(time.RFC3339, s); err == nil { return t }
    if t, err := time.Parse("2006-01-02", s); err == nil { return t }
    return time.Time{}
}

func mostRecentTime(c sb.Component) time.Time {
    tu := parseTime(c.UpdatedAt)
    tc := parseTime(c.CreatedAt)
    if tu.After(tc) { return tu }
    return tc
}

