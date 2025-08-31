package sync

import (
	"context"
	"testing"

	"storyblok-sync/internal/sb"
)

type fakeCDAWalk struct{ walked []string }

func (f *fakeCDAWalk) GetStoryRawBySlug(ctx context.Context, spaceID int, slug string, version string) (map[string]any, error) {
	f.walked = append(f.walked, "slug:"+slug+"|"+version)
	return map[string]any{"content": map[string]any{"c": 1}, "id": 999}, nil
}
func (f *fakeCDAWalk) WalkStoriesByPrefix(ctx context.Context, startsWith, version string, perPage int, fn func(map[string]any) error) error {
	f.walked = append(f.walked, "prefix:"+startsWith+"|"+version)
	_ = fn(map[string]any{"id": 1.0, "content": map[string]any{"a": 1}})
	_ = fn(map[string]any{"id": 2.0, "content": map[string]any{"a": 2}})
	return nil
}

func TestHydrateBatched_TopMostFolderOnly(t *testing.T) {
	// Source tree: root, root/sub, root/sub/leaf
	root := sb.Story{ID: 1, FullSlug: "root", IsFolder: true}
	sub := sb.Story{ID: 2, FullSlug: "root/sub", IsFolder: true}
	leaf := sb.Story{ID: 3, FullSlug: "root/sub/leaf"}
	all := []sb.Story{root, sub, leaf}

	// Preflight: root folder selected and leaf story selected
	items := []PreflightItem{
		{Story: root, Selected: true},
		{Story: sub, Selected: true},
		{Story: leaf, Selected: true},
	}

	f := &fakeCDAWalk{}
	cache := NewHydrationCache(100)
	stats := HydrateBatched(context.Background(), f, 1, items, all, "preview", 2, 2, 100, cache)

	// Expect only one prefix call for top-most root ("root") with both versions
	var prefixDraft, prefixPub bool
	for _, s := range f.walked {
		if s == "prefix:root|draft" {
			prefixDraft = true
		}
		if s == "prefix:root|published" {
			prefixPub = true
		}
		if s == "prefix:root/sub|draft" || s == "prefix:root/sub|published" {
			t.Fatalf("should not fetch subfolder when parent fully selected")
		}
	}
	if !prefixDraft || !prefixPub {
		t.Fatalf("expected prefix calls for both versions, got %v", f.walked)
	}
	if stats.Total == 0 {
		t.Fatalf("stats should reflect selected stories")
	}
}
