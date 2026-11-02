package audio

import "time"

// Cue is how Load and Crossfade start a track, beside its URL. Its zero
// value starts the track from its start, at its own speed, held there
// until Play for Load. A new way to start a track is a new field, whose
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
	// its own pitch: 1, as 0 is, is its own speed.
	Speed float64
}
