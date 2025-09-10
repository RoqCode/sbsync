package componentsync

import (
	"context"
	"errors"
	"storyblok-sync/internal/sb"
	"testing"
)

type fakeTagAPI struct {
	tags       map[string]int // name -> id
	next       int
	failCreate bool
}

func (f *fakeTagAPI) ListInternalTags(ctx context.Context, spaceID int) ([]sb.InternalTag, error) {
	out := make([]sb.InternalTag, 0, len(f.tags))
	for n, id := range f.tags {
		out = append(out, sb.InternalTag{ID: id, Name: n, ObjectType: "component"})
	}
	return out, nil
}
func (f *fakeTagAPI) CreateInternalTag(ctx context.Context, spaceID int, name string, objectType string) (sb.InternalTag, error) {
	if f.failCreate {
		return sb.InternalTag{}, errors.New("create failed")
	}
	if f.tags == nil {
		f.tags = make(map[string]int)
	}
	f.next++
	id := 1000 + f.next
	f.tags[name] = id
	return sb.InternalTag{ID: id, Name: name, ObjectType: objectType}, nil
}

func TestPrepareTagIDsForTarget_CreatesMissing(t *testing.T) {
	api := &fakeTagAPI{tags: map[string]int{"exists": 7}, next: 0}
	src := []sb.InternalTag{{Name: "exists"}, {Name: "newtag"}}
	ids, err := PrepareTagIDsForTarget(context.Background(), api, 1, src, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ids) != 2 {
		t.Fatalf("expected 2 ids, got %v", ids)
	}
	if ids[0] != 7 {
		t.Fatalf("first id should be existing (7), got %d", ids[0])
	}
	if ids[1] == 0 {
		t.Fatalf("second id should be created, got %d", ids[1])
	}
}

func TestPrepareTagIDsForTarget_ErrorOnCreate(t *testing.T) {
	api := &fakeTagAPI{tags: map[string]int{}, next: 0, failCreate: true}
	src := []sb.InternalTag{{Name: "x"}}
	_, err := PrepareTagIDsForTarget(context.Background(), api, 1, src, false)
	if err == nil {
		t.Fatal("expected error on create")
	}
}

func TestEnsureTagNameIDs_CreatesMissingAndMaps(t *testing.T) {
	api := &fakeTagAPI{tags: map[string]int{"exists": 7}, next: 0}
	names := []string{"exists", "newtag", "exists"} // duplicates should be ignored
	m, err := EnsureTagNameIDs(context.Background(), api, 1, names)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(m) != 2 {
		t.Fatalf("expected 2 entries, got %+v", m)
	}
	if m["exists"] != 7 {
		t.Fatalf("exists id=7, got %d", m["exists"])
	}
	if m["newtag"] == 0 {
		t.Fatalf("newtag should be created with non-zero id")
	}
}
