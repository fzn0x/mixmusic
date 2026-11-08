package main

import (
	"slices"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/volume"
)

func init() { addSettings("playback", 55, (*shell).volumeSettings) }

// volumeSettings choose the volume's curve, in Settings › Playback ›
// Volume (#383).
func (s *shell) volumeSettings(c *ui.Context) {
	s.group(c, "Volume", func() {
		s.row(c, "Volume curve", volumeCurveDescription, func() {
			curve := max(slices.Index(volume.Curves, s.set.VolumeCurve), 0)
			if s.segmented(c, "volume-curve", "Volume curve", &curve, "natural", "linear") {
				s.set.VolumeCurve = volume.Curves[curve]
				s.save()
			}
		}, "loudness", "decibel", "slider")
	})
}

// volumeCurveDescription tells what the curves do.
const volumeCurveDescription = "Natural gives the volume slider even steps of loudness: each tenth of it is 6 dB. " +
	"On Linear, the level grows evenly along the slider, so its middle is half the level, 6 dB down. " +
	"Spotify tracks play on the same curve."
