package sync

import (
	"testing"
	"time"

	"storyblok-sync/internal/sb"
)

func TestNormalizeSlug(t *testing.T) {
	cases := map[string]string{
		"Hello World":  "hello-world",
		"äöü ß":        "aeoeue-ss",
		"  --- A  B  ": "a-b",
		"":             "copy",
		"Foo_bar+Baz":  "foo-bar-baz",
	}
	for in, want := range cases {
		got := NormalizeSlug(in)
		if got != want {
			t.Fatalf("NormalizeSlug(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestBuildSlugPresets(t *testing.T) {
	presets := BuildSlugPresets("article", time.Date(2024, 12, 31, 23, 59, 0, 0, time.UTC))
	if len(presets) != 4 {
		t.Fatalf("expected 4 presets, got %d", len(presets))
	}
	if presets[0] != "article-copy" {
		t.Fatalf("unexpected first preset: %s", presets[0])
	}
}

func TestEnsureUniqueSlugInFolder(t *testing.T) {
	existing := []sb.Story{
		{FullSlug: "parent/article"},
		{FullSlug: "parent/article-copy"},
		{FullSlug: "parent/other"},
		{FullSlug: "parent/deeper/nested"}, // should be ignored
	}
	got := EnsureUniqueSlugInFolder("parent", "article-copy", existing)
	if got != "article-copy-1" {
		t.Fatalf("got %q, want article-copy-1", got)
	}
	got = EnsureUniqueSlugInFolder("parent", "new", existing)
	if got != "new" {
		t.Fatalf("got %q, want new", got)
	}
}

func TestBuildTranslatedPathsForNewSlug(t *testing.T) {
	s := sb.Story{TranslatedSlugs: []sb.TranslatedSlug{{Lang: "de", Path: "de/parent/article"}, {Lang: "en", Path: "en/parent/article"}}}
	m := BuildTranslatedPathsForNewSlug(s, "article-copy")
	if m["de"] != "de/parent/article-copy" || m["en"] != "en/parent/article-copy" {
		t.Fatalf("unexpected translated paths: %+v", m)
	}
}
