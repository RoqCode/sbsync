package ui

import (
	"context"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	comps "storyblok-sync/internal/core/componentsync"
	"storyblok-sync/internal/infra/logx"
	"storyblok-sync/internal/sb"
	"strings"
	"sync"
)

// compApplyDoneMsg signals completion of components apply
type compReportEntry struct {
	Name       string
	Operation  string
	Err        string
	DurationMs int64
}
type compApplyDoneMsg struct {
    errCount int
    entries  []compReportEntry
}
type compExecInitMsg struct{
    err error
    maps compRemapMaps
    plan []componentsyncPlanItem
}

// internal planning types kept in UI to avoid importing in types.go
type componentsyncPlanItem = comps.PlanItem
type compRemapMaps struct {
	srcUUIDToName map[string]string
	tgtNameToUUID map[string]string
	tgtID         int
}

// execCompApplyCmd builds a plan from preflight decisions and applies it serially.
// It ensures groups/tags and remaps schema whitelists before writes.
func (m Model) execCompApplyCmd() func() tea.Msg {
	// capture snapshot data needed
	token := m.cfg.Token
	tgtID := 0
	if m.targetSpace != nil {
		tgtID = m.targetSpace.ID
	}
	// decisions by name
	decisions := make(map[string]comps.Decision, len(m.compPre.items))
	for _, it := range m.compPre.items {
		if it.Skip {
			decisions[it.Source.Name] = comps.Decision{Action: "skip"}
			continue
		}
		if it.CopyAsNew {
			decisions[it.Source.Name] = comps.Decision{Action: "fork", ForkName: it.ForkName}
			continue
		}
		if it.Collision {
			decisions[it.Source.Name] = comps.Decision{Action: "update"}
		} else {
			decisions[it.Source.Name] = comps.Decision{Action: "create"}
		}
	}
	selected := make([]sb.Component, 0, len(m.compPre.items))
	for _, it := range m.compPre.items {
		if !it.Skip {
			selected = append(selected, it.Source)
		}
	}

	return func() tea.Msg {
		if m.api == nil {
			m.api = sb.New(token)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		// Ensure groups
		tgtGroups, err := comps.EnsureTargetGroups(ctx, m.api, tgtID, m.componentGroupsSource)
		if err != nil {
			logx.Errorf("COMP_APPLY ensure groups: %v", err)
			return compApplyDoneMsg{errCount: len(selected)}
		}
		// Build maps for remap
		srcUUIDToName, tgtNameToUUID := comps.BuildGroupNameMaps(m.componentGroupsSource, tgtGroups)
		// Build plan
		plan := comps.BuildPlan(selected, m.componentsTarget, decisions)
		// Concurrent worker pool
		workers := 4
		if len(plan) < workers {
			workers = len(plan)
		}
		jobs := make(chan comps.PlanItem)
		results := make(chan compReportEntry, len(plan))
		var wg sync.WaitGroup

		// worker function
		worker := func() {
			defer wg.Done()
			for p := range jobs {
				start := time.Now()
				// Prepare component
				comp := p.Source
				if p.Name != "" {
					comp.Name = p.Name
				}
				// Remap groups
				mapped, _, err := comps.RemapComponentGroups(comp, srcUUIDToName, tgtNameToUUID)
				if err != nil {
					logx.Errorf("COMP_APPLY remap: %v", err)
					results <- compReportEntry{Name: comp.Name, Operation: p.Action, Err: err.Error(), DurationMs: time.Since(start).Milliseconds()}
					continue
				}
				// Ensure tags
				ids, err := comps.PrepareTagIDsForTarget(ctx, m.api, tgtID, mapped.InternalTagsList, true)
				if err != nil {
					logx.Errorf("COMP_APPLY tags: %v", err)
					results <- compReportEntry{Name: comp.Name, Operation: p.Action, Err: err.Error(), DurationMs: time.Since(start).Milliseconds()}
					continue
				}
				mapped.InternalTagIDs = sb.IntSlice(ids)
				// Write
				switch p.Action {
				case "create":
					_, err = m.api.CreateComponent(ctx, tgtID, mapped)
					if err != nil {
						// fallback: refresh target list and try update by name
						id := findTargetComponentIDByName(m.componentsTarget, mapped.Name)
						if id == 0 {
							if compsNow, e := m.api.ListComponents(ctx, tgtID); e == nil {
								id = findTargetComponentIDByName(compsNow, mapped.Name)
							}
						}
						if id > 0 {
							mapped.ID = id
							if _, err2 := m.api.UpdateComponent(ctx, tgtID, mapped); err2 != nil {
								logx.Errorf("COMP_APPLY create->update fallback failed: %v", err2)
								results <- compReportEntry{Name: mapped.Name, Operation: "create", Err: err2.Error(), DurationMs: time.Since(start).Milliseconds()}
							} else {
								results <- compReportEntry{Name: mapped.Name, Operation: "update", DurationMs: time.Since(start).Milliseconds()}
							}
						} else {
							logx.Errorf("COMP_APPLY create: %v", err)
							results <- compReportEntry{Name: mapped.Name, Operation: "create", Err: err.Error(), DurationMs: time.Since(start).Milliseconds()}
						}
					} else {
						results <- compReportEntry{Name: mapped.Name, Operation: "create", DurationMs: time.Since(start).Milliseconds()}
					}
				case "update":
					mapped.ID = p.TargetID
					if _, err = m.api.UpdateComponent(ctx, tgtID, mapped); err != nil {
						logx.Errorf("COMP_APPLY update: %v", err)
						results <- compReportEntry{Name: mapped.Name, Operation: "update", Err: err.Error(), DurationMs: time.Since(start).Milliseconds()}
					} else {
						results <- compReportEntry{Name: mapped.Name, Operation: "update", DurationMs: time.Since(start).Milliseconds()}
					}
				default:
					// skip
					results <- compReportEntry{Name: comp.Name, Operation: "skip", DurationMs: time.Since(start).Milliseconds()}
				}
			}
		}

		wg.Add(workers)
		for i := 0; i < workers; i++ {
			go worker()
		}
		for _, p := range plan {
			jobs <- p
		}
		close(jobs)
		wg.Wait()
		close(results)

		entries := make([]compReportEntry, 0, len(plan))
		errCount := 0
		for r := range results {
			if r.Err != "" {
				errCount++
			}
			entries = append(entries, r)
		}
		return compApplyDoneMsg{errCount: errCount, entries: entries}
	}
}

// Start streaming apply with live per-item progress
func (m *Model) startCompApply() tea.Cmd {
	// Capture decisions and selected items
	decisions := make(map[string]comps.Decision, len(m.compPre.items))
	selected := make([]sb.Component, 0, len(m.compPre.items))
	for _, it := range m.compPre.items {
		if it.Skip {
			decisions[it.Source.Name] = comps.Decision{Action: "skip"}
			continue
		}
		if it.CopyAsNew {
			decisions[it.Source.Name] = comps.Decision{Action: "fork", ForkName: it.ForkName}
		} else if it.Collision {
			decisions[it.Source.Name] = comps.Decision{Action: "update"}
		} else {
			decisions[it.Source.Name] = comps.Decision{Action: "create"}
		}
		selected = append(selected, it.Source)
	}
	// Prepare immutable snapshots for the command
	token := m.cfg.Token
	tgtID := 0
	if m.targetSpace != nil {
		tgtID = m.targetSpace.ID
	}
	srcGroups := append([]sb.ComponentGroup(nil), m.componentGroupsSource...)
	tgtSnapshot := append([]sb.Component(nil), m.componentsTarget...)
	return func() tea.Msg {
		api := m.api
		if api == nil {
			api = sb.New(token)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		// Ensure groups and build maps
		tgtGroups, err := comps.EnsureTargetGroups(ctx, api, tgtID, srcGroups)
		if err != nil {
			logx.Errorf("COMP_PREP ensure groups: %v", err)
			return compExecInitMsg{err: err}
		}
		s2n, n2t := comps.BuildGroupNameMaps(srcGroups, tgtGroups)
		plan := comps.BuildPlan(selected, tgtSnapshot, decisions)
		return compExecInitMsg{maps: compRemapMaps{srcUUIDToName: s2n, tgtNameToUUID: n2t, tgtID: tgtID}, plan: plan}
	}
}

// Single item command: performs create/update and returns a done message
type compItemDoneMsg struct {
	idx   int
	entry compReportEntry
}

func (m Model) runCompItemCmd(idx int) tea.Cmd {
	// Capture data needed for this item
	p := componentsyncPlanItem{}
	// find plan entry by name
	name := m.compPre.items[idx].Source.Name
	for _, pi := range m.compPlan {
		if pi.Source.Name == name {
			p = pi
			break
		}
	}
	maps := m.compMaps
	api := m.api
	return func() tea.Msg {
		start := time.Now()
		// Prepare
		comp := p.Source
		if p.Name != "" {
			comp.Name = p.Name
		}
		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancel()
		mapped, _, err := comps.RemapComponentGroups(comp, maps.srcUUIDToName, maps.tgtNameToUUID)
		if err != nil {
			return compItemDoneMsg{idx: idx, entry: compReportEntry{Name: comp.Name, Operation: p.Action, Err: err.Error(), DurationMs: time.Since(start).Milliseconds()}}
		}
		ids, err := comps.PrepareTagIDsForTarget(ctx, api, maps.tgtID, mapped.InternalTagsList, true)
		if err != nil {
			return compItemDoneMsg{idx: idx, entry: compReportEntry{Name: comp.Name, Operation: p.Action, Err: err.Error(), DurationMs: time.Since(start).Milliseconds()}}
		}
		mapped.InternalTagIDs = sb.IntSlice(ids)
		// Write
		switch p.Action {
		case "create":
			_, err = api.CreateComponent(ctx, maps.tgtID, mapped)
			if err != nil {
				// fallback: try update by name
				id := findTargetComponentIDByName(nil, mapped.Name)
				if id == 0 {
					if compsNow, e := api.ListComponents(ctx, maps.tgtID); e == nil {
						id = findTargetComponentIDByName(compsNow, mapped.Name)
					}
				}
				if id > 0 {
					mapped.ID = id
					if _, err2 := api.UpdateComponent(ctx, maps.tgtID, mapped); err2 != nil {
						return compItemDoneMsg{idx: idx, entry: compReportEntry{Name: mapped.Name, Operation: "create", Err: err2.Error(), DurationMs: time.Since(start).Milliseconds()}}
					}
					return compItemDoneMsg{idx: idx, entry: compReportEntry{Name: mapped.Name, Operation: "update", DurationMs: time.Since(start).Milliseconds()}}
				}
				return compItemDoneMsg{idx: idx, entry: compReportEntry{Name: mapped.Name, Operation: "create", Err: err.Error(), DurationMs: time.Since(start).Milliseconds()}}
			}
			return compItemDoneMsg{idx: idx, entry: compReportEntry{Name: mapped.Name, Operation: "create", DurationMs: time.Since(start).Milliseconds()}}
		case "update":
			mapped.ID = p.TargetID
			if _, err = api.UpdateComponent(ctx, maps.tgtID, mapped); err != nil {
				return compItemDoneMsg{idx: idx, entry: compReportEntry{Name: mapped.Name, Operation: "update", Err: err.Error(), DurationMs: time.Since(start).Milliseconds()}}
			}
			return compItemDoneMsg{idx: idx, entry: compReportEntry{Name: mapped.Name, Operation: "update", DurationMs: time.Since(start).Milliseconds()}}
		default:
			return compItemDoneMsg{idx: idx, entry: compReportEntry{Name: comp.Name, Operation: "skip", DurationMs: time.Since(start).Milliseconds()}}
		}
	}
}

func findTargetComponentIDByName(tgt []sb.Component, name string) int {
	ln := strings.ToLower(name)
	for _, c := range tgt {
		if strings.ToLower(c.Name) == ln {
			return c.ID
		}
	}
	return 0
}
