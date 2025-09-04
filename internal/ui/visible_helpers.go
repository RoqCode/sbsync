package ui

import "storyblok-sync/internal/sb"

// visibleOrderBrowse returns visible stories for the browse view in order,
// and a copy of the underlying visibleIdx (mapping to storiesSource indices).
func (m *Model) visibleOrderBrowse() ([]sb.Story, []int) {
	total := m.itemsLen()
	if total <= 0 {
		return []sb.Story{}, []int{}
	}
	stories := make([]sb.Story, total)
	for i := 0; i < total; i++ {
		stories[i] = m.itemAt(i)
	}
	order := make([]int, len(m.visibleIdx))
	copy(order, m.visibleIdx)
	return stories, order
}

// visibleOrderPreflight returns visible preflight stories and the order slice
// mapping visible positions to indices in m.preflight.items.
func (m *Model) visibleOrderPreflight() ([]sb.Story, []int) {
	n := len(m.preflight.items)
	if n == 0 {
		return []sb.Story{}, []int{}
	}
    order := m.preflight.visibleIdx
    if len(order) == 0 {
        // Rebuild visible map if empty
        m.refreshPreflightVisible()
        order = m.preflight.visibleIdx
    }
    // Defensive: if still empty or out-of-sync, fall back to linear order
    if len(order) == 0 {
        order = make([]int, n)
        for i := 0; i < n; i++ {
            order[i] = i
        }
    } else {
        // Copy and sanitize indices to avoid stale/out-of-range values
        dup := make([]int, 0, len(order))
        for _, idx := range order {
            if idx >= 0 && idx < n {
                dup = append(dup, idx)
            }
        }
        if len(dup) == 0 {
            dup = make([]int, n)
            for i := 0; i < n; i++ {
                dup[i] = i
            }
        }
        order = dup
    }
	stories := make([]sb.Story, len(order))
	for i, idx := range order {
        stories[i] = m.preflight.items[idx].Story
	}
	return stories, order
}
