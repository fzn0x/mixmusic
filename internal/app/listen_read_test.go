package app

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/audio"
	"github.com/fzn0x/mixmusic/internal/player"
)

// fakeCaptures are the captures a listener started, of fakeCapture.
type fakeCaptures struct {
	mu      sync.Mutex
	started []*fakeCapture
	pids    []uint32
	err     error
}

type fakeCapture struct {
	stopped atomic.Bool
	err     atomic.Pointer[error]
}

func (c *fakeCapture) Stop() { c.stopped.Store(true) }

func (c *fakeCapture) Err() error {
	if p := c.err.Load(); p != nil {
		return *p
	}
	return nil
}

func (f *fakeCaptures) start(pid uint32, _ func([]float32)) (capture, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.pids = append(f.pids, pid)
	if f.err != nil {
		return nil, f.err
	}
	c := &fakeCapture{}
	f.started = append(f.started, c)
	return c, nil
}

// last returns the captures started, and the last one, nil for none.
func (f *fakeCaptures) last() (int, *fakeCapture) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.started) == 0 {
		return 0, nil
	}
	return len(f.started), f.started[len(f.started)-1]
}

func (f *fakeCaptures) tries() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.pids)
}

var spotifyPlays = player.State{Playing: true, Source: &player.Source{Mode: player.ModeSpotify}}

// TestListenerHearsWhileRead hears Spotify in MixMusic only while the
// analyser is read, as the visualizer reads it at each frame, and stops
// once it isn't: on macOS, hearing asks the user's permission.
func TestListenerHearsWhileRead(t *testing.T) {
	var f fakeCaptures
	l := listenWith(func(h hearing) uint32 {
		if h == hearMixMusic {
			return 42
		}
		return 0
	}, f.start)
	defer l.close()
	l.follow(spotifyPlays)
	time.Sleep(50 * time.Millisecond)
	if n, _ := f.last(); n != 0 {
		t.Fatal("heard Spotify while nothing read the analyser")
	}
	var a audio.Analysis
	l.analyse(&a)
	eventually(t, "hearing Spotify once the analyser was read", func() bool { n, _ := f.last(); return n == 1 })
	f.mu.Lock()
	if f.pids[0] != 42 {
		t.Errorf("heard process %d, want 42", f.pids[0])
	}
	f.mu.Unlock()
	// The reads stop.
	l.read.Store(time.Now().Add(-time.Hour).UnixNano())
	l.follow(spotifyPlays)
	_, c := f.last()
	eventually(t, "stopping once nothing read the analyser", c.stopped.Load)
	l.analyse(&a)
	eventually(t, "hearing again at a read after a while", func() bool { n, _ := f.last(); return n == 2 })
	l.follow(player.State{Source: spotifyPlays.Source}) // paused
	_, c = f.last()
	eventually(t, "stopping once Spotify paused", c.stopped.Load)
}

// TestListenerLooksAgain looks again for the process to hear while it
// isn't found, as before WebKit's GPU process plays.
func TestListenerLooksAgain(t *testing.T) {
	var f fakeCaptures
	var looked atomic.Int32
	l := listenWith(func(h hearing) uint32 {
		if h == hearMixMusic && looked.Add(1) > 1 {
			return 42
		}
		return 0
	}, f.start)
	defer l.close()
	l.follow(spotifyPlays)
	var a audio.Analysis
	l.analyse(&a)
	eventually(t, "looking again for the process", func() bool { n, _ := f.last(); return n == 1 })
}

// TestListenerFailsOnce says once why it can't hear, and tries again only
// when what it hears changes, not at each read.
func TestListenerFailsOnce(t *testing.T) {
	f := fakeCaptures{err: errors.New("refused")}
	l := listenWith(func(hearing) uint32 { return 42 }, f.start)
	l.follow(spotifyPlays)
	var a audio.Analysis
	l.analyse(&a)
	eventually(t, "trying to hear", func() bool { return f.tries() == 1 })
	for range 5 {
		l.analyse(&a)
		l.follow(spotifyPlays)
		time.Sleep(10 * time.Millisecond)
	}
	if n := f.tries(); n != 1 {
		t.Errorf("tried %d times as the analyser was read", n)
	}
	l.close() // the listener's goroutine, which owns failed, ends
	if !l.failed {
		t.Error("the failure wasn't noted")
	}
}

// TestListenerSaysWhyItStopped stops a capture that stopped hearing, as
// when the user refused it at macOS's question, and says why.
func TestListenerSaysWhyItStopped(t *testing.T) {
	var f fakeCaptures
	l := listenWith(func(hearing) uint32 { return 42 }, f.start)
	l.follow(spotifyPlays)
	var a audio.Analysis
	l.analyse(&a)
	eventually(t, "hearing", func() bool { n, _ := f.last(); return n == 1 })
	_, c := f.last()
	refused := errors.New("refused")
	c.err.Store(&refused)
	eventually(t, "stopping the capture that stopped hearing", c.stopped.Load)
	l.analyse(&a)
	l.close()
	if !l.failed {
		t.Error("the failure wasn't noted")
	}
	if n, _ := f.last(); n != 1 {
		t.Errorf("heard again %d times, with nothing changed", n-1)
	}
}

// TestListenerClose stops hearing as MixMusic quits.
func TestListenerClose(t *testing.T) {
	var f fakeCaptures
	l := listenWith(func(hearing) uint32 { return 42 }, f.start)
	l.follow(spotifyPlays)
	var a audio.Analysis
	l.analyse(&a)
	eventually(t, "hearing", func() bool { n, _ := f.last(); return n == 1 })
	l.close()
	if _, c := f.last(); !c.stopped.Load() {
		t.Error("the capture runs on once the listener closed")
	}
	select {
	case <-l.done:
	default:
		t.Error("the listener's goroutine runs on")
	}
}
