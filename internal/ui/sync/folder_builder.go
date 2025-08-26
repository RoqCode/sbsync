package sync

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"storyblok-sync/internal/sb"
)

// Timeout constants
const (
	DefaultTimeout = 15 * time.Second
)

// FolderAPI interface defines the methods needed for folder operations
type FolderAPI interface {
	GetStoriesBySlug(ctx context.Context, spaceID int, slug string) ([]sb.Story, error)
	GetStoryWithContent(ctx context.Context, spaceID, storyID int) (sb.Story, error)
	CreateStoryWithPublish(ctx context.Context, spaceID int, st sb.Story, publish bool) (sb.Story, error)
}

// Report interface for folder creation reporting
type Report interface {
	AddSuccess(slug, operation string, duration int64, story *sb.Story)
}

// FolderPathBuilder handles the creation of folder hierarchies
type FolderPathBuilder struct {
	api           FolderAPI
	report        Report
	sourceStories map[string]sb.Story
	contentMgr    *ContentManager
	srcSpaceID    int
	tgtSpaceID    int
	publish       bool
}

// NewFolderPathBuilder creates a new folder path builder
func NewFolderPathBuilder(api FolderAPI, report Report, sourceStories []sb.Story, srcSpaceID, tgtSpaceID int, publish bool) *FolderPathBuilder {
	// Build source stories map for quick lookup
	sourceMap := make(map[string]sb.Story)
	for _, story := range sourceStories {
		sourceMap[story.FullSlug] = story
	}

	return &FolderPathBuilder{
		api:           api,
		report:        report,
		sourceStories: sourceMap,
		contentMgr:    NewContentManager(api, srcSpaceID),
		srcSpaceID:    srcSpaceID,
		tgtSpaceID:    tgtSpaceID,
		publish:       publish,
	}
}

// CheckExistingFolder checks if a folder exists in the target space
func (fpb *FolderPathBuilder) CheckExistingFolder(ctx context.Context, path string) (*sb.Story, error) {
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

// PrepareSourceFolder prepares a source folder for creation in target space
func (fpb *FolderPathBuilder) PrepareSourceFolder(ctx context.Context, path string, parentID *int) (sb.Story, error) {
	source, exists := fpb.sourceStories[path]
	if !exists {
		return sb.Story{}, fmt.Errorf("source folder not found: %s", path)
	}

	// Ensure content is loaded
	folder, err := fpb.contentMgr.EnsureContent(ctx, source)
	if err != nil {
		log.Printf("DEBUG: Failed to fetch content for folder %s: %v", path, err)
		return sb.Story{}, err
	}

	// Prepare for creation
	folder = PrepareStoryForCreation(folder)
	folder.FolderID = parentID

	log.Printf("DEBUG: Prepared source folder %s with content: %t", path, folder.Content != nil)
	return folder, nil
}

// CreateFolder creates a single folder in the target space
func (fpb *FolderPathBuilder) CreateFolder(ctx context.Context, folder sb.Story) (sb.Story, error) {
	log.Printf("DEBUG: Creating folder: %s", folder.FullSlug)

	created, err := fpb.api.CreateStoryWithPublish(ctx, fpb.tgtSpaceID, folder, fpb.publish)
	if err != nil {
		log.Printf("DEBUG: Failed to create folder %s: %v", folder.FullSlug, err)
		return sb.Story{}, err
	}

	log.Printf("DEBUG: Successfully created folder %s (ID: %d)", created.FullSlug, created.ID)
	return created, nil
}

// EnsureFolderPath creates missing folders in a path hierarchy
func (fpb *FolderPathBuilder) EnsureFolderPath(slug string) ([]sb.Story, error) {
	parts := strings.Split(slug, "/")
	if len(parts) <= 1 {
		return nil, nil
	}

	var created []sb.Story
	var parentID *int

	// Process each folder in the path hierarchy
	for i := 0; i < len(parts)-1; i++ {
		path := strings.Join(parts[:i+1], "/")

		ctx, cancel := context.WithTimeout(context.Background(), DefaultTimeout)

		// Check if folder already exists
		existing, err := fpb.CheckExistingFolder(ctx, path)
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
		ctx, cancel = context.WithTimeout(context.Background(), DefaultTimeout)
		folder, err := fpb.PrepareSourceFolder(ctx, path, parentID)
		cancel()

		if err != nil {
			return created, err
		}

		// Create the folder
		ctx, cancel = context.WithTimeout(context.Background(), DefaultTimeout)
		createdFolder, err := fpb.CreateFolder(ctx, folder)
		cancel()

		if err != nil {
			return created, err
		}

		created = append(created, createdFolder)
		parentID = &createdFolder.ID

		// Update report
		if fpb.report != nil {
			fpb.report.AddSuccess(createdFolder.FullSlug, OperationCreate, 0, &createdFolder)
		}
	}

	return created, nil
}

// EnsureFolderPathStatic is a static utility function for ensuring folder paths
func EnsureFolderPathStatic(api FolderAPI, report Report, sourceStories []sb.Story, srcSpaceID, tgtSpaceID int, slug string, publish bool) ([]sb.Story, error) {
	builder := NewFolderPathBuilder(api, report, sourceStories, srcSpaceID, tgtSpaceID, publish)
	return builder.EnsureFolderPath(slug)
}