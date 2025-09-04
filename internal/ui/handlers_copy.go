package ui

import (
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	sync "storyblok-sync/internal/core/sync"
	"storyblok-sync/internal/sb"
)

// applyFolderFork rebases the selected folder subtree under a new folder slug.
// It mutates preflight items in-place, marks them CopyAsNew, enforces uniqueness, and re-optimizes.
func (m *Model) applyFolderFork(folderIdx int, newFolderSlug string, appendFolderCopy bool, appendChildStoryCopy bool) {
	if folderIdx < 0 || folderIdx >= len(m.preflight.items) {
		return
	}
	top := &m.preflight.items[folderIdx]
	if !top.Story.IsFolder {
		return
	}
	oldRoot := top.Story.FullSlug
	parent := parentSlug(oldRoot)
	newRoot := newFolderSlug
	if parent != "" {
		newRoot = parent + "/" + newFolderSlug
	}

	// Shadow occupancy per parent path for uniqueness checks
	shadow := make(map[string]map[string]bool) // parentFull -> set of leaf slugs
	// seed from target stories
	for _, st := range m.storiesTarget {
		p := sync.ParentSlug(st.FullSlug)
		leaf := st.Slug
		if shadow[p] == nil {
			shadow[p] = make(map[string]bool)
		}
		shadow[p][leaf] = true
	}

	// Helper: ensure unique under parent considering shadow
	ensureUnique := func(parentFull, desired string) string {
		// Build synthetic list from shadow for EnsureUniqueSlugInFolder
		synthetic := make([]sb.Story, 0, len(shadow[parentFull]))
		for leaf := range shadow[parentFull] {
			full := leaf
			if parentFull != "" {
				full = parentFull + "/" + leaf
			}
			synthetic = append(synthetic, sb.Story{FullSlug: full})
		}
		unique := sync.EnsureUniqueSlugInFolder(parentFull, desired, synthetic)
		if shadow[parentFull] == nil {
			shadow[parentFull] = make(map[string]bool)
		}
		shadow[parentFull][unique] = true
		return unique
	}

	// First, compute mapping of affected indices
	affected := make([]int, 0)
	for i := range m.preflight.items {
		it := m.preflight.items[i]
		if !it.Selected || it.Skip {
			continue
		}
		if it.Story.FullSlug == oldRoot || strings.HasPrefix(it.Story.FullSlug+"/", oldRoot+"/") {
			affected = append(affected, i)
		}
	}

	// Materialize intermediate folders if needed (ancestors within subtree)
	// Since preflight already includes ancestors via includeAncestors, we will operate on existing items.

	// Mutate top folder first
	{
		it := &m.preflight.items[folderIdx]
		// Unique top leaf under parent using requested newFolderSlug
		topLeaf := sync.NormalizeSlug(newFolderSlug)
		topLeaf = ensureUnique(parent, topLeaf)
		// Update story fields
		it.CopyAsNew = true
		it.NewSlug = topLeaf
		it.NewTranslatedPaths = sync.BuildTranslatedPathsForNewSlug(it.Story, topLeaf)
		it.AppendCopySuffixToName = appendFolderCopy
		if parent == "" {
			it.Story.FullSlug = topLeaf
		} else {
			it.Story.FullSlug = parent + "/" + topLeaf
		}
		it.Story.Slug = topLeaf
		if appendFolderCopy && it.Story.Name != "" && !strings.HasSuffix(it.Story.Name, " (copy)") {
			it.Story.Name = it.Story.Name + " (copy)"
		}
		it.Story.Published = false
		it.Story.UUID = ""
		if len(it.Story.TranslatedSlugs) > 0 {
			for i := range it.Story.TranslatedSlugs {
				p := it.Story.TranslatedSlugs[i].Path
				if p != "" {
					segs := strings.Split(p, "/")
					if len(segs) > 0 {
						segs[len(segs)-1] = topLeaf
						it.Story.TranslatedSlugs[i].Path = strings.Join(segs, "/")
					}
				}
				it.Story.TranslatedSlugs[i].ID = nil
			}
		}
		recalcState(it)
		// newRoot is the final FullSlug of top
		newRoot = it.Story.FullSlug
	}

	// Rebase descendants (excluding top, already handled)
	for _, i := range affected {
		if i == folderIdx {
			continue
		}
		it := &m.preflight.items[i]
		full := it.Story.FullSlug
		// Compute relative path from oldRoot
		rel := strings.TrimPrefix(full, oldRoot)
		rel = strings.TrimPrefix(rel, "/")
		// Split to parent and leaf
		parentRel := ""
		leaf := it.Story.Slug
		if rel != "" {
			parts := strings.Split(rel, "/")
			if len(parts) > 1 {
				parentRel = strings.Join(parts[:len(parts)-1], "/")
				leaf = parts[len(parts)-1]
			} else {
				leaf = parts[0]
			}
		}
		newParent := newRoot
		if parentRel != "" {
			newParent = newRoot + "/" + parentRel
		}
		leaf = sync.NormalizeSlug(leaf)
		uniqueLeaf := ensureUnique(newParent, leaf)

		// Update story
		it.CopyAsNew = true
		it.NewSlug = uniqueLeaf
		it.NewTranslatedPaths = sync.BuildTranslatedPathsForNewSlug(it.Story, uniqueLeaf)
		// Names
		if it.Story.IsFolder {
			if appendFolderCopy && it.Story.Name != "" && !strings.HasSuffix(it.Story.Name, " (copy)") {
				it.Story.Name = it.Story.Name + " (copy)"
			}
		} else if appendChildStoryCopy {
			if it.Story.Name != "" && !strings.HasSuffix(it.Story.Name, " (copy)") {
				it.Story.Name = it.Story.Name + " (copy)"
			}
		}
		// FullSlug / Slug
		if newParent == "" {
			it.Story.FullSlug = uniqueLeaf
		} else {
			it.Story.FullSlug = newParent + "/" + uniqueLeaf
		}
		it.Story.Slug = uniqueLeaf
		it.Story.Published = false
		it.Story.UUID = ""
		// Translated paths: replace last segment
		if len(it.Story.TranslatedSlugs) > 0 {
			for k := range it.Story.TranslatedSlugs {
				p := it.Story.TranslatedSlugs[k].Path
				if p != "" {
					segs := strings.Split(p, "/")
					if len(segs) > 0 {
						segs[len(segs)-1] = uniqueLeaf
						it.Story.TranslatedSlugs[k].Path = strings.Join(segs, "/")
					}
				}
				it.Story.TranslatedSlugs[k].ID = nil
			}
		}
		recalcState(it)
	}

	// Re-optimize list to ensure folders-first order and auto-add any missing parents
	m.optimizePreflight()
}

// handleCopyAsNewKey manages the full-screen copy-as-new flow.
func (m Model) handleCopyAsNewKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	key := msg.String()
	switch key {
	case "esc", "q":
		// Back to preflight without changes; refresh viewport to update any badges
		m.state = statePreflight
		m.updateViewportContent()
		return m, nil
	case "up", "k":
		if m.copy.selectedPreset > 0 {
			m.copy.selectedPreset--
			m.copy.input.SetValue(m.copy.presets[m.copy.selectedPreset])
		}
		return m, nil
	case "down", "j":
		if m.copy.selectedPreset < len(m.copy.presets)-1 {
			m.copy.selectedPreset++
			m.copy.input.SetValue(m.copy.presets[m.copy.selectedPreset])
		}
		return m, nil
	case "tab":
		// Toggle focus to input: ensure cursor visible
		m.copy.input.Focus()
		return m, nil
	case " ":
		// Toggle append to name
		m.copy.appendCopyToName = !m.copy.appendCopyToName
		return m, nil
	case "enter":
		// Validate and apply
		val := strings.TrimSpace(m.copy.input.Value())
		val = sync.NormalizeSlug(val)
		if val == "" {
			m.copy.errorMsg = "Slug darf nicht leer sein"
			return m, nil
		}
		// Ensure uniqueness using current target index
		unique := sync.EnsureUniqueSlugInFolder(m.copy.parent, val, m.storiesTarget)
		if unique != val {
			// Suggest the unique variant; require second enter to accept
			m.copy.input.SetValue(unique)
			m.copy.errorMsg = "Slug belegt – Vorschlag angepasst"
			return m, nil
		}

		// Apply to preflight item
		if m.copy.itemIdx >= 0 && m.copy.itemIdx < len(m.preflight.items) {
			it := &m.preflight.items[m.copy.itemIdx]
			it.CopyAsNew = true
			it.NewSlug = val
			it.NewTranslatedPaths = sync.BuildTranslatedPathsForNewSlug(it.Story, val)
			it.AppendCopySuffixToName = m.copy.appendCopyToName

			// Mutate story for create path
			// Update FullSlug and Slug
			if m.copy.parent == "" {
				it.Story.FullSlug = val
			} else {
				it.Story.FullSlug = m.copy.parent + "/" + val
			}
			it.Story.Slug = val
			// Name suffix
			if it.AppendCopySuffixToName && it.Story.Name != "" && !strings.HasSuffix(it.Story.Name, " (copy)") {
				it.Story.Name = it.Story.Name + " (copy)"
			}
			// Ensure draft
			it.Story.Published = false
			// Prevent UUID update behavior
			it.Story.UUID = ""
			// Update translated slugs paths in typed struct (IDs will be dropped in writer)
			if len(it.Story.TranslatedSlugs) > 0 {
				for i := range it.Story.TranslatedSlugs {
					p := it.Story.TranslatedSlugs[i].Path
					if p == "" {
						continue
					}
					segs := strings.Split(p, "/")
					if len(segs) == 0 {
						continue
					}
					segs[len(segs)-1] = val
					it.Story.TranslatedSlugs[i].Path = strings.Join(segs, "/")
					it.Story.TranslatedSlugs[i].ID = nil
				}
			}
			// Force state to create
			recalcState(it)
		}

		// Back to preflight
		m.state = statePreflight
		// Refresh viewport so the Fork badge appears immediately
		m.updateViewportContent()
		return m, nil
	}
	// Default: pass input message to textinput for editing
	var cmd tea.Cmd
	m.copy.input, cmd = m.copy.input.Update(msg)
	return m, cmd
}

// openCopyAsNewForIndex is a helper (unused externally yet) to initialize state.
func (m *Model) openCopyAsNewForIndex(idx int) {
	it := m.preflight.items[idx]
	m.copy.itemIdx = idx
	m.copy.parent = parentSlug(it.Story.FullSlug)
	m.copy.baseSlug = sync.NormalizeSlug(it.Story.Slug)
	m.copy.presets = sync.BuildSlugPresets(m.copy.baseSlug, time.Now())
	m.copy.selectedPreset = 0
	m.copy.input.SetValue(m.copy.presets[0])
	m.copy.input.CursorEnd()
	m.copy.appendCopyToName = false
	m.copy.errorMsg = ""
	m.state = stateCopyAsNew
}

// handleFolderForkKey manages the full-screen folder fork flow.
func (m Model) handleFolderForkKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	key := msg.String()
	switch key {
	case "esc", "q":
		m.state = statePreflight
		m.updateViewportContent()
		return m, nil
	case "up", "k":
		if m.folder.selectedPreset > 0 {
			m.folder.selectedPreset--
			m.folder.input.SetValue(m.folder.presets[m.folder.selectedPreset])
		}
		return m, nil
	case "down", "j":
		if m.folder.selectedPreset < len(m.folder.presets)-1 {
			m.folder.selectedPreset++
			m.folder.input.SetValue(m.folder.presets[m.folder.selectedPreset])
		}
		return m, nil
	case "tab":
		m.folder.input.Focus()
		return m, nil
	case " ":
		// Toggle first checkbox, then second with Shift+Space is not supported; cycle both using space
		if !m.folder.appendCopyToFolderName && !m.folder.appendCopyToChildStoryNames {
			m.folder.appendCopyToFolderName = true
		} else if m.folder.appendCopyToFolderName && !m.folder.appendCopyToChildStoryNames {
			m.folder.appendCopyToChildStoryNames = true
		} else {
			m.folder.appendCopyToFolderName = false
			m.folder.appendCopyToChildStoryNames = false
		}
		return m, nil
	case "enter":
		val := strings.TrimSpace(m.folder.input.Value())
		val = sync.NormalizeSlug(val)
		if val == "" {
			m.folder.errorMsg = "Slug darf nicht leer sein"
			return m, nil
		}
		unique := sync.EnsureUniqueSlugInFolder(m.folder.parent, val, m.storiesTarget)
		if unique != val {
			m.folder.input.SetValue(unique)
			m.folder.errorMsg = "Slug belegt – Vorschlag angepasst"
			return m, nil
		}
		// Apply subtree rebase and return to preflight
		if m.folder.itemIdx >= 0 && m.folder.itemIdx < len(m.preflight.items) {
			m.applyFolderFork(m.folder.itemIdx, val, m.folder.appendCopyToFolderName, m.folder.appendCopyToChildStoryNames)
		}
		// After mutating items and re-optimizing, rebuild visible order
		m.refreshPreflightVisible()
		m.state = statePreflight
		m.ensurePreflightCursorVisible()
		m.updateViewportContent()
		return m, nil
	}
	var cmd tea.Cmd
	m.folder.input, cmd = m.folder.input.Update(msg)
	return m, cmd
}
