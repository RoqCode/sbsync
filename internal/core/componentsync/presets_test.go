package componentsync

import (
    "encoding/json"
    "testing"
    "storyblok-sync/internal/sb"
)

func TestFilterPresetsForComponentID(t *testing.T) {
    all := []sb.ComponentPreset{
        {ID: 1, Name: "A", ComponentID: 10},
        {ID: 2, Name: "B", ComponentID: 11},
        {ID: 3, Name: "C", ComponentID: 10},
    }
    got := FilterPresetsForComponentID(all, 10)
    if len(got) != 2 || got[0].Name != "A" || got[1].Name != "C" {
        t.Fatalf("unexpected filter result: %+v", got)
    }
    if out := FilterPresetsForComponentID(all, 0); out != nil {
        t.Fatalf("expected nil for compID=0, got %#v", out)
    }
}

func TestDiffPresetsByName(t *testing.T) {
    srcPresetJSON := json.RawMessage(`{"x":1}`)
    src := []sb.ComponentPreset{
        {Name: "Default", ComponentID: 100, Preset: srcPresetJSON, Image: "//img-a"},
        {Name: "Alt", ComponentID: 100, Preset: json.RawMessage(`{"y":2}`)},
    }
    tgt := []sb.ComponentPreset{
        {ID: 7, Name: "Default", ComponentID: 200, Preset: json.RawMessage(`{"x":0}`), Image: "//old"},
        {ID: 8, Name: "Other", ComponentID: 200, Preset: json.RawMessage(`{"z":3}`)},
    }
    newp, updp := DiffPresetsByName(src, tgt)
    if len(newp) != 1 || newp[0].Name != "Alt" || newp[0].ID != 0 {
        t.Fatalf("unexpected new presets: %+v", newp)
    }
    if len(updp) != 1 || updp[0].Name != "Default" || updp[0].ID != 7 {
        t.Fatalf("unexpected update presets: %+v", updp)
    }
    if string(updp[0].Preset) != string(srcPresetJSON) {
        t.Fatalf("expected update preset payload from source")
    }
}

func TestNormalizePresetForTarget(t *testing.T) {
    p := sb.ComponentPreset{ID: 11, Name: "Default", ComponentID: 123, Preset: json.RawMessage(`{"a":1}`), Image: "//img"}
    norm := NormalizePresetForTarget(p, 999)
    if norm.ID != 11 || norm.ComponentID != 999 || norm.Name != "Default" || string(norm.Preset) != `{"a":1}` || norm.Image != "//img" {
        t.Fatalf("unexpected normalized preset: %+v", norm)
    }
    p2 := sb.ComponentPreset{Name: "Alt", Preset: json.RawMessage(`{"b":2}`)}
    norm2 := NormalizePresetForTarget(p2, 999)
    if norm2.ID != 0 || norm2.ComponentID != 999 || norm2.Name != "Alt" || string(norm2.Preset) != `{"b":2}` {
        t.Fatalf("unexpected normalized create preset: %+v", norm2)
    }
}

