package ui

import (
	"context"
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	sync "storyblok-sync/internal/core/sync"
	"storyblok-sync/internal/sb"
)

// ---------- Messages / Cmds ----------
type validateMsg struct {
	spaces []sb.Space
	err    error
}

type scanMsg struct {
	src          []sb.Story
	tgt          []sb.Story
	err          error
	cdaToken     string
	cdaTokenKind string
}

// hydrateMsg signals completion of CDA pre-hydration
type hydrateMsg struct {
	total     int
	hydrated  int
	drafts    int
	published int
	err       error
	cache     *sync.HydrationCache
}

func (m Model) validateTokenCmd() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		c := sb.New(m.cfg.Token)
		spaces, err := c.ListSpaces(ctx)
		if err != nil {
			return validateMsg{err: err}
		}
		return validateMsg{spaces: spaces, err: nil}
	}
}

func (m Model) scanStoriesCmd() tea.Cmd {
	srcID, tgtID := 0, 0
	if m.sourceSpace != nil {
		srcID = m.sourceSpace.ID
	}
	if m.targetSpace != nil {
		tgtID = m.targetSpace.ID
	}
	token := m.cfg.Token

	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		c := sb.New(token)

		// Phase 2: resolve CDA token for source space (preview preferred)
		var cdaToken, cdaKind string
		if srcID > 0 {
			if info, _ := sync.ResolveCDAToken(ctx, c, srcID); info.Available {
				cdaToken = info.Selected
				cdaKind = info.Kind
			}
		}

		// Parallel wäre nice-to-have, hier sequentiell für Klarheit
		src, err := c.ListStories(ctx, sb.ListStoriesOpts{SpaceID: srcID, PerPage: 50})
		if err != nil {
			return scanMsg{err: fmt.Errorf("source scan: %w", err)}
		}
		sortStories(src)
		tgt, err := c.ListStories(ctx, sb.ListStoriesOpts{SpaceID: tgtID, PerPage: 50})
		if err != nil {
			return scanMsg{err: fmt.Errorf("target scan: %w", err)}
		}
		sortStories(tgt)
		return scanMsg{src: src, tgt: tgt, err: nil, cdaToken: cdaToken, cdaTokenKind: cdaKind}
	}
}

// hydrateContentCmd performs pre-hydration using CDA if a token is available.
func (m Model) hydrateContentCmd() tea.Cmd {
	srcID := 0
	if m.sourceSpace != nil {
		srcID = m.sourceSpace.ID
	}
	token := m.sourceCDAToken
	kind := m.sourceCDATokenKind

	// Build list of items (include folders for batching detection)
	items := append([]sync.PreflightItem(nil), m.preflight.items...)

	return func() tea.Msg {
		// No token: skip hydration successfully
		if !m.hasSourceCDAToken || srcID == 0 || len(items) == 0 {
			return hydrateMsg{total: len(items), hydrated: 0, drafts: 0, published: 0, err: nil, cache: nil}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		cda := sb.NewCDA(token)
		cache := sync.NewHydrationCache(1000)
		// Use batched hydration with top-most fully selected folders
		stats := sync.HydrateBatched(ctx, cda, srcID, items, m.storiesSource, kind, 4, 12, 1000, cache)
		return hydrateMsg{total: stats.Total, hydrated: stats.Drafts + stats.Published, drafts: stats.Drafts, published: stats.Published, err: nil, cache: cache}
	}
}
