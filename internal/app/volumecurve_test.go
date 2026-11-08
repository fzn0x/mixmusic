package app

import (
	"math"
	"strconv"
	"testing"

	"github.com/fzn0x/mixmusic/internal/audio"
	"github.com/fzn0x/mixmusic/internal/player"
	"github.com/fzn0x/mixmusic/internal/settings"
	"github.com/fzn0x/mixmusic/internal/volume"
)

// TestVolumeCurveInBoth plays the volume on the curve of the settings in
// MixMusic's own engine and in Spotify's player in MixMusic alike (#383),
// as the curve changes, and leaves the volume's position where it was.
func TestVolumeCurveInBoth(t *testing.T) {
	a := newQuietApp(t, nil)
	// An engine of its own, which plays into no device, in place of the
	// one of the computer's output.
	e := audio.NewEngine(a.openNative, startSilent, engineEvents{a})
	t.Cleanup(e.Close)
	a.nativeOnce.Do(func() { a.native = e })
	a.applyEngine(a.settings.Get().Clean())
	p := a.GoPlayer()
	p.SetVolume(0.5)
	waitPlayer(t, p, "the volume at 0.5", func(s player.State) bool { return s.Volume == 0.5 })
	here := hereDevice{&a.here}
	check := func(c volume.Curve) {
		t.Helper()
		if a.nativeEngine() != nil {
			if got := e.VolumeCurve(); got != c {
				t.Errorf("the engine plays on %q, not %q", got, c)
			}
		}
		want := strconv.FormatFloat(c.Gain(0.5), 'f', -1, 64)
		if got := here.level(); got != want {
			t.Errorf("on %q, Spotify in MixMusic plays the volume at %s, not %s", c, got, want)
		}
		if got := here.gain(0.5); got != want {
			t.Errorf("on %q, Spotify in MixMusic is sent %s for the middle, not %s", c, got, want)
		}
		if v := p.State().Volume; v != 0.5 {
			t.Errorf("on %q, the volume moved to %v", c, v)
		}
	}
	check(volume.Natural)
	for _, c := range []volume.Curve{volume.Linear, volume.Natural} {
		if _, err := (&Preferences{a}).Update(func(s *settings.Settings) { s.VolumeCurve = c }); err != nil {
			t.Fatal(err)
		}
		check(c)
	}
	// What settings.json keeps of the volume is its position, whatever the
	// curve.
	if _, err := (&Preferences{a}).Update(func(s *settings.Settings) { s.VolumeCurve = volume.Linear }); err != nil {
		t.Fatal(err)
	}
	p.Flush()
	if v := a.settings.Get().Volume; math.Abs(v-volume.Saved(0.5)) > 1e-9 {
		t.Errorf("on Linear, settings.json keeps the volume as %v, not %v", v, volume.Saved(0.5))
	}
	// A volume key steps the position, which plays on the curve.
	a.command(PlayerCommand{Command: CommandVolumeBy, Value: 0.05})
	waitPlayer(t, p, "a step up", func(s player.State) bool { return math.Abs(s.Volume-0.55) < 1e-9 })
	if got, want := here.level(), strconv.FormatFloat(volume.Linear.Gain(p.State().Volume), 'f', -1, 64); got != want {
		t.Errorf("a step up on Linear plays Spotify in MixMusic at %s, not %s", got, want)
	}
}

// TestConnectPercent sends a Spotify Connect device the share of its
// volume for the loudness MixMusic plays the volume at, on each curve.
func TestConnectPercent(t *testing.T) {
	for _, c := range []struct {
		curve volume.Curve
		v     float64
		want  int
	}{
		{volume.Natural, 0, 0}, {volume.Natural, 0.5, 18}, {volume.Natural, 1, 100},
		{volume.Linear, 0, 0}, {volume.Linear, 0.25, 50}, {volume.Linear, 0.5, 71}, {volume.Linear, 1, 100},
	} {
		if got := connectPercent(c.curve, c.v); got != c.want {
			t.Errorf("on %q, %v sends %d%%, want %d%%", c.curve, c.v, got, c.want)
		}
	}
	// Natural sends what earlier versions sent.
	for v := 0.0; v <= 1; v += 0.05 {
		if got, want := connectPercent(volume.Natural, v), int(volume.Saved(v)*100+0.5); got != want {
			t.Errorf("on Natural, %v sends %d%%, not %d%% as before", v, got, want)
		}
	}
}
