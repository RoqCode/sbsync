package ui

import (
	"strings"

	"github.com/sahilm/fuzzy"
	"storyblok-sync/internal/sb"
)

// FilterConfig bundles tuning parameters for filtering and search operations.
type FilterConfig struct {
	MinCoverage float64 // minimal share of the query that must match
	MaxSpread   int     // maximal distance between first and last match index
	MaxResults  int     // upper limit of returned results
}

// filterByPrefix returns indices of stories whose full slug begins with the
// provided prefix. If the prefix is empty, indices of all stories are returned.
func filterByPrefix(stories []sb.Story, prefix string) []int {
	idx := make([]int, 0, len(stories))
	if prefix != "" {
		// Include items that start with the prefix (matches),
		// and also include ancestors of the prefix so the tree can render.
		for i, st := range stories {
			slug := strings.ToLower(st.FullSlug)
			if strings.HasPrefix(slug, prefix) || strings.HasPrefix(prefix, slug+"/") {
				idx = append(idx, i)
			}
		}
	} else {
		for i := range stories {
			idx = append(idx, i)
		}
	}
	return idx
}

// filterBySubstring performs a simple substring check against the prepared base
// list and returns matching indices limited by cfg.MaxResults.
func filterBySubstring(q string, base []string, idx []int, cfg FilterConfig) []int {
	sub := make([]int, 0, min(cfg.MaxResults, len(idx)))
	for _, i := range idx {
		if strings.Contains(base[i], q) {
			sub = append(sub, i)
			if len(sub) >= cfg.MaxResults {
				break
			}
		}
	}
	return sub
}

// filterByFuzzy applies fuzzy matching on the subset defined by idx and
// filters results based on coverage and spread thresholds from cfg.
func filterByFuzzy(q string, base []string, idx []int, cfg FilterConfig) []int {
	subset := make([]string, len(idx))
	mapBack := make([]int, len(idx))
	for j, i := range idx {
		subset[j] = base[i]
		mapBack[j] = i
	}
	matches := fuzzy.Find(q, subset)

	pruned := make([]int, 0, len(matches))
	for _, mt := range matches {
		if matchCoverage(q, mt) < cfg.MinCoverage {
			continue
		}
		if matchSpread(mt) > cfg.MaxSpread {
			continue
		}
		pruned = append(pruned, mapBack[mt.Index])
		if len(pruned) >= cfg.MaxResults {
			break
		}
	}
	if len(pruned) == 0 {
		for i := 0; i < len(matches) && i < cfg.MaxResults; i++ {
			pruned = append(pruned, mapBack[matches[i].Index])
		}
	}
	return pruned
}

// matchCoverage returns the ratio of matched characters to the query length.
func matchCoverage(q string, m fuzzy.Match) float64 {
	if len(q) == 0 {
		return 1
	}
	return float64(len(m.MatchedIndexes)) / float64(len(q))
}

// matchSpread returns the distance between the first and last matched index.
func matchSpread(m fuzzy.Match) int {
	if len(m.MatchedIndexes) == 0 {
		return 0
	}
	return m.MatchedIndexes[len(m.MatchedIndexes)-1] - m.MatchedIndexes[0]
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
