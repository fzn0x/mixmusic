package settings

import "slices"

// Bands are the center frequencies of the equalizer's bands, in hertz.
var Bands = []int{32, 64, 125, 250, 500, 1000, 2000, 4000, 8000, 16000}

// Equalizer shapes the sound of MixMusic's own player.
type Equalizer struct {
	// Enabled turns the equalizer on.
	Enabled bool `json:"enabled"`
	// Preset is the ID of the preset the gains are, or "custom" for gains
	// of the user's own.
	Preset string `json:"preset"`
	// Gains are in decibels, one per band, from -12 to 12.
	Gains []float64 `json:"gains"`
	// Preamp is in decibels, from -12 to 12.
	Preamp float64 `json:"preamp"`
}

// defaultEqualizer is the equalizer of a new install: off, and flat.
func defaultEqualizer() Equalizer {
	var e Equalizer
	e.Preset = "flat"
	e.Gains = make([]float64, len(Bands))
	return e
}

// clean is e with a gain for each band, of its own, and every gain in its
// range.
func (e Equalizer) clean() Equalizer {
	if len(e.Gains) != len(Bands) {
		e.Gains = make([]float64, len(Bands))
	} else {
		e.Gains = slices.Clone(e.Gains)
	}
	for i, g := range e.Gains {
		e.Gains[i] = min(max(g, -12), 12)
	}
	e.Preamp = min(max(e.Preamp, -12), 12)
	if e.Preset == "" {
		e.Preset = "custom"
	}
	return e
}
