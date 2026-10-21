// Package audio is MixMusic's own audio engine: it plays the Opus audio of
// YouTube's WebM files on two decks, so that one track can fade into the
// next, through the preamp, equalizer, levelling and analyser of the web
// player MixMusic had, whose Web Audio nodes it reproduces.
package audio

import (
	_ "embed"
	"math"
)

// License is the license of the code of Blink this package ports, whose
// binary copies must reproduce it.
//
//go:embed LICENSE.blink
var License string

const (
	// Rate is the sample rate of everything the engine plays, in Hz: Opus
	// decodes to it, and the device plays it.
	Rate = 48000
	// quantum is the frames the engine processes at a time, as Web Audio
	// does: its parameters move from one to the next.
	quantum = 128
)

// smoother moves a value toward a target as AudioParam.setTargetAtTime
// does: v(t) = target + (v0 - target)·e^(-t/τ).
type smoother struct {
	value, target float64
	// k is the share of the distance covered by a sample.
	k float64
}

// newSmoother returns a smoother at v, with time constant tau in seconds.
func newSmoother(v, tau float64) smoother {
	return smoother{value: v, target: v, k: 1 - math.Exp(-1/(tau*Rate))}
}

// next returns the value of the next sample.
func (s *smoother) next() float64 {
	s.value += (s.target - s.value) * s.k
	return s.value
}

// settled reports whether the value reached its target, which it then
// takes: an exponential approach never quite does.
func (s *smoother) settled() bool {
	if math.Abs(s.target-s.value) < 1e-6*max(1, math.Abs(s.target)) {
		s.value = s.target
		return true
	}
	return false
}

// advance moves the value by n samples at once.
func (s *smoother) advance(n int) {
	s.value = s.target + (s.value-s.target)*math.Pow(1-s.k, float64(n))
}
