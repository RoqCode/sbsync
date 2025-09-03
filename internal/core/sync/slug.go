package sync

import (
	"regexp"
	"sort"
	"strings"
	"time"

	"storyblok-sync/internal/sb"
)

// NormalizeSlug converts an arbitrary string to a kebab-case ASCII slug.
// - Lowercases
// - Replaces German umlauts and ß
// - Replaces non-alphanumeric with single dashes
// - Trims leading/trailing dashes
func NormalizeSlug(input string) string {
	if input == "" {
		return "copy"
	}
	// Basic transliteration for common German characters
	replacer := strings.NewReplacer(
		"Ä", "Ae", "Ö", "Oe", "Ü", "Ue",
		"ä", "ae", "ö", "oe", "ü", "ue",
		"ß", "ss",
	)
	s := replacer.Replace(input)
	s = strings.ToLower(s)
	// Replace any run of non a-z0-9 with a single dash
	reNonAlnum := regexp.MustCompile(`[^a-z0-9]+`)
	s = reNonAlnum.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if s == "" {
		return "copy"
	}
	return s
}

// BuildSlugPresets returns suggested new slug suffix variants.
func BuildSlugPresets(baseSlug string, now time.Time) []string {
	ts := now.Format("20060102-1504") // yyyyMMdd-HHmm
	return []string{
		baseSlug + "-copy",
		baseSlug + "-v2",
		baseSlug + "-new",
		baseSlug + "-" + ts,
	}
}

// EnsureUniqueSlugInFolder ensures that parent/newSlug does not collide with existing stories or folders in target.
// It returns newSlug or newSlug-1, -2, ... until unique within that parent path.
func EnsureUniqueSlugInFolder(parent string, newSlug string, existing []sb.Story) string {
	// Build a set of occupied full slugs under the same parent (direct children only)
	occupied := make(map[string]bool)
	prefix := parent
	if prefix != "" {
		prefix += "/"
	}
	for _, st := range existing {
		if !strings.HasPrefix(st.FullSlug, prefix) {
			continue
		}
		rest := strings.TrimPrefix(st.FullSlug, prefix)
		if strings.Contains(rest, "/") {
			continue // not a direct child
		}
		occupied[st.FullSlug] = true
	}

	full := func(slug string) string { return prefix + slug }
	candidate := newSlug
	if !occupied[full(candidate)] {
		return candidate
	}
	for i := 1; i < 10000; i++ {
		c := newSlug + "-" + itoa(i)
		if !occupied[full(c)] {
			return c
		}
	}
	return newSlug + "-x"
}

// BuildTranslatedPathsForNewSlug replaces the last segment of each translated path with newSlug.
// Returns a map lang -> new full path.
func BuildTranslatedPathsForNewSlug(source sb.Story, newSlug string) map[string]string {
	if len(source.TranslatedSlugs) == 0 {
		return nil
	}
	out := make(map[string]string, len(source.TranslatedSlugs))
	for _, ts := range source.TranslatedSlugs {
		path := ts.Path
		if path == "" {
			continue
		}
		segs := strings.Split(path, "/")
		if len(segs) == 0 {
			continue
		}
		segs[len(segs)-1] = newSlug
		out[ts.Lang] = strings.Join(segs, "/")
	}
	return out
}

// ParentFromFull returns the immediate parent full slug (without last segment), or "" for root.
func ParentFromFull(full string) string { return ParentSlug(full) }

// itoa converts a positive integer to string.
func itoa(n int) string {
	if n >= 0 && n < 10 {
		return string('0' + byte(n))
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}

// SortStoriesByFullSlug returns a new slice sorted by FullSlug for deterministic behavior.
func SortStoriesByFullSlug(in []sb.Story) []sb.Story {
	out := append([]sb.Story(nil), in...)
	sort.Slice(out, func(i, j int) bool { return out[i].FullSlug < out[j].FullSlug })
	return out
}
