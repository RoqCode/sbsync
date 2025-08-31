package sync

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"storyblok-sync/internal/sb"
)

type fakeCDA struct {
	data map[string]map[string]any
	err  map[string]error
}

func (f fakeCDA) GetStoryRawBySlug(ctx context.Context, spaceID int, slug string, version string) (map[string]any, error) {
	if f.err != nil {
		if e := f.err[slug+"|"+version]; e != nil {
			return nil, e
		}
	}
	if f.data == nil {
		return nil, errors.New("no data")
	}
	if m, ok := f.data[slug+"|"+version]; ok {
		return m, nil
	}
	return nil, errors.New("not found")
}

func (f fakeCDA) WalkStoriesByPrefix(ctx context.Context, startsWith, version string, perPage int, fn func(map[string]any) error) error {
	// not used in these tests
	return nil
}

func mkContent(body string) map[string]any {
	var c any
	_ = json.Unmarshal([]byte(body), &c)
	return map[string]any{"content": c}
}

func TestHydrate_PreviewPublishedAndDraft(t *testing.T) {
	cda := fakeCDA{data: map[string]map[string]any{
		"s1|draft":     mkContent(`{"component":"page","v":"d"}`),
		"s1|published": mkContent(`{"component":"page","v":"p"}`),
	}}
	items := []PreflightItem{
		{Story: sb.Story{ID: 1, FullSlug: "s1", Published: true}},
	}
	cache := NewHydrationCache(10)
	stats := Hydrate(context.Background(), cda, 1, items, "preview", 4, cache, nil)
	if stats.Drafts != 1 || stats.Published != 1 {
		t.Fatalf("want both variants, got %+v", stats)
	}
	v, ok := cache.Get(1)
	if !ok || len(v.Draft) == 0 || len(v.Published) == 0 {
		t.Fatalf("cache missing variants: %+v", v)
	}
}

func TestHydrate_PublicOnlyPublished(t *testing.T) {
	cda := fakeCDA{data: map[string]map[string]any{
		"s2|published": mkContent(`{"x":1}`),
	}}
	items := []PreflightItem{{Story: sb.Story{ID: 2, FullSlug: "s2", Published: true}}}
	cache := NewHydrationCache(10)
	stats := Hydrate(context.Background(), cda, 1, items, "public", 2, cache, nil)
	if stats.Published != 1 || stats.Drafts != 0 {
		t.Fatalf("unexpected stats: %+v", stats)
	}
	v, ok := cache.Get(2)
	if !ok || len(v.Published) == 0 {
		t.Fatalf("missing published content: %+v", v)
	}
}

func TestHydrate_UnpublishedPreviewGetsDraft(t *testing.T) {
	cda := fakeCDA{data: map[string]map[string]any{
		"s3|draft": mkContent(`{"a":2}`),
	}}
	items := []PreflightItem{{Story: sb.Story{ID: 3, FullSlug: "s3", Published: false}}}
	cache := NewHydrationCache(10)
	stats := Hydrate(context.Background(), cda, 1, items, "preview", 2, cache, nil)
	if stats.Drafts != 1 {
		t.Fatalf("want 1 draft, got %+v", stats)
	}
	v, ok := cache.Get(3)
	if !ok || len(v.Draft) == 0 {
		t.Fatalf("missing draft: %+v", v)
	}
}
