package sync

import (
	"fmt"
	"log"
	"sort"
	"strings"

	"storyblok-sync/internal/sb"
)

// BulkSyncer handles bulk synchronization operations
type BulkSyncer struct {
	api           SyncAPI
	sourceStories []sb.Story
	sourceSpaceID int
	targetSpaceID int
}

// NewBulkSyncer creates a new bulk synchronizer
func NewBulkSyncer(api SyncAPI, sourceStories []sb.Story, sourceSpaceID, targetSpaceID int) *BulkSyncer {
	return &BulkSyncer{
		api:           api,
		sourceStories: sourceStories,
		sourceSpaceID: sourceSpaceID,
		targetSpaceID: targetSpaceID,
	}
}

// SyncStartsWith syncs all content with the given slug prefix
func (bs *BulkSyncer) SyncStartsWith(slug string) error {
	log.Printf("Syncing all content starting with: %s", slug)

	// Get all stories/folders that match the prefix
	toSync := bs.getStoriesWithPrefix(slug)

	// Sort by type and depth (folders first, then stories)
	bs.sortByTypeAndDepth(toSync)

	// Create story syncer for individual operations
	syncer := NewStorySyncer(bs.api, bs.sourceSpaceID, bs.targetSpaceID)

	// Sync each item in the correct order
	for _, st := range toSync {
		var err error
		if st.IsFolder {
			_, err = syncer.SyncFolderDetailed(st, true) // Assume publish = true for bulk ops
		} else {
			_, err = syncer.SyncStoryDetailed(st, true)
		}

		if err != nil {
			return err
		}
	}

	log.Printf("Completed syncing %d items starting with %s", len(toSync), slug)
	return nil
}

// SyncStartsWithDetailed syncs all content with prefix and returns detailed results
func (bs *BulkSyncer) SyncStartsWithDetailed(slug string) (*SyncItemResult, error) {
	log.Printf("Syncing all content starting with: %s", slug)

	// Get all stories/folders that match the prefix
	toSync := bs.getStoriesWithPrefix(slug)

	// Sort by type and depth (folders first, then stories)
	bs.sortByTypeAndDepth(toSync)

	// Create story syncer for individual operations
	syncer := NewStorySyncer(bs.api, bs.sourceSpaceID, bs.targetSpaceID)

	var warnings []string
	totalCreated := 0
	totalUpdated := 0

	// Sync each item in the correct order
	for _, st := range toSync {
		var result *SyncItemResult
		var err error

		if st.IsFolder {
			result, err = syncer.SyncFolderDetailed(st, true)
		} else {
			result, err = syncer.SyncStoryDetailed(st, true)
		}

		if err != nil {
			return nil, err
		}

		if result != nil {
			if result.Operation == OperationCreate {
				totalCreated++
			} else if result.Operation == OperationUpdate {
				totalUpdated++
			}

			if result.Warning != "" {
				warnings = append(warnings, fmt.Sprintf("%s: %s", st.FullSlug, result.Warning))
			}
		}
	}

	operation := fmt.Sprintf("bulk (%d created, %d updated)", totalCreated, totalUpdated)
	warning := ""
	if len(warnings) > 0 {
		warning = strings.Join(warnings, "; ")
	}

	log.Printf("Completed syncing %d items starting with %s", len(toSync), slug)
	return &SyncItemResult{
		Operation: operation,
		Warning:   warning,
	}, nil
}

// getStoriesWithPrefix returns all stories that match the slug prefix
func (bs *BulkSyncer) getStoriesWithPrefix(slug string) []sb.Story {
	var toSync []sb.Story
	for _, st := range bs.sourceStories {
		if st.FullSlug == slug || strings.HasPrefix(st.FullSlug, slug+"/") {
			toSync = append(toSync, st)
		}
	}
	return toSync
}

// sortByTypeAndDepth sorts stories by type (folders first) and depth (shallow first)
func (bs *BulkSyncer) sortByTypeAndDepth(stories []sb.Story) {
	sort.Slice(stories, func(i, j int) bool {
		storyI, storyJ := stories[i], stories[j]

		// Folders always come before stories
		if storyI.IsFolder && !storyJ.IsFolder {
			return true
		}
		if !storyI.IsFolder && storyJ.IsFolder {
			return false
		}

		// Both are folders or both are stories - sort by depth (shallow first)
		depthI := strings.Count(storyI.FullSlug, "/")
		depthJ := strings.Count(storyJ.FullSlug, "/")

		if depthI != depthJ {
			return depthI < depthJ
		}

		return storyI.FullSlug < storyJ.FullSlug
	})
}

// FindTarget searches for a story in the target space by full slug
func FindTarget(targetStories []sb.Story, fullSlug string) int {
	for i, st := range targetStories {
		if st.FullSlug == fullSlug {
			return i
		}
	}
	return -1
}

// FindSource searches for a story in the source space by full slug
func FindSource(sourceStories []sb.Story, fullSlug string) (sb.Story, bool) {
	for _, st := range sourceStories {
		if st.FullSlug == fullSlug {
			return st, true
		}
	}
	return sb.Story{}, false
}

// NextTargetID returns the next available ID in the target space
func NextTargetID(targetStories []sb.Story) int {
	max := 0
	for _, st := range targetStories {
		if st.ID > max {
			max = st.ID
		}
	}
	return max + 1
}
