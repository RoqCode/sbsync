package componentsync

import (
    "encoding/json"
    "reflect"
    "storyblok-sync/internal/sb"
)

// EqualAfterMapping compares a source and target component for equality after
// remapping the source's group UUIDs (top-level and schema whitelists) to the
// target space using provided maps. It ignores IDs and timestamps and does not
// consider internal tags (IDs or names) for the MVP.
func EqualAfterMapping(src sb.Component, tgt sb.Component, srcUUIDToName map[string]string, tgtNameToUUID map[string]string) bool {
    // Quick checks on names: if names differ, it's not a collision/equal
    if !eqStringCase(src.Name, tgt.Name) {
        return false
    }

    // Remap groups/whitelists for the source into target env
    remapped, _, err := RemapComponentGroups(src, srcUUIDToName, tgtNameToUUID)
    if err != nil {
        return false
    }

    // Compare simple fields (display name and group uuid) after remap
    if remapped.DisplayName != tgt.DisplayName {
        return false
    }
    if remapped.ComponentGroupUUID != tgt.ComponentGroupUUID {
        return false
    }

    // Compare schemas structurally (canonical JSON)
    if !equalJSON(remapped.Schema, tgt.Schema) {
        return false
    }

    // Compare internal tag names as sets (order-insensitive). Names are stable
    // across spaces; IDs may differ, so prefer names for equality.
    if !equalTagNames(remapped.InternalTagsList, tgt.InternalTagsList) {
        return false
    }

    return true
}

func eqStringCase(a, b string) bool { return normalize(a) == normalize(b) }
func normalize(s string) string {
    // Case-insensitive for names; Storyblok component names are case-sensitive
    // technically, but collisions are detected by exact name elsewhere. Keep
    // simple equality to avoid surprises.
    return s
}

func equalJSON(a, b json.RawMessage) bool {
    // Handle empty/null
    if len(a) == 0 && len(b) == 0 {
        return true
    }
    var va any
    var vb any
    if len(a) > 0 && string(a) != "null" {
        if err := json.Unmarshal(a, &va); err != nil {
            return false
        }
    }
    if len(b) > 0 && string(b) != "null" {
        if err := json.Unmarshal(b, &vb); err != nil {
            return false
        }
    }
    return reflect.DeepEqual(va, vb)
}

func equalTagNames(a, b []sb.InternalTag) bool {
    if len(a) == 0 && len(b) == 0 {
        return true
    }
    setA := make(map[string]struct{}, len(a))
    for _, t := range a {
        if t.Name == "" { continue }
        setA[t.Name] = struct{}{}
    }
    setB := make(map[string]struct{}, len(b))
    for _, t := range b {
        if t.Name == "" { continue }
        setB[t.Name] = struct{}{}
    }
    if len(setA) != len(setB) {
        return false
    }
    for n := range setA {
        if _, ok := setB[n]; !ok {
            return false
        }
    }
    return true
}
