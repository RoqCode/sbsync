package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// handlePrefixFilterInput handles prefix filter input mode
func (m Model) handlePrefixFilterInput(msg tea.KeyMsg) (Model, tea.Cmd) {
	key := msg.String()

	switch key {
	case "esc":
		m.filter.prefixInput.Blur()
		if strings.TrimSpace(m.filter.prefixInput.Value()) == "" {
			m.filter.prefix = ""
		}
		m.filter.prefixing = false
		m.applyFilter()
		m.updateBrowseViewport()
		return m, nil
	case "enter":
		m.filter.prefix = strings.TrimSpace(m.filter.prefixInput.Value())
		m.filter.prefixing = false
		m.filter.prefixInput.Blur()
		m.applyFilter()
		m.updateBrowseViewport()
		return m, nil
	case "ctrl+c", "q":
		return m, tea.Quit
	default:
		var cmd tea.Cmd
		m.filter.prefixInput, cmd = m.filter.prefixInput.Update(msg)
		return m, cmd
	}
}

// handleSearchInput handles search input mode
func (m Model) handleSearchInput(msg tea.KeyMsg) (Model, tea.Cmd) {
	key := msg.String()

	switch key {
	case "esc":
		// ESC: wenn Query leer -> Suche schließen, sonst nur löschen
		current := strings.TrimSpace(m.search.searchInput.Value())
		if current == "" {
			m.search.searching = false
			m.search.query = ""
			m.search.searchInput.Blur()
			m.collapseAllFolders()
		} else {
			m.search.query = ""
			m.search.searchInput.SetValue("")
			m.collapseAllFolders()
			m.applyFilter()
		}
		m.updateBrowseViewport()
		return m, nil
	case "enter":
		m.search.query = strings.TrimSpace(m.search.searchInput.Value())
		m.search.searching = false
		m.search.searchInput.Blur()
		m.applyFilter()
		m.updateBrowseViewport()
		return m, nil
	case "ctrl+c", "q":
		return m, tea.Quit
	default:
		// Live search: update query as user types
		var cmd tea.Cmd
		m.search.searchInput, cmd = m.search.searchInput.Update(msg)
		// Update the query immediately for live filtering
		newQuery := strings.TrimSpace(m.search.searchInput.Value())
		if newQuery != m.search.query {
			// Expand folders when starting search, collapse when clearing
			if m.search.query == "" && newQuery != "" {
				m.expandAllFolders()
			}
			if m.search.query != "" && newQuery == "" {
				m.collapseAllFolders()
			}
			m.search.query = newQuery
			m.applyFilter()
			m.updateBrowseViewport()
		}
		return m, cmd
	}
}

// handleBrowseSearchAndFilterControls handles search and filter toggle keys
func (m Model) handleBrowseSearchAndFilterControls(key string) (Model, tea.Cmd) {
	switch key {
	case "f":
		// Toggle search
		if !m.search.searching {
			m.search.searching = true
			m.search.searchInput.Focus()
			return m, m.search.searchInput.Focus()
		} else {
			m.search.searching = false
			m.search.searchInput.Blur()
			return m, nil
		}
	case "F":
		// Clear search
		m.search.query = ""
		m.search.searchInput.SetValue("")
		m.collapseAllFolders()
		m.applyFilter()
		m.updateBrowseViewport()
		return m, nil

	case "p", "P":
		// Toggle prefix filter
		if !m.filter.prefixing {
			m.filter.prefixing = true
			m.filter.prefixInput.Focus()
			return m, m.filter.prefixInput.Focus()
		} else {
			m.filter.prefixing = false
			m.filter.prefixInput.Blur()
			return m, nil
		}

	case "c":
		// Clear all filters and selections
		m.search.query = ""
		m.filter.prefix = ""
		m.search.searchInput.SetValue("")
		m.filter.prefixInput.SetValue("")
		m.selection.selected = make(map[string]bool)
		m.collapseAllFolders()
		m.applyFilter()
		m.updateBrowseViewport()
		return m, nil
	}

	return m, nil
}