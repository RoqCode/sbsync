package componentsync

import (
	"context"
	"sort"
	"storyblok-sync/internal/sb"
)

// TagAPI defines the minimal API surface for ensuring internal tags.
type TagAPI interface {
	ListInternalTags(ctx context.Context, spaceID int) ([]sb.InternalTag, error)
	CreateInternalTag(ctx context.Context, spaceID int, name string, objectType string) (sb.InternalTag, error)
}

// PrepareTagIDsForTarget ensures that all source internal tags exist in the target
// and returns their IDs in a stable order (sorted by name for determinism unless
// retainOrder is requested).
func PrepareTagIDsForTarget(ctx context.Context, api TagAPI, targetSpaceID int, source []sb.InternalTag, retainOrder bool) ([]int, error) {
	if len(source) == 0 {
		return nil, nil
	}
	tgt, err := api.ListInternalTags(ctx, targetSpaceID)
	if err != nil {
		return nil, err
	}
	byName := make(map[string]int, len(tgt))
	for _, t := range tgt {
		byName[t.Name] = t.ID
	}

	// Collect distinct names from source (object_type component preferred)
	names := make([]string, 0, len(source))
	seen := make(map[string]bool)
	for _, s := range source {
		n := s.Name
		if n == "" {
			continue
		}
		if !seen[n] {
			names = append(names, n)
			seen[n] = true
		}
	}
	if !retainOrder {
		sort.Strings(names)
	}

	ids := make([]int, 0, len(names))
	for _, name := range names {
		if id, ok := byName[name]; ok {
			ids = append(ids, id)
			continue
		}
		// Create missing tag for components
		created, err := api.CreateInternalTag(ctx, targetSpaceID, name, "component")
		if err != nil {
			return nil, err
		}
		byName[name] = created.ID
		ids = append(ids, created.ID)
	}
	return ids, nil
}
