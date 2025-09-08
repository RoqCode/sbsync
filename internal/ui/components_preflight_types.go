package ui

import (
	"github.com/charmbracelet/bubbles/textinput"
	"storyblok-sync/internal/sb"
)

// Components Preflight types
type CompPreflightItem struct {
	Source    sb.Component
	TargetID  int
	Collision bool
	Selected  bool
	Skip      bool
	State     string // StateCreate, StateUpdate, StateSkip
	Run       string // RunPending, RunRunning, RunDone/RunCancelled
	// Fork (copy-as-new) support
	CopyAsNew bool
	ForkName  string
	Issue     string
}

type CompPreflightState struct {
	items     []CompPreflightItem
	listIndex int
	// inline rename input when Fork is selected
	input    textinput.Model
	renaming bool
}
