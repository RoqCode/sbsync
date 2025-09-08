package ui

import (
	"storyblok-sync/internal/sb"
	"testing"
)

func TestStartCompPreflight_ClassifiesCollisions(t *testing.T) {
	m := InitialModel()
	m.currentMode = modeComponents
    m.componentsSource = []sb.Component{{ID: 1, Name: "A"}, {ID: 2, Name: "B", DisplayName: "changed"}}
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

func TestCompPreflight_SpaceCycles(t *testing.T) {
	m := InitialModel()
	m.currentMode = modeComponents
	m.state = stateCompList
    m.componentsSource = []sb.Component{{ID: 1, Name: "A"}, {ID: 2, Name: "B", DisplayName: "changed"}}
    m.componentsTarget = []sb.Component{{ID: 10, Name: "B"}}
	m.comp.selected = map[string]bool{"A": true, "B": true}

	// Build preflight
	m.startCompPreflight()
	if len(m.compPre.items) != 2 {
		t.Fatalf("want 2 items, got %d", len(m.compPre.items))
	}
	// Find B index
	idx := 0
	for i := range m.compPre.items {
		if m.compPre.items[i].Source.Name == "B" {
			idx = i
			break
		}
	}
	m.compPre.listIndex = idx
	// Initial for collision should be update
	if m.compPre.items[idx].State != StateUpdate {
		t.Fatalf("expected initial StateUpdate, got %s", m.compPre.items[idx].State)
	}
	// Press space -> Skip
	m, _ = m.handleCompPreflightKey(createKeyMsg(" "))
	if m.compPre.items[idx].State != StateSkip || !m.compPre.items[idx].Skip {
		t.Fatalf("expected Skip after space, got %+v", m.compPre.items[idx])
	}
	// Press space -> back to Apply (Update)
	m, _ = m.handleCompPreflightKey(createKeyMsg(" "))
	if m.compPre.items[idx].State != StateUpdate || m.compPre.items[idx].Skip {
		t.Fatalf("expected Update after second space, got %+v", m.compPre.items[idx])
	}
}

func TestCompPreflight_ForkRenameSetsName(t *testing.T) {
	m := InitialModel()
	m.currentMode = modeComponents
	m.state = stateCompList
	m.componentsSource = []sb.Component{{ID: 2, Name: "B"}}
	m.componentsTarget = []sb.Component{{ID: 10, Name: "B"}}
	m.comp.selected = map[string]bool{"B": true}

	m.startCompPreflight()
	if len(m.compPre.items) != 1 {
		t.Fatalf("want 1 item, got %d", len(m.compPre.items))
	}
	m.compPre.listIndex = 0
	// Enter rename mode directly (avoid Focus/Blink in tests) and confirm
	m.compPre.renaming = true
	m.compPre.input.SetValue("B-copy")
	m, _ = m.handleCompPreflightKey(createKeyMsg("enter"))
	if m.compPre.renaming {
		t.Fatalf("expected renaming=false after enter")
	}
	it := m.compPre.items[0]
	if !it.CopyAsNew || it.ForkName != "B-copy" || it.State != StateCreate {
		t.Fatalf("expected fork name persisted and StateCreate; got %+v", it)
	}
}

func TestCompPreflight_AutoSkipWhenUnchanged(t *testing.T) {
    m := InitialModel()
    m.currentMode = modeComponents
    // Same component in source and target (same name, display, group, schema)
    schema := sb.Component{ID: 1, Name: "A", DisplayName: "AA", ComponentGroupUUID: "g1", Schema: []byte(`{"x":1}`)}
    m.componentsSource = []sb.Component{schema}
    m.componentsTarget = []sb.Component{{ID: 10, Name: "A", DisplayName: "AA", ComponentGroupUUID: "g1", Schema: []byte(`{"x":1}`)}}
    m.componentGroupsSource = []sb.ComponentGroup{{UUID: "g1", Name: "G"}}
    m.componentGroupsTarget = []sb.ComponentGroup{{UUID: "g1", Name: "G"}}
    m.comp.selected = map[string]bool{"A": true}
    m.startCompPreflight()
    if len(m.compPre.items) != 1 {
        t.Fatalf("want 1 item, got %d", len(m.compPre.items))
    }
    it := m.compPre.items[0]
    if !(it.Skip && it.State == StateSkip) {
        t.Fatalf("expected item to be auto-skipped due to no changes: %+v", it)
    }
}
