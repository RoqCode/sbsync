package sync

import (
	"context"
	"errors"
	"testing"

	"storyblok-sync/internal/sb"
)

type tokenAPIMock struct {
	keys []sb.APIKey
	err  error
}

func (m tokenAPIMock) ListSpaceAPIKeys(ctx context.Context, spaceID int) ([]sb.APIKey, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.keys, nil
}

func TestResolveCDAToken_PrefersPreview(t *testing.T) {
	api := tokenAPIMock{keys: []sb.APIKey{
		{Access: "public", Token: "pub"},
		{Access: "private", Token: "prev"},
	}}
	info, err := ResolveCDAToken(context.Background(), api, 1)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !info.Available || info.Selected != "prev" || info.Kind != "preview" {
		t.Fatalf("want preview selected, got %+v", info)
	}
}

func TestResolveCDAToken_FallbackPublic(t *testing.T) {
	api := tokenAPIMock{keys: []sb.APIKey{{Access: "public", Token: "pub"}}}
	info, err := ResolveCDAToken(context.Background(), api, 1)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !info.Available || info.Selected != "pub" || info.Kind != "public" {
		t.Fatalf("want public selected, got %+v", info)
	}
}

func TestResolveCDAToken_NoKeysOrDenied(t *testing.T) {
	// denied/error => non-fatal, Available=false
	api := tokenAPIMock{err: errors.New("403 forbidden")}
	info, err := ResolveCDAToken(context.Background(), api, 1)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if info.Available || info.Selected != "" || info.Kind != "" {
		t.Fatalf("want unavailable on error, got %+v", info)
	}

	// no keys
	api2 := tokenAPIMock{keys: []sb.APIKey{}}
	info2, err := ResolveCDAToken(context.Background(), api2, 1)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if info2.Available {
		t.Fatalf("want unavailable for no keys, got %+v", info2)
	}
}
