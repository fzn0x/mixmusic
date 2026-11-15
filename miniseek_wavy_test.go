package main

import (
	"fmt"
	"image"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/player"
)

// miniSeekPixels is the mini player's progress as the window last showed
// it, w DIPs wide.
func miniSeekPixels(t *testing.T, tt *ui.Tester, w float32) *image.RGBA {
	t.Helper()
	r, ok := tt.Find("Seek")
	if !ok {
		t.Fatalf("no progress: %q", tt.Texts())
	}
	img := tt.Image()
	scale := float32(img.Bounds().Dx()) / w
	box := image.Rect(int(r.X*scale), int(r.Y*scale), int(math.Ceil(float64((r.X+r.W)*scale))), int(math.Ceil(float64((r.Y+r.H)*scale))))
	out := image.NewRGBA(image.Rect(0, 0, box.Dx(), box.Dy()))
	for y := range box.Dy() {
		copy(out.Pix[y*out.Stride:], img.Pix[img.PixOffset(box.Min.X, box.Min.Y+y):img.PixOffset(box.Max.X, box.Min.Y+y)])
	}
	return out
}

// TestMiniSeekWavy waves the mini player's progress with the wavy progress
// bar on (#632), as the seek bar's: it moves while the track plays without
// building the view, lies flat as it pauses, holds still under Reduce
// Motion, and stays the straight line with the setting off.
func TestMiniSeekWavy(t *testing.T) {
	s := playingAt(t)
	builds := 0
	tt := ui.NewTester(func(c *ui.Context) { builds++; s.miniView(c) }, 420, 132)
	tt.Frame()
	tt.Advance(time.Second)
	straight := miniSeekPixels(t, tt, 420)

	setWavy(t, s, true)
	tt.Frame()
	tt.Advance(waveEase + 100*time.Millisecond)
	wave := miniSeekPixels(t, tt, 420)
	if differ(wave, straight) == 0 {
		t.Fatal("the mini player's progress stays straight while the track plays")
	}
	if !tt.FramePending() {
		t.Error("the wave asks for no frames while the track plays")
	}
	was := builds
	tt.Advance(wavePeriod / 4)
	if differ(miniSeekPixels(t, tt, 420), wave) == 0 {
		t.Error("the wave doesn't move while the track plays")
	}
	if builds != was {
		t.Errorf("the wave built the view %d times as it moved, rather than painting it again", builds-was)
	}

	// Under the pointer it is the thicker line, still waving.
	r, _ := tt.Find("Seek")
	tt.Move(r.X+r.W/4, r.Y+r.H/2)
	tt.Frame()
	if differ(miniSeekPixels(t, tt, 420), wave) == 0 {
		t.Error("the progress doesn't thicken under the pointer")
	}
	tt.Move(4, 4)
	tt.Frame()

	s.p.Pause()
	s.settle(t, func(st player.State) bool { return !st.Playing })
	tt.Frame()
	tt.Advance(waveEase * 2)
	if n := differ(miniSeekPixels(t, tt, 420), straight); n != 0 {
		t.Errorf("paused, the progress isn't straight: %d pixels differ", n)
	}
	tt.Advance(time.Second)
	if tt.FramePending() {
		t.Error("the flat progress still asks for frames")
	}

	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	s.p.Play()
	s.settle(t, func(st player.State) bool { return st.Playing })
	tt.Frame()
	still := miniSeekPixels(t, tt, 420)
	if differ(still, straight) == 0 {
		t.Error("under Reduce Motion, the progress doesn't wave as the track plays")
	}
	tt.Advance(wavePeriod / 4)
	if n := differ(miniSeekPixels(t, tt, 420), still); n != 0 {
		t.Errorf("under Reduce Motion, the wave moved: %d pixels differ", n)
	}

	tt.SetPreferences(ui.Preferences{})
	setWavy(t, s, false)
	tt.Frame()
	tt.Advance(time.Second)
	if n := differ(miniSeekPixels(t, tt, 420), straight); n != 0 {
		t.Errorf("with the setting off, the progress isn't straight: %d pixels differ", n)
	}
}

// TestRenderMiniSeekWavy writes close-ups of the mini player's wavy
// progress, at rest and under the pointer, to MIXMUSIC_RENDER, for a look.
func TestRenderMiniSeekWavy(t *testing.T) {
	dir := os.Getenv("MIXMUSIC_RENDER")
	if dir == "" {
		t.Skip("MIXMUSIC_RENDER names no folder")
	}
	s := playingAt(t)
	setWavy(t, s, true)
	tt := ui.NewTester(s.miniView, 420, 132)
	tt.Frame()
	tt.Advance(time.Second)
	save := func(name string) {
		f, err := os.Create(filepath.Join(dir, name+".png"))
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, tt.Image()); err != nil {
			t.Fatal(err)
		}
	}
	for i := range 2 {
		save(fmt.Sprintf("mini-wavy-rest-%d", i))
		tt.Advance(wavePeriod / 4)
	}
	r, _ := tt.Find("Seek")
	tt.Move(r.X+r.W/4, r.Y+r.H/2)
	tt.Frame()
	save("mini-wavy-active")
}
