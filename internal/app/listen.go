package app

// The analyser hears MixMusic's own engine. Spotify, when it plays in
// MixMusic, plays in the page of a webview, which the engine never hears,
// and on Spotify Connect in a Spotify app: on Windows, the listener hears
// them there, through process loopback, for the visualizer and the plugins'
// analyser. It hears the Spotify app only on this computer; other devices
// MixMusic can't hear.

import (
	"log"
	"os"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/fzn0x/mixmusic/internal/audio"
	"github.com/fzn0x/mixmusic/internal/audio/output"
	"github.com/fzn0x/mixmusic/internal/player"
)

// hearing is what the listener hears.
type hearing int

const (
	hearNothing hearing = iota
	// hearMixMusic hears the browser process of MixMusic's webview, whose
	// children play Spotify in MixMusic: Windows hears a process and its
	// children, but not theirs, so not WebView2's audio from MixMusic's own.
	hearMixMusic
	// hearSpotifyApp hears the Spotify app on this computer, which plays
	// what plays on Spotify Connect when it is the device.
	hearSpotifyApp
)

// heard is what the listener hears of what st plays outside the engine.
func heard(st player.State) hearing {
	if !st.Playing || st.Source == nil {
		return hearNothing
	}
	switch st.Source.Mode {
	case player.ModeSpotify:
		return hearMixMusic
	case player.ModeConnect:
		return hearSpotifyApp
	}
	return hearNothing
}

// process returns the process whose children play what h hears, or 0 for
// none, as while the Spotify app doesn't run.
func (h hearing) process() uint32 {
	switch h {
	case hearMixMusic:
		return childNamed(processes(), uint32(os.Getpid()), "msedgewebview2.exe")
	case hearSpotifyApp:
		return rootNamed(processes(), "Spotify.exe")
	}
	return 0
}

// proc is a process that runs.
type proc struct {
	pid, parent uint32
	name        string
}

// childNamed returns a child of the process parent named name, or 0 for
// none.
func childNamed(procs []proc, parent uint32, name string) uint32 {
	for _, p := range procs {
		if p.parent == parent && strings.EqualFold(p.name, name) {
			return p.pid
		}
	}
	return 0
}

// rootNamed returns a process named name whose parent is not, whose tree
// holds the others, or 0 for none.
func rootNamed(procs []proc, name string) uint32 {
	names := make(map[uint32]string, len(procs))
	for _, p := range procs {
		names[p.pid] = p.name
	}
	for _, p := range procs {
		if strings.EqualFold(p.name, name) && !strings.EqualFold(names[p.parent], name) {
			return p.pid
		}
	}
	return 0
}

// listener hears, into its tap, the processes that play what the engine
// doesn't.
type listener struct {
	tap *audio.Tap
	// outside is set while what plays plays outside the engine: the
	// analyser then reads the tap.
	outside atomic.Bool

	wake chan struct{}
	mu   sync.Mutex
	want hearing

	// find finds the process whose children play what is heard.
	find func(hearing) uint32
	// hearing is what capture hears, which run owns, as failed, set once a
	// capture failed and said why.
	hearing hearing
	capture *output.Capture
	failed  bool
}

func newListener() *listener { return listenFinding(hearing.process) }

// listenFinding returns a listener that finds what it hears with find.
func listenFinding(find func(hearing) uint32) *listener {
	l := &listener{tap: audio.NewTap(), wake: make(chan struct{}, 1), find: find}
	go l.run()
	return l
}

// follow has the listener hear what the player's state plays outside the
// engine. It returns at once: the listener starts and stops captures on
// its own goroutine.
func (l *listener) follow(st player.State) {
	l.outside.Store(st.Source != nil && st.Source.Mode != player.ModeStream)
	l.mu.Lock()
	l.want = heard(st)
	l.mu.Unlock()
	select {
	case l.wake <- struct{}{}:
	default:
	}
}

func (l *listener) run() {
	for range l.wake {
		l.mu.Lock()
		want := l.want
		l.mu.Unlock()
		if want == l.hearing {
			continue
		}
		if l.capture != nil {
			l.capture.Stop()
			l.capture = nil
		}
		l.tap.Silence()
		l.hearing = want
		pid := l.find(want)
		if pid == 0 {
			continue
		}
		c, err := output.CaptureProcess(pid, l.tap.Write)
		if err != nil {
			if !l.failed {
				l.failed = true
				log.Printf("mixmusic: the visualizer can't hear Spotify: %v", err)
			}
			continue
		}
		l.capture = c
	}
}
