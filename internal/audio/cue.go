package audio

import "time"

// Cue is how Load and Crossfade start a track, beside its URL. Its zero
// value starts a track from its start, at its own speed, and Load holds
// it there until Play. A new way to start a track is a new field, whose
// zero value starts it as before: the callers that don't use it don't
// change.
type Cue struct {
	// Start is where Load starts the track. Crossfade starts it from its
	// start.
	Start time.Duration
	// Autoplay has Load play the track at once, rather than hold it at
	// Start until Play. Crossfade always plays it.
	Autoplay bool
	// Speed is the speed the track plays at, from MinSpeed to MaxSpeed, at
	// its own pitch: 1, or 0, is its own speed.
	Speed float64
}
