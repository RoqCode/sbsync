package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"storyblok-sync/internal/ui"
)

func main() {
	// Enable debug logging when DEBUG environment variable is set
	if len(os.Getenv("DEBUG")) > 0 {
		f, err := tea.LogToFile("debug.log", "debug")
		if err != nil {
			fmt.Println("fatal:", err)
			os.Exit(1)
		}
		defer f.Close()
		fmt.Println("Debug logging enabled. Run 'tail -f debug.log' to view logs.")
	}

	if _, err := tea.NewProgram(
		ui.InitialModel(),
		tea.WithAltScreen(),
	).Run(); err != nil {
		fmt.Println("error:", err)
		os.Exit(1)
	}
}
