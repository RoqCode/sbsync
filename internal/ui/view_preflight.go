package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	tree "github.com/charmbracelet/lipgloss/tree"
)

func (m Model) viewPreflight() string {
	var b strings.Builder
	total := len(m.preflight.items)
	collisions := 0
	for _, it := range m.preflight.items {
		if it.Collision {
			collisions++
		}
	}
	b.WriteString(fmt.Sprintf("Preflight – %d Items  |  Kollisionen: %d\n\n", total, collisions))
	if total == 0 {
		b.WriteString(warnStyle.Render("Keine Stories markiert.") + "\n")
	} else {
		tr := tree.New()
		nodes := make(map[int]*tree.Tree, len(m.preflight.items))
		for _, it := range m.preflight.items {
			node := tree.Root(displayPreflightItem(it))
			nodes[it.Story.ID] = node
		}
		for _, it := range m.preflight.items {
			node := nodes[it.Story.ID]
			if it.Story.FolderID != nil {
				if parent, ok := nodes[*it.Story.FolderID]; ok {
					parent.Child(node)
					continue
				}
			}
			tr.Child(node)
		}
		lines := strings.Split(tr.String(), "\n")
		if len(lines) > 0 && lines[len(lines)-1] == "" {
			lines = lines[:len(lines)-1]
		}
		for i, it := range m.preflight.items {
			if i >= len(lines) {
				break
			}
			content := lines[i]
			if it.Collision {
				content = collisionSign + " " + content
			} else {
				content = "  " + content
			}
			lineStyle := lipgloss.NewStyle().Width(m.width - 2)
			if i == m.preflight.listIndex {
				lineStyle = cursorLineStyle.Copy().Width(m.width - 2)
			}
			if it.State == StateSkip {
				lineStyle = lineStyle.Faint(true)
			}
			content = lineStyle.Render(content)
			cursorCell := " "
			if i == m.preflight.listIndex {
				cursorCell = cursorBarStyle.Render(" ")
			}
			stateCell := " "
			switch it.Run {
			case RunRunning:
				stateCell = m.spinner.View()
			case RunDone:
				stateCell = stateDoneStyle.Render(string(it.State))
			case RunCancelled:
				stateCell = lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Background(lipgloss.Color("0")).Bold(true).Render("X")
			default:
				if it.State != "" {
					if st, ok := stateStyles[it.State]; ok {
						stateCell = st.Render(string(it.State))
					} else {
						stateCell = string(it.State)
					}
				}
			}
			lines[i] = cursorCell + stateCell + content
		}
		start := m.preflight.listOffset
		if start > len(lines) {
			start = len(lines)
		}
		end := start + m.preflight.listViewport
		if end > len(lines) {
			end = len(lines)
		}
		b.WriteString(strings.Join(lines[start:end], "\n"))
		b.WriteString("\n")
	}
	b.WriteString("\n")
	if m.syncing {
		b.WriteString(renderProgress(m.syncIndex, len(m.preflight.items), m.width-2))
		b.WriteString("\n\n")
	}
	if m.syncing {
		b.WriteString(helpStyle.Render("Syncing... | Ctrl+C to cancel"))
	} else {
		b.WriteString(helpStyle.Render("j/k bewegen  |  x skip  |  X alle skippen  |  c Skips entfernen  |  Enter OK  |  esc/q zurück"))
	}
	return b.String()
}

func displayPreflightItem(it PreflightItem) string {
	name := it.Story.Name
	if name == "" {
		name = it.Story.Slug
	}
	slug := "(" + it.Story.FullSlug + ")"
	if !it.Selected || it.State == StateSkip {
		name = subtleStyle.Render(name)
		slug = subtleStyle.Render(slug)
	}
	sym := storyTypeSymbol(it.Story)
	return fmt.Sprintf("%s %s  %s", sym, name, slug)
}

func (m *Model) updatePreflightViewport() {
	content := m.renderPreflightContent()
	m.viewport.SetContent(content)
}

func (m Model) renderPreflightHeader() string {
	total := len(m.preflight.items)
	collisions := 0
	for _, it := range m.preflight.items {
		if it.Collision {
			collisions++
		}
	}
	return fmt.Sprintf("Preflight – %d Items  |  Kollisionen: %d", total, collisions)
}

func (m Model) renderPreflightContent() string {
	var b strings.Builder
	total := len(m.preflight.items)

	if total == 0 {
		b.WriteString(warnStyle.Render("Keine Stories markiert.") + "\n")
		return b.String()
	}

	tr := tree.New()
	nodes := make(map[int]*tree.Tree, len(m.preflight.items))
	for _, it := range m.preflight.items {
		node := tree.Root(displayPreflightItem(it))
		nodes[it.Story.ID] = node
	}
	for _, it := range m.preflight.items {
		node := nodes[it.Story.ID]
		if it.Story.FolderID != nil {
			if parent, ok := nodes[*it.Story.FolderID]; ok {
				parent.Child(node)
				continue
			}
		}
		tr.Child(node)
	}
	lines := strings.Split(tr.String(), "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	for i, it := range m.preflight.items {
		if i >= len(lines) {
			break
		}
		content := lines[i]
		if it.Collision {
			content = collisionSign + " " + content
		} else {
			content = "  " + content
		}
		lineStyle := lipgloss.NewStyle().Width(m.width - 4)
		if i == m.preflight.listIndex {
			lineStyle = cursorLineStyle.Copy().Width(m.width - 4)
		}
		if it.State == StateSkip {
			lineStyle = lineStyle.Faint(true)
		}
		content = lineStyle.Render(content)
		cursorCell := " "
		if i == m.preflight.listIndex {
			cursorCell = cursorBarStyle.Render(" ")
		}
		stateCell := " "
		switch it.Run {
		case RunRunning:
			stateCell = m.spinner.View()
		case RunDone:
			stateCell = stateDoneStyle.Render(string(it.State))
		case RunCancelled:
			stateCell = lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Background(lipgloss.Color("0")).Bold(true).Render("X")
		default:
			if it.State != "" {
				if st, ok := stateStyles[it.State]; ok {
					stateCell = st.Render(string(it.State))
				} else {
					stateCell = string(it.State)
				}
			}
		}
		lines[i] = cursorCell + stateCell + content
	}
	b.WriteString(strings.Join(lines, "\n"))
	return b.String()
}

func (m Model) renderPreflightFooter() string {
	var statusLine string
	if m.syncing {
		statusLine = renderProgress(m.syncIndex, len(m.preflight.items), m.width-2)
	}

	var helpText string
	if m.syncing {
		helpText = "Syncing... | Ctrl+C to cancel"
	} else {
		helpText = "j/k bewegen  |  x skip  |  X alle skippen  |  c Skips entfernen  |  Enter OK  |  esc/q zurück"
	}

	return renderFooter(statusLine, helpText)
}

func renderProgress(done, total, width int) string {
	if total <= 0 {
		return ""
	}
	if width <= 0 {
		width = 20
	}
	filled := int(float64(done) / float64(total) * float64(width))
	if filled > width {
		filled = width
	}
	return "[" + strings.Repeat("#", filled) + strings.Repeat("-", width-filled) + fmt.Sprintf("] %d/%d", done, total)
}
