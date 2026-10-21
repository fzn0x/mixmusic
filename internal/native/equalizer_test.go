package native

import (
	"image/color"
	"testing"

	"github.com/fzn0x/mixmusic/internal/settings"
)

// TestEqualizerSliders draws each band's slider, which SliderBase leaves to
// its caller: a flat band shows its track and knob, and the accent fills
// up from the middle for a boost, down for a cut.
func TestEqualizerSliders(t *testing.T) {
	s := newTestShell(t)
	s.set.Equalizer.Enabled = true
	s.set.Equalizer.Gains = make([]float64, len(settings.Bands))
	s.set.Equalizer.Gains[1], s.set.Equalizer.Gains[2] = 12, -12 // 64 Hz up, 125 Hz down
	tt := testerOf(t, s, 1280, 2400)
	show(s, tt, "/settings/playback")
	img := tt.Image()
	near := func(c color.Color, want color.NRGBA) bool {
		r, g, b, _ := c.RGBA()
		d := func(x uint32, y uint8) bool { return int(x>>8)-int(y) <= 8 && int(y)-int(x>>8) <= 8 }
		return d(r, want.R) && d(g, want.G) && d(b, want.B)
	}
	for _, c := range []struct {
		label string
		gain  float64
	}{{"32 dB", 0}, {"64 dB", 12}, {"125 dB", -12}} {
		r, ok := tt.Find(c.label)
		if !ok {
			t.Fatalf("no slider %q: %q", c.label, tt.Texts())
		}
		mid := r.Y + r.H/2
		drawn, above, below := 0, 0, 0
		for y := int(r.Y); y < int(r.Y+r.H); y++ {
			for x := int(r.X); x < int(r.X+r.W); x++ {
				px := img.At(x, y)
				if !near(px, nrgba(s.lk.bg)) {
					drawn++
				}
				if near(px, nrgba(s.lk.accent)) {
					if float32(y) < mid {
						above++
					} else {
						below++
					}
				}
			}
		}
		switch {
		case drawn == 0:
			t.Errorf("%s: the slider draws nothing", c.label)
		case c.gain > 0 && (above == 0 || below > above/10):
			t.Errorf("%s at %+.0f: the accent fills %d pixels above the middle, %d below", c.label, c.gain, above, below)
		case c.gain < 0 && (below == 0 || above > below/10):
			t.Errorf("%s at %+.0f: the accent fills %d pixels above the middle, %d below", c.label, c.gain, above, below)
		}
	}
}
