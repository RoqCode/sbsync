package main

import (
	"context"
	"fmt"
	"os"
	"storyblok-sync/internal/config"
	"storyblok-sync/internal/sb"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// --- UI Styles ---
var (
	titleStyle   = lipgloss.NewStyle().Bold(true).Underline(true)
	subtleStyle  = lipgloss.NewStyle().Faint(true)
	okStyle      = lipgloss.NewStyle().Bold(true)
	warnStyle    = lipgloss.NewStyle().Bold(true)
	helpStyle    = lipgloss.NewStyle().Faint(true)
	dividerStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	focusStyle   = lipgloss.NewStyle().Bold(true)
)

// --- Model / State ---
type state int

const (
	stateWelcome state = iota
	stateTokenPrompt
	stateValidating
	stateSpaceSelect
	stateScanning
	stateQuit
)

type model struct {
	state         state
	cfg           config.Config
	hasSBRC       bool
	sbrcPath      string
	statusMsg     string
	validateErr   error
	width, height int

	// token input
	ti textinput.Model

	// spaces & selection
	spaces          []sb.Space
	selectedIndex   int
	selectingSource bool
	sourceSpace     *sb.Space
	targetSpace     *sb.Space
}

func initialModel() model {
	p := config.DefaultPath()
	cfg, err := config.Load(p)
	hasFile := err == nil

	m := model{
		state:     stateWelcome,
		cfg:       cfg,
		hasSBRC:   hasFile,
		sbrcPath:  p,
		statusMsg: "",
	}

	if cfg.Token == "" {
		m.statusMsg = "Keine ~/.sbrc oder kein Token – drück Enter für Token-Eingabe."
	} else {
		m.statusMsg = "Token gefunden – Enter zum Validieren, q zum Beenden."
	}

	// textinput
	ti := textinput.New()
	ti.Placeholder = "Storyblok Personal Access Token"
	ti.Focus()
	ti.EchoMode = textinput.EchoPassword
	ti.CharLimit = 200
	m.ti = ti

	return m
}

func (m model) Init() tea.Cmd {
	return nil
}

// --- Update ---
type keyMsg struct{ rune rune } // simple wrapper, wir nutzen tea.KeyMsg direkt

// ---------- Update ----------
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.KeyMsg:
		key := strings.ToLower(msg.String())

		switch m.state {

		case stateWelcome:
			switch key {
			case "ctrl+c", "q":
				return m, tea.Quit
			case "enter":
				if m.cfg.Token == "" {
					m.state = stateTokenPrompt
					m.statusMsg = "Bitte gib deinen Token ein."
				} else {
					m.state = stateValidating
					m.statusMsg = "Validiere Token…"
					return m, m.validateTokenCmd()
				}
			}

		case stateTokenPrompt:
			switch key {
			case "esc":
				m.state = stateWelcome
				m.statusMsg = "Zurück zum Welcome."
				return m, nil
			case "enter":
				m.cfg.Token = strings.TrimSpace(m.ti.Value())
				if m.cfg.Token == "" {
					m.statusMsg = "Token leer."
					return m, nil
				}
				m.state = stateValidating
				m.statusMsg = "Validiere Token…"
				return m, m.validateTokenCmd()
			default:
				var cmd tea.Cmd
				m.ti, cmd = m.ti.Update(msg)
				return m, cmd
			}

		case stateValidating:
			if key == "q" {
				return m, tea.Quit
			}

		case stateSpaceSelect:
			switch key {
			case "ctrl+c", "q":
				// zurück? Für MVP beenden wir lieber nicht – aber User erwartet "q" to quit:
				return m, tea.Quit
			case "j", "down":
				if m.selectedIndex < len(m.spaces)-1 {
					m.selectedIndex++
				}
			case "k", "up":
				if m.selectedIndex > 0 {
					m.selectedIndex--
				}
			case "enter":
				if len(m.spaces) == 0 {
					return m, nil
				}
				chosen := m.spaces[m.selectedIndex]
				if m.selectingSource {
					// Source wählen; Target-Auswahl vorbereiten
					m.sourceSpace = &chosen
					m.selectingSource = false
					// optional: Target nicht gleich Source erlauben?
					// wir lassen es erstmal zu; man kann später coden, dass source!=target sein muss.
					m.statusMsg = fmt.Sprintf("Source gesetzt: %s (%d). Wähle jetzt Target.", chosen.Name, chosen.ID)
					// Reset index für Target-Auswahl
					m.selectedIndex = 0
				} else {
					m.targetSpace = &chosen
					m.statusMsg = fmt.Sprintf("Target gesetzt: %s (%d). Scanne jetzt Stories…", chosen.Name, chosen.ID)
					m.state = stateScanning
					// hier später: Cmd für Scan starten
				}
			}

		case stateScanning:
			// Platzhalter – später starten wir hier den echten Scan und wechseln nach BrowseList.
			if key == "q" {
				return m, tea.Quit
			}
		}

	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height

	case validateMsg:
		if msg.err != nil {
			m.validateErr = msg.err
			m.statusMsg = "Validierung fehlgeschlagen: " + msg.err.Error()
			m.state = stateTokenPrompt
			return m, nil
		}
		// Token ok
		m.spaces = msg.spaces
		m.statusMsg = fmt.Sprintf("Token ok. %d Spaces gefunden.", len(m.spaces))
		m.state = stateSpaceSelect
		m.selectingSource = true
		m.selectedIndex = 0

		// Optional: direkt speichern
		// _ = config.Save(m.sbrcPath, m.cfg)
		return m, nil
	}

	return m, nil
}

// ---------- Messages / Cmds ----------
type validateMsg struct {
	spaces []sb.Space
	err    error
}

func (m model) validateTokenCmd() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		c := sb.New(m.cfg.Token)
		spaces, err := c.ListSpaces(ctx)
		if err != nil {
			return validateMsg{err: err}
		}
		return validateMsg{spaces: spaces, err: nil}
	}
}

// ---------- View ----------
func (m model) View() string {
	if m.state == stateQuit {
		return ""
	}

	var b strings.Builder
	b.WriteString(titleStyle.Render("Storyblok Sync (TUI)"))
	b.WriteString("\n")
	b.WriteString(dividerStyle.Render(strings.Repeat("─", max(10, m.width-2))))
	b.WriteString("\n\n")

	switch m.state {

	case stateWelcome:
		b.WriteString("Willkommen! Diese App hilft dir, Stories zwischen Spaces zu synchronisieren.\n\n")
		if m.cfg.Token != "" {
			b.WriteString(okStyle.Render("✓ Token vorhanden\n"))
		} else {
			b.WriteString(warnStyle.Render("! Kein Token gefunden (~/.sbrc oder SB_TOKEN)\n"))
		}
		if m.hasSBRC {
			b.WriteString(subtleStyle.Render(fmt.Sprintf("Konfiguration gefunden: %s\n", m.sbrcPath)))
		}
		b.WriteString(subtleStyle.Render(m.statusMsg) + "\n\n")
		b.WriteString(helpStyle.Render("Tasten: Enter weiter  |  q beenden"))

	case stateTokenPrompt:
		b.WriteString("Bitte gib deinen Storyblok Token ein:\n\n")
		b.WriteString(m.ti.View() + "\n\n")
		if m.validateErr != nil {
			b.WriteString(warnStyle.Render(m.validateErr.Error()) + "\n\n")
		}
		b.WriteString(helpStyle.Render("Enter bestätigen  |  Esc zurück"))

	case stateValidating:
		b.WriteString("Validiere Token…\n\n")
		b.WriteString(helpStyle.Render("q abbrechen"))

	case stateSpaceSelect:
		if m.selectingSource {
			b.WriteString("Spaces (wähle **Source**):\n\n")
		} else {
			b.WriteString("Spaces (wähle **Target**):\n\n")
			if m.sourceSpace != nil {
				b.WriteString(subtleStyle.Render(fmt.Sprintf("Source: %s (%d)\n\n", m.sourceSpace.Name, m.sourceSpace.ID)))
			}
		}
		if len(m.spaces) == 0 {
			b.WriteString(warnStyle.Render("Keine Spaces gefunden.\n"))
		} else {
			for i, sp := range m.spaces {
				cursor := "  "
				if i == m.selectedIndex {
					cursor = "> "
				}
				line := fmt.Sprintf("%s%s (id %d)", cursor, sp.Name, sp.ID)
				if i == m.selectedIndex {
					line = focusStyle.Render(line)
				}
				b.WriteString(line + "\n")
			}
		}
		b.WriteString("\n" + helpStyle.Render("j/k bewegen  |  Enter wählen  |  q beenden"))

	case stateScanning:
		src := "(none)"
		tgt := "(none)"
		if m.sourceSpace != nil {
			src = fmt.Sprintf("%s (%d)", m.sourceSpace.Name, m.sourceSpace.ID)
		}
		if m.targetSpace != nil {
			tgt = fmt.Sprintf("%s (%d)", m.targetSpace.Name, m.targetSpace.ID)
		}
		b.WriteString("Scanning…\n\n")
		b.WriteString(fmt.Sprintf("Source: %s\nTarget: %s\n\n", src, tgt))
		b.WriteString(subtleStyle.Render("Hier startet als nächstes der Stories-Scan…") + "\n\n")
		b.WriteString(helpStyle.Render("q beenden"))
	}

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
	if _, err := tea.NewProgram(
		initialModel(),
		tea.WithAltScreen(),
		// optional: tea.WithMouseCellMotion(),
	).Run(); err != nil {
		fmt.Println("error:", err)
		os.Exit(1)
	}
}
