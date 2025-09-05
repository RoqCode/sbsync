package sync

import (
	"context"
	"testing"

	"storyblok-sync/internal/sb"
)

// Verifies folder create path enforces slug/full_slug/name, clears UUID, and
// rewrites translated_slugs paths to the new slug when using raw create.
func TestSyncFolder_Create_ForkEnforcesSlugAndTranslatedPaths(t *testing.T) {
	api := newMockStoryRawSyncAPI()

	// Target already has parent folder with ID 77
	api.targetBySlug["parent"] = sb.Story{ID: 77, FullSlug: "parent", IsFolder: true}

	// Source typed + raw for the folder to be forked under parent
	sourceID := 42
	api.sourceTypedByID[sourceID] = sb.Story{
		ID:       sourceID,
		Name:     "Folder Copy", // UI-suffixed name
		Slug:     "new",         // UI-mutated slug
		FullSlug: "parent/new",  // UI-mutated full slug
		IsFolder: true,
		TranslatedSlugs: []sb.TranslatedSlug{
			{Lang: "de", Path: "de/parent/orig"},
			{Lang: "en", Path: "en/parent/orig"},
		},
	}
	// Raw payload from source still contains old fields; translated_slugs present
	api.sourceRawByID[sourceID] = map[string]interface{}{
		"uuid":      "uuid-old",
		"name":      "Folder",
		"slug":      "orig",
		"full_slug": "parent/orig",
		"is_folder": true,
		"translated_slugs": []interface{}{
			map[string]interface{}{"lang": "de", "name": "DE", "path": "de/parent/orig", "id": 1},
			map[string]interface{}{"lang": "en", "name": "EN", "path": "en/parent/orig", "id": 2},
		},
	}

	// Build syncer with empty existing index
	syncer := NewStorySyncer(api, 1, 2, map[string]sb.Story{})

	// Call SyncFolder with the typed (UI-mutated) folder
	folder := api.sourceTypedByID[sourceID]
	if _, err := syncer.SyncFolder(context.Background(), folder, false); err != nil {
		t.Fatalf("sync folder failed: %v", err)
	}

	if len(api.rawCreates) != 1 {
		t.Fatalf("expected one raw create, got %d", len(api.rawCreates))
	}
	raw := api.rawCreates[0]
	if raw["slug"] != "new" || raw["full_slug"] != "parent/new" {
		t.Fatalf("expected enforced slug/full_slug = parent/new, got %v / %v", raw["slug"], raw["full_slug"])
	}
	if raw["name"] != "Folder Copy" {
		t.Fatalf("expected name overridden to 'Folder Copy', got %v", raw["name"])
	}
	if _, ok := raw["uuid"]; ok {
		t.Fatalf("expected uuid removed to avoid collisions")
	}
	if raw["is_folder"] != true {
		t.Fatalf("expected is_folder=true")
	}
	// parent_id should be resolved to 77 (existing parent)
	if pid, ok := raw["parent_id"].(int); !ok || pid != 77 {
		t.Fatalf("expected parent_id 77, got %v", raw["parent_id"])
	}
	// translated_slugs converted to attributes and last path segment replaced with new slug
	attrs, ok := raw["translated_slugs_attributes"].([]map[string]interface{})
	if !ok || len(attrs) != 2 {
		t.Fatalf("expected translated_slugs_attributes with 2 entries, got %T len=%d", raw["translated_slugs_attributes"], len(attrs))
	}
	for _, m := range attrs {
		p, _ := m["path"].(string)
		if p == "" || !(p == "de/parent/new" || p == "en/parent/new") {
			t.Fatalf("unexpected translated path: %v", p)
		}
		if _, has := m["id"]; has {
			t.Fatalf("expected id removed from translated_slugs")
		}
	}
}
