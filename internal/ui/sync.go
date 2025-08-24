package ui

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"storyblok-sync/internal/sb"
)

// getContentKeys extracts keys from content map for debugging
func getContentKeys(content map[string]interface{}) []string {
	if content == nil {
		return nil
	}
	keys := make([]string, 0, len(content))
	for k := range content {
		keys = append(keys, k)
	}
	return keys
}

type syncItemResult struct {
	operation   string    // create|update|skip
	targetStory *sb.Story // created/updated story
	warning     string    // any warnings
}

type syncResultMsg struct {
	index    int
	err      error
	result   *syncItemResult
	duration int64 // in milliseconds
}

// logError logs comprehensive error information for debugging
func logError(operation, slug string, err error, story *sb.Story) {
	log.Printf("ERROR: %s failed for %s: %v", operation, slug, err)

	if story != nil {
		// Log story context
		log.Printf("ERROR CONTEXT for %s:", slug)
		log.Printf("  Story ID: %d", story.ID)
		log.Printf("  Story UUID: %s", story.UUID)
		log.Printf("  Story Name: %s", story.Name)
		log.Printf("  Full Slug: %s", story.FullSlug)
		log.Printf("  Is Folder: %t", story.IsFolder)
		log.Printf("  Published: %t", story.Published)

		if story.FolderID != nil {
			log.Printf("  Parent ID: %d", *story.FolderID)
		}

		if len(story.TagList) > 0 {
			log.Printf("  Tags: %v", story.TagList)
		}

		if len(story.TranslatedSlugs) > 0 {
			log.Printf("  Translated Slugs: %d entries", len(story.TranslatedSlugs))
			for _, ts := range story.TranslatedSlugs {
				log.Printf("    - %s: %s (%s)", ts.Lang, ts.Name, ts.Path)
			}
		}

		// Log content summary (first level keys only, to avoid huge logs)
		if story.Content != nil && len(story.Content) > 0 {
			contentKeys := make([]string, 0, len(story.Content))
			for key := range story.Content {
				contentKeys = append(contentKeys, key)
			}
			log.Printf("  Content Keys: %v", contentKeys)

			// Log component type if available
			if component, ok := story.Content["component"].(string); ok {
				log.Printf("  Component Type: %s", component)
			}
		}

		// Log full story as JSON for complete debugging (only if content is small enough)
		if storyJSON, err := json.Marshal(story); err == nil {
			if len(storyJSON) < 2000 { // Only log if less than 2KB
				log.Printf("  Full Story JSON: %s", string(storyJSON))
			} else {
				log.Printf("  Full Story JSON: [too large, %d bytes - see report file]", len(storyJSON))
			}
		}
	}

	// Log additional error context if available
	logExtendedErrorContext(err)
}

// logWarning logs comprehensive warning information
func logWarning(operation, slug, warning string, story *sb.Story) {
	log.Printf("WARNING: %s for %s: %s", operation, slug, warning)

	if story != nil {
		log.Printf("WARNING CONTEXT for %s:", slug)
		log.Printf("  Story ID: %d (UUID: %s)", story.ID, story.UUID)
		log.Printf("  Full Slug: %s", story.FullSlug)
		if story.FolderID != nil {
			log.Printf("  Parent ID: %d", *story.FolderID)
		}
	}
}

// logSuccess logs success with context information
func logSuccess(operation, slug string, duration int64, targetStory *sb.Story) {
	log.Printf("SUCCESS: %s completed for %s in %dms", operation, slug, duration)

	if targetStory != nil {
		log.Printf("SUCCESS CONTEXT for %s:", slug)
		log.Printf("  Created/Updated Story ID: %d (UUID: %s)", targetStory.ID, targetStory.UUID)
		if targetStory.FolderID != nil {
			log.Printf("  Parent ID: %d", *targetStory.FolderID)
		}
		log.Printf("  Published: %t", targetStory.Published)
	}
}

// logExtendedErrorContext extracts and logs additional context from errors
func logExtendedErrorContext(err error) {
	if err == nil {
		return
	}

	errStr := err.Error()

	// Check for common API error patterns and log additional context
	if strings.Contains(errStr, "status") {
		log.Printf("  HTTP Error Details: %s", errStr)
	}

	if strings.Contains(errStr, "timeout") {
		log.Printf("  Timeout Error - this may indicate network issues or server overload")
	}

	if strings.Contains(errStr, "401") || strings.Contains(errStr, "403") {
		log.Printf("  Authentication/Authorization Error - check token permissions")
	}

	if strings.Contains(errStr, "404") {
		log.Printf("  Resource Not Found - story or space may not exist")
	}

	if strings.Contains(errStr, "429") {
		log.Printf("  Rate Limited - will retry with backoff")
	}

	if strings.Contains(errStr, "500") || strings.Contains(errStr, "502") || strings.Contains(errStr, "503") {
		log.Printf("  Server Error - this may be temporary, will retry")
	}
}

// optimizePreflight deduplicates entries, sorts by sync order (folders first), and merges full folder selections into starts_with tasks.
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

	// Create optimized list
	optimized := make([]PreflightItem, 0, len(m.preflight.items))
	for _, it := range m.preflight.items {
		if it.Skip {
			continue
		}
		it.Run = RunPending
		optimized = append(optimized, it)
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
	log.Printf("Optimized to %d items, sync order: folders first, then stories", len(optimized))
}

func (m *Model) runNextItem() tea.Cmd {
	if m.syncIndex >= len(m.preflight.items) {
		return nil
	}
	idx := m.syncIndex
	m.preflight.items[idx].Run = RunRunning
	return func() tea.Msg {
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
			if result.warning != "" {
				logWarning(result.operation, it.Story.FullSlug, result.warning, &it.Story)
			} else {
				logSuccess(result.operation, it.Story.FullSlug, duration, result.targetStory)
			}
			time.Sleep(50 * time.Millisecond)
		} else {
			log.Printf("Sync completed for %s (no detailed result)", it.Story.FullSlug)
			time.Sleep(50 * time.Millisecond)
		}

		return syncResultMsg{index: idx, err: err, result: result, duration: duration}
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
	logExtendedErrorContext(lastErr)

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

func ensureFolderPathImpl(api folderAPI, report *Report, sourceStories []sb.Story, srcSpaceID, tgtSpaceID int, slug string, publish bool) ([]sb.Story, error) {
	parts := strings.Split(slug, "/")
	if len(parts) <= 1 {
		return nil, nil
	}

	var created []sb.Story
	var parentID *int

	for i := 0; i < len(parts)-1; i++ {
		path := strings.Join(parts[:i+1], "/")

		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		existing, err := api.GetStoriesBySlug(ctx, tgtSpaceID, path)
		cancel()
		if err != nil {
			return created, err
		}
		if len(existing) > 0 {
			id := existing[0].ID
			parentID = &id
			continue
		}

		var source *sb.Story
		for j := range sourceStories {
			if sourceStories[j].FullSlug == path {
				source = &sourceStories[j]
				break
			}
		}

		var folder sb.Story
		if source != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			full, err := api.GetStoryWithContent(ctx, srcSpaceID, source.ID)
			cancel()
			if err != nil {
				return created, err
			}
			folder = full
		} else {
			name := parts[i]
			folder = sb.Story{
				Name:     name,
				Slug:     name,
				FullSlug: path,
				IsFolder: true,
				Content:  map[string]interface{}{},
			}
		}

		folder.ID = 0
		folder.CreatedAt = ""
		folder.UpdatedAt = ""
		folder.FolderID = parentID
		ct := folder.ContentType
		folder.ContentType = ""
		if folder.Content == nil {
			folder.Content = map[string]interface{}{}
		}
		if _, ok := folder.Content["content_types"]; !ok && ct != "" {
			folder.Content["content_types"] = []string{ct}
			folder.Content["lock_subfolders_content_types"] = false
		}

		ctx, cancel = context.WithTimeout(context.Background(), 15*time.Second)
		createdFolder, err := api.CreateStoryWithPublish(ctx, tgtSpaceID, folder, publish)
		cancel()
		if err != nil {
			return created, err
		}

		if report != nil {
			report.AddSuccess(createdFolder.FullSlug, "create", 0, &createdFolder)
		}

		created = append(created, createdFolder)
		id := createdFolder.ID
		parentID = &id
	}

	return created, nil
}

func (m *Model) ensureFolderPath(slug string) ([]sb.Story, error) {
	return ensureFolderPathImpl(m.api, &m.report, m.storiesSource, m.sourceSpace.ID, m.targetSpace.ID, slug, m.shouldPublish())
}

func (m *Model) shouldPublish() bool {
	return true
}

// syncFolder handles folder synchronization with proper parent resolution
func (m *Model) syncFolder(sourceFolder sb.Story) error {
	log.Printf("Syncing folder: %s", sourceFolder.FullSlug)

	// Get full folder data from source
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	fullFolder, err := m.api.GetStoryWithContent(ctx, m.sourceSpace.ID, sourceFolder.ID)
	if err != nil {
		return err
	}
	ct := fullFolder.ContentType
	fullFolder.ContentType = ""
	if fullFolder.Content == nil {
		fullFolder.Content = map[string]interface{}{}
	}
	if _, ok := fullFolder.Content["content_types"]; !ok && ct != "" {
		fullFolder.Content["content_types"] = []string{ct}
		fullFolder.Content["lock_subfolders_content_types"] = false
	}

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
		updated, err := m.api.UpdateStory(ctx, m.targetSpace.ID, fullFolder, m.shouldPublish())
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

		created, err := m.api.CreateStoryWithPublish(ctx, m.targetSpace.ID, fullFolder, m.shouldPublish())
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

	// Get full folder data from source
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	fullFolder, err := m.api.GetStoryWithContent(ctx, m.sourceSpace.ID, sourceFolder.ID)
	if err != nil {
		log.Printf("Failed to get source folder content for %s (ID: %d): %v", sourceFolder.FullSlug, sourceFolder.ID, err)
		logExtendedErrorContext(err)
		return nil, err
	}
	ct := fullFolder.ContentType
	fullFolder.ContentType = ""
	if fullFolder.Content == nil {
		fullFolder.Content = map[string]interface{}{}
	}
	if _, ok := fullFolder.Content["content_types"]; !ok && ct != "" {
		fullFolder.Content["content_types"] = []string{ct}
		fullFolder.Content["lock_subfolders_content_types"] = false
	}

	// Check if folder already exists in target
	existing, err := m.api.GetStoriesBySlug(ctx, m.targetSpace.ID, sourceFolder.FullSlug)
	if err != nil {
		log.Printf("Failed to check if target folder exists for %s: %v", sourceFolder.FullSlug, err)
		logExtendedErrorContext(err)
		return nil, err
	}

	// Resolve parent folder ID
	var warning string
	if fullFolder.FolderID != nil {
		parentSlugStr := parentSlug(fullFolder.FullSlug)
		if parentSlugStr != "" {
			if targetParents, err := m.api.GetStoriesBySlug(ctx, m.targetSpace.ID, parentSlugStr); err == nil && len(targetParents) > 0 {
				fullFolder.FolderID = &targetParents[0].ID
			} else {
				fullFolder.FolderID = nil // Set to root if parent not found
				warning = fmt.Sprintf("Parent folder %s not found, creating in root", parentSlugStr)
			}
		}
	}

	// Handle translated slugs
	fullFolder = m.processTranslatedSlugs(fullFolder, existing)

	var targetStory *sb.Story
	var operation string

	if len(existing) > 0 {
		// Update existing folder
		existingFolder := existing[0]
		fullFolder.ID = existingFolder.ID
		updated, err := m.api.UpdateStory(ctx, m.targetSpace.ID, fullFolder, m.shouldPublish())
		if err != nil {
			log.Printf("Failed to update target folder %s (ID: %d): %v", fullFolder.FullSlug, fullFolder.ID, err)
			logExtendedErrorContext(err)
			return nil, err
		}

		// Update UUID if different
		if updated.UUID != fullFolder.UUID && fullFolder.UUID != "" {
			if err := m.api.UpdateStoryUUID(ctx, m.targetSpace.ID, updated.ID, fullFolder.UUID); err != nil {
				if warning == "" {
					warning = fmt.Sprintf("Failed to update UUID: %v", err)
				} else {
					warning += fmt.Sprintf("; Failed to update UUID: %v", err)
				}
				log.Printf("Warning: failed to update UUID for folder %s: %v", fullFolder.FullSlug, err)
			}
		}

		targetStory = &updated
		operation = "update"
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

		created, err := m.api.CreateStoryWithPublish(ctx, m.targetSpace.ID, fullFolder, m.shouldPublish())
		if err != nil {
			log.Printf("Failed to create target folder %s: %v", fullFolder.FullSlug, err)
			logExtendedErrorContext(err)
			return nil, err
		}

		// Update UUID if different
		if created.UUID != fullFolder.UUID && fullFolder.UUID != "" {
			if err := m.api.UpdateStoryUUID(ctx, m.targetSpace.ID, created.ID, fullFolder.UUID); err != nil {
				if warning == "" {
					warning = fmt.Sprintf("Failed to update UUID: %v", err)
				} else {
					warning += fmt.Sprintf("; Failed to update UUID: %v", err)
				}
				log.Printf("Warning: failed to update UUID for new folder %s: %v", fullFolder.FullSlug, err)
			}
		}

		targetStory = &created
		operation = "create"
		log.Printf("Created folder: %s", fullFolder.FullSlug)
	}

	return &syncItemResult{
		operation:   operation,
		targetStory: targetStory,
		warning:     warning,
	}, nil
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
func (m *Model) syncStoryContentDetailed(sourceStory sb.Story) (*syncItemResult, error) {
	log.Printf("Syncing story: %s", sourceStory.FullSlug)
	if _, err := m.ensureFolderPath(sourceStory.FullSlug); err != nil {
		log.Printf("Failed to ensure folder path for %s: %v", sourceStory.FullSlug, err)
		logExtendedErrorContext(err)
		return nil, err
	}

	// Get full story content from source
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	fullStory, err := m.api.GetStoryWithContent(ctx, m.sourceSpace.ID, sourceStory.ID)
	if err != nil {
		log.Printf("Failed to get source story content for %s (ID: %d): %v", sourceStory.FullSlug, sourceStory.ID, err)
		logExtendedErrorContext(err)
		return nil, err
	}

	// Check if story already exists in target
	existing, err := m.api.GetStoriesBySlug(ctx, m.targetSpace.ID, sourceStory.FullSlug)
	if err != nil {
		log.Printf("Failed to check if target story exists for %s: %v", sourceStory.FullSlug, err)
		logExtendedErrorContext(err)
		return nil, err
	}

	// Resolve parent folder ID if story is in a folder
	var warning string
	if fullStory.FolderID != nil {
		parentSlugStr := parentSlug(fullStory.FullSlug)
		if parentSlugStr != "" {
			if targetParents, err := m.api.GetStoriesBySlug(ctx, m.targetSpace.ID, parentSlugStr); err == nil && len(targetParents) > 0 {
				fullStory.FolderID = &targetParents[0].ID
			} else {
				log.Printf("Warning: parent folder %s not found for story %s", parentSlugStr, fullStory.FullSlug)
				warning = fmt.Sprintf("Parent folder %s not found, creating in root", parentSlugStr)
				fullStory.FolderID = nil // Set to root if parent not found
			}
		}
	}

	// Handle translated slugs
	fullStory = m.processTranslatedSlugs(fullStory, existing)

	var targetStory *sb.Story
	var operation string

	if len(existing) > 0 {
		// Update existing story
		existingStory := existing[0]
		fullStory.ID = existingStory.ID
		updated, err := updateStoryWithPublishRetry(ctx, m.api, m.targetSpace.ID, fullStory, m.shouldPublish())
		if err != nil {
			return nil, err
		}

		// Update UUID if different
		if updated.UUID != fullStory.UUID && fullStory.UUID != "" {
			if err := m.api.UpdateStoryUUID(ctx, m.targetSpace.ID, updated.ID, fullStory.UUID); err != nil {
				if warning == "" {
					warning = fmt.Sprintf("Failed to update UUID: %v", err)
				} else {
					warning += fmt.Sprintf("; Failed to update UUID: %v", err)
				}
				log.Printf("Warning: failed to update UUID for story %s: %v", fullStory.FullSlug, err)
			}
		}

		targetStory = &updated
		operation = "update"
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
			return nil, err
		}

		// Update UUID if different
		if created.UUID != fullStory.UUID && fullStory.UUID != "" {
			if err := m.api.UpdateStoryUUID(ctx, m.targetSpace.ID, created.ID, fullStory.UUID); err != nil {
				if warning == "" {
					warning = fmt.Sprintf("Failed to update UUID: %v", err)
				} else {
					warning += fmt.Sprintf("; Failed to update UUID: %v", err)
				}
				log.Printf("Warning: failed to update UUID for new story %s: %v", fullStory.FullSlug, err)
			}
		}

		targetStory = &created
		operation = "create"
		log.Printf("Created story: %s", fullStory.FullSlug)
	}

	return &syncItemResult{
		operation:   operation,
		targetStory: targetStory,
		warning:     warning,
	}, nil
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
			if result.operation == "create" {
				totalCreated++
			} else if result.operation == "update" {
				totalUpdated++
			}

			if result.warning != "" {
				warnings = append(warnings, fmt.Sprintf("%s: %s", st.FullSlug, result.warning))
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
		operation: operation,
		warning:   warning,
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
