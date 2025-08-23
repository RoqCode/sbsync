package main

import (
	"fmt"
	"os"
	"storyblok-sync/internal/config"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// func main() {
// 	token, err := config.GetSbRc()
// 	if err != nil {
// 		fmt.Println(err)
// 		return
// 	}
// 	client := sb.NewStoryblokClient(token)
// 	client.ListSpaces()
// }

// --- UI Styles ---
var (
	titleStyle   = lipgloss.NewStyle().Bold(true).Underline(true)
	subtleStyle  = lipgloss.NewStyle().Faint(true)
	okStyle      = lipgloss.NewStyle().Bold(true)
	warnStyle    = lipgloss.NewStyle().Bold(true)
	helpStyle    = lipgloss.NewStyle().Faint(true)
	dividerStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
)

// --- Model / State ---
type state int

const (
	stateWelcome state = iota
	stateQuit
)

type model struct {
	state         state
	hasSBRC       bool
	sbrcPath      string
	statusMsg     string
	width, height int
}

func initialModel() model {
	p := config.TOKEN_PATH
	_, err := os.Stat(p)
	has := err == nil
	status := "Keine ~/.sbrc gefunden – du kannst gleich einen Token eingeben."
	if has {
		status = "Gefundene ~/.sbrc – wir können daraus lesen."
	}
	return model{
		state:     stateWelcome,
		hasSBRC:   has,
		sbrcPath:  p,
		statusMsg: status,
	}
}

func (m model) Init() tea.Cmd {
	return nil
}

// --- Update ---
type keyMsg struct{ rune rune } // simple wrapper, wir nutzen tea.KeyMsg direkt

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		key := strings.ToLower(msg.String())
		switch key {
		case "ctrl+c", "q":
			m.state = stateQuit
			return m, tea.Quit
		case "enter", " ":
			// Hier würden wir in den nächsten Screen wechseln (Token-Prompt/SpaceSelect)
			// Für v0: nur kleine Rückmeldung
			m.statusMsg = "Okay – nächster Schritt wäre Token-Input oder Space-Select…"
			return m, nil
		}
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	}
	return m, nil
}

// --- View ---
func (m model) View() string {
	if m.state == stateQuit {
		return ""
	}

	var b strings.Builder
	b.WriteString(titleStyle.Render("Storyblok Sync (TUI)"))
	b.WriteString("\n")
	b.WriteString(dividerStyle.Render(strings.Repeat("─", max(10, m.width-2))))
	b.WriteString("\n\n")

	b.WriteString("Willkommen! Diese App hilft dir, Stories zwischen Spaces zu synchronisieren.\n\n")

	if m.hasSBRC {
		b.WriteString(okStyle.Render("✓ "))
		b.WriteString(fmt.Sprintf("Konfiguration gefunden: %s\n", m.sbrcPath))
	} else {
		b.WriteString(warnStyle.Render("! "))
		b.WriteString("Keine Konfiguration gefunden (~/.sbrc)\n")
	}
	b.WriteString(subtleStyle.Render(m.statusMsg))
	b.WriteString("\n\n")

	b.WriteString(helpStyle.Render("Tasten: "))
	b.WriteString("⏎ weiter  |  q beenden")
	b.WriteString("\n")

	return b.String()
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// --- main ---
func main() {
	if _, err := tea.NewProgram(initialModel()).Run(); err != nil {
		fmt.Println("error:", err)
		os.Exit(1)
	}
}
