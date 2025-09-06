package ui

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"storyblok-sync/internal/sb"
)

// TestSyncRetryLogic has been moved to test the extracted sync module
// The retry logic is now handled by sync.SyncOrchestrator.SyncWithRetry
func TestSyncRetryLogic(t *testing.T) {
	// This test has been replaced by tests in the sync module
	// The syncWithRetry functionality is now in sync.SyncOrchestrator
	t.Skip("Retry logic has been moved to extracted sync module")
}

// Conversion roundtrip test removed: UI now uses core PreflightItem directly

func TestTranslatedSlugsProcessing(t *testing.T) {
	sourceStory := sb.Story{
		ID:       1,
		Name:     "test",
		FullSlug: "test",
		TranslatedSlugs: []sb.TranslatedSlug{
			{Lang: "en", Name: "test", Path: "test"},
			{Lang: "de", Name: "test-de", Path: "test-de"},
		},
	}

	existingStory := sb.Story{
		ID:       2,
		Name:     "test",
		FullSlug: "test",
		TranslatedSlugs: []sb.TranslatedSlug{
			{ID: &[]int{100}[0], Lang: "en", Name: "test", Path: "test"},
			{ID: &[]int{101}[0], Lang: "de", Name: "test-de", Path: "test-de"},
		},
	}

	m := InitialModel()
	result := m.processTranslatedSlugs(sourceStory, []sb.Story{existingStory})

	// Should have no TranslatedSlugs but TranslatedSlugsAttributes instead
	if len(result.TranslatedSlugs) != 0 {
		t.Error("TranslatedSlugs should be cleared")
	}

	if len(result.TranslatedSlugsAttributes) != 2 {
		t.Fatalf("expected 2 translated slug attributes, got %d", len(result.TranslatedSlugsAttributes))
	}

	// Check that IDs were preserved from existing story
	for _, attr := range result.TranslatedSlugsAttributes {
		if attr.Lang == "en" && (attr.ID == nil || *attr.ID != 100) {
			t.Error("English translated slug ID should be preserved as 100")
		}
		if attr.Lang == "de" && (attr.ID == nil || *attr.ID != 101) {
			t.Error("German translated slug ID should be preserved as 101")
		}
	}
}

func TestParentSlugFunction(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"app/sub/page", "app/sub"},
		{"app/page", "app"},
		{"page", ""},
		{"", ""},
	}

	for _, test := range tests {
		result := parentSlug(test.input)
		if result != test.expected {
			t.Errorf("parentSlug(%q) = %q, expected %q", test.input, result, test.expected)
		}
	}
}

type mockAPI struct {
	source       map[int]sb.Story
	target       map[string]sb.Story
	nextID       int
	publishCalls []bool
}

func (m *mockAPI) GetStoriesBySlug(ctx context.Context, spaceID int, slug string) ([]sb.Story, error) {
	if st, ok := m.target[slug]; ok {
		return []sb.Story{st}, nil
	}
	return nil, nil
}

func (m *mockAPI) GetStoryWithContent(ctx context.Context, spaceID, storyID int) (sb.Story, error) {
	if st, ok := m.source[storyID]; ok {
		return st, nil
	}
	return sb.Story{}, fmt.Errorf("not found")
}

func (m *mockAPI) GetStoryRaw(ctx context.Context, spaceID, storyID int) (map[string]interface{}, error) {
	// Build raw from source for folder creation
	if s, ok := m.source[storyID]; ok {
		var content map[string]interface{}
		_ = json.Unmarshal(s.Content, &content)
		raw := map[string]interface{}{
			"uuid": s.UUID, "name": s.Name, "slug": s.Slug, "full_slug": s.FullSlug, "content": content, "is_folder": s.IsFolder,
		}
		if s.FolderID != nil {
			raw["parent_id"] = *s.FolderID
		} else {
			raw["parent_id"] = 0
		}
		return raw, nil
	}
	return map[string]interface{}{}, nil
}

func (m *mockAPI) CreateStoryRawWithPublish(ctx context.Context, spaceID int, story map[string]interface{}, publish bool) (sb.Story, error) {
	m.publishCalls = append(m.publishCalls, publish)
	m.nextID++
	st := sb.Story{ID: m.nextID, FullSlug: story["full_slug"].(string), IsFolder: true}
	// Store minimal typed representation
	b, _ := json.Marshal(story["content"])
	st.Content = json.RawMessage(b)
	if pid, ok := story["parent_id"].(int); ok {
		st.FolderID = &pid
	}
	m.target[st.FullSlug] = st
	return st, nil
}

func TestEnsureFolderPathCreatesFolders(t *testing.T) {
	srcFolders := []sb.Story{
		{ID: 1, Name: "foo", Slug: "foo", FullSlug: "foo", IsFolder: true, Content: json.RawMessage([]byte(`{"content_types":["page"],"lock_subfolders_content_types":false}`))},
		{ID: 2, Name: "bar", Slug: "bar", FullSlug: "foo/bar", IsFolder: true, FolderID: &[]int{1}[0], Content: json.RawMessage([]byte(`{"content_types":["page"],"lock_subfolders_content_types":false}`))},
	}
	api := &mockAPI{
		source: map[int]sb.Story{
			1: srcFolders[0],
			2: srcFolders[1],
		},
		target: make(map[string]sb.Story),
	}
	report := Report{}

	created, err := ensureFolderPathImpl(api, &report, srcFolders, 1, 2, "foo/bar/baz", false)
	if err != nil {
		t.Fatalf("ensureFolderPathImpl returned error: %v", err)
	}
	if len(created) != 2 {
		t.Fatalf("expected 2 folders created, got %d", len(created))
	}
	if foo, ok := api.target["foo"]; !ok {
		t.Errorf("expected folder 'foo' to be created")
	} else {
		var tmp map[string]interface{}
		_ = json.Unmarshal(foo.Content, &tmp)
		v, ok := tmp["content_types"]
		if !ok {
			t.Errorf("expected folder 'foo' to keep content type 'page'")
		} else {
			arr, _ := v.([]interface{})
			if len(arr) != 1 || arr[0] != "page" {
				t.Errorf("expected folder 'foo' to keep content type 'page'")
			}
		}
	}
	if bar, ok := api.target["foo/bar"]; !ok {
		t.Errorf("expected folder 'foo/bar' to be created")
	} else {
		parent := api.target["foo"]
		if bar.FolderID == nil || *bar.FolderID != parent.ID {
			t.Errorf("expected 'foo/bar' to reference parent 'foo'")
		}
		var tmp map[string]interface{}
		_ = json.Unmarshal(bar.Content, &tmp)
		v, ok := tmp["content_types"]
		if !ok {
			t.Errorf("expected folder 'foo/bar' to keep content type 'page'")
		} else {
			arr, _ := v.([]interface{})
			if len(arr) != 1 || arr[0] != "page" {
				t.Errorf("expected folder 'foo/bar' to keep content type 'page'")
			}
		}
	}
	if len(report.Entries) != 2 {
		t.Fatalf("expected 2 report entries, got %d", len(report.Entries))
	}
	if report.Entries[0].Operation != "create" {
		t.Errorf("expected operation 'create', got %s", report.Entries[0].Operation)
	}
	// In raw path, folders are never published; no publish flags expected
}

type publishLimitCreateMock struct {
	calls []bool
}

func (m *publishLimitCreateMock) CreateStoryWithPublish(ctx context.Context, spaceID int, st sb.Story, publish bool) (sb.Story, error) {
	m.calls = append(m.calls, publish)
	if len(m.calls) == 1 {
		return sb.Story{}, errors.New("This space is in the development mode. Publishing is limited to 3 publishes per day. Please upgrade the space.")
	}
	st.ID = 1
	return st, nil
}

func TestCreateStoryWithPublishRetryDevMode(t *testing.T) {
	// This test has been moved to the extracted sync module
	// The retry logic is now in sync.APIAdapter.CreateStoryWithPublishRetry
	t.Skip("Publish retry logic has been moved to extracted sync module")
}

type publishLimitUpdateMock struct {
	calls []bool
}

func (m *publishLimitUpdateMock) UpdateStory(ctx context.Context, spaceID int, st sb.Story, publish bool) (sb.Story, error) {
	m.calls = append(m.calls, publish)
	if len(m.calls) == 1 {
		return sb.Story{}, errors.New("This space is in the development mode. Publishing is limited to 3 publishes per day. Please upgrade the space.")
	}
	return st, nil
}

func TestUpdateStoryWithPublishRetryDevMode(t *testing.T) {
	// This test has been moved to the extracted sync module
	// The retry logic is now in sync.APIAdapter.UpdateStoryWithPublishRetry
	t.Skip("Publish retry logic has been moved to extracted sync module")
}

func TestShouldPublishChecksPlanLevel(t *testing.T) {
	m := InitialModel()
	if !m.shouldPublish() {
		t.Errorf("expected default shouldPublish to be true")
	}

	m.targetSpace = &sb.Space{PlanLevel: 999}
	if m.shouldPublish() {
		t.Errorf("expected shouldPublish false for plan level 999")
	}

	m.targetSpace.PlanLevel = 1
	if !m.shouldPublish() {
		t.Errorf("expected shouldPublish true for plan level 1")
	}
}

func TestResumeLogicFindsNextPendingAndStarts(t *testing.T) {
	m := InitialModel()
	// Seed preflight with 3 items, mark first done, second pending, third pending
	m.preflight.items = []PreflightItem{{Run: RunDone}, {Run: RunPending}, {Run: RunPending}}
	m.state = stateSync
	m.syncing = false
	// Provide minimal spaces and API to satisfy orchestrator construction
	src := sb.Space{ID: 1, Name: "src"}
	tgt := sb.Space{ID: 2, Name: "tgt"}
	m.sourceSpace = &src
	m.targetSpace = &tgt
	m.api = sb.New("")

	// Simulate pressing 'r'
	model, cmd := m.handleSyncKey("r")
	if cmd == nil {
		t.Fatalf("expected a command to be returned on resume")
	}
	resumed, ok := model.(Model)
	if !ok {
		t.Fatalf("expected Model type after resume")
	}
	if !resumed.syncing {
		t.Fatalf("expected syncing=true after resume")
	}
	if resumed.syncIndex != 1 {
		t.Fatalf("expected syncIndex=1 (first pending), got %d", resumed.syncIndex)
	}
}

func TestRunNextItemScansForPending(t *testing.T) {
	m := InitialModel()
	// Seed items with different run states; pending at index 2 should be picked
	m.preflight.items = []PreflightItem{
		{Run: RunDone, Story: sb.Story{ID: 1, FullSlug: "a"}},
		{Run: RunCancelled, Story: sb.Story{ID: 2, FullSlug: "b"}},
		{Run: RunPending, Story: sb.Story{ID: 3, FullSlug: "c"}},
		{Run: RunPending, Story: sb.Story{ID: 4, FullSlug: "d"}},
	}
	m.sourceSpace = &sb.Space{ID: 1, Name: "src"}
	m.targetSpace = &sb.Space{ID: 2, Name: "tgt"}
	m.api = sb.New("")

	// Start from index 0; runNextItem should set index 2 to running
	m.syncIndex = 0
	cmd := m.runNextItem()
	if cmd == nil {
		t.Fatalf("expected a command to be returned")
	}
	if m.syncIndex != 2 {
		t.Fatalf("expected syncIndex=2, got %d", m.syncIndex)
	}
	if m.preflight.items[2].Run != RunRunning {
		t.Fatalf("expected item 2 to be RunRunning")
	}
}

func TestRunNextItemReturnsNilWhenNoPending(t *testing.T) {
	m := InitialModel()
	m.preflight.items = []PreflightItem{
		{Run: RunDone, Story: sb.Story{ID: 1, FullSlug: "a"}},
		{Run: RunCancelled, Story: sb.Story{ID: 2, FullSlug: "b"}},
	}
	m.sourceSpace = &sb.Space{ID: 1, Name: "src"}
	m.targetSpace = &sb.Space{ID: 2, Name: "tgt"}
	m.api = sb.New("")

	cmd := m.runNextItem()
	if cmd != nil {
		t.Fatalf("expected nil cmd when no pending items")
	}
}

func TestRunNextItemRespectsFolderPhase(t *testing.T) {
	m := InitialModel()
	// One folder and one story, both pending; set syncIndex near the story
	folder := sb.Story{ID: 1, FullSlug: "app", Slug: "app", IsFolder: true}
	story := sb.Story{ID: 2, FullSlug: "app/page", Slug: "page"}
	m.preflight.items = []PreflightItem{
		{Run: RunPending, Story: folder},
		{Run: RunPending, Story: story},
	}
	m.sourceSpace = &sb.Space{ID: 1, Name: "src"}
	m.targetSpace = &sb.Space{ID: 2, Name: "tgt"}
	m.api = sb.New("")

	// Put syncIndex at 1 so naive scan would pick the story first
	m.syncIndex = 1
	cmd := m.runNextItem()
	if cmd == nil {
		t.Fatalf("expected a command to be returned")
	}
	if m.preflight.items[0].Run != RunRunning {
		t.Fatalf("expected folder scheduled first, got %v", m.preflight.items[0].Run)
	}
	if m.preflight.items[1].Run == RunRunning {
		t.Fatalf("did not expect story scheduled while folders pending")
	}

	// Mark folder done; next call should schedule the story
	m.preflight.items[0].Run = RunDone
	cmd = m.runNextItem()
	if cmd == nil {
		t.Fatalf("expected a command to schedule story after folders done")
	}
	if m.preflight.items[1].Run != RunRunning {
		t.Fatalf("expected story scheduled after folder phase, got %v", m.preflight.items[1].Run)
	}
}

func TestUpdateContinuesWhilePending(t *testing.T) {
	m := InitialModel()
	m.state = stateSync
	m.sourceSpace = &sb.Space{ID: 1, Name: "src"}
	m.targetSpace = &sb.Space{ID: 2, Name: "tgt"}
	m.api = sb.New("")

	// Two items: first running finishes, second is pending -> should continue
	m.preflight.items = []PreflightItem{
		{Run: RunRunning, Story: sb.Story{ID: 1, FullSlug: "a"}},
		{Run: RunPending, Story: sb.Story{ID: 2, FullSlug: "b"}},
	}

	msg := syncResultMsg{Index: 0, Duration: 5}
	model, cmd := m.Update(msg)
	if cmd == nil {
		t.Fatalf("expected a continuation command when pending remains")
	}
	mm, ok := model.(Model)
	if !ok {
		t.Fatalf("expected Model return type")
	}
	if mm.preflight.items[0].Run != RunDone {
		t.Fatalf("expected first item marked RunDone")
	}
	if mm.syncIndex != 1 {
		t.Fatalf("expected syncIndex moved to next pending (1), got %d", mm.syncIndex)
	}
}

func TestCtrlC_PausesSchedulingAndAllowsResume(t *testing.T) {
	m := InitialModel()
	m.state = stateSync
	m.sourceSpace = &sb.Space{ID: 1, Name: "src"}
	m.targetSpace = &sb.Space{ID: 2, Name: "tgt"}
	m.api = sb.New("")
	m.syncing = true
	// One running, one pending
	m.preflight.items = []PreflightItem{
		{Run: RunRunning, Story: sb.Story{ID: 1, FullSlug: "a"}},
		{Run: RunPending, Story: sb.Story{ID: 2, FullSlug: "b"}},
	}

	// Send Ctrl+C globally via Update
	model, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	mm := model.(Model)
	if !mm.paused {
		t.Fatalf("expected paused=true after Ctrl+C")
	}

	// A running worker finishes; should not schedule new work while paused
	model, cmd := mm.Update(syncResultMsg{Index: 0, Duration: 1})
	mm = model.(Model)
	if cmd != nil {
		t.Fatalf("expected no new scheduling while paused")
	}
	if mm.syncing {
		t.Fatalf("expected syncing=false when paused and nothing running")
	}

	// Resume with 'r' should clear pause and schedule next pending
	model, cmd = mm.handleSyncKey("r")
	mm = model.(Model)
	if !mm.syncing || mm.paused {
		t.Fatalf("expected syncing=true and paused=false after resume")
	}
	if cmd == nil {
		t.Fatalf("expected a command to be returned on resume")
	}
}

func TestUIPhasedSyncAndIndexUpdate_PreventsRootPlacement(t *testing.T) {
	// Setup model with a folder and a child story, both pending
	folder := sb.Story{ID: 1, Name: "app", Slug: "app", FullSlug: "app", IsFolder: true}
	story := sb.Story{ID: 2, Name: "p", Slug: "p", FullSlug: "app/p"}

	m := InitialModel()
	m.state = stateSync
	m.sourceSpace = &sb.Space{ID: 1, Name: "src"}
	m.targetSpace = &sb.Space{ID: 2, Name: "tgt"}
	m.api = sb.New("")
	m.preflight.items = []PreflightItem{
		{Story: folder, Selected: true, Run: RunPending},
		{Story: story, Selected: true, Run: RunPending},
	}

	// Initially, with folder pending, only one worker should start and it must be the folder
	cmd := m.runNextItem()
	if cmd == nil {
		t.Fatalf("expected a command to start folder phase")
	}
	if m.preflight.items[0].Run != RunRunning || m.preflight.items[1].Run == RunRunning {
		t.Fatalf("expected folder running first and story not scheduled yet")
	}

	// Simulate folder completion with a resulting folder returned; UI should refresh target index
	folderResult := syncItemResult{Operation: "create", TargetStory: &sb.Story{ID: 55, FullSlug: "app", IsFolder: true}}
	modelAny, cont := m.Update(syncResultMsg{Index: 0, Result: &folderResult})
	mm := modelAny.(Model)
	// Update should immediately schedule the next pending story
	if cont == nil {
		t.Fatalf("expected Update to return continuation command after folder completion")
	}

	// Now, a new run should schedule the story (folder phase complete)
	// The story should already be marked running by the continuation
	if mm.preflight.items[1].Run != RunRunning {
		t.Fatalf("expected story to be scheduled after folder done")
	}

	// Build a syncer with the UI's target index to validate parent resolution will find the folder (no API call)
	tgtIndex := make(map[string]sb.Story)
	for _, s := range mm.storiesTarget {
		tgtIndex[s.FullSlug] = s
	}
	if _, ok := tgtIndex["app"]; !ok {
		t.Fatalf("expected target index to include created folder 'app'")
	}
}

func TestOverwriteThenUnpublishSchedulesUnpublish(t *testing.T) {
	// Setup a published source, published target; choose Draft to trigger overwrite+unpublish
	st := sb.Story{ID: 1, Name: "one", Slug: "one", FullSlug: "one", Published: true}
	tgt := sb.Story{ID: 9, Name: "one", Slug: "one", FullSlug: "one", Published: true}
	m := InitialModel()
	m.state = stateSync
	m.sourceSpace = &sb.Space{ID: 1, Name: "src"}
	m.targetSpace = &sb.Space{ID: 2, Name: "tgt"}
	m.api = sb.New("") // won't be used in test (we don't execute the cmd)
	m.preflight.items = []PreflightItem{{Story: st, Selected: true, Run: RunRunning}}
	m.storiesTarget = []sb.Story{tgt}
	// Choose Draft explicitly
	m.setPublishMode(st.FullSlug, PublishModeDraft)

	// Simulate sync result success; should schedule unpublish (cmd != nil) and clear flag
	res := &syncItemResult{Operation: "update", TargetStory: &sb.Story{ID: 9, FullSlug: st.FullSlug}}
	model, cmd := m.Update(syncResultMsg{Index: 0, Result: res, Duration: 5})
	if cmd == nil {
		t.Fatalf("expected unpublish command to be scheduled")
	}
	mm := model.(Model)
	if mm.unpublishAfter[st.FullSlug] {
		t.Fatalf("expected unpublishAfter flag cleared after scheduling")
	}

	// Simulate completion of unpublish
	model, _ = mm.Update(unpublishDoneMsg{Index: 0, Duration: 2})
	mm2 := model.(Model)
	// There should be at least one unpublish report entry
	found := false
	for _, e := range mm2.report.Entries {
		if e.Operation == "unpublish" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected an unpublish entry in report")
	}
}
