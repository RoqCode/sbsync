package ui

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	sync "storyblok-sync/internal/core/sync"
	"storyblok-sync/internal/sb"
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
func prepareStoryForCreation(story sb.Story) sb.Story { return sync.PrepareStoryForCreation(story) }

// removed legacy wrappers (moved to core sync)

// Legacy type aliases for backward compatibility
type syncItemResult = sync.SyncItemResult
type syncResultMsg = sync.SyncResultMsg

// removed unused type alias syncCancelledMsg

// optimizePreflight deduplicates entries, pre-plans missing folders, and sorts by sync order (folders first).
func (m *Model) optimizePreflight() {
	planner := sync.NewPreflightPlanner(m.storiesSource, m.storiesTarget)
	m.preflight.items = planner.OptimizePreflight(m.preflight.items)
}

func (m *Model) runNextItem() tea.Cmd {
	// Find next pending item, preferring current syncIndex, then scanning forward, then wrap-around
	if len(m.preflight.items) == 0 {
		return nil
	}
	// Do not schedule new work while paused (after Ctrl+C)
	if m.paused {
		return nil
	}
	// Phase barrier: if any folder is not yet done (pending or running),
	// do not start stories. Folders must complete fully before stories run.
	hasActiveFolders := false
	for i := range m.preflight.items {
		if m.preflight.items[i].Story.IsFolder && m.preflight.items[i].Run != RunDone {
			hasActiveFolders = true
			break
		}
	}
	idx := -1
	// First pass: from current index to end
	start := m.syncIndex
	if start < 0 {
		start = 0
	}
	for i := start; i < len(m.preflight.items); i++ {
		if m.preflight.items[i].Run == RunPending {
			if hasActiveFolders && !m.preflight.items[i].Story.IsFolder {
				continue // defer stories until all folders are handled
			}
			idx = i
			break
		}
	}
	// Second pass: from 0 to current index
	if idx == -1 {
		for i := 0; i < start; i++ {
			if m.preflight.items[i].Run == RunPending {
				if hasActiveFolders && !m.preflight.items[i].Story.IsFolder {
					continue // still in folder phase
				}
				idx = i
				break
			}
		}
	}
	if idx == -1 {
		// Either no pending items or only stories pending while folders still running.
		// In both cases, do not schedule anything new.
		return nil
	}
	// Budgeted scheduling: limit concurrent write units to maxWorkers
	runningUnits := 0
	for i := range m.preflight.items {
		if m.preflight.items[i].Run == RunRunning {
			runningUnits += m.expectedWriteUnits(m.preflight.items[i])
		}
	}
	candUnits := m.expectedWriteUnits(m.preflight.items[idx])
	allowed := m.maxWorkers
	if allowed <= 0 {
		allowed = 1
	}
	if runningUnits+candUnits > allowed {
		return nil
	}
	m.syncIndex = idx
	m.preflight.items[idx].Run = RunRunning

	// Capture metrics snapshot to compute per-item retry deltas later
	if m.api != nil {
		m.syncStartMetrics[idx] = m.api.MetricsSnapshot()
	}

	// Lazily create orchestrator inside the returned command to avoid panics
	// when spaces/API are not initialized in tests.
	// Build a lightweight adapter to capture current index and item.
	// Build adapter and compute publish override for stories
	it := m.preflight.items[idx]
	item := &preflightItemAdapter{item: it}
	if !it.Story.IsFolder {
		mode := m.getPublishMode(it.Story.FullSlug)
		exists, tgtPublished := false, false
		for _, t := range m.storiesTarget {
			if t.FullSlug == it.Story.FullSlug {
				exists = true
				tgtPublished = t.Published
				break
			}
		}
		publishFlag := false
		switch mode {
		case PublishModePublish:
			publishFlag = true
		case PublishModePublishChanges:
			publishFlag = false
		default:
			publishFlag = false
		}
		if mode == PublishModeDraft && it.Story.Published && exists && tgtPublished {
			publishFlag = true
			if m.unpublishAfter == nil {
				m.unpublishAfter = make(map[string]bool)
			}
			m.unpublishAfter[it.Story.FullSlug] = true
			log.Printf("UNPUBLISH_MARK: slug=%s op=update will schedule unpublish after overwrite (mode=draft, srcPub=true, tgtPub=true)", it.Story.FullSlug)
		}
		log.Printf("PUBLISH_OVERRIDE: slug=%s mode=%s exists=%t tgtPublished=%t publishFlag=%t", it.Story.FullSlug, mode, exists, tgtPublished, publishFlag)
		item.overridePublish = true
		item.publishFlag = publishFlag
	}

	return func() tea.Msg {
		// If essential dependencies are missing, fail fast with a result message.
		if m.api == nil || m.sourceSpace == nil || m.targetSpace == nil {
			return syncResultMsg{Index: idx, Err: fmt.Errorf("sync prerequisites not initialized")}
		}

		// Rebuild report adapter and target index at execution time.
		reportAdapter := &reportAdapter{report: &m.report}
		tgtIndex := make(map[string]sb.Story, len(m.storiesTarget))
		for _, s := range m.storiesTarget {
			tgtIndex[s.FullSlug] = s
		}
		orchestrator := sync.NewSyncOrchestrator(m.api, reportAdapter, m.sourceSpace, m.targetSpace, tgtIndex)
		// Delegate to orchestrator command
		cmd := orchestrator.RunSyncItem(m.syncContext, idx, item)
		return cmd()
	}
}

// preflightItemAdapter adapts PreflightItem to sync.SyncItem interface
type preflightItemAdapter struct {
	item            PreflightItem
	overridePublish bool
	publishFlag     bool
}

func (pia *preflightItemAdapter) GetStory() sb.Story {
	st := pia.item.Story
	if pia.overridePublish && !st.IsFolder {
		st.Published = pia.publishFlag
	}
	return st
}

func (pia *preflightItemAdapter) IsFolder() bool {
	return pia.item.Story.IsFolder
}

// Legacy wrapper functions for backward compatibility
// removed legacy wrappers and interfaces

type folderAPI interface {
	GetStoriesBySlug(ctx context.Context, spaceID int, slug string) ([]sb.Story, error)
	GetStoryWithContent(ctx context.Context, spaceID, storyID int) (sb.Story, error)
	CreateStoryRawWithPublish(ctx context.Context, spaceID int, story map[string]interface{}, publish bool) (sb.Story, error)
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

	log.Printf("DEBUG: Prepared source folder %s with content: %t", path, len(folder.Content) > 0)
	return folder, nil
}

// createFolder creates a single folder in the target space
func (fpb *folderPathBuilder) createFolder(ctx context.Context, folder sb.Story) (sb.Story, error) {
	log.Printf("DEBUG: Creating folder: %s", folder.FullSlug)
	// Convert typed folder to raw minimal for creation
	raw := map[string]interface{}{
		"uuid":      folder.UUID,
		"name":      folder.Name,
		"slug":      folder.Slug,
		"full_slug": folder.FullSlug,
		"content":   sync.ToRawMap(folder.Content),
		"is_folder": true,
	}
	if folder.FolderID != nil {
		raw["parent_id"] = *folder.FolderID
	} else {
		raw["parent_id"] = 0
	}
	created, err := fpb.api.CreateStoryRawWithPublish(ctx, fpb.tgtSpaceID, raw, false)
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

func (m *Model) shouldPublish() bool {
	if m.targetSpace != nil && m.targetSpace.PlanLevel == 999 {
		return false
	}
	return true
}

// syncFolder handles folder synchronization with proper parent resolution
// removed legacy syncFolder implementation

// syncFolderDetailed handles folder synchronization and returns detailed results
// Note: legacy syncFolderDetailed removed; use orchestrator and sync module instead.

// executeSync has been moved to sync/api_adapters.go as ExecuteSync

// itemType returns a string describing the item type for logging
// removed legacy itemType

// syncStoryContent handles story synchronization with proper UUID management
// removed legacy syncStoryContent

// syncStoryContentDetailed handles story synchronization and returns detailed results
// Note: Folder structure is now pre-planned in optimizePreflight(), so no need to ensure folder path here
// Note: legacy syncStoryContentDetailed removed; use orchestrator and sync module instead.

// processTranslatedSlugs handles translated slug processing like the Storyblok CLI
func (m *Model) processTranslatedSlugs(sourceStory sb.Story, existingStories []sb.Story) sb.Story {
	return sync.ProcessTranslatedSlugs(sourceStory, existingStories)
}

// Starts-with bulk sync removed; prefix is a filter only.

// Legacy helper functions removed as bulk operations module was deleted.

// Legacy wrapper for parent slug extraction
func parentSlug(full string) string { return sync.ParentSlug(full) }

// Adapter functions to convert between UI and sync module types

// reportAdapter adapts UI Report to sync ReportInterface
type reportAdapter struct {
	report *Report
}

func (ra *reportAdapter) AddSuccess(slug, operation string, duration int64, story *sb.Story) {
	ra.report.AddSuccess(slug, operation, duration, story)
}

func (ra *reportAdapter) AddWarning(slug, operation, warning string, duration int64, sourceStory, targetStory *sb.Story) {
	ra.report.AddWarning(slug, operation, warning, duration, sourceStory, targetStory)
}

func (ra *reportAdapter) AddError(slug, operation string, duration int64, sourceStory *sb.Story, err error) {
	ra.report.AddError(slug, operation, err.Error(), duration, sourceStory)
}

// Removed type conversion helpers: UI now uses core PreflightItem directly
