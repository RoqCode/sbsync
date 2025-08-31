package sync

import (
	"context"
	"encoding/json"
	"sync"
)

// CDAAPI is the minimal interface from the CDA client needed for hydration.
type CDAAPI interface {
	GetStoryRawBySlug(ctx context.Context, spaceID int, slug string, version string) (map[string]any, error)
}

// ContentVariants holds optional draft and published content blobs.
type ContentVariants struct {
	Draft     json.RawMessage
	Published json.RawMessage
}

// HydrationCache stores per-story content variants with a simple bound on entries.
type HydrationCache struct {
	mu         sync.RWMutex
	items      map[int]ContentVariants
	order      []int // insertion order for naive eviction
	maxEntries int
}

// NewHydrationCache creates a new cache with a max entry bound.
func NewHydrationCache(maxEntries int) *HydrationCache {
	if maxEntries <= 0 {
		maxEntries = 1000
	}
	return &HydrationCache{items: make(map[int]ContentVariants), maxEntries: maxEntries}
}

func (hc *HydrationCache) PutDraft(id int, blob json.RawMessage) {
	hc.mu.Lock()
	defer hc.mu.Unlock()
	v := hc.items[id]
	v.Draft = blob
	if _, ok := hc.items[id]; !ok {
		hc.order = append(hc.order, id)
	}
	hc.items[id] = v
	hc.evictIfNeeded()
}

func (hc *HydrationCache) PutPublished(id int, blob json.RawMessage) {
	hc.mu.Lock()
	defer hc.mu.Unlock()
	v := hc.items[id]
	v.Published = blob
	if _, ok := hc.items[id]; !ok {
		hc.order = append(hc.order, id)
	}
	hc.items[id] = v
	hc.evictIfNeeded()
}

func (hc *HydrationCache) Get(id int) (ContentVariants, bool) {
	hc.mu.RLock()
	defer hc.mu.RUnlock()
	v, ok := hc.items[id]
	return v, ok
}

func (hc *HydrationCache) evictIfNeeded() {
	if len(hc.items) <= hc.maxEntries {
		return
	}
	// Evict oldest entries until under cap.
	excess := len(hc.items) - hc.maxEntries
	for i := 0; i < excess && len(hc.order) > 0; i++ {
		id := hc.order[0]
		hc.order = hc.order[1:]
		delete(hc.items, id)
	}
}

// HydrationStats reports outcomes.
type HydrationStats struct {
	Total     int
	Drafts    int
	Published int
	Misses    int
}

// Hydrate fetches content for the given stories using CDA. tokenKind is "preview" or "public".
// workers bounds concurrency. Errors are per-item; function completes regardless.
func Hydrate(ctx context.Context, cda CDAAPI, srcSpaceID int, items []PreflightItem, tokenKind string, workers int, cache *HydrationCache) HydrationStats {
	if workers <= 0 {
		workers = 10
	}
	// Build list of story tasks (skip folders)
	type task struct {
		id        int
		slug      string
		published bool
	}
	tasks := make([]task, 0, len(items))
	for _, it := range items {
		if it.Story.IsFolder {
			continue
		}
		tasks = append(tasks, task{id: it.Story.ID, slug: it.Story.FullSlug, published: it.Story.Published})
	}

	stats := HydrationStats{Total: len(tasks)}
	var wg sync.WaitGroup
	ch := make(chan task)
	// Workers
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for t := range ch {
				select {
				case <-ctx.Done():
					return
				default:
				}
				switch tokenKind {
				case "preview":
					// Always try draft
					if m, err := cda.GetStoryRawBySlug(ctx, srcSpaceID, t.slug, "draft"); err == nil {
						if raw, ok := extractContentRaw(m); ok {
							cache.PutDraft(t.id, raw)
							add(&stats.Drafts, 1)
						}
					}
					// If published, also fetch published variant
					if t.published {
						if m, err := cda.GetStoryRawBySlug(ctx, srcSpaceID, t.slug, "published"); err == nil {
							if raw, ok := extractContentRaw(m); ok {
								cache.PutPublished(t.id, raw)
								add(&stats.Published, 1)
							}
						}
					}
				case "public":
					if t.published {
						if m, err := cda.GetStoryRawBySlug(ctx, srcSpaceID, t.slug, "published"); err == nil {
							if raw, ok := extractContentRaw(m); ok {
								cache.PutPublished(t.id, raw)
								add(&stats.Published, 1)
							}
						}
					}
				}
				// Miss if neither draft nor published ended up stored for this id
				if v, ok := cache.Get(t.id); ok {
					if len(v.Draft) == 0 && len(v.Published) == 0 {
						add(&stats.Misses, 1)
					}
				} else {
					add(&stats.Misses, 1)
				}
			}
		}()
	}

	go func() {
		defer close(ch)
		for _, t := range tasks {
			select {
			case <-ctx.Done():
				return
			case ch <- t:
			}
		}
	}()

	wg.Wait()
	return stats
}

func extractContentRaw(st map[string]any) (json.RawMessage, bool) {
	if st == nil {
		return nil, false
	}
	if c, ok := st["content"]; ok {
		b, err := json.Marshal(c)
		if err == nil {
			return json.RawMessage(b), true
		}
	}
	return nil, false
}

func add(p *int, v int) { *p += v }
