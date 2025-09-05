package sync

import (
	"context"
	"encoding/json"
	"log"
	"strings"
	"time"

	"storyblok-sync/internal/sb"
)

// StorySyncer handles story and folder synchronization operations
type StorySyncer struct {
	api            SyncAPI
	contentMgr     *ContentManager
	sourceSpaceID  int
	targetSpaceID  int
	existingBySlug map[string]sb.Story
	limiter        *SpaceLimiter
}

// storyRawAPI captures optional raw story methods available on the API client
type storyRawAPI interface {
	GetStoryRaw(ctx context.Context, spaceID, storyID int) (map[string]interface{}, error)
	CreateStoryRawWithPublish(ctx context.Context, spaceID int, story map[string]interface{}, publish bool) (sb.Story, error)
	UpdateStoryRawWithPublish(ctx context.Context, spaceID int, storyID int, story map[string]interface{}, publish bool) (sb.Story, error)
}

// NewStorySyncer creates a new story synchronizer
func NewStorySyncer(api SyncAPI, sourceSpaceID, targetSpaceID int, existing map[string]sb.Story) *StorySyncer {
	return &StorySyncer{
		api:            api,
		contentMgr:     NewContentManager(api, sourceSpaceID),
		sourceSpaceID:  sourceSpaceID,
		targetSpaceID:  targetSpaceID,
		existingBySlug: existing,
		limiter:        NewSpaceLimiter(7, 7, 7),
	}
}

// Content manager is internal; on-demand MA reads ensure correctness.

// SyncStory synchronizes a single story
func (ss *StorySyncer) SyncStory(ctx context.Context, story sb.Story, shouldPublish bool) (sb.Story, error) {
	log.Printf("Syncing story: %s", story.FullSlug)
	fullStory := story

	// Determine existence from in-memory index
	var existingTarget *sb.Story
	if s, ok := ss.existingBySlug[story.FullSlug]; ok {
		existingTarget = &s
	}
	// Fallback: if not in index, query target API to detect existing story
	if existingTarget == nil {
		_ = ss.limiter.WaitRead(ctx, ss.targetSpaceID)
		if existing, err := ss.api.GetStoriesBySlug(ctx, ss.targetSpaceID, story.FullSlug); err == nil && len(existing) > 0 {
			// use the first match; slugs are unique per space
			st := existing[0]
			existingTarget = &st
			ss.limiter.NudgeRead(ss.targetSpaceID, +0.02, 1, 7)
		} else if IsRateLimited(err) {
			ss.limiter.NudgeRead(ss.targetSpaceID, -0.2, 1, 7)
		}
	}

	// Resolve parent folder ID using in-memory target index only to avoid extra API calls.
	// The UI updates its in-memory target index (m.storiesTarget) as folders are created,
	// so by the time stories are synced, their parents should be present here.
	fullStory = ss.resolveParentFolderFromIndex(fullStory)

	// Handle translated slugs
	var existingSlice []sb.Story
	if existingTarget != nil {
		existingSlice = []sb.Story{*existingTarget}
	}
	fullStory = ProcessTranslatedSlugs(fullStory, existingSlice)

	if existingTarget != nil {
		// Update existing story
		existingStory := *existingTarget

		// Prefer raw update if available to preserve unknown fields
		if rawAPI, ok := any(ss.api).(storyRawAPI); ok {
			// Fetch raw source payload
			_ = ss.limiter.WaitRead(ctx, ss.sourceSpaceID)
			raw, err := rawAPI.GetStoryRaw(ctx, ss.sourceSpaceID, story.ID)
			if err != nil {
				return sb.Story{}, err
			}
			ss.limiter.NudgeRead(ss.sourceSpaceID, +0.02, 1, 7)

			// Strip read-only fields and ensure correct parent_id
			delete(raw, "id")
			delete(raw, "created_at")
			delete(raw, "updated_at")
			if fullStory.FolderID != nil {
				raw["parent_id"] = *fullStory.FolderID
			} else {
				raw["parent_id"] = 0
			}

			// Translate translated_slugs -> translated_slugs_attributes without IDs
			if ts, ok := raw["translated_slugs"].([]interface{}); ok && len(ts) > 0 {
				attrs := make([]map[string]interface{}, 0, len(ts))
				for _, item := range ts {
					if m, ok := item.(map[string]interface{}); ok {
						delete(m, "id")
						attrs = append(attrs, m)
					}
				}
				raw["translated_slugs_attributes"] = attrs
				delete(raw, "translated_slugs")
			}

			// DEBUG: omit raw payload dump to keep logs readable
			log.Printf("DEBUG: PUSH_RAW_UPDATE story %s (payload omitted)", story.FullSlug)

			_ = ss.limiter.WaitWrite(ctx, ss.targetSpaceID)
			updated, err := ss.updateWithFolderFallback(ctx, rawAPI, existingStory.ID, raw, shouldPublish, ParentSlug(story.FullSlug))
			if err != nil {
				if IsRateLimited(err) {
					ss.limiter.NudgeWrite(ss.targetSpaceID, -0.2, 1, 7)
				}
				return sb.Story{}, err
			}
			ss.limiter.NudgeWrite(ss.targetSpaceID, +0.02, 1, 7)

			// Update UUID if different
			if updated.UUID != fullStory.UUID && fullStory.UUID != "" {
				_ = ss.limiter.WaitWrite(ctx, ss.targetSpaceID)
				if err := ss.api.UpdateStoryUUID(ctx, ss.targetSpaceID, updated.ID, fullStory.UUID); err != nil {
					log.Printf("Warning: failed to update UUID for story %s: %v", fullStory.FullSlug, err)
					if IsRateLimited(err) {
						ss.limiter.NudgeWrite(ss.targetSpaceID, -0.2, 1, 7)
					}
				}
			}

			log.Printf("Updated story: %s", fullStory.FullSlug)
			return updated, nil
		}

		// Fallback to typed update
		updateStory := PrepareStoryForUpdate(fullStory, existingStory)
		// DEBUG: omit typed payload dump to keep logs readable
		log.Printf("DEBUG: PUSH_TYPED_UPDATE story %s (payload omitted)", story.FullSlug)
		_ = ss.limiter.WaitWrite(ctx, ss.targetSpaceID)
		updated, err := ss.api.UpdateStoryRawWithPublish(ctx, ss.targetSpaceID, existingStory.ID, map[string]interface{}{"uuid": updateStory.UUID, "name": updateStory.Name, "slug": updateStory.Slug, "full_slug": updateStory.FullSlug, "content": toMap(updateStory.Content), "is_folder": updateStory.IsFolder, "parent_id": valueOrZero(updateStory.FolderID)}, shouldPublish)
		if err != nil {
			if IsRateLimited(err) {
				ss.limiter.NudgeWrite(ss.targetSpaceID, -0.2, 1, 7)
			}
			return sb.Story{}, err
		}
		ss.limiter.NudgeWrite(ss.targetSpaceID, +0.02, 1, 7)

		// Update UUID if different
		if updated.UUID != fullStory.UUID && fullStory.UUID != "" {
			_ = ss.limiter.WaitWrite(ctx, ss.targetSpaceID)
			if err := ss.api.UpdateStoryUUID(ctx, ss.targetSpaceID, updated.ID, fullStory.UUID); err != nil {
				log.Printf("Warning: failed to update UUID for story %s: %v", fullStory.FullSlug, err)
				if IsRateLimited(err) {
					ss.limiter.NudgeWrite(ss.targetSpaceID, -0.2, 1, 7)
				}
			}
		}

		// Update UUID if different after update
		if updated.UUID != fullStory.UUID && fullStory.UUID != "" {
			if err := ss.api.UpdateStoryUUID(ctx, ss.targetSpaceID, updated.ID, fullStory.UUID); err != nil {
				log.Printf("Warning: failed to update UUID for story %s: %v", fullStory.FullSlug, err)
			}
		}

		log.Printf("Updated story: %s", fullStory.FullSlug)
		return updated, nil
	} else {
		// Create new story
		// Prefer raw create if available to preserve unknown fields
		if rawAPI, ok := any(ss.api).(storyRawAPI); ok {
			// Fetch raw source payload
			_ = ss.limiter.WaitRead(ctx, ss.sourceSpaceID)
			raw, err := rawAPI.GetStoryRaw(ctx, ss.sourceSpaceID, story.ID)
			if err != nil {
				return sb.Story{}, err
			}
			ss.limiter.NudgeRead(ss.sourceSpaceID, +0.02, 1, 7)

			// Strip read-only fields
			delete(raw, "id")
			delete(raw, "created_at")
			delete(raw, "updated_at")
			// For copy-as-new or slug changes, enforce slug/full_slug from typed story
			if fullStory.Slug != "" {
				raw["slug"] = fullStory.Slug
			}
			if fullStory.FullSlug != "" {
				raw["full_slug"] = fullStory.FullSlug
			}
			// Ensure name matches the (possibly suffixed) typed story name
			if fullStory.Name != "" {
				raw["name"] = fullStory.Name
			}
			// Avoid UUID collisions when creating copies
			delete(raw, "uuid")

			// Set parent_id from resolved target parent
			if fullStory.FolderID != nil {
				raw["parent_id"] = *fullStory.FolderID
			} else {
				raw["parent_id"] = 0
			}

			// Translate translated_slugs -> translated_slugs_attributes without IDs
			if ts, ok := raw["translated_slugs"].([]interface{}); ok && len(ts) > 0 {
				attrs := make([]map[string]interface{}, 0, len(ts))
				for _, item := range ts {
					if m, ok := item.(map[string]interface{}); ok {
						delete(m, "id")
						// If path present, replace last segment with fullStory.Slug
						if p, ok2 := m["path"].(string); ok2 && p != "" && fullStory.Slug != "" {
							parts := strings.Split(p, "/")
							if len(parts) > 0 {
								parts[len(parts)-1] = fullStory.Slug
								m["path"] = strings.Join(parts, "/")
							}
						}
						attrs = append(attrs, m)
					}
				}
				raw["translated_slugs_attributes"] = attrs
				delete(raw, "translated_slugs")
			}

			// DEBUG: omit raw create payload dump
			log.Printf("DEBUG: PUSH_RAW_CREATE story %s (payload omitted)", story.FullSlug)

			_ = ss.limiter.WaitWrite(ctx, ss.targetSpaceID)
			created, err := ss.createWithFolderFallback(ctx, rawAPI, raw, shouldPublish, ParentSlug(story.FullSlug))
			if err != nil {
				if IsRateLimited(err) {
					ss.limiter.NudgeWrite(ss.targetSpaceID, -0.2, 1, 7)
				}
				return sb.Story{}, err
			}
			ss.limiter.NudgeWrite(ss.targetSpaceID, +0.02, 1, 7)

			// Update UUID if different after create
			if created.UUID != fullStory.UUID && fullStory.UUID != "" {
				_ = ss.limiter.WaitWrite(ctx, ss.targetSpaceID)
				if err := ss.api.UpdateStoryUUID(ctx, ss.targetSpaceID, created.ID, fullStory.UUID); err != nil {
					log.Printf("Warning: failed to update UUID for new story %s: %v", fullStory.FullSlug, err)
					if IsRateLimited(err) {
						ss.limiter.NudgeWrite(ss.targetSpaceID, -0.2, 1, 7)
					}
				}
			}

			log.Printf("Created story: %s", fullStory.FullSlug)
			return created, nil
		}

		// Fallback to typed create
		createStory := PrepareStoryForCreation(fullStory)

		// DEBUG: omit typed create payload dump
		log.Printf("DEBUG: PUSH_TYPED_CREATE story %s (payload omitted)", story.FullSlug)

		_ = ss.limiter.WaitWrite(ctx, ss.targetSpaceID)
		created, err := ss.api.CreateStoryRawWithPublish(ctx, ss.targetSpaceID, map[string]interface{}{"uuid": createStory.UUID, "name": createStory.Name, "slug": createStory.Slug, "full_slug": createStory.FullSlug, "content": toMap(createStory.Content), "is_folder": createStory.IsFolder, "parent_id": valueOrZero(createStory.FolderID)}, shouldPublish)
		if err != nil {
			if IsRateLimited(err) {
				ss.limiter.NudgeWrite(ss.targetSpaceID, -0.2, 1, 7)
			}
			return sb.Story{}, err
		}
		ss.limiter.NudgeWrite(ss.targetSpaceID, +0.02, 1, 7)

		// Update UUID if different after create
		if created.UUID != fullStory.UUID && fullStory.UUID != "" {
			_ = ss.limiter.WaitWrite(ctx, ss.targetSpaceID)
			if err := ss.api.UpdateStoryUUID(ctx, ss.targetSpaceID, created.ID, fullStory.UUID); err != nil {
				log.Printf("Warning: failed to update UUID for new story %s: %v", fullStory.FullSlug, err)
				if IsRateLimited(err) {
					ss.limiter.NudgeWrite(ss.targetSpaceID, -0.2, 1, 7)
				}
			}
		}

		log.Printf("Created story: %s", fullStory.FullSlug)
		return created, nil
	}
}

// resolveParentFolderFromIndex resolves and sets the correct parent folder ID using the in-memory target index
func (ss *StorySyncer) resolveParentFolderFromIndex(story sb.Story) sb.Story {
	parent := ParentSlug(story.FullSlug)
	if parent == "" {
		story.FolderID = nil
		return story
	}
	if p, ok := ss.existingBySlug[parent]; ok {
		story.FolderID = &p.ID
	} else {
		story.FolderID = nil
	}
	return story
}

func isUnprocessable(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	return strings.Contains(s, "422") || strings.Contains(strings.ToLower(s), "unprocessable")
}

func (ss *StorySyncer) createWithFolderFallback(ctx context.Context, rawAPI storyRawAPI, raw map[string]interface{}, publish bool, parentSlug string) (sb.Story, error) {
	created, err := rawAPI.CreateStoryRawWithPublish(ctx, ss.targetSpaceID, raw, publish)
	if err == nil {
		return created, nil
	}
	if !isUnprocessable(err) {
		return sb.Story{}, err
	}
	// ensure parent folders and retry
	_, _ = EnsureFolderPathStatic(ss.api, nil, nil, ss.sourceSpaceID, ss.targetSpaceID, parentSlug, false)
	// best effort: fetch parent id and set into raw
	if parentSlug != "" {
		if parents, e := ss.api.GetStoriesBySlug(ctx, ss.targetSpaceID, parentSlug); e == nil && len(parents) > 0 {
			raw["parent_id"] = parents[0].ID
		}
	}
	return rawAPI.CreateStoryRawWithPublish(ctx, ss.targetSpaceID, raw, publish)
}

func (ss *StorySyncer) updateWithFolderFallback(ctx context.Context, rawAPI storyRawAPI, storyID int, raw map[string]interface{}, publish bool, parentSlug string) (sb.Story, error) {
	updated, err := rawAPI.UpdateStoryRawWithPublish(ctx, ss.targetSpaceID, storyID, raw, publish)
	if err == nil {
		return updated, nil
	}
	if !isUnprocessable(err) {
		return sb.Story{}, err
	}
	_, _ = EnsureFolderPathStatic(ss.api, nil, nil, ss.sourceSpaceID, ss.targetSpaceID, parentSlug, false)
	if parentSlug != "" {
		if parents, e := ss.api.GetStoriesBySlug(ctx, ss.targetSpaceID, parentSlug); e == nil && len(parents) > 0 {
			raw["parent_id"] = parents[0].ID
		}
	}
	return rawAPI.UpdateStoryRawWithPublish(ctx, ss.targetSpaceID, storyID, raw, publish)
}

// toMap converts a JSON blob to map[string]interface{} for raw payloads
func toMap(raw json.RawMessage) map[string]interface{} {
	if len(raw) == 0 {
		return map[string]interface{}{}
	}
	var m map[string]interface{}
	_ = json.Unmarshal(raw, &m)
	return m
}

// valueOrZero returns the dereferenced int or 0 if nil
func valueOrZero(p *int) int {
	if p == nil {
		return 0
	}
	return *p
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
		folder.FullSlug, len(fullFolder.Content) > 0, folder.IsFolder)

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

		updated, err := ss.api.UpdateStoryRawWithPublish(ctx, ss.targetSpaceID, existingFolder.ID, map[string]interface{}{"uuid": updateFolder.UUID, "name": updateFolder.Name, "slug": updateFolder.Slug, "full_slug": updateFolder.FullSlug, "content": toMap(updateFolder.Content), "is_folder": true, "parent_id": valueOrZero(updateFolder.FolderID)}, shouldPublish)
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
		// Prefer raw create if available to preserve unknown folder fields
		if rawAPI, ok := any(ss.api).(storyRawAPI); ok {
			// Fetch raw source payload
			raw, err := rawAPI.GetStoryRaw(ctx, ss.sourceSpaceID, folder.ID)
			if err != nil {
				return sb.Story{}, err
			}

			// Strip read-only fields
			delete(raw, "id")
			delete(raw, "created_at")
			delete(raw, "updated_at")
			// For folder forks, enforce slug/full_slug/name from typed story and avoid UUID collisions
			if fullFolder.Slug != "" {
				raw["slug"] = fullFolder.Slug
			}
			if fullFolder.FullSlug != "" {
				raw["full_slug"] = fullFolder.FullSlug
			}
			if fullFolder.Name != "" {
				raw["name"] = fullFolder.Name
			}
			delete(raw, "uuid")

			// Ensure is_folder true
			raw["is_folder"] = true

			// Set parent_id from resolved target parent (already computed in fullFolder)
			if fullFolder.FolderID != nil {
				raw["parent_id"] = *fullFolder.FolderID
			} else {
				raw["parent_id"] = 0
			}

			// Translate translated_slugs -> translated_slugs_attributes without IDs
			if ts, ok := raw["translated_slugs"].([]interface{}); ok && len(ts) > 0 {
				attrs := make([]map[string]interface{}, 0, len(ts))
				for _, item := range ts {
					if m, ok := item.(map[string]interface{}); ok {
						delete(m, "id")
						// Ensure last path segment matches the new slug if present
						if p, ok2 := m["path"].(string); ok2 && p != "" && fullFolder.Slug != "" {
							parts := strings.Split(p, "/")
							if len(parts) > 0 {
								parts[len(parts)-1] = fullFolder.Slug
								m["path"] = strings.Join(parts, "/")
							}
						}
						attrs = append(attrs, m)
					}
				}
				raw["translated_slugs_attributes"] = attrs
				delete(raw, "translated_slugs")
			}

			// DEBUG: log outgoing raw create payload for folder
			if b, err := json.MarshalIndent(raw, "", "  "); err == nil {
				log.Printf("DEBUG: PUSH_RAW_CREATE folder %s:\n%s", fullFolder.FullSlug, string(b))
			}

			created, err := rawAPI.CreateStoryRawWithPublish(ctx, ss.targetSpaceID, raw, false)
			if err != nil {
				return sb.Story{}, err
			}

			log.Printf("Created folder: %s", fullFolder.FullSlug)
			return created, nil
		}

		// Fallback to typed folder create
		createFolder := PrepareStoryForCreation(fullFolder)
		if createFolder.IsFolder && len(createFolder.Content) == 0 {
			createFolder.Content = json.RawMessage([]byte(`{}`))
		}
		created, err := ss.api.CreateStoryRawWithPublish(ctx, ss.targetSpaceID, map[string]interface{}{"uuid": createFolder.UUID, "name": createFolder.Name, "slug": createFolder.Slug, "full_slug": createFolder.FullSlug, "content": toMap(createFolder.Content), "is_folder": true, "parent_id": valueOrZero(createFolder.FolderID)}, shouldPublish)
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
	// Attach per-item retry counters to context so transport can attribute retries
	rc := &sb.RetryCounters{}
	ctx = sb.WithRetryCounters(ctx, rc)

	// Determine operation type from in-memory index only (avoid extra GET);
	// fall back to SyncStory internal checks for correctness.
	operation := OperationCreate
	if _, ok := ss.existingBySlug[story.FullSlug]; ok {
		operation = OperationUpdate
	} else {
		// Fallback to a single GET only when index lacks entry
		if existing, _ := ss.api.GetStoriesBySlug(ctx, ss.targetSpaceID, story.FullSlug); len(existing) > 0 {
			operation = OperationUpdate
		}
	}

	targetStory, err := ss.SyncStory(ctx, story, shouldPublish)
	if err != nil {
		// Return counters even on error
		return &SyncItemResult{Operation: operation, RetryTotal: int(rc.Total), Retry429: int(rc.Status429)}, err
	}

	return &SyncItemResult{
		Operation:   operation,
		TargetStory: &targetStory,
		Warning:     "",
		RetryTotal:  int(rc.Total),
		Retry429:    int(rc.Status429),
	}, nil
}

// SyncFolderDetailed synchronizes a folder and returns detailed result
func (ss *StorySyncer) SyncFolderDetailed(folder sb.Story, shouldPublish bool) (*SyncItemResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	// Attach per-item retry counters to context
	rc := &sb.RetryCounters{}
	ctx = sb.WithRetryCounters(ctx, rc)

	// Determine operation type from in-memory index only (avoid extra GET)
	operation := OperationCreate
	if _, ok := ss.existingBySlug[folder.FullSlug]; ok {
		operation = OperationUpdate
	} else {
		// Fallback only when index lacks entry
		if existing, _ := ss.api.GetStoriesBySlug(ctx, ss.targetSpaceID, folder.FullSlug); len(existing) > 0 {
			operation = OperationUpdate
		}
	}

	targetFolder, err := ss.SyncFolder(ctx, folder, shouldPublish)
	if err != nil {
		return &SyncItemResult{Operation: operation, RetryTotal: int(rc.Total), Retry429: int(rc.Status429)}, err
	}

	return &SyncItemResult{
		Operation:   operation,
		TargetStory: &targetFolder,
		Warning:     "",
		RetryTotal:  int(rc.Total),
		Retry429:    int(rc.Status429),
	}, nil
}

// resolveParentFolder resolves and sets the correct parent folder ID for a story
func (ss *StorySyncer) resolveParentFolder(ctx context.Context, story sb.Story) sb.Story {
	// Compute parent slug from full slug and resolve against target space.
	// Ignore any existing FolderID from source; always map by slug.
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
