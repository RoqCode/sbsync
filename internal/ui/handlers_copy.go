package ui

import (
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	sync "storyblok-sync/internal/core/sync"
)

// handleCopyAsNewKey manages the full-screen copy-as-new flow.
func (m Model) handleCopyAsNewKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	key := msg.String()
	switch key {
	case "esc", "q":
		// Back to preflight without changes
		m.state = statePreflight
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
