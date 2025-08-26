package ui

import (
	"reflect"
	"testing"

	"github.com/sahilm/fuzzy"
	"storyblok-sync/internal/sb"
)

func TestFilterByPrefix(t *testing.T) {
	stories := []sb.Story{{FullSlug: "root/a"}, {FullSlug: "root/b"}, {FullSlug: "other"}}
	want := []int{0, 1}
	if got := filterByPrefix(stories, "root"); !reflect.DeepEqual(got, want) {
		t.Fatalf("prefix filter mismatch: want %v got %v", want, got)
	}
	want = []int{0, 1, 2}
	if got := filterByPrefix(stories, ""); !reflect.DeepEqual(got, want) {
		t.Fatalf("empty prefix mismatch: want %v got %v", want, got)
	}
}

func TestFilterBySubstring(t *testing.T) {
	base := []string{"hello world", "foo bar", "hello bar"}
	idx := []int{0, 1, 2}
	cfg := FilterConfig{MaxResults: 10}
	want := []int{0, 2}
	if got := filterBySubstring("hello", base, idx, cfg); !reflect.DeepEqual(got, want) {
		t.Fatalf("substring filter mismatch: want %v got %v", want, got)
	}
	cfg.MaxResults = 1
	want = []int{0}
	if got := filterBySubstring("hello", base, idx, cfg); !reflect.DeepEqual(got, want) {
		t.Fatalf("substring maxresults mismatch: want %v got %v", want, got)
	}
}

func TestFilterByFuzzyThresholds(t *testing.T) {
	base := []string{"abc", "axc", "ac"}
	idx := []int{0, 1, 2}
	cfg := FilterConfig{MinCoverage: 1, MaxSpread: 1, MaxResults: 10}
	want := []int{2}
	if got := filterByFuzzy("ac", base, idx, cfg); !reflect.DeepEqual(got, want) {
		t.Fatalf("fuzzy filter mismatch: want %v got %v", want, got)
	}
}

func TestFilterByFuzzyFallback(t *testing.T) {
	base := []string{"abcd", "abxd"}
	idx := []int{0, 1}
	cfg := FilterConfig{MinCoverage: 1, MaxSpread: 0, MaxResults: 1}
	got := filterByFuzzy("ad", base, idx, cfg)
	if len(got) != 1 {
		t.Fatalf("fuzzy fallback expected one result, got %v", got)
	}
	if got[0] != 0 && got[0] != 1 {
		t.Fatalf("fuzzy fallback returned unexpected index %v", got)
	}
}

func TestMatchCoverage(t *testing.T) {
	m := fuzzy.Match{MatchedIndexes: []int{0, 2}}
	if c := matchCoverage("abcd", m); c != 0.5 {
		t.Fatalf("coverage want 0.5 got %v", c)
	}
}

func TestMatchSpread(t *testing.T) {
	m := fuzzy.Match{MatchedIndexes: []int{1, 4}}
	if s := matchSpread(m); s != 3 {
		t.Fatalf("spread want 3 got %d", s)
	}
	if s := matchSpread(fuzzy.Match{}); s != 0 {
		t.Fatalf("empty spread want 0 got %d", s)
	}
}
