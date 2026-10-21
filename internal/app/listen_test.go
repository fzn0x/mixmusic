package app

import (
	"math"
	"os"
	"testing"

	"github.com/fzn0x/mixmusic/internal/audio"
	"github.com/fzn0x/mixmusic/internal/player"
)

// TestListenerFollows hears MixMusic's own process tree, whose webview
// plays Spotify, while Spotify plays in MixMusic, and nothing otherwise,
// and has the analyser read the tap while the track plays outside the
// engine.
func TestListenerFollows(t *testing.T) {
	own := uint32(os.Getpid())
	for _, c := range []struct {
		name    string
		st      player.State
		want    uint32
		outside bool
	}{
		{"spotify in mixmusic", player.State{Playing: true, Source: &player.Source{Mode: player.ModeSpotify}}, own, true},
		{"paused", player.State{Source: &player.Source{Mode: player.ModeSpotify}}, 0, true},
		{"the engine", player.State{Playing: true, Source: &player.Source{Mode: player.ModeStream}}, 0, false},
		{"not prepared yet", player.State{Playing: true}, 0, false},
	} {
		// No goroutine: the listener only decides what to hear.
		l := &listener{tap: audio.NewTap(), wake: make(chan struct{}, 1)}
		l.follow(c.st)
		if l.want != c.want || l.outside.Load() != c.outside {
			t.Errorf("%s: hears %d, outside %v; want %d, %v", c.name, l.want, l.outside.Load(), c.want, c.outside)
		}
	}
}

// TestNativeAnalyserOfOutside reads the listener's tap while the track
// plays outside the engine, and the engine's analyser otherwise.
func TestNativeAnalyserOfOutside(t *testing.T) {
	a := &App{}
	l := &listener{tap: audio.NewTap()}
	a.listen.Store(l)
	frames := make([]float32, 2*audio.FFTSize)
	for i := range audio.FFTSize {
		v := float32(0.01 * math.Sin(2*math.Pi*8*float64(i)/audio.FFTSize))
		frames[2*i], frames[2*i+1] = v, v
	}
	for range 20 {
		l.tap.Write(frames)
	}
	p := &Player{a: a}
	loud := func(bytes []int) bool {
		for _, b := range bytes {
			if b > 0 {
				return true
			}
		}
		return false
	}
	l.outside.Store(true)
	if got := p.NativeAnalyser(false, false); !loud(got.Bytes) {
		t.Errorf("outside the engine, the analyser hears nothing of the tap: %v", got.Bytes)
	}
	l.outside.Store(false)
	if got := p.NativeAnalyser(false, false); loud(got.Bytes) {
		t.Errorf("in the engine, the analyser hears the tap: %v", got.Bytes)
	}
	if e := a.native; e != nil {
		e.Close()
	}
}
