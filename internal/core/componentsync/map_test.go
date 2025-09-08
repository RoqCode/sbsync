package componentsync

import (
	"context"
	"encoding/json"
	"fmt"
	"storyblok-sync/internal/sb"
	"testing"
)

func TestBuildGroupNameMaps(t *testing.T) {
	src := []sb.ComponentGroup{{UUID: "s1", Name: "Alpha"}, {UUID: "s2", Name: "Beta"}}
	tgt := []sb.ComponentGroup{{UUID: "tA", Name: "Alpha"}, {UUID: "tB", Name: "Beta"}}
	s2n, n2t := BuildGroupNameMaps(src, tgt)
	if s2n["s1"] != "Alpha" || s2n["s2"] != "Beta" {
		t.Fatalf("src map unexpected: %+v", s2n)
	}
	if n2t["Alpha"] != "tA" || n2t["Beta"] != "tB" {
		t.Fatalf("tgt map unexpected: %+v", n2t)
	}
}

func TestRemapComponentGroups_TopLevelAndSchema(t *testing.T) {
	// Source has group UUID s1 -> Alpha; target Alpha -> tA
	srcGroups := []sb.ComponentGroup{{UUID: "s1", Name: "Alpha"}}
	tgtGroups := []sb.ComponentGroup{{UUID: "tA", Name: "Alpha"}}
	s2n, n2t := BuildGroupNameMaps(srcGroups, tgtGroups)

	// Schema contains a field with component_group_whitelist
	schema := map[string]any{
		"field1": map[string]any{
			"type":                      "bloks",
			"component_group_whitelist": []any{"s1", "unknown"},
		},
	}
	b, _ := json.Marshal(schema)
	comp := sb.Component{ID: 1, Name: "Comp", ComponentGroupUUID: "s1", Schema: b}

	mapped, changed, err := RemapComponentGroups(comp, s2n, n2t)
	if err != nil {
		t.Fatalf("remap error: %v", err)
	}
	if mapped.ComponentGroupUUID != "tA" {
		t.Fatalf("top-level group not remapped: %q", mapped.ComponentGroupUUID)
	}
	// Verify whitelist remapped
	var got map[string]any
	if err := json.Unmarshal(mapped.Schema, &got); err != nil {
		t.Fatalf("unmarshal mapped schema: %v", err)
	}
	f1 := got["field1"].(map[string]any)
	wl := f1["component_group_whitelist"].([]any)
	if wl[0].(string) != "tA" || wl[1].(string) != "unknown" {
		t.Fatalf("whitelist not remapped: %#v", wl)
	}
	if changed != 1 {
		t.Fatalf("expected 1 change, got %d", changed)
	}
}

type fakeGroupAPI struct {
	groups map[string]string // name -> uuid
	next   int
}

func (f *fakeGroupAPI) ListComponentGroups(ctx context.Context, spaceID int) ([]sb.ComponentGroup, error) {
	out := make([]sb.ComponentGroup, 0, len(f.groups))
	for n, u := range f.groups {
		out = append(out, sb.ComponentGroup{Name: n, UUID: u})
	}
	return out, nil
}
func (f *fakeGroupAPI) CreateComponentGroup(ctx context.Context, spaceID int, name string) (sb.ComponentGroup, error) {
	if f.groups == nil {
		f.groups = make(map[string]string)
	}
	f.next++
	uuid := fmt.Sprintf("t%02d", f.next)
	f.groups[name] = uuid
	return sb.ComponentGroup{Name: name, UUID: uuid}, nil
}

func TestEnsureTargetGroups(t *testing.T) {
	api := &fakeGroupAPI{groups: map[string]string{"Alpha": "tA"}}
	src := []sb.ComponentGroup{{UUID: "s1", Name: "Alpha"}, {UUID: "s2", Name: "Beta"}}
	got, err := EnsureTargetGroups(context.Background(), api, 1, src)
	if err != nil {
		t.Fatalf("ensure groups: %v", err)
	}
	// Expect both Alpha and Beta present afterwards
	names := make(map[string]bool)
	for _, g := range got {
		names[g.Name] = true
	}
	if !names["Alpha"] || !names["Beta"] {
		t.Fatalf("missing groups after ensure: %+v", got)
	}
}
