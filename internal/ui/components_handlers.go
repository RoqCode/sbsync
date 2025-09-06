package ui

import (
    tea "github.com/charmbracelet/bubbletea"
    "strings"
    "time"
)

func (m Model) handleCompListKey(msg tea.KeyMsg) (Model, tea.Cmd) {
    key := msg.String()
    switch key {
    case "q":
        return m, tea.Quit
    case "m":
        m.state = stateModePicker
        m.statusMsg = "Zurück zur Modus-Auswahl."
        return m, nil
    case "j", "down":
        m.comp.listIndex++
        lines := m.visibleCompLines()
        if m.comp.listIndex >= len(lines) { m.comp.listIndex = len(lines)-1 }
        return m, nil
    case "k", "up":
        m.comp.listIndex--
        if m.comp.listIndex < 0 { m.comp.listIndex = 0 }
        return m, nil
    case "h":
        // collapse current group if cursor on group line
        m.toggleCurrentGroupCollapse(true)
        return m, nil
    case "l":
        m.toggleCurrentGroupCollapse(false)
        return m, nil
    case " ":
        // toggle selection if on component line (starts with symbolComp)
        lines := m.visibleCompLines()
        if m.comp.listIndex >= 0 && m.comp.listIndex < len(lines) {
            line := lines[m.comp.listIndex]
            if strings.Contains(line, symbolComp+" ") {
                name := strings.TrimSpace(line[strings.Index(line, symbolComp)+len(symbolComp):])
                // name might include styles; use a naive last token approach (after symbol)
                parts := strings.Fields(name)
                if len(parts) > 0 {
                    nm := parts[0]
                    if m.comp.selected[nm] { delete(m.comp.selected, nm) } else { m.comp.selected[nm] = true }
                }
            }
        }
        return m, nil
    case "f":
        // Start/stop lightweight search (no input component yet)
        if m.comp.search.searching {
            m.comp.search.searching = false
            m.comp.search.query = ""
        } else {
            m.comp.search.searching = true
            // no input widget: use status message as prompt for now
            m.statusMsg = "Suche aktiv – tippe /<text> in späterer Iteration"
        }
        return m, nil
    case "t":
        // cycle sort key
        m.comp.sortKey = (m.comp.sortKey + 1) % 3
        return m, nil
    case "o":
        m.comp.sortAsc = !m.comp.sortAsc
        return m, nil
    case "d":
        // toggle a simple date cutoff of today for now; later accept input
        if m.comp.dateCutoff.IsZero() {
            m.comp.dateCutoff = nowMidnight()
        } else {
            m.comp.dateCutoff = timeZero()
        }
        return m, nil
    }
    return m, nil
}

func (m *Model) toggleCurrentGroupCollapse(collapse bool) {
    lines := m.visibleCompLines()
    if m.comp.listIndex < 0 || m.comp.listIndex >= len(lines) { return }
    line := lines[m.comp.listIndex]
    // group line contains symbolFolder and no symbolComp
    if strings.Contains(line, symbolFolder) && !strings.Contains(line, symbolComp) {
        // group name is after symbolFolder
        idx := strings.Index(line, symbolFolder)
        name := strings.TrimSpace(line[idx+len(symbolFolder):])
        if m.comp.collapsed == nil { m.comp.collapsed = make(map[string]bool) }
        m.comp.collapsed[name] = collapse
    }
}

func nowMidnight() time.Time { t := time.Now(); return time.Date(t.Year(), t.Month(), t.Day(), 0,0,0,0, t.Location()) }
func timeZero() time.Time { return time.Time{} }
