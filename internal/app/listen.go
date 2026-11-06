package app

// The analyser hears MixMusic's own engine. Spotify, when it plays in
// MixMusic, plays in the page of a webview, which the engine never hears,
// and on Spotify Connect in a Spotify app: the listener hears them there,
// for the visualizer and the plugins' analyser, while one of them reads it.
// Windows hears a process and its children through process loopback, and
// macOS from 14.2 on what a process plays through a process tap, of
// WebKit's GPU process, which plays what MixMusic's web views play, once
// the user allowed MixMusic to record what other apps play. It hears the
// Spotify app only on Windows, on this computer; other devices MixMusic
// can't hear.

import (
	"log"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/fzn0x/mixmusic/internal/audio"
	"github.com/fzn0x/mixmusic/internal/audio/output"
	"github.com/fzn0x/mixmusic/internal/player"
)

// unread is how long the listener goes on hearing after the analyser was
// last read: the visualizer reads it at each frame it draws.
const unread = 2 * time.Second

// recheck is how often the listener looks again whether the analyser is
// still read, and for the process to hear while it isn't found.
const recheck = time.Second

// hearing is what the listener hears.
type hearing int

const (
	hearNothing hearing = iota
	// hearMixMusic hears what MixMusic's web views play, Spotify in
	// MixMusic among them: webViewAudio's process.
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

// process returns the process that plays what h hears, or 0 for none, as
// while the Spotify app doesn't run.
func (h hearing) process() uint32 {
	switch h {
	case hearMixMusic:
		return webViewAudio(processes())
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

// capture is what the listener hears through, an output.Capture: Err tells
// why it stopped hearing before Stop.
type capture interface {
	Stop()
	Err() error
}

// listener hears, into its tap, the processes that play what the engine
// doesn't, while the analyser reads the tap.
type listener struct {
	tap *audio.Tap
	// outside is set while what plays plays outside the engine: the
	// analyser then reads the tap.
	outside atomic.Bool
	// read is when the analyser last read the tap, in Unix nanoseconds, and
	// unheard is set while what plays outside the engine goes unheard as
	// nothing reads it: a read then wakes the listener.
	read    atomic.Int64
	unheard atomic.Bool

	wake chan struct{}
	done chan struct{}
	mu   sync.Mutex
	want hearing
	// closed is set as MixMusic quits.
	closed bool

	// find finds the process that plays what is heard, and start starts
	// hearing it.
	find  func(hearing) uint32
	start func(pid uint32, write func([]float32)) (capture, error)
	// hearing is what capture hears, which run owns, as missing, set while
	// the process of hearing isn't found, and failed, set once a capture
	// failed and said why.
	hearing hearing
	capture capture
	missing bool
	failed  bool
}

func newListener() *listener { return listenFinding(hearing.process) }

// listenFinding returns a listener that finds what it hears with find.
func listenFinding(find func(hearing) uint32) *listener { return listenWith(find, captureProcess) }

// listenWith returns a listener that finds what it hears with find, and
// starts hearing it with start.
func listenWith(find func(hearing) uint32, start func(uint32, func([]float32)) (capture, error)) *listener {
	l := &listener{tap: audio.NewTap(), wake: make(chan struct{}, 1), done: make(chan struct{}), find: find, start: start}
	go l.run()
	return l
}

// captureProcess starts hearing pid through an output.Capture.
func captureProcess(pid uint32, write func([]float32)) (capture, error) {
	c, err := output.CaptureProcess(pid, write)
	if err != nil {
		return nil, err
	}
	return c, nil
}

// follow has the listener hear what the player's state plays outside the
// engine. It returns at once: the listener starts and stops captures on
// its own goroutine.
func (l *listener) follow(st player.State) {
	l.outside.Store(st.Source != nil && st.Source.Mode != player.ModeStream)
	l.mu.Lock()
	l.want = heard(st)
	l.mu.Unlock()
	l.poke()
}

// analyse writes what the tap heard into out, for the visualizer or a
// plugin: the listener hears while they read it.
func (l *listener) analyse(out *audio.Analysis) {
	l.read.Store(time.Now().UnixNano())
	if l.unheard.Load() {
		l.poke()
	}
	l.tap.Analyse(out)
}

// reading reports whether the analyser read the tap lately.
func (l *listener) reading() bool {
	return time.Since(time.Unix(0, l.read.Load())) < unread
}

// close stops the listener's capture, as MixMusic quits, waiting a moment
// at most.
func (l *listener) close() {
	l.mu.Lock()
	l.closed = true
	l.mu.Unlock()
	l.poke()
	select {
	case <-l.done:
	case <-time.After(time.Second):
	}
}

func (l *listener) poke() {
	select {
	case l.wake <- struct{}{}:
	default:
	}
}

func (l *listener) run() {
	defer close(l.done)
	for {
		var again <-chan time.Time
		if l.capture != nil || l.missing {
			again = time.After(recheck)
		}
		select {
		case <-l.wake:
		case <-again:
		}
		l.mu.Lock()
		want, closed := l.want, l.closed
		l.mu.Unlock()
		if l.capture != nil {
			// As when the user refused it at macOS's question: it hears
			// again once what it hears changes.
			if err := l.capture.Err(); err != nil {
				l.capture.Stop()
				l.capture = nil
				l.fail(err)
			}
		}
		// While nothing reads the analyser, the listener hears nothing: on
		// macOS, where hearing asks the user's permission, it asks only
		// once the visualizer or a plugin would show what it hears.
		l.unheard.Store(want != hearNothing)
		if closed || !l.reading() {
			want = hearNothing
		} else {
			l.unheard.Store(false)
		}
		if want != l.hearing || l.missing {
			l.hear(want)
		}
		if closed {
			return
		}
	}
}

// hear stops hearing what the listener hears, and starts hearing want.
func (l *listener) hear(want hearing) {
	if l.capture != nil {
		l.capture.Stop()
		l.capture = nil
	}
	if want != l.hearing {
		l.tap.Silence()
	}
	l.hearing, l.missing = want, false
	if want == hearNothing {
		return
	}
	pid := l.find(want)
	if pid == 0 {
		l.missing = true
		return
	}
	c, err := l.start(pid, l.tap.Write)
	if err != nil {
		l.fail(err)
		return
	}
	l.capture = c
}

// fail says why the listener can't hear, once a run.
func (l *listener) fail(err error) {
	if !l.failed {
		l.failed = true
		log.Printf("mixmusic: the visualizer can't hear Spotify: %v", err)
	}
}
