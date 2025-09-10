package ui

import (
	"context"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	comps "storyblok-sync/internal/core/componentsync"
	synccore "storyblok-sync/internal/core/sync"
	"storyblok-sync/internal/infra/logx"
	"storyblok-sync/internal/sb"
	"strings"
)

// compApplyDoneMsg signals completion of components apply
type compReportEntry struct {
	Name       string
	Operation  string
	Err        string
	DurationMs int64
	Retry429   int
	RetryTotal int
}
type compApplyDoneMsg struct {
	entries []compReportEntry
}
type compExecInitMsg struct {
	err  error
	maps compRemapMaps
	plan []componentsyncPlanItem
}

// internal planning types kept in UI to avoid importing in types.go
type componentsyncPlanItem = comps.PlanItem
type compRemapMaps struct {
	srcUUIDToName map[string]string
	tgtNameToUUID map[string]string
	tgtID         int
	tagNameToID   map[string]int
	srcPresets    []sb.ComponentPreset
	tgtPresets    []sb.ComponentPreset
}

// execCompApplyCmd builds a plan from preflight decisions and applies it serially.
// It ensures groups/tags and remaps schema whitelists before writes.
/*
// Deprecated: execCompApplyCmd superseded by startCompApply with streaming UI.
// Keeping this comment to explain removal during rebase.
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
		// Ensure limiter
		if m.compLimiter == nil {
			plan := 0
			if m.targetSpace != nil {
				plan = m.targetSpace.PlanLevel
			}
			r, w, b := synccore.DefaultLimitsForPlan(plan)
			m.compLimiter = synccore.NewSpaceLimiter(r, w, b)
		}
		// Ensure groups
		tgtGroups, err := comps.EnsureTargetGroups(ctx, m.api, tgtID, m.componentGroupsSource)
        if err != nil {
            logx.Errorf("COMP_APPLY ensure groups: %v", err)
            return compApplyDoneMsg{entries: nil}
        }
		// Build maps for remap
		srcUUIDToName, tgtNameToUUID := comps.BuildGroupNameMaps(m.componentGroupsSource, tgtGroups)
		// Pre-ensure internal tags across all selected components
		tagNames := make([]string, 0)
		for _, c := range selected {
			for _, t := range c.InternalTagsList {
				if t.Name != "" {
					tagNames = append(tagNames, t.Name)
				}
			}
		}
		tagMap, err := comps.EnsureTagNameIDs(ctx, m.api, tgtID, tagNames)
        if err != nil {
            logx.Errorf("COMP_APPLY ensure tags: %v", err)
            return compApplyDoneMsg{entries: nil}
        }
		// Fetch presets from source and target spaces
		var srcPresets []sb.ComponentPreset
		if m.sourceSpace != nil {
			if pp, e := m.api.ListPresets(ctx, m.sourceSpace.ID); e == nil {
				srcPresets = pp
			} else {
				logx.Errorf("COMP_APPLY list source presets: %v", e)
			}
		}
		var tgtPresets []sb.ComponentPreset
		if pp, e := m.api.ListPresets(ctx, tgtID); e == nil {
			tgtPresets = pp
		} else {
			logx.Errorf("COMP_APPLY list target presets: %v", e)
		}
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
				// Map internal tag IDs from pre-ensured map
				if len(mapped.InternalTagsList) > 0 {
					ids := make([]int, 0, len(mapped.InternalTagsList))
					for _, t := range mapped.InternalTagsList {
						if id, ok := tagMap[t.Name]; ok && id > 0 {
							ids = append(ids, id)
						}
					}
					mapped.InternalTagIDs = sb.IntSlice(ids)
				}
				// Attach retry counters to context for per-item attribution
				rc := &sb.RetryCounters{}
				ctx2 := sb.WithRetryCounters(ctx, rc)
				// Write
				switch p.Action {
				case "create":
					_ = m.compLimiter.WaitWrite(ctx2, tgtID)
					createdComp, err2 := m.api.CreateComponent(ctx2, tgtID, mapped)
					err = err2
					if err != nil {
						// fallback: refresh target list and try update by name
						id := findTargetComponentIDByName(m.componentsTarget, mapped.Name)
						if id == 0 {
							if compsNow, e := m.api.ListComponents(ctx2, tgtID); e == nil {
								id = findTargetComponentIDByName(compsNow, mapped.Name)
							}
						}
						if id > 0 {
							mapped.ID = id
							_ = m.compLimiter.WaitWrite(ctx2, tgtID)
							if _, err2 := m.api.UpdateComponent(ctx2, tgtID, mapped); err2 != nil {
								logx.Errorf("COMP_APPLY create->update fallback failed: %v", err2)
								if synccore.IsRateLimited(err2) {
									m.compLimiter.NudgeWrite(tgtID, -0.2, 1, 7)
								}
								results <- compReportEntry{Name: mapped.Name, Operation: "create", Err: err2.Error(), DurationMs: time.Since(start).Milliseconds(), Retry429: int(rc.Status429), RetryTotal: int(rc.Total)}
							} else {
								// Sync presets on update path (diff by name)
								srcCP := comps.FilterPresetsForComponentID(srcPresets, p.Source.ID)
								tgtCP := comps.FilterPresetsForComponentID(tgtPresets, mapped.ID)
								newP, updP := comps.DiffPresetsByName(srcCP, tgtCP)
								createdCount, updatedCount := 0, 0
								for _, np := range newP {
									norm := comps.NormalizePresetForTarget(np, mapped.ID)
									_ = m.compLimiter.WaitWrite(ctx2, tgtID)
									if _, e := m.api.CreatePreset(ctx2, tgtID, norm); e != nil {
										logx.Errorf("COMP_APPLY preset create: %v", e)
									} else {
										createdCount++
									}
								}
								for _, up := range updP {
									norm := comps.NormalizePresetForTarget(up, mapped.ID)
									_ = m.compLimiter.WaitWrite(ctx2, tgtID)
									if _, e := m.api.UpdatePreset(ctx2, tgtID, norm); e != nil {
										logx.Errorf("COMP_APPLY preset update: %v", e)
									} else {
										updatedCount++
									}
								}
								logx.Infof("Presets in sync for %s — created: %d, updated: %d", mapped.Name, createdCount, updatedCount)
								m.compLimiter.NudgeWrite(tgtID, +0.02, 1, 7)
								results <- compReportEntry{Name: mapped.Name, Operation: "update", DurationMs: time.Since(start).Milliseconds(), Retry429: int(rc.Status429), RetryTotal: int(rc.Total)}
							}
						} else {
							logx.Errorf("COMP_APPLY create: %v", err)
							if synccore.IsRateLimited(err) {
								m.compLimiter.NudgeWrite(tgtID, -0.2, 1, 7)
							}
							results <- compReportEntry{Name: mapped.Name, Operation: "create", Err: err.Error(), DurationMs: time.Since(start).Milliseconds(), Retry429: int(rc.Status429), RetryTotal: int(rc.Total)}
						}
					} else {
						// Sync presets on create path: push all source presets for this component
						srcCP := comps.FilterPresetsForComponentID(srcPresets, p.Source.ID)
						createdCount := 0
						for _, sp := range srcCP {
							norm := comps.NormalizePresetForTarget(sp, createdComp.ID)
							_ = m.compLimiter.WaitWrite(ctx2, tgtID)
							if _, e := m.api.CreatePreset(ctx2, tgtID, norm); e != nil {
								logx.Errorf("COMP_APPLY preset create: %v", e)
							} else {
								createdCount++
							}
						}
						logx.Infof("Presets in sync for %s — created: %d, updated: %d", mapped.Name, createdCount, 0)
						m.compLimiter.NudgeWrite(tgtID, +0.02, 1, 7)
						results <- compReportEntry{Name: mapped.Name, Operation: "create", DurationMs: time.Since(start).Milliseconds(), Retry429: int(rc.Status429), RetryTotal: int(rc.Total)}
					}
				case "update":
					mapped.ID = p.TargetID
					_ = m.compLimiter.WaitWrite(ctx2, tgtID)
					if _, err = m.api.UpdateComponent(ctx2, tgtID, mapped); err != nil {
						logx.Errorf("COMP_APPLY update: %v", err)
						if synccore.IsRateLimited(err) {
							m.compLimiter.NudgeWrite(tgtID, -0.2, 1, 7)
						}
						results <- compReportEntry{Name: mapped.Name, Operation: "update", Err: err.Error(), DurationMs: time.Since(start).Milliseconds(), Retry429: int(rc.Status429), RetryTotal: int(rc.Total)}
					} else {
						// Sync presets on update path (diff by name)
						srcCP := comps.FilterPresetsForComponentID(srcPresets, p.Source.ID)
						tgtCP := comps.FilterPresetsForComponentID(tgtPresets, mapped.ID)
						newP, updP := comps.DiffPresetsByName(srcCP, tgtCP)
						createdCount, updatedCount := 0, 0
						for _, np := range newP {
							norm := comps.NormalizePresetForTarget(np, mapped.ID)
							_ = m.compLimiter.WaitWrite(ctx2, tgtID)
							if _, e := m.api.CreatePreset(ctx2, tgtID, norm); e != nil {
								logx.Errorf("COMP_APPLY preset create: %v", e)
							} else {
								createdCount++
							}
						}
						for _, up := range updP {
							norm := comps.NormalizePresetForTarget(up, mapped.ID)
							_ = m.compLimiter.WaitWrite(ctx2, tgtID)
							if _, e := m.api.UpdatePreset(ctx2, tgtID, norm); e != nil {
								logx.Errorf("COMP_APPLY preset update: %v", e)
							} else {
								updatedCount++
							}
						}
						logx.Infof("Presets in sync for %s — created: %d, updated: %d", mapped.Name, createdCount, updatedCount)
						m.compLimiter.NudgeWrite(tgtID, +0.02, 1, 7)
						results <- compReportEntry{Name: mapped.Name, Operation: "update", DurationMs: time.Since(start).Milliseconds(), Retry429: int(rc.Status429), RetryTotal: int(rc.Total)}
					}
				default:
					// skip
					results <- compReportEntry{Name: comp.Name, Operation: "skip", DurationMs: time.Since(start).Milliseconds(), Retry429: int(rc.Status429), RetryTotal: int(rc.Total)}
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
        for r := range results {
            entries = append(entries, r)
        }
        return compApplyDoneMsg{entries: entries}
    }
}
*/

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
		// Pre-ensure internal tags across all selected components
		tagNames := make([]string, 0)
		for _, c := range selected {
			for _, t := range c.InternalTagsList {
				if t.Name != "" {
					tagNames = append(tagNames, t.Name)
				}
			}
		}
		tagMap, err := comps.EnsureTagNameIDs(ctx, api, tgtID, tagNames)
		if err != nil {
			logx.Errorf("COMP_PREP ensure tags: %v", err)
			return compExecInitMsg{err: err}
		}
		// Fetch presets from source and target
		var srcPresets []sb.ComponentPreset
		if m.sourceSpace != nil {
			if pp, e := api.ListPresets(ctx, m.sourceSpace.ID); e == nil {
				srcPresets = pp
			} else {
				logx.Errorf("COMP_PREP list source presets: %v", e)
			}
		}
		var tgtPresets []sb.ComponentPreset
		if pp, e := api.ListPresets(ctx, tgtID); e == nil {
			tgtPresets = pp
		} else {
			logx.Errorf("COMP_PREP list target presets: %v", e)
		}
		plan := comps.BuildPlan(selected, tgtSnapshot, decisions)
		return compExecInitMsg{maps: compRemapMaps{srcUUIDToName: s2n, tgtNameToUUID: n2t, tgtID: tgtID, tagNameToID: tagMap, srcPresets: srcPresets, tgtPresets: tgtPresets}, plan: plan}
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
		// Ensure limiter
		if m.compLimiter == nil {
			plan := 0
			if m.targetSpace != nil {
				plan = m.targetSpace.PlanLevel
			}
			r, w, b := synccore.DefaultLimitsForPlan(plan)
			m.compLimiter = synccore.NewSpaceLimiter(r, w, b)
		}
		mapped, _, err := comps.RemapComponentGroups(comp, maps.srcUUIDToName, maps.tgtNameToUUID)
		if err != nil {
			return compItemDoneMsg{idx: idx, entry: compReportEntry{Name: comp.Name, Operation: p.Action, Err: err.Error(), DurationMs: time.Since(start).Milliseconds()}}
		}
		if len(mapped.InternalTagsList) > 0 {
			ids := make([]int, 0, len(mapped.InternalTagsList))
			for _, t := range mapped.InternalTagsList {
				if id, ok := maps.tagNameToID[t.Name]; ok && id > 0 {
					ids = append(ids, id)
				}
			}
			mapped.InternalTagIDs = sb.IntSlice(ids)
		}
		// Attach retry counters to context for per-item attribution
		rc := &sb.RetryCounters{}
		ctx2 := sb.WithRetryCounters(ctx, rc)
		// Write
		switch p.Action {
		case "create":
			_ = m.compLimiter.WaitWrite(ctx2, maps.tgtID)
			createdComp, err2 := api.CreateComponent(ctx2, maps.tgtID, mapped)
			err = err2
			if err != nil {
				// fallback: try update by name
				id := findTargetComponentIDByName(nil, mapped.Name)
				if id == 0 {
					if compsNow, e := api.ListComponents(ctx2, maps.tgtID); e == nil {
						id = findTargetComponentIDByName(compsNow, mapped.Name)
					}
				}
				if id > 0 {
					mapped.ID = id
					_ = m.compLimiter.WaitWrite(ctx2, maps.tgtID)
					if _, err2 := api.UpdateComponent(ctx2, maps.tgtID, mapped); err2 != nil {
						if synccore.IsRateLimited(err2) {
							m.compLimiter.NudgeWrite(maps.tgtID, -0.2, 1, 7)
						}
						return compItemDoneMsg{idx: idx, entry: compReportEntry{Name: mapped.Name, Operation: "create", Err: err2.Error(), DurationMs: time.Since(start).Milliseconds(), Retry429: int(rc.Status429), RetryTotal: int(rc.Total)}}
					}
					// Sync presets on update path
					srcCP := comps.FilterPresetsForComponentID(maps.srcPresets, p.Source.ID)
					tgtCP := comps.FilterPresetsForComponentID(maps.tgtPresets, mapped.ID)
					newP, updP := comps.DiffPresetsByName(srcCP, tgtCP)
					createdCount, updatedCount := 0, 0
					for _, np := range newP {
						norm := comps.NormalizePresetForTarget(np, mapped.ID)
						_ = m.compLimiter.WaitWrite(ctx2, maps.tgtID)
						if _, e := api.CreatePreset(ctx2, maps.tgtID, norm); e != nil {
							logx.Errorf("COMP_ITEM preset create: %v", e)
						} else {
							createdCount++
						}
					}
					for _, up := range updP {
						norm := comps.NormalizePresetForTarget(up, mapped.ID)
						_ = m.compLimiter.WaitWrite(ctx2, maps.tgtID)
						if _, e := api.UpdatePreset(ctx2, maps.tgtID, norm); e != nil {
							logx.Errorf("COMP_ITEM preset update: %v", e)
						} else {
							updatedCount++
						}
					}
					logx.Infof("Presets in sync for %s — created: %d, updated: %d", mapped.Name, createdCount, updatedCount)
					m.compLimiter.NudgeWrite(maps.tgtID, +0.02, 1, 7)
					return compItemDoneMsg{idx: idx, entry: compReportEntry{Name: mapped.Name, Operation: "update", DurationMs: time.Since(start).Milliseconds(), Retry429: int(rc.Status429), RetryTotal: int(rc.Total)}}
				}
				if synccore.IsRateLimited(err) {
					m.compLimiter.NudgeWrite(maps.tgtID, -0.2, 1, 7)
				}
				return compItemDoneMsg{idx: idx, entry: compReportEntry{Name: mapped.Name, Operation: "create", Err: err.Error(), DurationMs: time.Since(start).Milliseconds(), Retry429: int(rc.Status429), RetryTotal: int(rc.Total)}}
			}
			// Sync presets on create path
			srcCP := comps.FilterPresetsForComponentID(maps.srcPresets, p.Source.ID)
			createdCount := 0
			for _, sp := range srcCP {
				norm := comps.NormalizePresetForTarget(sp, createdComp.ID)
				_ = m.compLimiter.WaitWrite(ctx2, maps.tgtID)
				if _, e := api.CreatePreset(ctx2, maps.tgtID, norm); e != nil {
					logx.Errorf("COMP_ITEM preset create: %v", e)
				} else {
					createdCount++
				}
			}
			logx.Infof("Presets in sync for %s — created: %d, updated: %d", mapped.Name, createdCount, 0)
			m.compLimiter.NudgeWrite(maps.tgtID, +0.02, 1, 7)
			return compItemDoneMsg{idx: idx, entry: compReportEntry{Name: mapped.Name, Operation: "create", DurationMs: time.Since(start).Milliseconds(), Retry429: int(rc.Status429), RetryTotal: int(rc.Total)}}
		case "update":
			mapped.ID = p.TargetID
			_ = m.compLimiter.WaitWrite(ctx2, maps.tgtID)
			if _, err = api.UpdateComponent(ctx2, maps.tgtID, mapped); err != nil {
				if synccore.IsRateLimited(err) {
					m.compLimiter.NudgeWrite(maps.tgtID, -0.2, 1, 7)
				}
				return compItemDoneMsg{idx: idx, entry: compReportEntry{Name: mapped.Name, Operation: "update", Err: err.Error(), DurationMs: time.Since(start).Milliseconds(), Retry429: int(rc.Status429), RetryTotal: int(rc.Total)}}
			}
			// Sync presets on update path
			srcCP := comps.FilterPresetsForComponentID(maps.srcPresets, p.Source.ID)
			tgtCP := comps.FilterPresetsForComponentID(maps.tgtPresets, mapped.ID)
			newP, updP := comps.DiffPresetsByName(srcCP, tgtCP)
			createdCount, updatedCount := 0, 0
			for _, np := range newP {
				norm := comps.NormalizePresetForTarget(np, mapped.ID)
				_ = m.compLimiter.WaitWrite(ctx2, maps.tgtID)
				if _, e := api.CreatePreset(ctx2, maps.tgtID, norm); e != nil {
					logx.Errorf("COMP_ITEM preset create: %v", e)
				} else {
					createdCount++
				}
			}
			for _, up := range updP {
				norm := comps.NormalizePresetForTarget(up, mapped.ID)
				_ = m.compLimiter.WaitWrite(ctx2, maps.tgtID)
				if _, e := api.UpdatePreset(ctx2, maps.tgtID, norm); e != nil {
					logx.Errorf("COMP_ITEM preset update: %v", e)
				} else {
					updatedCount++
				}
			}
			logx.Infof("Presets in sync for %s — created: %d, updated: %d", mapped.Name, createdCount, updatedCount)
			m.compLimiter.NudgeWrite(maps.tgtID, +0.02, 1, 7)
			return compItemDoneMsg{idx: idx, entry: compReportEntry{Name: mapped.Name, Operation: "update", DurationMs: time.Since(start).Milliseconds(), Retry429: int(rc.Status429), RetryTotal: int(rc.Total)}}
		default:
			return compItemDoneMsg{idx: idx, entry: compReportEntry{Name: comp.Name, Operation: "skip", DurationMs: time.Since(start).Milliseconds(), Retry429: int(rc.Status429), RetryTotal: int(rc.Total)}}
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
