package app

import (
	"math"
	"os"
	"testing"

	"github.com/fzn0x/mixmusic/internal/audio"
	"github.com/fzn0x/mixmusic/internal/player"
)

// TestListenerFollows hears MixMusic's own process tree, whose webview
// plays Spotify, while Spotify plays in MixMusic, the Spotify app while it
// plays on Spotify Connect, and nothing otherwise, and has the analyser
// read the tap while the track plays outside the engine.
func TestListenerFollows(t *testing.T) {
	for _, c := range []struct {
		name    string
		st      player.State
		want    hearing
		outside bool
	}{
		{"spotify in mixmusic", player.State{Playing: true, Source: &player.Source{Mode: player.ModeSpotify}}, hearMixMusic, true},
		{"spotify connect", player.State{Playing: true, Source: &player.Source{Mode: player.ModeConnect}}, hearSpotifyApp, true},
		{"paused", player.State{Source: &player.Source{Mode: player.ModeSpotify}}, hearNothing, true},
		{"the engine", player.State{Playing: true, Source: &player.Source{Mode: player.ModeStream}}, hearNothing, false},
		{"not prepared yet", player.State{Playing: true}, hearNothing, false},
	} {
		// No goroutine: the listener only decides what to hear.
		l := &listener{tap: audio.NewTap(), wake: make(chan struct{}, 1)}
		l.follow(c.st)
		if l.want != c.want || l.outside.Load() != c.outside {
			t.Errorf("%s: hears %d, outside %v; want %d, %v", c.name, l.want, l.outside.Load(), c.want, c.outside)
		}
	}
	if got := hearMixMusic.process(); got != uint32(os.Getpid()) {
		t.Errorf("hearing MixMusic hears process %d, not its own", got)
	}
}

// TestRootNamed finds the process of an app whose parent isn't the app,
// whose tree holds the app's other processes.
func TestRootNamed(t *testing.T) {
	procs := []proc{
		{pid: 4, parent: 0, name: "System"},
		{pid: 30, parent: 20, name: "Spotify.exe"},
		{pid: 20, parent: 10, name: "Spotify.exe"},
		{pid: 10, parent: 4, name: "explorer.exe"},
		{pid: 31, parent: 20, name: "spotify.exe"},
	}
	if got := rootNamed(procs, "Spotify.exe"); got != 20 {
		t.Errorf("the Spotify app is process %d, want 20", got)
	}
	if got := rootNamed(procs[:1], "Spotify.exe"); got != 0 {
		t.Errorf("without the Spotify app, found process %d", got)
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
