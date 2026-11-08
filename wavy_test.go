package main

import (
	"fmt"
	"image"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/player"
	"github.com/fzn0x/mixmusic/internal/settings"
)

// setWavy turns the wavy progress bar on or off, as Settings does.
func setWavy(t *testing.T, s *shell, on bool) {
	t.Helper()
	st := s.svc.Preferences.Get()
	st.WavyProgress = on
	if _, err := s.svc.Preferences.Set(st); err != nil {
		t.Fatal(err)
	}
	s.refresh()
}

// playingAt is a test shell playing the first test track, at 100 s of its
// 211.
func playingAt(t *testing.T) *shell {
	t.Helper()
	s := newTestShell(t)
	s.p.PlayTracks(testTracks, 0)
	s.settle(t, func(st player.State) bool { return st.Playing })
	s.p.Time(100*time.Second, 211*time.Second)
	s.settle(t, func(st player.State) bool { return st.Playing && st.Position >= 100*time.Second })
	return s
}

// seekPixels is the player bar's seek bar as the window last showed it.
func seekPixels(t *testing.T, tt *ui.Tester) *image.RGBA {
	t.Helper()
	r, ok := tt.Find("Seek")
	if !ok {
		t.Fatalf("no seek bar: %q", tt.Texts())
	}
	img := tt.Image()
	scale := float32(img.Bounds().Dx()) / 1280
	box := image.Rect(int(r.X*scale), int(r.Y*scale), int(math.Ceil(float64((r.X+r.W)*scale))), int(math.Ceil(float64((r.Y+r.H)*scale))))
	out := image.NewRGBA(image.Rect(0, 0, box.Dx(), box.Dy()))
	for y := range box.Dy() {
		copy(out.Pix[y*out.Stride:], img.Pix[img.PixOffset(box.Min.X, box.Min.Y+y):img.PixOffset(box.Max.X, box.Min.Y+y)])
	}
	return out
}

// differ is how many pixels of a and b differ.
func differ(a, b *image.RGBA) int {
	if a.Bounds() != b.Bounds() {
		return a.Bounds().Dx() * a.Bounds().Dy()
	}
	n := 0
	for i := 0; i+3 < len(a.Pix); i += 4 {
		if a.Pix[i] != b.Pix[i] || a.Pix[i+1] != b.Pix[i+1] || a.Pix[i+2] != b.Pix[i+2] || a.Pix[i+3] != b.Pix[i+3] {
			n++
		}
	}
	return n
}

// TestWavyProgress draws the played part of the seek bar as a wave while
// the setting is on and the track plays (#382): it moves along in frames
// that only paint, without building the view, and lies flat, as the
// theme's slider, once the track pauses. Under Reduce Motion it holds
// still, and goes flat at once.
func TestWavyProgress(t *testing.T) {
	s := playingAt(t)
	builds := 0
	tt := ui.NewTester(func(c *ui.Context) { builds++; s.view(c) }, 1280, 820)
	tt.Frame()
	tt.Advance(time.Second)
	straight := seekPixels(t, tt)

	setWavy(t, s, true)
	tt.Frame()
	tt.Advance(waveEase + 100*time.Millisecond)
	wave := seekPixels(t, tt)
	if differ(wave, straight) == 0 {
		t.Fatal("the seek bar stays straight while the track plays")
	}
	if !tt.FramePending() {
		t.Error("the wave asks for no frames while the track plays")
	}
	was := builds
	tt.Advance(wavePeriod / 4)
	moved := seekPixels(t, tt)
	if differ(moved, wave) == 0 {
		t.Error("the wave doesn't move while the track plays")
	}
	if builds != was {
		t.Errorf("the wave built the view %d times as it moved, rather than painting it again", builds-was)
	}

	s.p.Pause()
	s.settle(t, func(st player.State) bool { return !st.Playing })
	tt.Frame()
	tt.Advance(waveEase / 2)
	if n := differ(seekPixels(t, tt), straight); n == 0 {
		t.Error("the wave lay flat at once as the track paused, rather than easing flat")
	}
	tt.Advance(waveEase)
	if n := differ(seekPixels(t, tt), straight); n != 0 {
		t.Errorf("paused, the seek bar isn't the theme's straight slider: %d pixels differ", n)
	}
	tt.Advance(time.Second)
	if tt.FramePending() {
		t.Error("the flat seek bar still asks for frames")
	}

	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	s.p.Play()
	s.settle(t, func(st player.State) bool { return st.Playing })
	tt.Frame()
	still := seekPixels(t, tt)
	if differ(still, straight) == 0 {
		t.Error("under Reduce Motion, the seek bar doesn't wave as the track plays")
	}
	tt.Advance(wavePeriod / 4)
	if n := differ(seekPixels(t, tt), still); n != 0 {
		t.Errorf("under Reduce Motion, the wave moved: %d pixels differ", n)
	}
	s.p.Pause()
	s.settle(t, func(st player.State) bool { return !st.Playing })
	tt.Frame()
	if n := differ(seekPixels(t, tt), straight); n != 0 {
		t.Errorf("under Reduce Motion, the seek bar didn't go flat at once as the track paused: %d pixels differ", n)
	}

	// Off, it is the theme's slider, playing or not.
	tt.SetPreferences(ui.Preferences{})
	s.p.Play()
	s.settle(t, func(st player.State) bool { return st.Playing })
	setWavy(t, s, false)
	tt.Frame()
	tt.Advance(time.Second)
	if n := differ(seekPixels(t, tt), straight); n != 0 {
		t.Errorf("with the setting off, the seek bar isn't straight: %d pixels differ", n)
	}
}

// TestWavyProgressSeeks seeks with the wavy progress bar as with the
// straight one: a click seeks where it lands, a drag once it ends, and the
// keys while it has the keyboard's focus.
func TestWavyProgressSeeks(t *testing.T) {
	s := playingAt(t)
	setWavy(t, s, true)
	tt := tester(t, s)
	r, ok := tt.Find("Seek")
	if !ok {
		t.Fatalf("no seek bar: %q", tt.Texts())
	}
	// The knob moves within the bar but for half its width at each end.
	at := func(f float32) float32 { return r.X + 10 + (r.W-20)*f }
	y := r.Y + r.H/2
	near := func(want time.Duration) func(player.State) bool {
		return func(st player.State) bool { return (st.Position - want).Abs() < 2*time.Second }
	}
	tt.ClickAt(at(0.75), y)
	s.settle(t, near(158*time.Second))

	tt.Press(at(0.25), y)
	s.drain()
	tt.Frame()
	tt.Move(at(0.5), y)
	s.drain()
	tt.Frame()
	if st := s.p.State(); !near(158 * time.Second)(st) {
		t.Errorf("the drag seeked to %v before it ended", st.Position)
	}
	tt.Release(at(0.5), y)
	s.drain()
	tt.Frame()
	s.settle(t, near(105*time.Second))

	for i := 0; i < 300 && !tt.Focused("Seek"); i++ {
		tt.Key(0, ui.KeyTab)
	}
	if !tt.Focused("Seek") {
		t.Fatalf("Tab doesn't reach the seek bar: %q", tt.Texts())
	}
	tt.Key(0, ui.KeyHome)
	s.drain()
	tt.Frame()
	s.settle(t, near(0))
}

// TestWavePoints keeps the end of the wave on the middle of the track, and
// the wave within its height.
func TestWavePoints(t *testing.T) {
	for _, phase := range []float32{0, 0.25, 0.5, 0.9} {
		pts := wavePoints(nil, 10, 300, 50, waveHeight, phase)
		if pts[0] != [2]float32{10, 50} {
			t.Errorf("at %v of a wave, it starts at %v, not on the track's middle", phase, pts[0])
		}
		if last := pts[len(pts)-1]; last[0] != 300 {
			t.Errorf("at %v of a wave, it ends at x %v, not 300", phase, last[0])
		}
		top, bottom := float32(50), float32(50)
		for _, p := range pts {
			top, bottom = min(top, p[1]), max(bottom, p[1])
		}
		if top < 50-waveHeight-1e-3 || bottom > 50+waveHeight+1e-3 || bottom-top < waveHeight {
			t.Errorf("at %v of a wave, it goes from %v to %v", phase, top, bottom)
		}
	}
	if pts := wavePoints(nil, 10, 5, 50, waveHeight, 0); len(pts) != 1 {
		t.Errorf("a wave of no length has %d points", len(pts))
	}
}

// TestRenderWavyProgress draws the player bar and the now playing page
// with the wavy progress bar in each layout, in Mix, Daylight, Aurora and
// Terminal, to MIXMUSIC_RENDER, for a look; MIXMUSIC_THEMES lists others.
func TestRenderWavyProgress(t *testing.T) {
	dir := os.Getenv("MIXMUSIC_RENDER")
	if dir == "" {
		t.Skip("MIXMUSIC_RENDER names no folder")
	}
	themes := []string{"mix", "daylight", "aurora", "terminal"}
	if only := os.Getenv("MIXMUSIC_THEMES"); only != "" {
		themes = strings.Split(only, ",")
	}
	s := playingAt(t)
	setWavy(t, s, true)
	for _, layout := range settings.Layouts {
		for _, theme := range themes {
			st := s.svc.Preferences.Get()
			st.FollowSystem, st.Theme, st.Layout = false, theme, layout
			if _, err := s.svc.Preferences.Set(st); err != nil {
				t.Fatal(err)
			}
			s.refresh()
			name := fmt.Sprintf("wavy-%s-%s", layout, theme)
			s.navigate("/")
			renderTo(t, s, ui.NewTester(s.view, 1280, 820), dir, name)
			s.navigate("/now-playing")
			s.panel = ""
			renderTo(t, s, ui.NewTester(s.view, 1280, 820), dir, name+"-now-playing")
			s.panel = "queue"
		}
	}
	// Close up, as it moves.
	s.navigate("/")
	tt := ui.NewTester(s.view, 1280, 820)
	s.drain()
	tt.Frame()
	tt.Advance(time.Second)
	for i := range 4 {
		img := seekPixels(t, tt)
		f, err := os.Create(filepath.Join(dir, fmt.Sprintf("wavy-close-%d.png", i)))
		if err != nil {
			t.Fatal(err)
		}
		if err := png.Encode(f, img); err != nil {
			t.Fatal(err)
		}
		f.Close()
		tt.Advance(wavePeriod / 8)
	}
}
