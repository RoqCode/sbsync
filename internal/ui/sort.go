package ui

import (
	"sort"
	"strings"

	"storyblok-sync/internal/sb"
)

// sortStories orders the slice so that folders come first, followed by root stories
// (startpages) and finally regular stories. Items of the same type are sorted by
// their display name in a case-insensitive manner.
func sortStories(stories []sb.Story) {
	sort.SliceStable(stories, func(i, j int) bool {
		a, b := stories[i], stories[j]
		pa, pb := sortPriority(a), sortPriority(b)
		if pa != pb {
			return pa < pb
		}
		na := strings.ToLower(displayName(a))
		nb := strings.ToLower(displayName(b))
		return na < nb
	})
}

func sortPriority(st sb.Story) int {
	switch {
	case st.IsFolder:
		return 0
	case st.IsStartpage:
		return 1
	default:
		return 2
	}
}

func displayName(st sb.Story) string {
	if st.Name != "" {
		return st.Name
	}
	return st.Slug
}
