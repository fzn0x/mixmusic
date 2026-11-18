package app

import (
	"github.com/fzn0x/mixmusic/internal/audio"
	"github.com/fzn0x/mixmusic/internal/settings"
)

// The equalizer and the stages after it, as the settings have them
// (#615).

// applySound gives the engine the equalizer of the settings, its preamp
// Auto headroom's while that's on, and the stages after it. Sound effects
// off makes the equalizer flat, and crossfeed and width neutral, keeping
// their settings; balance and mono still apply.
func applySound(e *audio.Engine, q settings.Equalizer) {
	gains := bandGains(q.Gains)
	preamp := q.Preamp
	if q.AutoHeadroom {
		preamp = audio.Headroom(gains)
	}
	e.SetEqualizer(q.Enabled && q.Effects, gains, preamp)
	e.SetSound(soundOf(q))
}

// bandGains are the gains of the bands, as the engine takes them.
func bandGains(gains []float64) [len(audio.Bands)]float64 {
	var g [len(audio.Bands)]float64
	copy(g[:], gains)
	return g
}

// soundOf is what the stages after the equalizer do with the settings q.
func soundOf(q settings.Equalizer) audio.Sound {
	s := audio.Sound{Balance: float64(q.Balance) / settings.MaxBalance, Mono: q.Mono, Width: 1}
	if q.Effects {
		s.Crossfeed = float64(q.Crossfeed) / settings.MaxCrossfeed
		s.Width = float64(q.Width) / 100
	}
	return s
}

// Headroom is the preamp, in dB, that Auto headroom sets for the bands at
// gains, which the window shows on the preamp's slider.
func Headroom(gains []float64) float64 { return audio.Headroom(bandGains(gains)) }
