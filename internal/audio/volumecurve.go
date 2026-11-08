package audio

import "github.com/fzn0x/mixmusic/internal/volume"

// SetVolumeCurve sets the curve the volume plays on (#383). The volume
// keeps its position, which now plays at its gain on c, gliding there as
// a change of the volume does.
func (e *Engine) SetVolumeCurve(c volume.Curve) {
	e.mu.Lock()
	e.curve = c
	e.master.target = c.Gain(e.vol)
	e.mu.Unlock()
}

// VolumeCurve is the curve SetVolumeCurve chose last: none, which is
// Natural, at first.
func (e *Engine) VolumeCurve() volume.Curve {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.curve
}
