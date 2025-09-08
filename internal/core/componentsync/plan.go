package componentsync

import (
	"storyblok-sync/internal/sb"
	"strings"
)

// Decision represents a user choice from preflight per component name.
// Action: create | update | skip | fork
type Decision struct {
	Action   string
	ForkName string // when Action=fork, new component name to create
}

// PlanItem is a concrete execution step for the executor
type PlanItem struct {
	Source   sb.Component
	Action   string // create|update
	TargetID int    // for update
	Name     string // final name (fork may change it)
}

// BuildPlan classifies actions using target name→ID and applies decisions.
// - default: if source name exists in target => update; else create
// - skip: drop from plan
// - fork: force create with Name=ForkName (or source name + "-copy")
func BuildPlan(selected []sb.Component, target []sb.Component, decisions map[string]Decision) []PlanItem {
	tgtByName := make(map[string]int, len(target))
	for _, t := range target {
		if t.Name == "" {
			continue
		}
		tgtByName[strings.ToLower(t.Name)] = t.ID
	}
	out := make([]PlanItem, 0, len(selected))
	for _, s := range selected {
		d := decisions[s.Name]
		// normalize action
		action := d.Action
		if action == "skip" {
			continue
		}
		name := s.Name
		if action == "fork" {
			action = "create"
			if d.ForkName != "" {
				name = d.ForkName
			} else {
				name = s.Name + "-copy"
			}
			out = append(out, PlanItem{Source: s, Action: action, TargetID: 0, Name: name})
			continue
		}
		// default classification
		if action != "create" && action != "update" {
			if id, ok := tgtByName[strings.ToLower(s.Name)]; ok {
				action = "update"
				out = append(out, PlanItem{Source: s, Action: action, TargetID: id, Name: name})
			} else {
				action = "create"
				out = append(out, PlanItem{Source: s, Action: action, TargetID: 0, Name: name})
			}
			continue
		}
		// explicit create/update
		id := 0
		if action == "update" {
			if id2, ok := tgtByName[strings.ToLower(s.Name)]; ok {
				id = id2
			}
		}
		out = append(out, PlanItem{Source: s, Action: action, TargetID: id, Name: name})
	}
	return out
}
