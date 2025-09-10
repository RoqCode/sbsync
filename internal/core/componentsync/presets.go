package componentsync

import (
	"storyblok-sync/internal/sb"
)

// FilterPresetsForComponentID returns presets belonging to a specific component id.
func FilterPresetsForComponentID(all []sb.ComponentPreset, compID int) []sb.ComponentPreset {
	if compID == 0 || len(all) == 0 {
		return nil
	}
	out := make([]sb.ComponentPreset, 0)
	for _, p := range all {
		if p.ComponentID == compID {
			out = append(out, p)
		}
	}
	return out
}

// DiffPresetsByName computes new vs update sets using name equality.
// - new: in src but not in tgt by name
// - update: in both by name; returned preset carries target ID (for PUT)
// Parity with Storyblok: we do not compare JSON; we PUT all matches by name.
func DiffPresetsByName(src, tgt []sb.ComponentPreset) (newPresets, updatePresets []sb.ComponentPreset) {
	if len(src) == 0 {
		return nil, nil
	}
	tgtByName := make(map[string]sb.ComponentPreset, len(tgt))
	for _, t := range tgt {
		if t.Name == "" {
			continue
		}
		tgtByName[t.Name] = t
	}
	newPresets = make([]sb.ComponentPreset, 0)
	updatePresets = make([]sb.ComponentPreset, 0)
	for _, s := range src {
		if s.Name == "" {
			continue
		}
		if t, ok := tgtByName[s.Name]; ok {
			// Carry target ID for PUT; keep source name/preset/image
			upd := sb.ComponentPreset{
				ID:          t.ID,
				Name:        s.Name,
				ComponentID: t.ComponentID, // will be normalized later
				Preset:      s.Preset,
				Image:       s.Image,
			}
			updatePresets = append(updatePresets, upd)
		} else {
			// Create from source; ID empty; ComponentID will be normalized later
			crt := sb.ComponentPreset{
				Name:        s.Name,
				ComponentID: 0,
				Preset:      s.Preset,
				Image:       s.Image,
			}
			newPresets = append(newPresets, crt)
		}
	}
	return newPresets, updatePresets
}

// NormalizePresetForTarget prepares a preset for a specific target component id.
// - sets ComponentID
// - keeps Name, Preset, Image
// - keeps ID if present (update), leaves zero for create
func NormalizePresetForTarget(p sb.ComponentPreset, targetComponentID int) sb.ComponentPreset {
	return sb.ComponentPreset{
		ID:          p.ID,
		Name:        p.Name,
		ComponentID: targetComponentID,
		Preset:      p.Preset,
		Image:       p.Image,
	}
}
