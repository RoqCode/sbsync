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
)

// --- Model / State ---
type state int

const (
	stateWelcome state = iota
	stateTokenPrompt
	stateValidating
	stateQuit
)

type model struct {
	state         state
	cfg           config.Config
	hasSBRC       bool
	sbrcPath      string
	statusMsg     string
	width, height int

	// token input
	ti          textinput.Model
	validateErr error
	spacesCount int
}

func initialModel() model {
	p := config.DefaultPath()
	cfg, err := config.Load(p)
	hasFile := err == nil
	m := model{
		state: stateWelcome,
		cfg:   cfg, hasSBRC: hasFile, sbrcPath: p,
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
				} else {
					m.state = stateValidating
					return m, m.validateTokenCmd()
				}
			}

		case stateTokenPrompt:
			switch key {
			case "esc":
				m.state = stateWelcome
				return m, nil
			case "enter":
				m.cfg.Token = strings.TrimSpace(m.ti.Value())
				if m.cfg.Token == "" {
					m.statusMsg = "Token leer."
					return m, nil
				}
				m.state = stateValidating
				return m, m.validateTokenCmd()
			}

			// Delegiere an textinput
			var cmd tea.Cmd
			m.ti, cmd = m.ti.Update(msg)
			return m, cmd
		case stateValidating:
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

		m.spacesCount = msg.count
		m.statusMsg = fmt.Sprintf("Token ok. %d Spaces gefunden. (Enter für nächsten Schritt)", m.spacesCount)
		m.state = stateWelcome
		// Optional: config.Save(m.sbrcPath, m.cfg)
		return m, nil
	}
	return m, nil
}

// async cmd + msg
type validateMsg struct {
	count int
	err   error
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
		return validateMsg{count: len(spaces), err: nil}
	}
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

	switch m.state {
	case stateWelcome:
		b.WriteString("Willkommen! Diese App hilft dir, Stories zwischen Spaces zu synchronisieren.\n\n")
		if m.cfg.Token != "" {
			b.WriteString(okStyle.Render("✓ Token vorhanden\n"))
		} else {
			b.WriteString(warnStyle.Render("! Kein Token gefunden\n"))
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
	if _, err := tea.NewProgram(initialModel()).Run(); err != nil {
		fmt.Println("error:", err)
		os.Exit(1)
	}
}
