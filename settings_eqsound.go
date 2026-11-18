package main

import (
	"math"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/locales"
)

// Balance, mono, crossfeed and stereo width, in Settings › Playback › Sound
// (#615): the stages of the sound after the equalizer.

func init() { addSettings(soundRows, 52, (*shell).balanceRow) }

// balanceDetent is how near the centre the balance comes to rest there,
// as a slider moves into it: in from either side, it stops at the centre,
// and from there it moves on freely.
const balanceDetent = 4

// balanceAt is the balance a slider moved from was to v, which rests at
// the centre as it comes within balanceDetent of it.
func balanceAt(was int, v float64) int {
	n := int(math.Round(v))
	if max(n, -n) <= balanceDetent && max(was, -was) > balanceDetent {
		return 0
	}
	return n
}

// balanceRow weighs the left side of the sound against the right.
func (s *shell) balanceRow(c *ui.Context) {
	eq := &s.set.Equalizer
	desc := locales.EqualizerBalanceCentre()
	switch {
	case eq.Balance < 0:
		desc = locales.EqualizerBalanceLeft(-eq.Balance)
	case eq.Balance > 0:
		desc = locales.EqualizerBalanceRight(eq.Balance)
	}
	s.row(c, locales.EqualizerBalance(), desc, func() {
		// Its left is the left side, in every language.
		ui.Row(c).Direction(ui.LeftToRight).Children(func() {
			v := float64(eq.Balance)
			if ui.Slider(c.Key("balance"), &v, -100, 100).Step(1).Width(200).Label(locales.EqualizerBalance()).Changed() {
				eq.Balance = balanceAt(eq.Balance, v)
				s.save()
			}
		})
	}, "left right")
}

func init() { addSettings(soundRows, 53, (*shell).monoRow) }

// monoRow plays both sides of the sound mixed on each.
func (s *shell) monoRow(c *ui.Context) {
	s.row(c, locales.EqualizerMono(), locales.EqualizerMonoDesc(), func() {
		if s.switchOf(c.Key("mono"), &s.set.Equalizer.Mono).Label(locales.EqualizerMono()).Changed() {
			s.save()
		}
	})
}

func init() { addSettings(soundRows, 56, (*shell).crossfeedRow) }

// crossfeedRow mixes a softened copy of each side into the other, for
// headphones.
func (s *shell) crossfeedRow(c *ui.Context) {
	eq := &s.set.Equalizer
	desc := locales.EqualizerCrossfeedOff()
	if eq.Crossfeed > 0 {
		desc = locales.EqualizerCrossfeedAt(eq.Crossfeed)
	}
	s.row(c, locales.EqualizerCrossfeed(), desc, func() {
		v := float64(eq.Crossfeed)
		if ui.Slider(c.Key("crossfeed"), &v, 0, 100).Step(5).Width(200).Label(locales.EqualizerCrossfeed()).Changed() {
			eq.Crossfeed = int(math.Round(v))
			s.save()
		}
	}, "headphones")
}

func init() { addSettings(soundRows, 57, (*shell).widthRow) }

// widthRow sets how far apart the sides sound, and tells when it and the
// crossfeed wait: while Sound effects is off, and on 5.1.
func (s *shell) widthRow(c *ui.Context) {
	eq := &s.set.Equalizer
	s.row(c, locales.EqualizerWidth(), locales.EqualizerWidthAt(eq.Width), func() {
		v := float64(eq.Width)
		if ui.Slider(c.Key("width"), &v, 0, 200).Step(5).Width(200).Label(locales.EqualizerWidth()).Changed() {
			eq.Width = int(math.Round(v))
			s.save()
		}
	}, "stereo wide narrow")
	if !eq.Effects {
		s.hint(c, locales.EqualizerEffectsOff())
	}
	if s.set.Surround {
		s.hint(c, locales.EqualizerSurroundNote())
	}
}
