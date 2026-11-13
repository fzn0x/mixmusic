package main

import (
	"slices"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/volume"
	"github.com/fzn0x/mixmusic/locales"
)

func init() { addSettings("playback", 55, (*shell).volumeSettings) }

// volumeSettings choose the volume's curve, in Settings › Playback ›
// Volume (#383).
func (s *shell) volumeSettings(c *ui.Context) {
	s.group(c, locales.VolumeCurveGroup(), func() {
		s.row(c, locales.VolumeCurveTitle(), volumeCurveDescription(), func() {
			curve := max(slices.Index(volume.Curves, s.set.VolumeCurve), 0)
			if s.segmented(c, "volume-curve", locales.VolumeCurveTitle(), &curve, locales.VolumeCurveNatural(), locales.VolumeCurveLinear()) {
				s.set.VolumeCurve = volume.Curves[curve]
				s.save()
			}
		}, "loudness", "decibel", "slider")
	})
}

// volumeCurveDescription tells what the curves do.
func volumeCurveDescription() string { return locales.VolumeCurveDescription() }
