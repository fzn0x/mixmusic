package audio

// Measured is a Source that knows how loud its track is: the integrated
// loudness of its audio, in LKFS, as YouTube measured it.
type Measured interface {
	Loudness() (lkfs float64, ok bool)
}

// loudnessTarget is the level, in LKFS, that Even loudness turns loud
// tracks down to: the target YouTube sends with each track.
const loudnessTarget = -14.0

// loudnessTrim returns the gain, in dB, that turns a track of lkfs down to
// loudnessTarget. It never turns a track up: YouTube tells no peak level,
// and a quiet recording turned up could pass full scale, which nothing
// after the decks holds back. A measure out of range trims nothing.
func loudnessTrim(lkfs float64) float64 {
	if !(lkfs >= -70 && lkfs <= 0) {
		return 0
	}
	return min(0, loudnessTarget-lkfs)
}
