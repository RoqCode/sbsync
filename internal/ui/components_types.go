package ui

import (
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
	dateInput  string    // last entered string
}
