package ui

import (
	"testing"

	"storyblok-sync/internal/sb"
)

func TestSortStoriesByTypeAndName(t *testing.T) {
	stories := []sb.Story{
		{ID: 1, Name: "storyB"},
		{ID: 2, Name: "folderB", IsFolder: true},
		{ID: 3, Name: "rootA", IsStartpage: true},
		{ID: 4, Name: "storyA"},
		{ID: 5, Name: "folderA", IsFolder: true},
		{ID: 6, Name: "rootB", IsStartpage: true},
	}

	sortStories(stories)

	want := []int{5, 2, 3, 6, 4, 1}
	for i, st := range stories {
		if st.ID != want[i] {
			t.Fatalf("at %d want %d got %d", i, want[i], st.ID)
		}
	}
}
