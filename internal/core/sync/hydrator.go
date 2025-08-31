package sync

import (
	"context"
	"encoding/json"
	"sort"
	"strings"
	"sync"

	"storyblok-sync/internal/sb"
)

// CDAAPI is the minimal interface from the CDA client needed for hydration.
type CDAAPI interface {
	GetStoryRawBySlug(ctx context.Context, spaceID int, slug string, version string) (map[string]any, error)
	WalkStoriesByPrefix(ctx context.Context, startsWith, version string, perPage int, fn func(map[string]any) error) error
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

// HydrationProgress provides incremental updates for UI.
type HydrationProgress struct {
	// Total is set on the initial call to indicate the expected total units.
	// For preview tokens this equals the number of selected stories.
	// For public tokens this equals the number of selected published stories.
	Total int
	// Increments to apply to current counters.
	IncrDrafts    int
	IncrPublished int
}

// Hydrate fetches content for the given stories using CDA. tokenKind is "preview" or "public".
// workers bounds concurrency. Errors are per-item; function completes regardless.
func Hydrate(ctx context.Context, cda CDAAPI, srcSpaceID int, items []PreflightItem, tokenKind string, workers int, cache *HydrationCache, progress func(HydrationProgress)) HydrationStats {
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
	if progress != nil {
		// For plain per-slug hydration, Total equals tasks length regardless of token kind
		progress(HydrationProgress{Total: len(tasks)})
	}
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
							if progress != nil {
								progress(HydrationProgress{IncrDrafts: 1})
							}
						}
					}
					// If published, also fetch published variant
					if t.published {
						if m, err := cda.GetStoryRawBySlug(ctx, srcSpaceID, t.slug, "published"); err == nil {
							if raw, ok := extractContentRaw(m); ok {
								cache.PutPublished(t.id, raw)
								add(&stats.Published, 1)
								if progress != nil {
									progress(HydrationProgress{IncrPublished: 1})
								}
							}
						}
					}
				case "public":
					if t.published {
						if m, err := cda.GetStoryRawBySlug(ctx, srcSpaceID, t.slug, "published"); err == nil {
							if raw, ok := extractContentRaw(m); ok {
								cache.PutPublished(t.id, raw)
								add(&stats.Published, 1)
								if progress != nil {
									progress(HydrationProgress{IncrPublished: 1})
								}
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

// HydrateBatched hydrates using batched prefix reads for fully-selected folders
// and falls back to per-slug reads for remaining stories.
func HydrateBatched(ctx context.Context, cda CDAAPI, srcSpaceID int, items []PreflightItem, allSource []sb.Story, tokenKind string, batchWorkers, slugWorkers, cacheMax int, cache *HydrationCache, progress func(HydrationProgress)) HydrationStats {
	if cache == nil {
		cache = NewHydrationCache(cacheMax)
	}
	// Build selected story slugs (non-folders) and candidate folders
	selectedStories := make(map[string]bool)
	folderCandidates := make([]sb.Story, 0)
	for _, it := range items {
		if it.Skip || !it.Selected {
			continue
		}
		if it.Story.IsFolder {
			folderCandidates = append(folderCandidates, it.Story)
		} else {
			selectedStories[it.Story.FullSlug] = true
		}
	}

	// Detect fully-selected folders
	fullRoots := topMostFullySelectedFolders(folderCandidates, selectedStories, allSource)

	// Determine which selected stories are not covered by any root
	covered := make(map[string]bool)
	for _, root := range fullRoots {
		pref := root.FullSlug + "/"
		for slug := range selectedStories {
			if strings.HasPrefix(slug, pref) {
				covered[slug] = true
			}
		}
	}
	perSlug := make([]PreflightItem, 0, len(items))
	for _, it := range items {
		if it.Skip || !it.Selected || it.Story.IsFolder {
			continue
		}
		if !covered[it.Story.FullSlug] {
			perSlug = append(perSlug, it)
		}
	}

	// Determine totals for progress: preview => all selected stories; public => only published stories among selected
	selectedPublished := 0
	if tokenKind == "public" {
		// Build quick map from slug to published flag
		pub := make(map[string]bool, len(allSource))
		for _, s := range allSource {
			pub[s.FullSlug] = s.Published
		}
		for slug := range selectedStories {
			if pub[slug] {
				selectedPublished++
			}
		}
	}
	progressTotal := len(selectedStories)
	if tokenKind == "public" {
		progressTotal = selectedPublished
	}

	stats := HydrationStats{Total: len(selectedStories)}
	if progress != nil {
		progress(HydrationProgress{Total: progressTotal})
	}
	// Batch stage: iterate roots concurrently
	if len(fullRoots) > 0 {
		var wg sync.WaitGroup
		rootCh := make(chan sb.Story)
		for w := 0; w < max(1, batchWorkers); w++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for r := range rootCh {
					// Preview: fetch draft + published
					if tokenKind == "preview" {
						_ = cda.WalkStoriesByPrefix(ctx, r.FullSlug, "draft", 100, func(m map[string]any) error {
							if id, raw, ok := storyIDAndContent(m); ok {
								cache.PutDraft(id, raw)
								add(&stats.Drafts, 1)
								if progress != nil {
									progress(HydrationProgress{IncrDrafts: 1})
								}
							}
							return nil
						})
						_ = cda.WalkStoriesByPrefix(ctx, r.FullSlug, "published", 100, func(m map[string]any) error {
							if id, raw, ok := storyIDAndContent(m); ok {
								cache.PutPublished(id, raw)
								add(&stats.Published, 1)
								if progress != nil {
									progress(HydrationProgress{IncrPublished: 1})
								}
							}
							return nil
						})
					} else {
						_ = cda.WalkStoriesByPrefix(ctx, r.FullSlug, "published", 100, func(m map[string]any) error {
							if id, raw, ok := storyIDAndContent(m); ok {
								cache.PutPublished(id, raw)
								add(&stats.Published, 1)
								if progress != nil {
									progress(HydrationProgress{IncrPublished: 1})
								}
							}
							return nil
						})
					}
				}
			}()
		}
		go func() {
			defer close(rootCh)
			for _, r := range fullRoots {
				select {
				case <-ctx.Done():
					return
				case rootCh <- r:
				}
			}
		}()
		wg.Wait()
	}

	// Per-slug fallback stage
	if len(perSlug) > 0 {
		// reuse existing Hydrate logic with a filtered item list
		part := Hydrate(ctx, cda, srcSpaceID, perSlug, tokenKind, slugWorkers, cache, progress)
		stats.Drafts += part.Drafts
		stats.Published += part.Published
		stats.Misses += part.Misses
	}

	// Compute misses for all selected stories
	for slug := range selectedStories {
		// Need to map slug to ID; use allSource to find ID then check cache
		for _, s := range allSource {
			if s.FullSlug == slug {
				if v, ok := cache.Get(s.ID); ok {
					if len(v.Draft) == 0 && len(v.Published) == 0 {
						stats.Misses++
					}
				} else {
					stats.Misses++
				}
				break
			}
		}
	}

	return stats
}

func storyIDAndContent(m map[string]any) (int, json.RawMessage, bool) {
	if m == nil {
		return 0, nil, false
	}
	// CDA flattens story fields; ensure not a folder (if present)
	if v, ok := m["is_folder"]; ok {
		if b, ok2 := v.(bool); ok2 && b {
			return 0, nil, false
		}
	}
	id := 0
	if iv, ok := m["id"]; ok {
		switch x := iv.(type) {
		case float64:
			id = int(x)
		case int:
			id = x
		}
	}
	if id == 0 {
		return 0, nil, false
	}
	raw, ok := extractContentRaw(m)
	if !ok {
		return 0, nil, false
	}
	return id, raw, true
}

func topMostFullySelectedFolders(candidates []sb.Story, selected map[string]bool, all []sb.Story) []sb.Story {
	// Build list of non-folder stories under candidates and verify full selection
	full := make([]sb.Story, 0, len(candidates))
	for _, f := range candidates {
		pref := f.FullSlug + "/"
		allSelected := true
		for _, s := range all {
			if s.IsFolder {
				continue
			}
			if strings.HasPrefix(s.FullSlug, pref) {
				if !selected[s.FullSlug] {
					allSelected = false
					break
				}
			}
		}
		if allSelected {
			full = append(full, f)
		}
	}
	// Sort by depth ascending and deduplicate to top-most
	sort.Slice(full, func(i, j int) bool {
		di := strings.Count(full[i].FullSlug, "/")
		dj := strings.Count(full[j].FullSlug, "/")
		if di != dj {
			return di < dj
		}
		return full[i].FullSlug < full[j].FullSlug
	})
	kept := make([]sb.Story, 0, len(full))
	for _, f := range full {
		isDesc := false
		for _, k := range kept {
			if strings.HasPrefix(f.FullSlug, k.FullSlug+"/") {
				isDesc = true
				break
			}
		}
		if !isDesc {
			kept = append(kept, f)
		}
	}
	return kept
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
