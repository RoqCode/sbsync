package ui

import (
    "fmt"
    "strings"
)

func (m Model) viewModePicker() string {
    title := listHeaderStyle.Render("Wähle Sync-Modus")
    var lines []string
    src := "(none)"
    tgt := "(none)"
    if m.sourceSpace != nil {
        src = fmt.Sprintf("%s (ID: %d)", m.sourceSpace.Name, m.sourceSpace.ID)
    }
    if m.targetSpace != nil {
        tgt = fmt.Sprintf("%s (ID: %d)", m.targetSpace.Name, m.targetSpace.ID)
    }
    lines = append(lines, subtitleStyle.Render("Quelle: ")+okStyle.Render(src))
    lines = append(lines, subtitleStyle.Render("Ziel:   ")+okStyle.Render(tgt))
    lines = append(lines, "")

    // Options
    options := []string{"Stories", "Components"}
    for i, opt := range options {
        marker := "  "
        if i == m.modePickerIndex {
            marker = "> "
        }
        lines = append(lines, spaceItemStyle.Render(marker+opt))
    }
    content := strings.Join(lines, "\n")
    help := renderFooter("", "⌨️  ↑↓/j/k: wählen  •  Enter: bestätigen  •  b/Esc: zurück  •  q: beenden")
    return title + "\n\n" + content + "\n\n" + help
}

