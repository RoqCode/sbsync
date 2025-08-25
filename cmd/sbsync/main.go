package main

import (
	"fmt"
	"io"
	"log"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"storyblok-sync/internal/ui"
)

func main() {
	// Configure logging based on DEBUG environment variable
	if len(os.Getenv("DEBUG")) > 0 {
		// Enable Bubble Tea debug logging to file
		f, err := tea.LogToFile("debug.log", "debug")
		if err != nil {
			fmt.Println("fatal:", err)
			os.Exit(1)
		}
		defer f.Close()

		// Redirect Go's standard logger to the same debug file
		debugFile, err := os.OpenFile("debug.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err != nil {
			fmt.Println("fatal: could not open debug log file:", err)
			os.Exit(1)
		}
		defer debugFile.Close()
		log.SetOutput(debugFile)

		fmt.Println("Debug logging enabled. Run 'tail -f debug.log' to view logs.")
	} else {
		// Disable all logging output to prevent TUI interference
		log.SetOutput(io.Discard)
	}

	if _, err := tea.NewProgram(
		ui.InitialModel(),
		tea.WithAltScreen(),
	).Run(); err != nil {
		fmt.Println("error:", err)
		os.Exit(1)
	}
}
