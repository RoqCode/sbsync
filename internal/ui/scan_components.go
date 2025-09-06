package ui

import (
    "context"
    "fmt"
    "time"
    "storyblok-sync/internal/infra/logx"
    "storyblok-sync/internal/sb"

    tea "github.com/charmbracelet/bubbletea"
)

// compScanMsg carries results of a components scan
type compScanMsg struct {
	srcComps  []sb.Component
	tgtComps  []sb.Component
	srcGroups []sb.ComponentGroup
	tgtGroups []sb.ComponentGroup
	err       error
}

func (m Model) scanComponentsCmd() tea.Cmd {
    token := m.cfg.Token
    return func() tea.Msg {
        if m.api == nil {
            m.api = sb.New(token)
        }
        // Use a bounded context similar to story scanning
        ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
        defer cancel()
        srcID, tgtID := 0, 0
        if m.sourceSpace != nil {
            srcID = m.sourceSpace.ID
        }
        if m.targetSpace != nil {
            tgtID = m.targetSpace.ID
        }
        logx.Infof("COMP_SCAN start src=%d tgt=%d", srcID, tgtID)

        // Load groups first
        srcGroups, err := m.api.ListComponentGroups(ctx, srcID)
        if err != nil {
            logx.Errorf("COMP_SCAN groups source error: %v", err)
            return compScanMsg{err: err}
        }
        tgtGroups, err := m.api.ListComponentGroups(ctx, tgtID)
        if err != nil {
            logx.Errorf("COMP_SCAN groups target error: %v", err)
            return compScanMsg{err: err}
        }

        logx.Debugf("COMP_SCAN groups loaded src=%d tgt=%d", len(srcGroups), len(tgtGroups))
        srcComps, err := m.api.ListComponents(ctx, srcID)
        if err != nil {
            logx.Errorf("COMP_SCAN components source error: %v", err)
            return compScanMsg{err: err}
        }
        tgtComps, err := m.api.ListComponents(ctx, tgtID)
        if err != nil {
            logx.Errorf("COMP_SCAN components target error: %v", err)
            return compScanMsg{err: err}
        }
        logx.Infof("COMP_SCAN done src comps=%d tgt comps=%d", len(srcComps), len(tgtComps))
        return compScanMsg{srcComps: srcComps, tgtComps: tgtComps, srcGroups: srcGroups, tgtGroups: tgtGroups}
    }
}

func (m Model) handleCompScanResult(msg compScanMsg) (Model, tea.Cmd) {
    if msg.err != nil {
        m.statusMsg = "Component-Scan-Fehler: " + msg.err.Error()
        return m, nil
    }
    m.componentsSource = msg.srcComps
    m.componentsTarget = msg.tgtComps
    m.componentGroupsSource = msg.srcGroups
    m.componentGroupsTarget = msg.tgtGroups
    m.statusMsg = fmt.Sprintf("Scan ok. Source: %d Components, %d Groups. Target: %d Components, %d Groups.", len(msg.srcComps), len(msg.srcGroups), len(msg.tgtComps), len(msg.tgtGroups))
    return m, nil
}
