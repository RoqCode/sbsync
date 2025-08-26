package ui

// ensureCursorInViewport adjusts the viewport Y offset so that the given
// absolute cursorLine is within the visible window with a scroll margin.
func (m *Model) ensureCursorInViewport(cursorLine int) {
	topLine := m.viewport.YOffset
	bottomLine := topLine + m.viewport.Height - 1

	scrollMargin := 3
	if m.viewport.Height < 8 {
		scrollMargin = 1
	}

	if cursorLine < topLine+scrollMargin {
		target := cursorLine - scrollMargin
		if target < 0 {
			target = 0
		}
		m.viewport.SetYOffset(target)
		return
	}
	if cursorLine > bottomLine-scrollMargin {
		target := cursorLine - m.viewport.Height + scrollMargin + 1
		if target < 0 {
			target = 0
		}
		m.viewport.SetYOffset(target)
	}
}
