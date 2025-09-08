package ui

import (
    "context"
    "time"
    "strings"
    tea "github.com/charmbracelet/bubbletea"
    comps "storyblok-sync/internal/core/componentsync"
    "storyblok-sync/internal/infra/logx"
    "storyblok-sync/internal/sb"
)

// compApplyDoneMsg signals completion of components apply
type compApplyDoneMsg struct{ errCount int }

// execCompApplyCmd builds a plan from preflight decisions and applies it serially.
// It ensures groups/tags and remaps schema whitelists before writes.
func (m Model) execCompApplyCmd() func() tea.Msg {
    // capture snapshot data needed
    token := m.cfg.Token
    tgtID := 0
    if m.targetSpace != nil { tgtID = m.targetSpace.ID }
    // decisions by name
    decisions := make(map[string]comps.Decision, len(m.compPre.items))
    for _, it := range m.compPre.items {
        if it.Skip { decisions[it.Source.Name] = comps.Decision{Action: "skip"}; continue }
        if it.CopyAsNew { decisions[it.Source.Name] = comps.Decision{Action: "fork", ForkName: it.ForkName} ; continue }
        if it.Collision { decisions[it.Source.Name] = comps.Decision{Action: "update"} } else { decisions[it.Source.Name] = comps.Decision{Action: "create"} }
    }
    selected := make([]sb.Component, 0, len(m.compPre.items))
    for _, it := range m.compPre.items { if !it.Skip { selected = append(selected, it.Source) } }

    return func() tea.Msg {
        if m.api == nil { m.api = sb.New(token) }
        ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
        defer cancel()
        // Ensure groups
        tgtGroups, err := comps.EnsureTargetGroups(ctx, m.api, tgtID, m.componentGroupsSource)
        if err != nil { logx.Errorf("COMP_APPLY ensure groups: %v", err); return compApplyDoneMsg{errCount: len(selected)} }
        // Build maps for remap
        srcUUIDToName, tgtNameToUUID := comps.BuildGroupNameMaps(m.componentGroupsSource, tgtGroups)
        // Build plan
        plan := comps.BuildPlan(selected, m.componentsTarget, decisions)
        errCount := 0
        for _, p := range plan {
            // Prepare component
            comp := p.Source
            // Override name when forking
            if p.Name != "" { comp.Name = p.Name }
            // Remap groups
            mapped, _, err := comps.RemapComponentGroups(comp, srcUUIDToName, tgtNameToUUID)
            if err != nil { logx.Errorf("COMP_APPLY remap: %v", err); errCount++; continue }
            // Ensure tags
            ids, err := comps.PrepareTagIDsForTarget(ctx, m.api, tgtID, mapped.InternalTagsList, true)
            if err != nil { logx.Errorf("COMP_APPLY tags: %v", err); errCount++; continue }
            mapped.InternalTagIDs = sb.IntSlice(ids)
            // Write
            switch p.Action {
            case "create":
                _, err = m.api.CreateComponent(ctx, tgtID, mapped)
                if err != nil {
                    // fallback: try update if target now exists by name
                    id := findTargetComponentIDByName(m.componentsTarget, mapped.Name)
                    if id > 0 {
                        mapped.ID = id
                        _, err2 := m.api.UpdateComponent(ctx, tgtID, mapped)
                        if err2 != nil { errCount++ ; logx.Errorf("COMP_APPLY create->update fallback failed: %v", err2) }
                    } else {
                        errCount++; logx.Errorf("COMP_APPLY create: %v", err)
                    }
                }
            case "update":
                mapped.ID = p.TargetID
                _, err = m.api.UpdateComponent(ctx, tgtID, mapped)
                if err != nil { errCount++; logx.Errorf("COMP_APPLY update: %v", err) }
            default:
                // skip
            }
        }
        return compApplyDoneMsg{errCount: errCount}
    }
}

func findTargetComponentIDByName(tgt []sb.Component, name string) int {
    ln := strings.ToLower(name)
    for _, c := range tgt { if strings.ToLower(c.Name) == ln { return c.ID } }
    return 0
}
