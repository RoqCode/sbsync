package componentsync

import (
	"context"
	"encoding/json"
	"fmt"
	"storyblok-sync/internal/sb"
)

// BuildGroupNameMaps builds helper maps for mapping component groups
// - srcUUIDToName: source group UUID -> group name
// - tgtNameToUUID: target group name -> target group UUID
func BuildGroupNameMaps(src []sb.ComponentGroup, tgt []sb.ComponentGroup) (map[string]string, map[string]string) {
	srcUUIDToName := make(map[string]string, len(src))
	for _, g := range src {
		if g.UUID == "" || g.Name == "" {
			continue
		}
		srcUUIDToName[g.UUID] = g.Name
	}
	tgtNameToUUID := make(map[string]string, len(tgt))
	for _, g := range tgt {
		if g.UUID == "" || g.Name == "" {
			continue
		}
		tgtNameToUUID[g.Name] = g.UUID
	}
	return srcUUIDToName, tgtNameToUUID
}

// RemapComponentGroups remaps a component's group UUID and any occurrences
// of "component_group_whitelist" inside the JSON schema from source UUIDs
// to their corresponding target UUIDs based on group names.
// Returns a shallow-copied Component with updated fields and the number of
// whitelist entries changed.
func RemapComponentGroups(c sb.Component, srcUUIDToName, tgtNameToUUID map[string]string) (sb.Component, int, error) {
	out := c // shallow copy
	// Map top-level ComponentGroupUUID via name -> target uuid
	if c.ComponentGroupUUID != "" {
		if name, ok := srcUUIDToName[c.ComponentGroupUUID]; ok {
			if tuid, ok2 := tgtNameToUUID[name]; ok2 {
				out.ComponentGroupUUID = tuid
			}
		}
	}

	// Remap any whitelist UUIDs within the schema
	if len(c.Schema) == 0 || string(c.Schema) == "null" {
		return out, 0, nil
	}
	var v any
	if err := json.Unmarshal(c.Schema, &v); err != nil {
		return out, 0, fmt.Errorf("schema unmarshal: %w", err)
	}
	changed := 0
	remap := func(uuid string) string {
		if name, ok := srcUUIDToName[uuid]; ok {
			if tuid, ok2 := tgtNameToUUID[name]; ok2 {
				if tuid != uuid {
					changed++
				}
				return tuid
			}
		}
		return uuid
	}
	v2 := remapWhitelistUUIDs(v, remap)
	b, err := json.Marshal(v2)
	if err != nil {
		return out, 0, fmt.Errorf("schema marshal: %w", err)
	}
	out.Schema = json.RawMessage(b)
	return out, changed, nil
}

// remapWhitelistUUIDs walks an arbitrary JSON value and remaps values in
// keys named "component_group_whitelist" using the provided mapper.
func remapWhitelistUUIDs(v any, mapUUID func(string) string) any {
	switch vv := v.(type) {
	case map[string]any:
		// Copy-on-write to avoid mutating input
		out := make(map[string]any, len(vv))
		for k, val := range vv {
			if k == "component_group_whitelist" {
				// Expect an array of strings (UUIDs)
				switch arr := val.(type) {
				case []any:
					newArr := make([]any, 0, len(arr))
					for _, e := range arr {
						if s, ok := e.(string); ok {
							newArr = append(newArr, mapUUID(s))
						} else {
							newArr = append(newArr, e)
						}
					}
					out[k] = newArr
				default:
					// Leave as-is if unexpected type
					out[k] = val
				}
			} else {
				out[k] = remapWhitelistUUIDs(val, mapUUID)
			}
		}
		return out
	case []any:
		out := make([]any, len(vv))
		for i := range vv {
			out[i] = remapWhitelistUUIDs(vv[i], mapUUID)
		}
		return out
	default:
		return v
	}
}

// GroupAPI defines minimal operations to list and create component groups.
type GroupAPI interface {
	ListComponentGroups(ctx context.Context, spaceID int) ([]sb.ComponentGroup, error)
	CreateComponentGroup(ctx context.Context, spaceID int, name string) (sb.ComponentGroup, error)
}

// EnsureTargetGroups makes sure all source group names exist in the target space.
// It returns the refreshed target group slice after creating any missing ones.
func EnsureTargetGroups(ctx context.Context, api GroupAPI, targetSpaceID int, source []sb.ComponentGroup) ([]sb.ComponentGroup, error) {
	tgt, err := api.ListComponentGroups(ctx, targetSpaceID)
	if err != nil {
		return nil, err
	}
	have := make(map[string]bool, len(tgt))
	for _, g := range tgt {
		have[g.Name] = true
	}
	for _, sg := range source {
		if sg.Name == "" {
			continue
		}
		if !have[sg.Name] {
			if _, err := api.CreateComponentGroup(ctx, targetSpaceID, sg.Name); err != nil {
				return nil, err
			}
			have[sg.Name] = true
		}
	}
	// Fetch final list to return UUIDs of newly created groups
	return api.ListComponentGroups(ctx, targetSpaceID)
}
