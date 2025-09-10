package componentsync

import (
	"storyblok-sync/internal/sb"
	"testing"
)

func TestBuildPlan_DefaultsAndDecisions(t *testing.T) {
	src := []sb.Component{{ID: 1, Name: "Hero"}, {ID: 2, Name: "Button"}}
	tgt := []sb.Component{{ID: 10, Name: "Hero"}}
	// Decisions: fork Button to ButtonX; leave Hero default (should update)
	dec := map[string]Decision{"Button": {Action: "fork", ForkName: "ButtonX"}}
	plan := BuildPlan(src, tgt, dec)
	if len(plan) != 2 {
		t.Fatalf("want 2 plan items, got %d", len(plan))
	}
	// Find entries
	var hero, button PlanItem
	for _, p := range plan {
		if p.Source.Name == "Hero" {
			hero = p
		} else if p.Source.Name == "Button" {
			button = p
		}
	}
	if hero.Action != "update" || hero.TargetID != 10 {
		t.Fatalf("hero not update to 10: %+v", hero)
	}
	if button.Action != "create" || button.Name != "ButtonX" {
		t.Fatalf("button not forked: %+v", button)
	}
}

func TestBuildPlan_Skip(t *testing.T) {
	src := []sb.Component{{Name: "A"}, {Name: "B"}}
	tgt := []sb.Component{}
	dec := map[string]Decision{"A": {Action: "skip"}}
	plan := BuildPlan(src, tgt, dec)
	if len(plan) != 1 || plan[0].Source.Name != "B" || plan[0].Action != "create" {
		t.Fatalf("unexpected plan: %+v", plan)
	}
}
