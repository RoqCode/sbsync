package sync

import (
	"context"
	"errors"
	"testing"

	"storyblok-sync/internal/sb"
)

// mockSyncAPI implements SyncAPI for orchestrator tests
type mockSyncAPI struct {
	source       map[int]sb.Story
	targetBySlug map[string]sb.Story

	createCalls []bool
	updateCalls []bool

	// for retry simulation
	failFirstCreate bool
	createAttempts  int

	nextID int
}

func newMockSyncAPI() *mockSyncAPI {
	return &mockSyncAPI{
		source:       make(map[int]sb.Story),
		targetBySlug: make(map[string]sb.Story),
		nextID:       100,
	}
}

func (m *mockSyncAPI) GetStoriesBySlug(_ context.Context, spaceID int, slug string) ([]sb.Story, error) {
	// spaceID 2 := target, 1 := source
	if spaceID == 2 {
		if st, ok := m.targetBySlug[slug]; ok {
			return []sb.Story{st}, nil
		}
		return nil, nil
	}
	// minimal support for source lookups
	for _, st := range m.source {
		if st.FullSlug == slug {
			return []sb.Story{st}, nil
		}
	}
	return nil, nil
}

func (m *mockSyncAPI) GetStoryWithContent(_ context.Context, spaceID, storyID int) (sb.Story, error) {
	if spaceID != 1 {
		return sb.Story{}, nil
	}
	if st, ok := m.source[storyID]; ok {
		// ensure some content
		if len(st.Content) == 0 {
			st.Content = []byte(`{"component":"page"}`)
		}
		return st, nil
	}
	return sb.Story{}, errors.New("not found")
}

func (m *mockSyncAPI) UpdateStoryUUID(_ context.Context, _ int, _ int, _ string) error {
	return nil
}

func (m *mockSyncAPI) GetStoryRaw(_ context.Context, spaceID, storyID int) (map[string]interface{}, error) {
	if spaceID != 1 {
		return map[string]interface{}{}, nil
	}
	st, ok := m.source[storyID]
	if !ok {
		return map[string]interface{}{}, errors.New("not found")
	}
	// Build minimal raw payload similar to Storyblok
	raw := map[string]interface{}{
		"uuid":      st.UUID,
		"name":      st.Name,
		"slug":      st.Slug,
		"full_slug": st.FullSlug,
		"is_folder": st.IsFolder,
	}
	// keep a tiny content object
	raw["content"] = map[string]interface{}{"component": "page"}
	// parent_id is set by syncer/folder builder
	return raw, nil
}

func (m *mockSyncAPI) CreateStoryRawWithPublish(_ context.Context, spaceID int, story map[string]interface{}, publish bool) (sb.Story, error) {
	m.createCalls = append(m.createCalls, publish)
	m.createAttempts++
	if m.failFirstCreate && m.createAttempts == 1 {
		return sb.Story{}, errors.New("temporary failure")
	}
	m.nextID++
	full := ""
	if v, ok := story["full_slug"].(string); ok {
		full = v
	}
	isFolder := false
	if v, ok := story["is_folder"].(bool); ok {
		isFolder = v
	}
	st := sb.Story{ID: m.nextID, FullSlug: full, Slug: full, Name: full, IsFolder: isFolder, Published: publish}
	if full != "" {
		m.targetBySlug[full] = st
	}
	return st, nil
}

func (m *mockSyncAPI) UpdateStoryRawWithPublish(_ context.Context, _ int, storyID int, story map[string]interface{}, publish bool) (sb.Story, error) {
	m.updateCalls = append(m.updateCalls, publish)
	full := ""
	if v, ok := story["full_slug"].(string); ok {
		full = v
	}
	st := sb.Story{ID: storyID, FullSlug: full, Slug: full, Name: full, Published: publish}
	if full != "" {
		m.targetBySlug[full] = st
	}
	return st, nil
}

// mockReport is a no-op ReportInterface for orchestrator
type mockReport struct{}

func (mockReport) AddSuccess(string, string, int64, *sb.Story)                    {}
func (mockReport) AddWarning(string, string, string, int64, *sb.Story, *sb.Story) {}
func (mockReport) AddError(string, string, int64, *sb.Story, error)               {}

// testSyncItem adapts a story to SyncItem
type testSyncItem struct {
	s      sb.Story
	folder bool
}

func (t testSyncItem) GetStory() sb.Story { return t.s }
func (t testSyncItem) IsFolder() bool     { return t.folder }

func TestRunSyncItem_FolderCreate_NoPublish(t *testing.T) {
	api := newMockSyncAPI()
	// seed source folder
	api.source[1] = sb.Story{ID: 1, Name: "foo", Slug: "foo", FullSlug: "foo", IsFolder: true}

	src := &sb.Space{ID: 1, Name: "src"}
	tgt := &sb.Space{ID: 2, Name: "tgt"}
	orch := NewSyncOrchestrator(api, mockReport{}, src, tgt, map[string]sb.Story{})

	item := testSyncItem{s: sb.Story{ID: 1, Name: "foo", Slug: "foo", FullSlug: "foo", IsFolder: true}, folder: true}
	cmd := orch.RunSyncItem(context.Background(), 0, item)

	// Execute the command synchronously
	msg := cmd()
	res, ok := msg.(SyncResultMsg)
	if !ok {
		t.Fatalf("expected SyncResultMsg, got %T", msg)
	}
	if res.Err != nil {
		t.Fatalf("unexpected error: %v", res.Err)
	}
	if res.Result == nil || res.Result.Operation != OperationCreate {
		t.Fatalf("expected OperationCreate for folder, got %+v", res.Result)
	}
	if len(api.createCalls) == 0 {
		t.Fatalf("expected a create API call")
	}
	if api.createCalls[len(api.createCalls)-1] != false {
		t.Fatalf("expected folder create publish=false, got %v", api.createCalls[len(api.createCalls)-1])
	}
}

func TestRunSyncItem_StoryCreate_PublishRespectsPlan(t *testing.T) {
	// Table: plan -> expected publish flag
	cases := []struct {
		name      string
		planLevel int
		expectPub bool
	}{
		{"non-dev-plan", 1, true},
		{"dev-plan-999", 999, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			api := newMockSyncAPI()
			// source story
			api.source[10] = sb.Story{ID: 10, Name: "s1", Slug: "s1", FullSlug: "s1", IsFolder: false, Published: true}

			src := &sb.Space{ID: 1, Name: "src"}
			tgt := &sb.Space{ID: 2, Name: "tgt", PlanLevel: tc.planLevel}
			tgtIndex := map[string]sb.Story{"s3": {ID: 999, FullSlug: "s3"}}
			orch := NewSyncOrchestrator(api, mockReport{}, src, tgt, tgtIndex)

			item := testSyncItem{s: api.source[10], folder: false}
			cmd := orch.RunSyncItem(context.Background(), 0, item)
			msg := cmd()
			res, ok := msg.(SyncResultMsg)
			if !ok {
				t.Fatalf("expected SyncResultMsg, got %T", msg)
			}
			if res.Err != nil {
				t.Fatalf("unexpected error: %v", res.Err)
			}
			if res.Result == nil || res.Result.Operation != OperationCreate {
				t.Fatalf("expected OperationCreate for story, got %+v", res.Result)
			}
			if len(api.createCalls) == 0 {
				t.Fatalf("expected a create call for story")
			}
			got := api.createCalls[len(api.createCalls)-1]
			if got != tc.expectPub {
				t.Fatalf("expected publish=%v, got %v", tc.expectPub, got)
			}
		})
	}
}

func TestRunSyncItem_NoRetry_TransportOwnsRetries(t *testing.T) {
	api := newMockSyncAPI()
	api.failFirstCreate = true // first create fails
	api.source[20] = sb.Story{ID: 20, Name: "s2", Slug: "s2", FullSlug: "s2", Published: true}

	src := &sb.Space{ID: 1, Name: "src"}
	tgt := &sb.Space{ID: 2, Name: "tgt"}
	tgtIndex := map[string]sb.Story{"foo": {ID: 500, FullSlug: "foo", IsFolder: true}}
	orch := NewSyncOrchestrator(api, mockReport{}, src, tgt, tgtIndex)

	item := testSyncItem{s: api.source[20], folder: false}
	msg := orch.RunSyncItem(context.Background(), 0, item)()
	res, ok := msg.(SyncResultMsg)
	if !ok {
		t.Fatalf("expected SyncResultMsg, got %T", msg)
	}
	if res.Err == nil {
		t.Fatalf("expected error on first failure without orchestrator retry")
	}
	if got := api.createAttempts; got != 1 {
		t.Fatalf("expected exactly 1 create attempt, got %d", got)
	}
}

func TestRunSyncItem_StoryUpdate_PublishRespectsPlan(t *testing.T) {
	cases := []struct {
		name      string
		planLevel int
		expectPub bool
	}{
		{"non-dev-plan", 1, true},
		{"dev-plan-999", 999, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			api := newMockSyncAPI()
			// source story is published
			srcStory := sb.Story{ID: 30, Name: "s3", Slug: "s3", FullSlug: "s3", Published: true}
			api.source[30] = srcStory
			// target already has story -> triggers update path
			api.targetBySlug["s3"] = sb.Story{ID: 999, FullSlug: "s3"}

			src := &sb.Space{ID: 1, Name: "src"}
			tgt := &sb.Space{ID: 2, Name: "tgt", PlanLevel: tc.planLevel}
			orch := NewSyncOrchestrator(api, mockReport{}, src, tgt, map[string]sb.Story{})

			item := testSyncItem{s: srcStory, folder: false}
			msg := orch.RunSyncItem(context.Background(), 0, item)()
			res, ok := msg.(SyncResultMsg)
			if !ok {
				t.Fatalf("expected SyncResultMsg, got %T", msg)
			}
			if res.Err != nil {
				t.Fatalf("unexpected error: %v", res.Err)
			}
			if res.Result == nil || res.Result.Operation != OperationUpdate {
				t.Fatalf("expected OperationUpdate, got %+v", res.Result)
			}
			if len(api.updateCalls) == 0 {
				t.Fatalf("expected an update API call")
			}
			got := api.updateCalls[len(api.updateCalls)-1]
			if got != tc.expectPub {
				t.Fatalf("expected publish=%v on update, got %v", tc.expectPub, got)
			}
		})
	}
}

func TestRunSyncItem_FolderUpdate_NoPublish(t *testing.T) {
	api := newMockSyncAPI()
	// source folder
	api.source[40] = sb.Story{ID: 40, Name: "foo", Slug: "foo", FullSlug: "foo", IsFolder: true}
	// target folder already exists
	api.targetBySlug["foo"] = sb.Story{ID: 500, FullSlug: "foo", IsFolder: true}

	src := &sb.Space{ID: 1, Name: "src"}
	tgt := &sb.Space{ID: 2, Name: "tgt"}
	orch := NewSyncOrchestrator(api, mockReport{}, src, tgt, map[string]sb.Story{})

	item := testSyncItem{s: api.source[40], folder: true}
	msg := orch.RunSyncItem(context.Background(), 0, item)()
	res, ok := msg.(SyncResultMsg)
	if !ok {
		t.Fatalf("expected SyncResultMsg, got %T", msg)
	}
	if res.Err != nil {
		t.Fatalf("unexpected error: %v", res.Err)
	}
	if res.Result == nil || res.Result.Operation != OperationUpdate {
		t.Fatalf("expected OperationUpdate for folder, got %+v", res.Result)
	}
	if len(api.updateCalls) == 0 {
		t.Fatalf("expected an update call for folder")
	}
	if got := api.updateCalls[len(api.updateCalls)-1]; got != false {
		t.Fatalf("expected folder update publish=false, got %v", got)
	}
}
