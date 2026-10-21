package app

// The analyser hears MixMusic's own engine. Spotify, when it plays in
// MixMusic, plays in the page of a webview, which the engine never hears:
// on Windows, the listener hears it there, through process loopback, for
// the visualizer and the plugins' analyser.

import (
	"log"
	"os"
	"sync"
	"sync/atomic"

	"github.com/fzn0x/mixmusic/internal/audio"
	"github.com/fzn0x/mixmusic/internal/audio/output"
	"github.com/fzn0x/mixmusic/internal/player"
)

// listener hears, into its tap, the processes that play what the engine
// doesn't.
type listener struct {
	tap *audio.Tap
	// outside is set while what plays plays outside the engine: the
	// analyser then reads the tap.
	outside atomic.Bool

	wake chan struct{}
	mu   sync.Mutex
	want uint32 // the process whose tree to hear, 0 for none

	// pid is the process whose tree capture hears, which run owns, as
	// failed, set once a capture failed and said why.
	pid     uint32
	capture *output.Capture
	failed  bool
}

func newListener() *listener {
	l := &listener{tap: audio.NewTap(), wake: make(chan struct{}, 1)}
	go l.run()
	return l
}

// follow has the listener hear what the player's state plays outside the
// engine. It returns at once.
func (l *listener) follow(st player.State) {
	l.outside.Store(st.Source != nil && st.Source.Mode != player.ModeStream)
	l.hear(heardProcess(st))
}

// heardProcess is the process whose tree plays what st plays outside the
// engine, as far as MixMusic can hear it, or 0: MixMusic's own, whose
// webview plays Spotify, while Spotify plays in MixMusic.
func heardProcess(st player.State) uint32 {
	if !st.Playing || st.Source == nil {
		return 0
	}
	switch st.Source.Mode {
	case player.ModeSpotify:
		return uint32(os.Getpid())
	}
	return 0
}

// hear has the listener hear the tree of the process pid, or nothing for 0.
// It returns at once: the listener starts and stops captures on its own
// goroutine.
func (l *listener) hear(pid uint32) {
	l.mu.Lock()
	l.want = pid
	l.mu.Unlock()
	select {
	case l.wake <- struct{}{}:
	default:
	}
}

func (l *listener) run() {
	for range l.wake {
		l.mu.Lock()
		pid := l.want
		l.mu.Unlock()
		if pid == l.pid {
			continue
		}
		if l.capture != nil {
			l.capture.Stop()
			l.capture = nil
		}
		l.tap.Silence()
		l.pid = pid
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
