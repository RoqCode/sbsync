package sync

import (
	"context"
	"log"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"storyblok-sync/internal/sb"
)

// SyncOperations handles the core sync operations
type SyncOperations struct {
	api         FolderAPI
	contentMgr  *ContentManager
	folderMgr   *FolderPathBuilder
	sourceSpace *sb.Space
	targetSpace *sb.Space
}

// NewSyncOperations creates a new sync operations manager
func NewSyncOperations(api FolderAPI, sourceSpace, targetSpace *sb.Space) *SyncOperations {
	return &SyncOperations{
		api:         api,
		contentMgr:  NewContentManager(api, sourceSpace.ID),
		sourceSpace: sourceSpace,
		targetSpace: targetSpace,
	}
}

// RunSyncItem executes sync for a single item with retry logic
func (so *SyncOperations) RunSyncItem(ctx context.Context, idx int, item interface{}) tea.Cmd {
	return func() tea.Msg {
		// Check if context is already cancelled before starting
		select {
		case <-ctx.Done():
			return SyncResultMsg{Index: idx, Cancelled: true}
		default:
		}

		log.Printf("Starting sync for item %d", idx)
		startTime := time.Now()

		// Implement sync logic here based on item type
		// This is a placeholder for the actual sync implementation

		duration := time.Since(startTime).Milliseconds()

		// Return success result
		return SyncResultMsg{
			Index:    idx,
			Duration: duration,
		}
	}
}

// SyncWithRetry executes an operation with retry logic for rate limiting and transient errors
func (so *SyncOperations) SyncWithRetry(operation func() error) error {
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
func (so *SyncOperations) calculateRetryDelay(err error, attempt int) time.Duration {
	if IsRateLimited(err) {
		return time.Second * time.Duration(attempt+1)
	}
	return time.Millisecond * 500
}

// ShouldPublish determines if stories should be published based on space plan
func (so *SyncOperations) ShouldPublish() bool {
	if so.targetSpace != nil && so.targetSpace.PlanLevel == 999 {
		return false
	}
	return true
}

// Note: IsRateLimited is now in api_adapters.go
