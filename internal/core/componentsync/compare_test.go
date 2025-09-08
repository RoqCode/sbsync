package componentsync

import (
    "encoding/json"
    "storyblok-sync/internal/sb"
    "testing"
)

func TestEqualAfterMapping_SimpleMatch(t *testing.T) {
    // Source uses source group UUIDs; target uses mapped UUIDs
    srcSchema := map[string]any{
        "field": map[string]any{
            "type": "bloks",
            "component_group_whitelist": []any{"sA"},
        },
    }
    bSrc, _ := json.Marshal(srcSchema)
    tgtSchema := map[string]any{
        "field": map[string]any{
            "type": "bloks",
            "component_group_whitelist": []any{"tA"},
        },
    }
    bTgt, _ := json.Marshal(tgtSchema)
    src := sb.Component{Name: "Comp", DisplayName: "C", ComponentGroupUUID: "sA", Schema: bSrc}
    tgt := sb.Component{Name: "Comp", DisplayName: "C", ComponentGroupUUID: "tA", Schema: bTgt}
    s2n := map[string]string{"sA": "Alpha"}
    n2t := map[string]string{"Alpha": "tA"}
    if !EqualAfterMapping(src, tgt, s2n, n2t) {
        t.Fatalf("expected equal after remap")
    }
}

func TestEqualAfterMapping_DiffersBySchema(t *testing.T) {
    a := sb.Component{Name: "X", Schema: json.RawMessage(`{"a":1}`)}
    b := sb.Component{Name: "X", Schema: json.RawMessage(`{"a":2}`)}
    if EqualAfterMapping(a, b, nil, nil) {
        t.Fatalf("expected not equal when schema differs")
    }
}

func TestEqualAfterMapping_IgnoresIDsAndTimestamps(t *testing.T) {
    schema := json.RawMessage(`{"k":"v"}`)
    src := sb.Component{Name: "Z", Schema: schema, ComponentGroupUUID: "g1", CreatedAt: "2020-01-01"}
    tgt := sb.Component{Name: "Z", Schema: schema, ComponentGroupUUID: "g1", UpdatedAt: "2025-01-01"}
    if !EqualAfterMapping(src, tgt, nil, nil) {
        t.Fatalf("timestamps/ids should be ignored for equality")
    }
}

func TestEqualAfterMapping_TagsMatchAndDiffer(t *testing.T) {
    schema := json.RawMessage(`{"k":"v"}`)
    src := sb.Component{Name: "Z", Schema: schema, InternalTagsList: []sb.InternalTag{{Name: "a"}, {Name: "b"}}}
    tgtSame := sb.Component{Name: "Z", Schema: schema, InternalTagsList: []sb.InternalTag{{Name: "b"}, {Name: "a"}}}
    if !EqualAfterMapping(src, tgtSame, nil, nil) {
        t.Fatalf("expected equal when tag name sets match")
    }
    tgtDiff := sb.Component{Name: "Z", Schema: schema, InternalTagsList: []sb.InternalTag{{Name: "a"}}}
    if EqualAfterMapping(src, tgtDiff, nil, nil) {
        t.Fatalf("expected not equal when tag name sets differ")
    }
}
