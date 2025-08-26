package ui

import (
	"context"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"storyblok-sync/internal/sb"
	"storyblok-sync/internal/ui/sync"
)

// Constants for sync operations and timeouts
const (
	// API timeout constants
	defaultTimeout = 15 * time.Second
	longTimeout    = 30 * time.Second
	
	// Operation types - use sync package constants
	operationCreate = sync.OperationCreate
	operationUpdate = sync.OperationUpdate
	operationSkip   = sync.OperationSkip
)


// Legacy wrapper for content management - now uses the extracted module
type contentManager struct {
	*sync.ContentManager
}

// newContentManager creates a new content manager using the extracted module
func newContentManager(api folderAPI, spaceID int) *contentManager {
	return &contentManager{
		ContentManager: sync.NewContentManager(api, spaceID),
	}
}

// ensureContent is a legacy wrapper that calls the new EnsureContent method
func (cm *contentManager) ensureContent(ctx context.Context, story sb.Story) (sb.Story, error) {
	return cm.EnsureContent(ctx, story)
}

// Legacy wrappers for utility functions - now uses the extracted module
func prepareStoryForCreation(story sb.Story) sb.Story {
	return sync.PrepareStoryForCreation(story)
}

func prepareStoryForUpdate(source, target sb.Story) sb.Story {
	return sync.PrepareStoryForUpdate(source, target)
}

// resolveParentFolder resolves and sets the correct parent folder ID for a story
func (m *Model) resolveParentFolder(ctx context.Context, story sb.Story) (sb.Story, string, error) {
	var warning string

	if story.FolderID == nil {
		return story, warning, nil
	}

	parentSlugStr := parentSlug(story.FullSlug)
	if parentSlugStr == "" {
		story.FolderID = nil
		return story, warning, nil
	}

	targetParents, err := m.api.GetStoriesBySlug(ctx, m.targetSpace.ID, parentSlugStr)
	if err != nil {
		return story, warning, err
	}

	if len(targetParents) > 0 {
		story.FolderID = &targetParents[0].ID
	} else {
		story.FolderID = nil
		warning = "Parent folder not found in target space"
	}

	return story, warning, nil
}

// syncUUID updates the UUID of a target story if it differs from source
func (m *Model) syncUUID(ctx context.Context, targetStory sb.Story, sourceUUID string) error {
	if targetStory.UUID == sourceUUID || sourceUUID == "" {
		return nil
	}

	log.Printf("DEBUG: Updating UUID for %s from %s to %s",
		targetStory.FullSlug, targetStory.UUID, sourceUUID)

	err := m.api.UpdateStoryUUID(ctx, m.targetSpace.ID, targetStory.ID, sourceUUID)
	if err != nil {
		log.Printf("Warning: failed to update UUID for story %s: %v", targetStory.FullSlug, err)
		return err
	}

	return nil
}

func ensureDefaultContent(story sb.Story) sb.Story {
	return sync.EnsureDefaultContent(story)
}

// Legacy type aliases for backward compatibility
type syncItemResult = sync.SyncItemResult
type syncResultMsg = sync.SyncResultMsg
type syncCancelledMsg = sync.SyncCancelledMsg

// Legacy wrapper for logging functions - now uses the extracted module
func logError(operation, slug string, err error, story *sb.Story) {
	sync.LogError(operation, slug, err, story)
}

func logWarning(operation, slug, warning string, story *sb.Story) {
	sync.LogWarning(operation, slug, warning, story)
}

func logSuccess(operation, slug string, duration int64, targetStory *sb.Story) {
	sync.LogSuccess(operation, slug, duration, targetStory)
}

// logExtendedErrorContext is now handled within the sync.LogError function

func getFolderPaths(slug string) []string {
	return sync.GetFolderPaths(slug)
}

// buildTargetFolderMap creates a map of existing folders in target space for quick lookup
func (m *Model) buildTargetFolderMap() map[string]sb.Story {
	folderMap := make(map[string]sb.Story)
	for _, story := range m.storiesTarget {
		if story.IsFolder {
			folderMap[story.FullSlug] = story
		}
	}
	return folderMap
}

// findMissingFolderPaths analyzes selected items and identifies missing parent folders
func (m *Model) findMissingFolderPaths(items []PreflightItem) map[string]sb.Story {
	targetFolders := m.buildTargetFolderMap()
	sourceFolders := make(map[string]sb.Story)

	// Build source folder map for quick lookup
	for _, story := range m.storiesSource {
		if story.IsFolder {
			sourceFolders[story.FullSlug] = story
		}
	}

	missingFolders := make(map[string]sb.Story)

	for _, item := range items {
		if item.Skip || item.Story.IsFolder {
			continue
		}

		// Get all parent folder paths for this story
		folderPaths := getFolderPaths(item.Story.FullSlug)
		for _, folderPath := range folderPaths {
			// Skip if folder already exists in target or already identified as missing
			if _, exists := targetFolders[folderPath]; exists {
				continue
			}
			if _, alreadyFound := missingFolders[folderPath]; alreadyFound {
				continue
			}

			// Find folder in source space
			if sourceFolder, found := sourceFolders[folderPath]; found {
				missingFolders[folderPath] = sourceFolder
				log.Printf("DEBUG: Found missing folder path: %s", folderPath)
			} else {
				log.Printf("WARNING: Required folder %s not found in source space", folderPath)
			}
		}
	}

	return missingFolders
}

// optimizePreflight deduplicates entries, pre-plans missing folders, sorts by sync order (folders first), and merges full folder selections into starts_with tasks.
func (m *Model) optimizePreflight() {
	log.Printf("Optimizing preflight with %d items", len(m.preflight.items))

	selected := make(map[string]*PreflightItem)
	for i := range m.preflight.items {
		it := &m.preflight.items[i]
		if it.Skip {
			continue
		}
		if _, ok := selected[it.Story.FullSlug]; ok {
			it.Skip = true
			continue
		}
		selected[it.Story.FullSlug] = it
	}

	// Create initial optimized list
	optimized := make([]PreflightItem, 0, len(m.preflight.items))
	for _, it := range m.preflight.items {
		if it.Skip {
			continue
		}
		it.Run = RunPending
		optimized = append(optimized, it)
	}

	// Find and add missing folder paths
	missingFolders := m.findMissingFolderPaths(optimized)
	log.Printf("Found %d missing folders that need to be created", len(missingFolders))

	// Build a map of already included slugs to avoid duplicates
	existingSlugs := make(map[string]bool)
	for _, item := range optimized {
		existingSlugs[item.Story.FullSlug] = true
	}

	for _, folder := range missingFolders {
		// Skip if folder is already included in the optimization list
		if existingSlugs[folder.FullSlug] {
			log.Printf("DEBUG: Folder %s already in optimization list, skipping auto-add", folder.FullSlug)
			continue
		}

		// Create preflight item for missing folder
		folderItem := PreflightItem{
			Story:     folder,
			Collision: false, // Missing folders don't have collisions
			Skip:      false,
			Selected:  true, // Auto-selected for sync
			State:     StateCreate,
			Run:       RunPending,
		}
		optimized = append(optimized, folderItem)
		existingSlugs[folder.FullSlug] = true
		log.Printf("DEBUG: Auto-added missing folder to preflight: %s", folder.FullSlug)
	}

	// Sort by sync priority: folders first (by depth), then stories
	sort.Slice(optimized, func(i, j int) bool {
		itemI, itemJ := optimized[i], optimized[j]

		// Folders always come before stories
		if itemI.Story.IsFolder && !itemJ.Story.IsFolder {
			return true
		}
		if !itemI.Story.IsFolder && itemJ.Story.IsFolder {
			return false
		}

		// Both are folders or both are stories - sort by depth (shallow first)
		depthI := strings.Count(itemI.Story.FullSlug, "/")
		depthJ := strings.Count(itemJ.Story.FullSlug, "/")

		if depthI != depthJ {
			return depthI < depthJ
		}

		// Same depth - sort alphabetically for consistent order
		return itemI.Story.FullSlug < itemJ.Story.FullSlug
	})

	m.preflight.items = optimized
	log.Printf("Optimized to %d items (%d missing folders auto-added), sync order: folders first, then stories", len(optimized), len(missingFolders))
}

func (m *Model) runNextItem() tea.Cmd {
	if m.syncIndex >= len(m.preflight.items) {
		return nil
	}
	idx := m.syncIndex
	m.preflight.items[idx].Run = RunRunning

	// Capture the context for this operation
	ctx := m.syncContext

	return func() tea.Msg {
		// Check if context is already cancelled before starting
		select {
		case <-ctx.Done():
			return syncResultMsg{Index: idx, Cancelled: true}
		default:
		}

		it := m.preflight.items[idx]
		log.Printf("Starting sync for item %d: %s (folder: %t)", idx, it.Story.FullSlug, it.Story.IsFolder)

		startTime := time.Now()
		var err error
		var result *syncItemResult

		switch {
		case it.StartsWith:
			err = m.syncWithRetry(func() error {
				var syncErr error
				result, syncErr = m.syncStartsWithDetailed(it.Story.FullSlug)
				return syncErr
			})
		case it.Story.IsFolder:
			err = m.syncWithRetry(func() error {
				var syncErr error
				result, syncErr = m.syncFolderDetailed(it.Story)
				return syncErr
			})
		default:
			err = m.syncWithRetry(func() error {
				var syncErr error
				result, syncErr = m.syncStoryContentDetailed(it.Story)
				return syncErr
			})
		}

		duration := time.Since(startTime).Milliseconds()

		if err != nil {
			logError("sync", it.Story.FullSlug, err, &it.Story)
		} else if result != nil {
			if result.Warning != "" {
				logWarning(result.Operation, it.Story.FullSlug, result.Warning, &it.Story)
			} else {
				logSuccess(result.Operation, it.Story.FullSlug, duration, result.TargetStory)
			}
			time.Sleep(50 * time.Millisecond)
		} else {
			log.Printf("Sync completed for %s (no detailed result)", it.Story.FullSlug)
			time.Sleep(50 * time.Millisecond)
		}

		return syncResultMsg{Index: idx, Err: err, Result: result, Duration: duration}
	}
}

// syncWithRetry executes an operation with retry logic for rate limiting and transient errors
func (m *Model) syncWithRetry(operation func() error) error {
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		err := operation()
		if err == nil {
			return nil
		}

		lastErr = err
		log.Printf("Sync attempt %d failed: %v", attempt+1, err)

		// Log additional context for retries
		if attempt < 2 {
			log.Printf("  Will retry in %v (attempt %d/3)",
				func() time.Duration {
					if isRateLimited(err) {
						return time.Second * time.Duration(attempt+1)
					}
					return time.Millisecond * 500
				}(), attempt+2)
		} else {
			log.Printf("  Max retries (3) exceeded, giving up")
		}

		// Check if it's a rate limiting error (implement based on actual API responses)
		if isRateLimited(err) {
			sleepDuration := time.Second * time.Duration(attempt+1)
			log.Printf("Rate limited, sleeping for %v", sleepDuration)
			time.Sleep(sleepDuration)
			continue
		}

		// For other errors, don't retry immediately
		if attempt < 2 {
			time.Sleep(time.Millisecond * 500)
		}
	}

	// Log final failure with additional context
	log.Printf("RETRY FAILED: Operation failed after 3 attempts, final error: %v", lastErr)
	// Extended error context now handled in sync.LogError

	return lastErr
}

// isRateLimited checks if the error indicates rate limiting (customize based on API responses)
func isRateLimited(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "rate limit") || strings.Contains(errStr, "429")
}

func isDevModePublishLimit(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "development mode") && strings.Contains(msg, "Publishing is limited")
}

type updateAPI interface {
	UpdateStory(ctx context.Context, spaceID int, st sb.Story, publish bool) (sb.Story, error)
}

type createAPI interface {
	CreateStoryWithPublish(ctx context.Context, spaceID int, st sb.Story, publish bool) (sb.Story, error)
}

func updateStoryWithPublishRetry(ctx context.Context, api updateAPI, spaceID int, st sb.Story, publish bool) (sb.Story, error) {
	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		updated, err := api.UpdateStory(ctx, spaceID, st, publish)
		if err == nil {
			return updated, nil
		}
		lastErr = err
		if publish && isDevModePublishLimit(err) {
			log.Printf("Publish limit reached, retrying without publish for %s", st.FullSlug)
			publish = false
			continue
		}
		return sb.Story{}, err
	}
	return sb.Story{}, lastErr
}

func createStoryWithPublishRetry(ctx context.Context, api createAPI, spaceID int, st sb.Story, publish bool) (sb.Story, error) {
	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		created, err := api.CreateStoryWithPublish(ctx, spaceID, st, publish)
		if err == nil {
			return created, nil
		}
		lastErr = err
		if publish && isDevModePublishLimit(err) {
			log.Printf("Publish limit reached, retrying without publish for %s", st.FullSlug)
			publish = false
			continue
		}
		return sb.Story{}, err
	}
	return sb.Story{}, lastErr
}

type folderAPI interface {
	GetStoriesBySlug(ctx context.Context, spaceID int, slug string) ([]sb.Story, error)
	GetStoryWithContent(ctx context.Context, spaceID, storyID int) (sb.Story, error)
	CreateStoryWithPublish(ctx context.Context, spaceID int, st sb.Story, publish bool) (sb.Story, error)
}

// folderPathBuilder handles the creation of folder hierarchies
type folderPathBuilder struct {
	api           folderAPI
	report        *Report
	sourceStories map[string]sb.Story
	contentMgr    *contentManager
	srcSpaceID    int
	tgtSpaceID    int
	publish       bool
}

// newFolderPathBuilder creates a new folder path builder
func newFolderPathBuilder(api folderAPI, report *Report, sourceStories []sb.Story, srcSpaceID, tgtSpaceID int, publish bool) *folderPathBuilder {
	// Build source stories map for quick lookup
	sourceMap := make(map[string]sb.Story)
	for _, story := range sourceStories {
		sourceMap[story.FullSlug] = story
	}

	return &folderPathBuilder{
		api:           api,
		report:        report,
		sourceStories: sourceMap,
		contentMgr:    newContentManager(api, srcSpaceID),
		srcSpaceID:    srcSpaceID,
		tgtSpaceID:    tgtSpaceID,
		publish:       publish,
	}
}

// checkExistingFolder checks if a folder exists in the target space
func (fpb *folderPathBuilder) checkExistingFolder(ctx context.Context, path string) (*sb.Story, error) {
	existing, err := fpb.api.GetStoriesBySlug(ctx, fpb.tgtSpaceID, path)
	if err != nil {
		return nil, err
	}

	if len(existing) == 0 {
		return nil, nil
	}

	folder := existing[0]
	log.Printf("DEBUG: Found existing folder: %s (ID: %d)", path, folder.ID)
	return &folder, nil
}

// prepareSourceFolder prepares a source folder for creation in target space
func (fpb *folderPathBuilder) prepareSourceFolder(ctx context.Context, path string, parentID *int) (sb.Story, error) {
	source, exists := fpb.sourceStories[path]
	if !exists {
		return sb.Story{}, fmt.Errorf("source folder not found: %s", path)
	}

	// Ensure content is loaded
	folder, err := fpb.contentMgr.ensureContent(ctx, source)
	if err != nil {
		log.Printf("DEBUG: Failed to fetch content for folder %s: %v", path, err)
		return sb.Story{}, err
	}

	// Prepare for creation
	folder = prepareStoryForCreation(folder)
	folder.FolderID = parentID

	log.Printf("DEBUG: Prepared source folder %s with content: %t", path, folder.Content != nil)
	return folder, nil
}

// createFolder creates a single folder in the target space
func (fpb *folderPathBuilder) createFolder(ctx context.Context, folder sb.Story) (sb.Story, error) {
	log.Printf("DEBUG: Creating folder: %s", folder.FullSlug)

	created, err := fpb.api.CreateStoryWithPublish(ctx, fpb.tgtSpaceID, folder, fpb.publish)
	if err != nil {
		log.Printf("DEBUG: Failed to create folder %s: %v", folder.FullSlug, err)
		return sb.Story{}, err
	}

	log.Printf("DEBUG: Successfully created folder %s (ID: %d)", created.FullSlug, created.ID)

	return created, nil
}

// ensureFolderPathImpl creates missing folders in a path hierarchy using modular approach
func ensureFolderPathImpl(api folderAPI, report *Report, sourceStories []sb.Story, srcSpaceID, tgtSpaceID int, slug string, publish bool) ([]sb.Story, error) {
	parts := strings.Split(slug, "/")
	if len(parts) <= 1 {
		return nil, nil
	}

	builder := newFolderPathBuilder(api, report, sourceStories, srcSpaceID, tgtSpaceID, publish)
	var created []sb.Story
	var parentID *int

	// Process each folder in the path hierarchy
	for i := 0; i < len(parts)-1; i++ {
		path := strings.Join(parts[:i+1], "/")

		ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)

		// Check if folder already exists
		existing, err := builder.checkExistingFolder(ctx, path)
		cancel()

		if err != nil {
			return created, err
		}

		if existing != nil {
			// Folder exists, use its ID as parent for next level
			parentID = &existing.ID
			continue
		}

		// Folder doesn't exist, create it
		ctx, cancel = context.WithTimeout(context.Background(), defaultTimeout)
		folder, err := builder.prepareSourceFolder(ctx, path, parentID)
		cancel()

		if err != nil {
			return created, err
		}

		// Create the folder
		ctx, cancel = context.WithTimeout(context.Background(), defaultTimeout)
		createdFolder, err := builder.createFolder(ctx, folder)
		cancel()

		if err != nil {
			return created, err
		}

		created = append(created, createdFolder)
		parentID = &createdFolder.ID

		// Update report
		if report != nil {
			report.AddSuccess(createdFolder.FullSlug, operationCreate, 0, &createdFolder)
		}
	}

	return created, nil
}

func (m *Model) ensureFolderPath(slug string) ([]sb.Story, error) {
	return ensureFolderPathImpl(m.api, &m.report, m.storiesSource, m.sourceSpace.ID, m.targetSpace.ID, slug, m.shouldPublish())
}

func (m *Model) shouldPublish() bool {
	if m.targetSpace != nil && m.targetSpace.PlanLevel == 999 {
		return false
	}
	return true
}

// syncFolder handles folder synchronization with proper parent resolution
func (m *Model) syncFolder(sourceFolder sb.Story) error {
	log.Printf("Syncing folder: %s", sourceFolder.FullSlug)

	// Use the sourceFolder data directly, which should already have content
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	fullFolder := sourceFolder

	// DEBUG: Log content preservation
	log.Printf("DEBUG: syncFolder %s has content: %t, is_folder: %t", sourceFolder.FullSlug, sourceFolder.Content != nil, sourceFolder.IsFolder)
	if sourceFolder.Content != nil {
		contentKeys := make([]string, 0, len(sourceFolder.Content))
		for k := range sourceFolder.Content {
			contentKeys = append(contentKeys, k)
		}
		log.Printf("DEBUG: syncFolder source content keys: %v", contentKeys)

		// Special logging for content_types field
		if sourceFolder.IsFolder {
			if contentTypes, ok := sourceFolder.Content["content_types"]; ok {
				log.Printf("DEBUG: syncFolder %s has content_types: %v", sourceFolder.FullSlug, contentTypes)
			} else {
				log.Printf("DEBUG: syncFolder %s missing content_types field", sourceFolder.FullSlug)
			}
		}
	}
	log.Printf("DEBUG: syncFolder %s ContentType field: '%s'", sourceFolder.FullSlug, sourceFolder.ContentType)

	// If the source folder doesn't have content, try to fetch it from API
	if fullFolder.Content == nil {
		apiFolder, err := m.api.GetStoryWithContent(ctx, m.sourceSpace.ID, sourceFolder.ID)
		if err != nil {
			return err
		}
		// Preserve any content that came from the API
		if apiFolder.Content != nil {
			fullFolder.Content = apiFolder.Content
		} else {
			// Create minimal content structure for folders
			fullFolder.Content = map[string]interface{}{}
		}
	}

	// Don't modify ContentType or Content - preserve exactly as from source

	// Check if folder already exists in target
	existing, err := m.api.GetStoriesBySlug(ctx, m.targetSpace.ID, sourceFolder.FullSlug)
	if err != nil {
		return err
	}

	// Resolve parent folder ID
	if fullFolder.FolderID != nil {
		parentSlug := parentSlug(fullFolder.FullSlug)
		if parentSlug != "" {
			if targetParents, err := m.api.GetStoriesBySlug(ctx, m.targetSpace.ID, parentSlug); err == nil && len(targetParents) > 0 {
				fullFolder.FolderID = &targetParents[0].ID
			} else {
				fullFolder.FolderID = nil // Set to root if parent not found
			}
		}
	}

	// Handle translated slugs
	fullFolder = m.processTranslatedSlugs(fullFolder, existing)

	if len(existing) > 0 {
		// Update existing folder
		existingFolder := existing[0]
		fullFolder.ID = existingFolder.ID
		updated, err := updateStoryWithPublishRetry(ctx, m.api, m.targetSpace.ID, fullFolder, m.shouldPublish())
		if err != nil {
			return err
		}

		// Update UUID if different
		if updated.UUID != fullFolder.UUID && fullFolder.UUID != "" {
			if err := m.api.UpdateStoryUUID(ctx, m.targetSpace.ID, updated.ID, fullFolder.UUID); err != nil {
				log.Printf("Warning: failed to update UUID for folder %s: %v", fullFolder.FullSlug, err)
			}
		}

		log.Printf("Updated folder: %s", fullFolder.FullSlug)
	} else {
		// Create new folder
		// Clear ALL fields that shouldn't be set on creation (based on Storyblok CLI)
		fullFolder.ID = 0
		fullFolder.CreatedAt = ""
		fullFolder.UpdatedAt = "" // This was causing 422!

		// Note: Don't reset Position and FolderID here as they are set by parent resolution above

		// Ensure folders have proper content structure
		if fullFolder.IsFolder && fullFolder.Content == nil {
			fullFolder.Content = map[string]interface{}{}
		}

		created, err := createStoryWithPublishRetry(ctx, m.api, m.targetSpace.ID, fullFolder, m.shouldPublish())
		if err != nil {
			return err
		}

		// Update UUID if different
		if created.UUID != fullFolder.UUID && fullFolder.UUID != "" {
			if err := m.api.UpdateStoryUUID(ctx, m.targetSpace.ID, created.ID, fullFolder.UUID); err != nil {
				log.Printf("Warning: failed to update UUID for new folder %s: %v", fullFolder.FullSlug, err)
			}
		}

		log.Printf("Created folder: %s", fullFolder.FullSlug)
	}

	return nil
}

// syncFolderDetailed handles folder synchronization and returns detailed results
func (m *Model) syncFolderDetailed(sourceFolder sb.Story) (*syncItemResult, error) {
	log.Printf("Syncing folder: %s", sourceFolder.FullSlug)

	ctx, cancel := context.WithTimeout(context.Background(), longTimeout)
	defer cancel()

	// Ensure folder has content
	contentMgr := newContentManager(m.api, m.sourceSpace.ID)
	fullFolder, err := contentMgr.ensureContent(ctx, sourceFolder)
	if err != nil {
		log.Printf("Failed to ensure content for folder %s: %v", sourceFolder.FullSlug, err)
		// Extended error context now handled in sync.LogError
		return nil, err
	}

	log.Printf("DEBUG: syncFolderDetailed preserving content for %s", sourceFolder.FullSlug)

	// Check if folder already exists in target
	existing, err := m.api.GetStoriesBySlug(ctx, m.targetSpace.ID, sourceFolder.FullSlug)
	if err != nil {
		log.Printf("Failed to check if target folder exists for %s: %v", sourceFolder.FullSlug, err)
		// Extended error context now handled in sync.LogError
		return nil, err
	}

	// Resolve parent folder ID and handle translated slugs
	fullFolder, warning, err := m.resolveParentFolder(ctx, fullFolder)
	if err != nil {
		return nil, err
	}

	fullFolder = m.processTranslatedSlugs(fullFolder, existing)

	// Execute create or update operation
	return m.executeSync(ctx, fullFolder, existing, warning)
}

// executeSync handles the common create/update logic for both folders and stories
func (m *Model) executeSync(ctx context.Context, story sb.Story, existing []sb.Story, warning string) (*syncItemResult, error) {
	var targetStory *sb.Story
	var operation string

	if len(existing) > 0 {
		// Update existing item
		story = prepareStoryForUpdate(story, existing[0])
		updated, err := updateStoryWithPublishRetry(ctx, m.api, m.targetSpace.ID, story, m.shouldPublish())
		if err != nil {
			log.Printf("Failed to update %s (ID: %d): %v", story.FullSlug, story.ID, err)
			// Extended error context now handled in sync.LogError
			return nil, err
		}

		// Update UUID if different
		if uuidErr := m.syncUUID(ctx, updated, story.UUID); uuidErr != nil {
			if warning == "" {
				warning = fmt.Sprintf("Failed to update UUID: %v", uuidErr)
			} else {
				warning += fmt.Sprintf("; Failed to update UUID: %v", uuidErr)
			}
		}

		targetStory = &updated
		operation = operationUpdate
		log.Printf("Updated %s: %s", itemType(story), story.FullSlug)
	} else {
		// Create new item
		story = prepareStoryForCreation(story)
		story = ensureDefaultContent(story)

		created, err := createStoryWithPublishRetry(ctx, m.api, m.targetSpace.ID, story, m.shouldPublish())
		if err != nil {
			log.Printf("Failed to create %s: %v", story.FullSlug, err)
			// Extended error context now handled in sync.LogError
			return nil, err
		}

		// Update UUID if different
		if uuidErr := m.syncUUID(ctx, created, story.UUID); uuidErr != nil {
			if warning == "" {
				warning = fmt.Sprintf("Failed to update UUID: %v", uuidErr)
			} else {
				warning += fmt.Sprintf("; Failed to update UUID: %v", uuidErr)
			}
		}

		targetStory = &created
		operation = operationCreate
		log.Printf("Created %s: %s", itemType(story), story.FullSlug)
	}

	return &syncItemResult{
		Operation:   operation,
		TargetStory: targetStory,
		Warning:     warning,
	}, nil
}

// itemType returns a string describing the item type for logging
func itemType(story sb.Story) string {
	if story.IsFolder {
		return "folder"
	}
	return "story"
}

// syncStoryContent handles story synchronization with proper UUID management
func (m *Model) syncStoryContent(sourceStory sb.Story) error {
	log.Printf("Syncing story: %s", sourceStory.FullSlug)

	// Get full story content from source
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	fullStory, err := m.api.GetStoryWithContent(ctx, m.sourceSpace.ID, sourceStory.ID)
	if err != nil {
		return err
	}

	// Check if story already exists in target
	existing, err := m.api.GetStoriesBySlug(ctx, m.targetSpace.ID, sourceStory.FullSlug)
	if err != nil {
		return err
	}

	// Resolve parent folder ID if story is in a folder
	if fullStory.FolderID != nil {
		parentSlug := parentSlug(fullStory.FullSlug)
		if parentSlug != "" {
			if targetParents, err := m.api.GetStoriesBySlug(ctx, m.targetSpace.ID, parentSlug); err == nil && len(targetParents) > 0 {
				fullStory.FolderID = &targetParents[0].ID
			} else {
				log.Printf("Warning: parent folder %s not found for story %s", parentSlug, fullStory.FullSlug)
				fullStory.FolderID = nil // Set to root if parent not found
			}
		}
	}

	// Handle translated slugs
	fullStory = m.processTranslatedSlugs(fullStory, existing)

	if len(existing) > 0 {
		// Update existing story
		existingStory := existing[0]
		fullStory.ID = existingStory.ID
		updated, err := updateStoryWithPublishRetry(ctx, m.api, m.targetSpace.ID, fullStory, m.shouldPublish())
		if err != nil {
			return err
		}

		// Update UUID if different
		if updated.UUID != fullStory.UUID && fullStory.UUID != "" {
			if err := m.api.UpdateStoryUUID(ctx, m.targetSpace.ID, updated.ID, fullStory.UUID); err != nil {
				log.Printf("Warning: failed to update UUID for story %s: %v", fullStory.FullSlug, err)
			}
		}

		log.Printf("Updated story: %s", fullStory.FullSlug)
	} else {
		// Create new story
		// Clear ALL fields that shouldn't be set on creation (based on Storyblok CLI)
		fullStory.ID = 0
		fullStory.CreatedAt = ""
		fullStory.UpdatedAt = "" // This was causing 422!

		// Note: Don't reset Position and FolderID here as they are set by parent resolution above

		// Ensure stories have content (required for Storyblok API)
		if !fullStory.IsFolder && fullStory.Content == nil {
			fullStory.Content = map[string]interface{}{
				"component": "page", // Default component type
			}
		}

		created, err := createStoryWithPublishRetry(ctx, m.api, m.targetSpace.ID, fullStory, m.shouldPublish())
		if err != nil {
			return err
		}

		// Update UUID if different
		if created.UUID != fullStory.UUID && fullStory.UUID != "" {
			if err := m.api.UpdateStoryUUID(ctx, m.targetSpace.ID, created.ID, fullStory.UUID); err != nil {
				log.Printf("Warning: failed to update UUID for new story %s: %v", fullStory.FullSlug, err)
			}
		}

		log.Printf("Created story: %s", fullStory.FullSlug)
	}

	return nil
}

// syncStoryContentDetailed handles story synchronization and returns detailed results
// Note: Folder structure is now pre-planned in optimizePreflight(), so no need to ensure folder path here
func (m *Model) syncStoryContentDetailed(sourceStory sb.Story) (*syncItemResult, error) {
	log.Printf("Syncing story: %s", sourceStory.FullSlug)

	ctx, cancel := context.WithTimeout(context.Background(), longTimeout)
	defer cancel()

	// Ensure story has content
	contentMgr := newContentManager(m.api, m.sourceSpace.ID)
	fullStory, err := contentMgr.ensureContent(ctx, sourceStory)
	if err != nil {
		log.Printf("Failed to ensure content for story %s: %v", sourceStory.FullSlug, err)
		// Extended error context now handled in sync.LogError
		return nil, err
	}

	// Check if story already exists in target
	existing, err := m.api.GetStoriesBySlug(ctx, m.targetSpace.ID, sourceStory.FullSlug)
	if err != nil {
		log.Printf("Failed to check if target story exists for %s: %v", sourceStory.FullSlug, err)
		// Extended error context now handled in sync.LogError
		return nil, err
	}

	// Resolve parent folder ID
	fullStory, warning, err := m.resolveParentFolder(ctx, fullStory)
	if err != nil {
		return nil, err
	}

	// Handle translated slugs
	fullStory = m.processTranslatedSlugs(fullStory, existing)

	// Execute create or update operation
	return m.executeSync(ctx, fullStory, existing, warning)
}

// processTranslatedSlugs handles translated slug processing like the Storyblok CLI
func (m *Model) processTranslatedSlugs(sourceStory sb.Story, existingStories []sb.Story) sb.Story {
	if len(sourceStory.TranslatedSlugs) == 0 {
		return sourceStory
	}

	// Copy translated slugs and remove IDs
	translatedSlugs := make([]sb.TranslatedSlug, len(sourceStory.TranslatedSlugs))
	for i, ts := range sourceStory.TranslatedSlugs {
		translatedSlugs[i] = sb.TranslatedSlug{
			Lang: ts.Lang,
			Name: ts.Name,
			Path: ts.Path,
		}
	}

	// If there's an existing story, merge the translated slug IDs
	if len(existingStories) > 0 {
		existingStory := existingStories[0]
		if len(existingStory.TranslatedSlugs) > 0 {
			for i := range translatedSlugs {
				for _, existingTS := range existingStory.TranslatedSlugs {
					if translatedSlugs[i].Lang == existingTS.Lang {
						translatedSlugs[i].ID = existingTS.ID
						break
					}
				}
			}
		}
	}

	// Set the attributes for the API call
	sourceStory.TranslatedSlugsAttributes = translatedSlugs
	sourceStory.TranslatedSlugs = nil // Clear the original field

	return sourceStory
}

func (m *Model) syncStartsWith(slug string) error {
	log.Printf("Syncing all content starting with: %s", slug)

	// Get all stories/folders that match the prefix
	var toSync []sb.Story
	for _, st := range m.storiesSource {
		if st.FullSlug == slug || strings.HasPrefix(st.FullSlug, slug+"/") {
			toSync = append(toSync, st)
		}
	}

	// Sort by type and depth (folders first, then stories)
	sort.Slice(toSync, func(i, j int) bool {
		storyI, storyJ := toSync[i], toSync[j]

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

	// Sync each item in the correct order
	for _, st := range toSync {
		var err error
		if st.IsFolder {
			err = m.syncFolder(st)
		} else {
			err = m.syncStoryContent(st)
		}

		if err != nil {
			return err
		}
	}

	log.Printf("Completed syncing %d items starting with %s", len(toSync), slug)
	return nil
}

// syncStartsWithDetailed syncs all content with prefix and returns results
func (m *Model) syncStartsWithDetailed(slug string) (*syncItemResult, error) {
	log.Printf("Syncing all content starting with: %s", slug)

	// Get all stories/folders that match the prefix
	var toSync []sb.Story
	for _, st := range m.storiesSource {
		if st.FullSlug == slug || strings.HasPrefix(st.FullSlug, slug+"/") {
			toSync = append(toSync, st)
		}
	}

	// Sort by type and depth (folders first, then stories)
	sort.Slice(toSync, func(i, j int) bool {
		storyI, storyJ := toSync[i], toSync[j]

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

	var warnings []string
	totalCreated := 0
	totalUpdated := 0

	// Sync each item in the correct order
	for _, st := range toSync {
		var result *syncItemResult
		var err error

		if st.IsFolder {
			result, err = m.syncFolderDetailed(st)
		} else {
			result, err = m.syncStoryContentDetailed(st)
		}

		if err != nil {
			return nil, err
		}

		if result != nil {
			if result.Operation == "create" {
				totalCreated++
			} else if result.Operation == "update" {
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
	return &syncItemResult{
		Operation: operation,
		Warning:   warning,
	}, nil
}

func (m *Model) findTarget(fullSlug string) int {
	for i, st := range m.storiesTarget {
		if st.FullSlug == fullSlug {
			return i
		}
	}
	return -1
}

func (m *Model) findSource(fullSlug string) (sb.Story, bool) {
	for _, st := range m.storiesSource {
		if st.FullSlug == fullSlug {
			return st, true
		}
	}
	return sb.Story{}, false
}

func (m *Model) nextTargetID() int {
	max := 0
	for _, st := range m.storiesTarget {
		if st.ID > max {
			max = st.ID
		}
	}
	return max + 1
}

func parentSlug(full string) string {
	if i := strings.LastIndex(full, "/"); i >= 0 {
		return full[:i]
	}
	return ""
}
