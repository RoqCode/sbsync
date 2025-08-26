package sync

import (
	"context"
	"log"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"storyblok-sync/internal/sb"
)

// SyncOrchestrator manages the execution of sync operations with Bubble Tea integration
type SyncOrchestrator struct {
	api           SyncAPI
	contentMgr    *ContentManager
	report        ReportInterface
	sourceSpace   *sb.Space
	targetSpace   *sb.Space
}

// SyncAPI defines the interface for sync API operations
type SyncAPI interface {
	GetStoriesBySlug(ctx context.Context, spaceID int, slug string) ([]sb.Story, error)
	GetStoryWithContent(ctx context.Context, spaceID, storyID int) (sb.Story, error)
	CreateStoryWithPublish(ctx context.Context, spaceID int, st sb.Story, publish bool) (sb.Story, error)
	UpdateStory(ctx context.Context, spaceID int, st sb.Story, publish bool) (sb.Story, error)
	UpdateStoryUUID(ctx context.Context, spaceID, storyID int, uuid string) error
}

// ReportInterface defines the interface for reporting sync progress
type ReportInterface interface {
	AddSuccess(slug, operation string, duration int64, story *sb.Story)
	AddWarning(slug, operation, warning string, duration int64, sourceStory, targetStory *sb.Story)
	AddError(slug, operation string, duration int64, sourceStory *sb.Story, err error)
}

// SyncItem represents an item to be synchronized
type SyncItem interface {
	GetStory() sb.Story
	IsStartsWith() bool
	IsFolder() bool
}

// NewSyncOrchestrator creates a new sync orchestrator
func NewSyncOrchestrator(api SyncAPI, report ReportInterface, sourceSpace, targetSpace *sb.Space) *SyncOrchestrator {
	return &SyncOrchestrator{
		api:         api,
		contentMgr:  NewContentManager(api, sourceSpace.ID),
		report:      report,
		sourceSpace: sourceSpace,
		targetSpace: targetSpace,
	}
}

// RunSyncItem executes sync for a single item and returns a Bubble Tea command
func (so *SyncOrchestrator) RunSyncItem(ctx context.Context, idx int, item SyncItem) tea.Cmd {
	return func() tea.Msg {
		// Check if context is already cancelled before starting
		select {
		case <-ctx.Done():
			return SyncResultMsg{Index: idx, Cancelled: true}
		default:
		}

		story := item.GetStory()
		log.Printf("Starting sync for item %d: %s (folder: %t)", idx, story.FullSlug, story.IsFolder)

		startTime := time.Now()
		var err error
		var result *SyncItemResult

		// Choose sync operation based on item type
		switch {
		case item.IsStartsWith():
			err = so.SyncWithRetry(func() error {
				var syncErr error
				result, syncErr = so.SyncStartsWithDetailed(story.FullSlug)
				return syncErr
			})
		case item.IsFolder():
			err = so.SyncWithRetry(func() error {
				var syncErr error
				result, syncErr = so.SyncFolderDetailed(story)
				return syncErr
			})
		default:
			err = so.SyncWithRetry(func() error {
				var syncErr error
				result, syncErr = so.SyncStoryDetailed(story)
				return syncErr
			})
		}

		duration := time.Since(startTime).Milliseconds()

		// Log results
		if err != nil {
			LogError("sync", story.FullSlug, err, &story)
		} else if result != nil {
			if result.Warning != "" {
				LogWarning(result.Operation, story.FullSlug, result.Warning, &story)
			} else {
				LogSuccess(result.Operation, story.FullSlug, duration, result.TargetStory)
			}
			time.Sleep(50 * time.Millisecond) // Brief pause between operations
		} else {
			log.Printf("Sync completed for %s (no detailed result)", story.FullSlug)
			time.Sleep(50 * time.Millisecond)
		}

		return SyncResultMsg{Index: idx, Err: err, Result: result, Duration: duration}
	}
}

// SyncWithRetry executes an operation with retry logic for rate limiting and transient errors
func (so *SyncOrchestrator) SyncWithRetry(operation func() error) error {
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
			retryDelay := so.calculateRetryDelay(err, attempt)
			log.Printf("  Will retry in %v (attempt %d/3)", retryDelay, attempt+2)
			time.Sleep(retryDelay)
		} else {
			log.Printf("  Max retries (3) exceeded, giving up")
		}

		// Check if it's a rate limiting error
		if IsRateLimited(err) {
			sleepDuration := time.Second * time.Duration(attempt+1)
			log.Printf("Rate limited, sleeping for %v", sleepDuration)
			time.Sleep(sleepDuration)
			continue
		}

		// For other errors, use shorter delay
		if attempt < 2 {
			time.Sleep(500 * time.Millisecond)
		}
	}
	return lastErr
}

// calculateRetryDelay calculates the delay before retrying based on error type
func (so *SyncOrchestrator) calculateRetryDelay(err error, attempt int) time.Duration {
	if IsRateLimited(err) {
		return time.Second * time.Duration(attempt+1)
	}
	return time.Millisecond * 500
}

// ShouldPublish determines if stories should be published based on space plan
func (so *SyncOrchestrator) ShouldPublish() bool {
	if so.targetSpace != nil && so.targetSpace.PlanLevel == 999 {
		return false
	}
	return true
}

// SyncStartsWithDetailed synchronizes all stories starting with a prefix
func (so *SyncOrchestrator) SyncStartsWithDetailed(prefix string) (*SyncItemResult, error) {
	// TODO: Implement bulk starts-with synchronization
	// This will need to find all stories with the prefix and sync them
	return &SyncItemResult{
		Operation:   OperationSkip,
		TargetStory: nil,
		Warning:     "starts-with sync not yet implemented",
	}, nil
}

// SyncFolderDetailed synchronizes a folder using StorySyncer
func (so *SyncOrchestrator) SyncFolderDetailed(story sb.Story) (*SyncItemResult, error) {
	syncer := NewStorySyncer(so.api, so.sourceSpace.ID, so.targetSpace.ID)
	return syncer.SyncFolderDetailed(story, so.ShouldPublish())
}

// SyncStoryDetailed synchronizes a story using StorySyncer
func (so *SyncOrchestrator) SyncStoryDetailed(story sb.Story) (*SyncItemResult, error) {
	syncer := NewStorySyncer(so.api, so.sourceSpace.ID, so.targetSpace.ID)
	return syncer.SyncStoryDetailed(story, so.ShouldPublish())
}