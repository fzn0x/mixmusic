package audio

import (
	"time"

	"github.com/fzn0x/mixmusic/internal/media"
)

// Cue is how Load, Preload and Crossfade start a track, beside its URL.
// Its zero value starts a track from its start, at its own speed, and Load
// holds it there until Play. A new way to start a track is a new field,
// whose zero value starts it as before: the callers that don't use it
// don't change.
type Cue struct {
	// Start is where Load starts the track. Preload and Crossfade start it
	// from its start.
	Start time.Duration
	// Autoplay has Load play the track at once, rather than hold it at
	// Start until Play. Crossfade always plays it.
	Autoplay bool
	// Speed is the speed the track plays at, from MinSpeed to MaxSpeed, at
	// its own pitch: 1, or 0, is its own speed.
	Speed float64
	// Track is the track, for an output that shows what plays, as a cast
	// device's screen: the engine plays without it.
	Track *media.Track
	// Follow has Preload arm the track to follow the one that plays on its
	// very next sample, without a crossfade: as that one ends, the track
	// plays on from there at full level, with no fade-in, and the engine
	// tells Followed in place of Playing(false) and Ended (follow.go).
	// Load, Crossfade, Stop and Unload disarm it.
	Follow bool
	// Glide has a track that follows start at the loudness the one before
	// plays at, under Even loudness, and glide to its own, as the songs of
	// one album do, so that the album doesn't step in level.
	Glide bool
}
