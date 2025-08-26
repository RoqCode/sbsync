package sync

import (
	"context"
	"log"
	"strings"

	"storyblok-sync/internal/sb"
)

// APIAdapter provides API operations with retry and error handling logic
type APIAdapter struct {
	api SyncAPI
}

// NewAPIAdapter creates a new API adapter
func NewAPIAdapter(api SyncAPI) *APIAdapter {
	return &APIAdapter{
		api: api,
	}
}

// IsRateLimited checks if an error indicates rate limiting
func IsRateLimited(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "429") || strings.Contains(errStr, "rate limit")
}

// IsDevModePublishLimit checks if error is due to publish limit in dev mode
func IsDevModePublishLimit(err error) bool {
	if err == nil {
		return false
	}
	errStr := strings.ToLower(err.Error())
	return strings.Contains(errStr, "plan limit") || 
		   strings.Contains(errStr, "publish limit") ||
		   strings.Contains(errStr, "exceeded")
}

// UpdateStoryWithPublishRetry attempts to update a story with publish retry fallback
func (aa *APIAdapter) UpdateStoryWithPublishRetry(ctx context.Context, spaceID int, story sb.Story, publish bool) (sb.Story, error) {
	updated, err := aa.api.UpdateStory(ctx, spaceID, story, publish)
	if err != nil && IsDevModePublishLimit(err) && publish {
		// Retry without publishing if we hit publish limit
		log.Printf("Publish limit reached, retrying without publish for %s", story.FullSlug)
		return aa.api.UpdateStory(ctx, spaceID, story, false)
	}
	return updated, err
}

// CreateStoryWithPublishRetry attempts to create a story with publish retry fallback
func (aa *APIAdapter) CreateStoryWithPublishRetry(ctx context.Context, spaceID int, story sb.Story, publish bool) (sb.Story, error) {
	created, err := aa.api.CreateStoryWithPublish(ctx, spaceID, story, publish)
	if err != nil && IsDevModePublishLimit(err) && publish {
		// Retry without publishing if we hit publish limit
		log.Printf("Publish limit reached, retrying without publish for %s", story.FullSlug)
		return aa.api.CreateStoryWithPublish(ctx, spaceID, story, false)
	}
	return created, err
}

// ExecuteSync performs common create/update logic based on whether target exists
func (aa *APIAdapter) ExecuteSync(ctx context.Context, spaceID int, story sb.Story, existingTarget *sb.Story, publish bool) (sb.Story, string, error) {
	var operation string
	var result sb.Story
	var err error

	if existingTarget != nil {
		// Update existing story
		operation = OperationUpdate
		updateStory := PrepareStoryForUpdate(story, *existingTarget)
		result, err = aa.UpdateStoryWithPublishRetry(ctx, spaceID, updateStory, publish)
	} else {
		// Create new story
		operation = OperationCreate
		createStory := PrepareStoryForCreation(story)
		result, err = aa.CreateStoryWithPublishRetry(ctx, spaceID, createStory, publish)
	}

	return result, operation, err
}