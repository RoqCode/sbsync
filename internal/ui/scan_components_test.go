package ui

import (
	"storyblok-sync/internal/sb"
	"testing"
)

func TestHandleCompScanResult_SetsModel_NoStateChange(t *testing.T) {
	m := InitialModel()
	m.state = stateScanning
	msg := compScanMsg{
		srcComps:  []sb.Component{{ID: 1, Name: "A"}},
		tgtComps:  []sb.Component{{ID: 2, Name: "B"}},
		srcGroups: []sb.ComponentGroup{{UUID: "u1", Name: "G1"}},
		tgtGroups: []sb.ComponentGroup{{UUID: "u2", Name: "G2"}},
	}
	m2, _ := m.handleCompScanResult(msg)
	if m2.state != stateScanning {
		t.Fatalf("expected state unchanged (scanning), got %v", m2.state)
	}
	if len(m2.componentsSource) != 1 || len(m2.componentsTarget) != 1 {
		t.Fatalf("unexpected components in model: %+v / %+v", m2.componentsSource, m2.componentsTarget)
	}
	if len(m2.componentGroupsSource) != 1 || len(m2.componentGroupsTarget) != 1 {
		t.Fatalf("unexpected groups in model: %+v / %+v", m2.componentGroupsSource, m2.componentGroupsTarget)
	}
}

func TestHandleCompScanResult_Error_NoStateChange(t *testing.T) {
	m := InitialModel()
	m.state = stateScanning
	m2, _ := m.handleCompScanResult(compScanMsg{err: ErrTest})
	if m2.state != stateScanning {
		t.Fatalf("expected state unchanged on error")
	}
}

// ErrTest is a sentinel used only in tests
var ErrTest = dummyErr{}

type dummyErr struct{}

func (dummyErr) Error() string { return "test" }
