package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Preflight is rendered via viewport header/content/footer.

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
	// Add a small badge for copy-as-new
	badge := ""
	if it.CopyAsNew {
		badge = " " + helpStyle.Render("[Fork]")
	}
	// Publish mode badge (stories only)
	if !it.Story.IsFolder {
		// safe access via model is not available here; badges are appended in render
	}
	sym := storyTypeSymbol(it.Story)
	return fmt.Sprintf("%s %s%s  %s", sym, name, badge, slug)
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

	// Build stories slice in preflight visible order (shared helper)
	stories, order := m.visibleOrderPreflight()
	lines := generateTreeLinesFromStories(stories)
	for visPos, idx := range order {
		if visPos >= len(lines) {
			break
		}
		it := m.preflight.items[idx]
		content := lines[visPos]
		if it.Collision {
			content = collisionSign + " " + content
		} else {
			content = "  " + content
		}
		lineStyle := lipgloss.NewStyle().Width(m.width - 4)
		if visPos == m.preflight.listIndex {
			lineStyle = cursorLineStyle.Width(m.width - 4)
		}
		if strings.ToLower(it.State) == StateSkip {
			lineStyle = lineStyle.Faint(true)
		}
		// Append badges: Fork, Publish mode, Dev
		var badges []string
		if it.CopyAsNew {
			badges = append(badges, "[Fork]")
		}
		if !it.Story.IsFolder {
			mode := m.getPublishMode(it.Story.FullSlug)
			switch mode {
			case PublishModePublish:
				badges = append(badges, "[Pub]")
			case PublishModePublishChanges:
				badges = append(badges, "[Pub+∆]")
			default:
				badges = append(badges, "[Draft]")
			}
			if m.targetSpace != nil && m.targetSpace.PlanLevel == 999 {
				badges = append(badges, "[Dev]")
			}
		}
		if len(badges) > 0 {
			content += " " + helpStyle.Render(strings.Join(badges, ""))
		}
		content = lineStyle.Render(content)
		cursorCell := " "
		if visPos == m.preflight.listIndex {
			cursorCell = cursorBarStyle.Render(" ")
		}
		stateCell := " "
		switch it.Run {
		case RunRunning:
			stateCell = m.spinner.View()
		case RunDone:
			stateCell = stateDoneStyle.Render(stateLabel(it.State))
		case RunCancelled:
			stateCell = lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Background(lipgloss.Color("0")).Bold(true).Render("X")
		default:
			if it.State != "" {
				if st, ok := stateStyles[strings.ToLower(it.State)]; ok {
					stateCell = st.Render(stateLabel(it.State))
				} else {
					stateCell = it.State
				}
			}
		}
		lines[visPos] = cursorCell + stateCell + content
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
		helpText = "j/k bewegen  |  f Fork  |  F Quick-Fork  |  p Publish/Draft/Pub+∆  |  P auf Geschwister/Unterordner anwenden  |  x skip  |  X alle skippen  |  c Skips entfernen  |  Enter OK  |  esc/q zurück"
	}

	return renderFooter(statusLine, helpText)
}

// viewFolderFork renders the full-screen folder fork UI
func (m Model) viewFolderFork() string {
	var item PreflightItem
	if m.folder.itemIdx >= 0 && m.folder.itemIdx < len(m.preflight.items) {
		item = m.preflight.items[m.folder.itemIdx]
	}

	title := titleStyle.Render("🍴 Ordner-Fork vorbereiten")
	subtitle := subtitleStyle.Render("Ordnerbaum unter neuem Slug kopieren")

	// Live preview values
	baseName := item.Story.Name
	if baseName == "" {
		baseName = item.Story.Slug
	}
	nameLine := subtleStyle.Render("Name: ") + okStyle.Render(baseName)
	if m.folder.appendCopyToFolderName && baseName != "" && !strings.HasSuffix(baseName, " (copy)") {
		nameLine += whiteTextStyle.Render(" (copy)")
	}

	selected := strings.TrimSpace(m.folder.input.Value())
	var fullSelected string
	if m.folder.parent != "" {
		fullSelected = m.folder.parent + "/" + selected
	} else {
		fullSelected = selected
	}
	slugLine := subtleStyle.Render("Slug: ") + okStyle.Render(item.Story.Slug)
	if selected != "" {
		slugLine += whiteTextStyle.Render(" → " + fullSelected)
	}

	// Compute subtree counts for preview (selected, not skipped, under this folder)
	storiesCount := 0
	foldersCount := 0
	oldRoot := item.Story.FullSlug
	for _, it := range m.preflight.items {
		if !it.Selected || it.Skip {
			continue
		}
		if strings.HasPrefix(it.Story.FullSlug+"/", oldRoot+"/") || it.Story.FullSlug == oldRoot {
			if it.Story.IsFolder {
				foldersCount++
			} else {
				storiesCount++
			}
		}
	}
	sumLine := subtleStyle.Render("Zusammenfassung: ") + whiteTextStyle.Render(
		fmt.Sprintf("Forkt %d Stories, %d Ordner", storiesCount, foldersCount),
	)

	var lines []string
	lines = append(lines, nameLine)
	lines = append(lines, slugLine)
	lines = append(lines, "")
	lines = append(lines, "Vorschläge:")
	for i, p := range m.folder.presets {
		marker := "  "
		if i == m.folder.selectedPreset {
			marker = "> "
		}
		lines = append(lines, spaceItemStyle.Render(marker+p))
	}
	lines = append(lines, "")
	lines = append(lines, "Neuer Ordner-Slug:")
	lines = append(lines, m.folder.input.View())
	lines = append(lines, "")
	// Checkboxes
	chk1 := checkboxOff
	if m.folder.appendCopyToFolderName {
		chk1 = checkboxOn
	}
	lines = append(lines, chk1+" "+"Ordnernamen um \" (copy)\" ergänzen")
	chk2 := checkboxOff
	if m.folder.appendCopyToChildStoryNames {
		chk2 = checkboxOn
	}
	lines = append(lines, chk2+" "+"Story-Namen der Kinder um \" (copy)\" ergänzen")
	if m.folder.errorMsg != "" {
		lines = append(lines, "")
		lines = append(lines, errorStyle.Render("❌ "+m.folder.errorMsg))
	}

	content := title + "\n" + subtitle + "\n\n" + strings.Join(lines, "\n") + "\n\n" + sumLine
	box := welcomeBoxStyle.Render(content)
	help := renderFooter("", "⌨️  ↑↓: Preset  •  Tab: Feld  •  Space: Checkboxen  •  Enter: übernehmen  •  Esc: zurück")
	return centeredStyle.Width(m.width).Render(box) + "\n\n" + centeredStyle.Width(m.width).Render(help)
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
