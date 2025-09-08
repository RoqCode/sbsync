package ui

import (
	"storyblok-sync/internal/sb"
	"testing"
)

func TestStartCompPreflight_ClassifiesCollisions(t *testing.T) {
	m := InitialModel()
	m.currentMode = modeComponents
	m.componentsSource = []sb.Component{{ID: 1, Name: "A"}, {ID: 2, Name: "B"}}
	m.componentsTarget = []sb.Component{{ID: 10, Name: "B"}}
	m.comp.selected = map[string]bool{"A": true, "B": true}
	m.startCompPreflight()
	if len(m.compPre.items) != 2 {
		t.Fatalf("want 2 items, got %d", len(m.compPre.items))
	}
	var a, b *CompPreflightItem
	for i := range m.compPre.items {
		if m.compPre.items[i].Source.Name == "A" {
			a = &m.compPre.items[i]
		}
		if m.compPre.items[i].Source.Name == "B" {
			b = &m.compPre.items[i]
		}
	}
	if a == nil || b == nil {
		t.Fatalf("missing items: %+v", m.compPre.items)
	}
	if a.State != StateCreate || a.Collision {
		t.Fatalf("A should be create without collision: %+v", a)
	}
	if b.State != StateUpdate || !b.Collision || b.TargetID != 10 {
		t.Fatalf("B should be update with id=10: %+v", b)
	}
}
