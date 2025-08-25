package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"

	tea "github.com/charmbracelet/bubbletea"

	"storyblok-sync/internal/ui"
)

func main() {
	// Clean up old log files before starting
	cleanupOldLogFiles()

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

// cleanupOldLogFiles removes old debug log files to prevent disk space accumulation
func cleanupOldLogFiles() {
	// Remove old debug.log files if they get too large (>10MB)
	if stat, err := os.Stat("debug.log"); err == nil {
		if stat.Size() > 10*1024*1024 { // 10MB limit
			// Rotate the log file by renaming it with timestamp
			if err := os.Rename("debug.log", fmt.Sprintf("debug.log.%d", stat.ModTime().Unix())); err == nil {
				// Clean up very old rotated log files (keep only last 3)
				if files, err := filepath.Glob("debug.log.*"); err == nil && len(files) > 3 {
					// Sort files to get proper ordering
					sort.Strings(files)
					// Remove oldest files, keeping only the 3 most recent
					for i := 0; i < len(files)-3; i++ {
						os.Remove(files[i])
					}
				}
			}
		}
	}
}
