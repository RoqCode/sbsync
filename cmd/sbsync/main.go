package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"storyblok-sync/internal/ui"
)

func main() {
	if _, err := tea.NewProgram(
		ui.InitialModel(),
		tea.WithAltScreen(),
	).Run(); err != nil {
		fmt.Println("error:", err)
		os.Exit(1)
	}
}
