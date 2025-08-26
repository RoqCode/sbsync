package sync

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"storyblok-sync/internal/sb"
)

// StorySyncer handles story and folder synchronization operations
type StorySyncer struct {
	api           SyncAPI
	contentMgr    *ContentManager
	sourceSpaceID int
	targetSpaceID int
}

// NewStorySyncer creates a new story synchronizer
func NewStorySyncer(api SyncAPI, sourceSpaceID, targetSpaceID int) *StorySyncer {
	return &StorySyncer{
		api:           api,
		contentMgr:    NewContentManager(api, sourceSpaceID),
		sourceSpaceID: sourceSpaceID,
		targetSpaceID: targetSpaceID,
	}
}

// SyncStory synchronizes a single story
func (ss *StorySyncer) SyncStory(ctx context.Context, story sb.Story, shouldPublish bool) (sb.Story, error) {
	log.Printf("Syncing story: %s", story.FullSlug)

	// Ensure content is loaded
	fullStory, err := ss.contentMgr.EnsureContent(ctx, story)
	if err != nil {
		return sb.Story{}, err
	}

	// Ensure non-folder stories have default content
	fullStory = EnsureDefaultContent(fullStory)

	// Check if story already exists in target
	existing, err := ss.api.GetStoriesBySlug(ctx, ss.targetSpaceID, story.FullSlug)
	if err != nil {
		return sb.Story{}, err
	}

	// Resolve parent folder ID if needed
	fullStory = ss.resolveParentFolder(ctx, fullStory)

	// Handle translated slugs
	fullStory = ProcessTranslatedSlugs(fullStory, existing)

	if len(existing) > 0 {
		// Update existing story
		existingStory := existing[0]
		updateStory := PrepareStoryForUpdate(fullStory, existingStory)

		updated, err := ss.api.UpdateStory(ctx, ss.targetSpaceID, updateStory, shouldPublish)
		if err != nil {
			return sb.Story{}, err
		}

		// Update UUID if different
		if updated.UUID != fullStory.UUID && fullStory.UUID != "" {
			if err := ss.api.UpdateStoryUUID(ctx, ss.targetSpaceID, updated.ID, fullStory.UUID); err != nil {
				log.Printf("Warning: failed to update UUID for story %s: %v", fullStory.FullSlug, err)
			}
		}

		log.Printf("Updated story: %s", fullStory.FullSlug)
		return updated, nil
	} else {
		// Create new story
		createStory := PrepareStoryForCreation(fullStory)

		created, err := ss.api.CreateStoryWithPublish(ctx, ss.targetSpaceID, createStory, shouldPublish)
		if err != nil {
			return sb.Story{}, err
		}

		log.Printf("Created story: %s", fullStory.FullSlug)
		return created, nil
	}
}

// SyncFolder synchronizes a single folder
func (ss *StorySyncer) SyncFolder(ctx context.Context, folder sb.Story, shouldPublish bool) (sb.Story, error) {
	log.Printf("Syncing folder: %s", folder.FullSlug)

	// Ensure content is loaded for folder
	fullFolder, err := ss.contentMgr.EnsureContent(ctx, folder)
	if err != nil {
		// If content loading fails, use folder as-is with minimal content
		fullFolder = folder
		if len(fullFolder.Content) == 0 {
			fullFolder.Content = json.RawMessage([]byte(`{}`))
		}
	}

	// Debug logging
	log.Printf("DEBUG: syncFolder %s has content: %t, is_folder: %t",
		folder.FullSlug, len(fullFolder.Content) > 0, fullFolder.IsFolder)

	// Check if folder already exists in target
	existing, err := ss.api.GetStoriesBySlug(ctx, ss.targetSpaceID, folder.FullSlug)
	if err != nil {
		return sb.Story{}, err
	}

	// Resolve parent folder ID if needed
	fullFolder = ss.resolveParentFolder(ctx, fullFolder)

	// Handle translated slugs
	fullFolder = ProcessTranslatedSlugs(fullFolder, existing)

	if len(existing) > 0 {
		// Update existing folder
		existingFolder := existing[0]
		updateFolder := PrepareStoryForUpdate(fullFolder, existingFolder)

		updated, err := ss.api.UpdateStory(ctx, ss.targetSpaceID, updateFolder, shouldPublish)
		if err != nil {
			return sb.Story{}, err
		}

		// Update UUID if different
		if updated.UUID != fullFolder.UUID && fullFolder.UUID != "" {
			if err := ss.api.UpdateStoryUUID(ctx, ss.targetSpaceID, updated.ID, fullFolder.UUID); err != nil {
				log.Printf("Warning: failed to update UUID for folder %s: %v", fullFolder.FullSlug, err)
			}
		}

		log.Printf("Updated folder: %s", fullFolder.FullSlug)
		return updated, nil
	} else {
		// Create new folder
		createFolder := PrepareStoryForCreation(fullFolder)

		// Ensure folders have proper content structure
		if createFolder.IsFolder && len(createFolder.Content) == 0 {
			createFolder.Content = json.RawMessage([]byte(`{}`))
		}

		created, err := ss.api.CreateStoryWithPublish(ctx, ss.targetSpaceID, createFolder, shouldPublish)
		if err != nil {
			return sb.Story{}, err
		}

		log.Printf("Created folder: %s", fullFolder.FullSlug)
		return created, nil
	}
}

// SyncStoryDetailed synchronizes a story and returns detailed result
func (ss *StorySyncer) SyncStoryDetailed(story sb.Story, shouldPublish bool) (*SyncItemResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	targetStory, err := ss.SyncStory(ctx, story, shouldPublish)
	if err != nil {
		return nil, err
	}

	// Determine operation type based on whether story existed
	operation := OperationCreate
	existing, _ := ss.api.GetStoriesBySlug(ctx, ss.targetSpaceID, story.FullSlug)
	if len(existing) > 0 {
		operation = OperationUpdate
	}

	return &SyncItemResult{
		Operation:   operation,
		TargetStory: &targetStory,
		Warning:     "",
	}, nil
}

// SyncFolderDetailed synchronizes a folder and returns detailed result
func (ss *StorySyncer) SyncFolderDetailed(folder sb.Story, shouldPublish bool) (*SyncItemResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	targetFolder, err := ss.SyncFolder(ctx, folder, shouldPublish)
	if err != nil {
		return nil, err
	}

	// Determine operation type based on whether folder existed
	operation := OperationCreate
	existing, _ := ss.api.GetStoriesBySlug(ctx, ss.targetSpaceID, folder.FullSlug)
	if len(existing) > 0 {
		operation = OperationUpdate
	}

	return &SyncItemResult{
		Operation:   operation,
		TargetStory: &targetFolder,
		Warning:     "",
	}, nil
}

// resolveParentFolder resolves and sets the correct parent folder ID for a story
func (ss *StorySyncer) resolveParentFolder(ctx context.Context, story sb.Story) sb.Story {
	if story.FolderID == nil {
		return story
	}

	parentSlugStr := ParentSlug(story.FullSlug)
	if parentSlugStr == "" {
		story.FolderID = nil
		return story
	}

	targetParents, err := ss.api.GetStoriesBySlug(ctx, ss.targetSpaceID, parentSlugStr)
	if err != nil {
		log.Printf("Warning: failed to resolve parent folder for %s: %v", story.FullSlug, err)
		return story
	}

	if len(targetParents) > 0 {
		story.FolderID = &targetParents[0].ID
	} else {
		story.FolderID = nil
		log.Printf("Warning: Parent folder %s not found in target space for %s", parentSlugStr, story.FullSlug)
	}

	return story
}
