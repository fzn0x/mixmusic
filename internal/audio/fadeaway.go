package audio

import (
	"math"
	"time"
)

// The sleep timer's fade (#594): a gain of its own, after the volume,
// which falls to silence on an equal-power curve and comes back, while
// the volume, which the user sets and the settings keep, never moves.

// fadeAway is the sleep timer's fade: its phase goes from 0, at full gain,
// to 1, silent, and its gain is cos(phase·π/2), whose power falls evenly.
type fadeAway struct{ phase ramp }

// gain returns the fade's gain of the next frame. On the audio thread.
func (f *fadeAway) gain() float64 {
	if f.phase.value == 0 && f.phase.target == 0 {
		return 1
	}
	ph := f.phase.next()
	if ph >= 1 {
		return 0 // where the cosine of π/2 is a hair above it
	}
	return math.Cos(ph * math.Pi / 2)
}

// FadeAway fades what the engine plays out to silence over d, at once for
// none, apart from the volume, and holds it silent until FadeBack: tracks
// that start meanwhile play silent too.
func (e *Engine) FadeAway(d time.Duration) {
	e.mu.Lock()
	e.away.phase.to(1, d)
	e.mu.Unlock()
}

// FadeBack brings what the engine plays back from the fade of FadeAway, to
// full over d, at once for none.
func (e *Engine) FadeBack(d time.Duration) {
	e.mu.Lock()
	e.away.phase.to(0, d)
	e.mu.Unlock()
}
