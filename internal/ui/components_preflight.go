package ui

import (
    "fmt"
    "strings"

    comps "storyblok-sync/internal/core/componentsync"
    "storyblok-sync/internal/sb"
)

// startCompPreflight builds component preflight items from current selection
func (m *Model) startCompPreflight() {
	// Build target name -> ID map
    tgtByName := make(map[string]int, len(m.componentsTarget))
	for _, t := range m.componentsTarget {
		if t.Name == "" {
			continue
		}
		ln := strings.ToLower(t.Name)
        tgtByName[ln] = t.ID
	}
	// Build name->component map for target (lower-cased)
	tgtCompByName := make(map[string]sb.Component, len(m.componentsTarget))
	for _, t := range m.componentsTarget {
		if t.Name == "" { continue }
		ln := strings.ToLower(t.Name)
		tgtCompByName[ln] = t
	}
	// Build group remap maps
	s2n, n2t := comps.BuildGroupNameMaps(m.componentGroupsSource, m.componentGroupsTarget)
	items := make([]CompPreflightItem, 0, len(m.componentsSource))
	for _, c := range m.componentsSource {
		if !m.comp.selected[c.Name] {
			continue
		}
		lower := strings.ToLower(c.Name)
		id, exists := tgtByName[lower]
		it := CompPreflightItem{Source: c, Selected: true, Collision: exists, TargetID: id}
		if exists {
			it.State = StateUpdate
			// Auto-skip if equal after mapping (no changes)
			if tgt, ok := tgtCompByName[lower]; ok {
				if comps.EqualAfterMapping(c, tgt, s2n, n2t) {
					it.State = StateSkip
					it.Skip = true
					it.Issue = "no changes"
				}
			}
		} else {
			it.State = StateCreate
		}
		it.Run = RunPending
		items = append(items, it)
	}
	m.compPre.items = items
	m.compPre.listIndex = 0
	m.updateCompPreflightViewport()
	if len(items) == 0 {
		m.statusMsg = "Keine markierten Components – zurück mit 'm' oder 'q'"
	} else {
		m.statusMsg = fmt.Sprintf("Preflight: %d ausgewählt (Kollisionen: %d)", len(items), countCompCollisions(items))
	}
}

func countCompCollisions(items []CompPreflightItem) int {
	n := 0
	for _, it := range items {
		if it.Collision {
			n++
		}
	}
	return n
}

// Rendering helpers
func (m *Model) updateCompPreflightViewport() {
	lines := make([]string, 0, len(m.compPre.items))
	for i, it := range m.compPre.items {
		cursor := " "
		if i == m.compPre.listIndex {
			cursor = cursorBarStyle.Render(" ")
		}
		// compact state cell uses a single-character label with color
		stateCell := stateStyles[it.State].Render(stateLabel(it.State))
		name := it.Source.Name
		suffix := ""
		// If currently renaming this item, render a live input with cursor
		if m.compPre.renaming && i == m.compPre.listIndex {
			suffix = okStyle.Render(" → ") + m.compPre.input.View()
		} else {
			if it.Collision && !it.CopyAsNew {
				suffix = subtleStyle.Render(" (overwrite)")
			}
			if it.CopyAsNew {
				fn := it.ForkName
				if fn == "" {
					fn = name + "-copy"
				}
				suffix = okStyle.Render(" → ") + fn
			}
		}
		line := cursor + stateCell + fmt.Sprintf(" %s %s", symbolComp, name) + suffix
		lines = append(lines, line)
	}
	if len(lines) == 0 {
		lines = append(lines, warnStyle.Render("Keine Items im Preflight."))
	}
	m.viewport.SetContent(strings.Join(lines, "\n"))
}
