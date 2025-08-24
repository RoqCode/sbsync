package ui

import (
	"encoding/json"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"unsafe"

	"storyblok-sync/internal/sb"
)

func TestSyncStructureCreatesFolders(t *testing.T) {
	root := sb.Story{ID: 1, Name: "a__portal", Slug: "a__portal", FullSlug: "a__portal", IsFolder: true}
	de := sb.Story{ID: 2, Name: "de", Slug: "de", FullSlug: "a__portal/de", FolderID: &root.ID, IsFolder: true}
	shop := sb.Story{ID: 3, Name: "shop", Slug: "shop", FullSlug: "a__portal/de/shop", FolderID: &de.ID, IsFolder: true}
	detail := sb.Story{ID: 4, Name: "detail", Slug: "detail", FullSlug: "a__portal/de/shop/detail", FolderID: &shop.ID, IsFolder: true}
	item := sb.Story{ID: 5, Name: "item1", Slug: "item1", FullSlug: "a__portal/de/shop/detail/item1", FolderID: &detail.ID}

	m := InitialModel()
	m.storiesSource = []sb.Story{root, de, shop, detail, item}

	if err := m.syncStructure(item); err != nil {
		t.Fatalf("syncStructure returned error: %v", err)
	}

	expected := []string{root.FullSlug, de.FullSlug, shop.FullSlug, detail.FullSlug}
	if len(m.storiesTarget) != len(expected) {
		t.Fatalf("expected %d folders, got %d", len(expected), len(m.storiesTarget))
	}
	for _, slug := range expected {
		if idx := m.findTarget(slug); idx < 0 {
			t.Fatalf("folder %s not created", slug)
		}
	}
}

func TestSyncStoryCreatesAndUpdates(t *testing.T) {
	folder := sb.Story{ID: 1, Name: "app", Slug: "app", FullSlug: "app", IsFolder: true}
	story := sb.Story{ID: 2, Name: "one", Slug: "one", FullSlug: "app/one", FolderID: &folder.ID}

	m := InitialModel()
	m.storiesSource = []sb.Story{folder, story}

	if err := m.syncStructure(story); err != nil {
		t.Fatalf("syncStructure: %v", err)
	}
	if err := m.syncStory(story); err != nil {
		t.Fatalf("syncStory create: %v", err)
	}
	if idx := m.findTarget(story.FullSlug); idx < 0 {
		t.Fatalf("story not created")
	}

	updated := story
	updated.Name = "eins"
	if err := m.syncStory(updated); err != nil {
		t.Fatalf("syncStory update: %v", err)
	}
	idx := m.findTarget(story.FullSlug)
	if got := m.storiesTarget[idx].Name; got != "eins" {
		t.Fatalf("expected updated name 'eins', got %q", got)
	}
	if len(m.storiesTarget) != 2 {
		t.Fatalf("expected 2 items total, got %d", len(m.storiesTarget))
	}
}

func TestSyncStartsWithCopiesSubtree(t *testing.T) {
	parent := sb.Story{ID: 1, Name: "app", Slug: "app", FullSlug: "app", IsFolder: true}
	child1 := sb.Story{ID: 2, Name: "one", Slug: "one", FullSlug: "app/one", FolderID: &parent.ID}
	child2 := sb.Story{ID: 3, Name: "two", Slug: "two", FullSlug: "app/two", FolderID: &parent.ID}

	m := InitialModel()
	m.storiesSource = []sb.Story{parent, child1, child2}

	if err := m.syncStartsWith("app"); err != nil {
		t.Fatalf("syncStartsWith: %v", err)
	}
	if len(m.storiesTarget) != 3 {
		t.Fatalf("expected 3 items, got %d", len(m.storiesTarget))
	}
	for _, slug := range []string{parent.FullSlug, child1.FullSlug, child2.FullSlug} {
		if idx := m.findTarget(slug); idx < 0 {
			t.Fatalf("missing %s", slug)
		}
	}
	folderIdx := m.findTarget(parent.FullSlug)
	folderID := m.storiesTarget[folderIdx].ID
	for _, slug := range []string{child1.FullSlug, child2.FullSlug} {
		idx := m.findTarget(slug)
		if m.storiesTarget[idx].FolderID == nil || *m.storiesTarget[idx].FolderID != folderID {
			t.Fatalf("child %s does not reference folder", slug)
		}
	}
}


type rtFunc func(*http.Request) (*http.Response, error)

func (f rtFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestSyncStructurePreservesTranslatedSlugs(t *testing.T) {
	c := sb.New("token")
	var createdBody []byte
	httpClient := &http.Client{Transport: rtFunc(func(req *http.Request) (*http.Response, error) {
		switch req.Method {
		case http.MethodGet:
			body := `{"story":{"id":1,"name":"root","slug":"root","full_slug":"root","is_folder":true,"translated_slugs":[{"lang":"en","name":"root","slug":"root","full_slug":"root"},{"lang":"de","name":"wurzel","slug":"wurzel","full_slug":"de/wurzel"}]}}`
			res := &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}
			return res, nil
		case http.MethodPost:
			b, _ := io.ReadAll(req.Body)
			createdBody = b
			res := &http.Response{StatusCode: 201, Body: io.NopCloser(strings.NewReader(`{"story":{"id":99}}`)), Header: make(http.Header)}
			return res, nil
		default:
			t.Fatalf("unexpected method %s", req.Method)
		}
		return nil, nil
	})}
	v := reflect.ValueOf(c).Elem().FieldByName("http")
	reflect.NewAt(v.Type(), unsafe.Pointer(v.UnsafeAddr())).Elem().Set(reflect.ValueOf(httpClient))

	m := InitialModel()
	m.api = c
	m.sourceSpace = &sb.Space{ID: 1}
	m.targetSpace = &sb.Space{ID: 2}
	root := sb.Story{ID: 1, Name: "root", Slug: "root", FullSlug: "root", IsFolder: true}
	m.storiesSource = []sb.Story{root}

	if err := m.syncStructure(root); err != nil {
		t.Fatalf("syncStructure: %v", err)
	}

	var payload struct {
		Story sb.Story `json:"story"`
	}
	if err := json.Unmarshal(createdBody, &payload); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(payload.Story.TranslatedSlugs) != 2 {
		t.Fatalf("expected 2 translated slugs, got %d", len(payload.Story.TranslatedSlugs))
	}
	var found bool
	for _, ts := range payload.Story.TranslatedSlugs {
		if ts.Lang == "de" {
			found = true
			if ts.Slug != "wurzel" || ts.FullSlug != "de/wurzel" {
				t.Fatalf("de translation not preserved: %+v", ts)
			}
		}
	}
	if !found {
		t.Fatalf("de translation missing")
  }
}

func TestSyncStructureTracksChangedSlugs(t *testing.T) {
	root := sb.Story{ID: 1, Name: "root", Slug: "root", FullSlug: "root", IsFolder: true}
	child := sb.Story{ID: 2, Name: "child", Slug: "child", FullSlug: "root/child", FolderID: &root.ID, IsFolder: true}
	grand := sb.Story{ID: 3, Name: "grand", Slug: "grand", FullSlug: "root/child/grand", FolderID: &child.ID, IsFolder: true}

	m := InitialModel()
	m.storiesSource = []sb.Story{root, child, grand}

	if err := m.syncStructure(child); err != nil {
		t.Fatalf("syncStructure(child): %v", err)
	}

	rIdx := m.findTarget(root.FullSlug)
	if rIdx < 0 {
		t.Fatalf("root not created")
	}
	cIdx := m.findTarget(child.FullSlug)
	if cIdx < 0 {
		t.Fatalf("child not created")
	}
	childID := m.storiesTarget[cIdx].ID

	// simulate API slug changes
	m.storiesTarget[rIdx].Slug = "root-1"
	m.storiesTarget[rIdx].FullSlug = "root-1"
	m.storiesTarget[cIdx].Slug = "child-1"
	m.storiesTarget[cIdx].FullSlug = "root-1/child-1"

	if err := m.syncStructure(grand); err != nil {
		t.Fatalf("syncStructure(grand): %v", err)
	}

	if len(m.storiesTarget) != 3 {
		t.Fatalf("expected 3 folders, got %d", len(m.storiesTarget))
	}
	gIdx := m.findTarget(grand.FullSlug)
	if gIdx < 0 {
		t.Fatalf("grandchild not created")
	}
	if m.storiesTarget[gIdx].FolderID == nil || *m.storiesTarget[gIdx].FolderID != childID {
		t.Fatalf("grandchild not linked to child")
	}
}
