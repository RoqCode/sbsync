package ui

import (
	"github.com/charmbracelet/bubbles/textinput"
	"time"
)

// Component list state and controls
type compSortKey int

const (
	compSortUpdated compSortKey = iota
	compSortCreated
	compSortName
)

type CompSearchState struct {
	searching bool
	query     string
	input     textinput.Model
}

type CompListState struct {
	listIndex int
	selected  map[string]bool // key: component name
	collapsed map[string]bool // key: group name -> collapsed

	search     CompSearchState
	sortKey    compSortKey
	sortAsc    bool
	group      string    // empty => all
	dateCutoff time.Time // zero => off
	// input modes: "" | "search" | "date"
	inputMode string
	dateInput textinput.Model
}
