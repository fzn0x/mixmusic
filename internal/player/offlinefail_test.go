package player

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/media"
)

// fileServices are keptServices whose kept tracks are found again without
// the network, as a file on this computer or a download is, unless the
// fake's streamErr fails them, as a file gone.
type fileServices struct{ keptServices }

func (s fileServices) PrepareStream(ctx context.Context, t media.Track) (Source, error) {
	if s.kept[t.ID] {
		return s.fakeServices.PrepareStream(ctx, t)
	}
	return s.keptServices.PrepareStream(ctx, t)
}

func newFileHarness(t *testing.T, net *fakeNet, setup func(*fakeServices), kept ...string) *harness {
	t.Helper()
	set := map[string]bool{}
	for _, id := range kept {
		set["yt:"+id] = true
	}
	if setup == nil {
		setup = func(*fakeServices) {}
	}
	return newHarnessWith(t, Saved{}, func(f *fakeServices) Services {
		return fileServices{keptServices{netServices{f, net}, set}}
	}, setup)
}

// TestKeptTrackFails tries a track that plays without the network once,
// at once, when its audio fails, and then moves on with why it failed,
// whether the network answers or not: waiting for it wouldn't help.
func TestKeptTrackFails(t *testing.T) {
	for _, up := range []bool{false, true} {
		network := map[bool]string{false: "offline", true: "online"}[up]
		t.Run(network+", can't be read", func(t *testing.T) {
			longWaits(t, 3) // a try that waits would never come
			h := newFileHarness(t, newNet(up), nil, "a", "b")
			h.p.PlayTracks(tracks("a", "b"), 0)
			h.wait("a playing", playing("a"))
			h.p.Time(42*time.Second, 200*time.Second)
			h.p.Playing(false)
			h.p.Error("Can't read the file")
			h.wait("a tried", func(p *Player) bool { return retries(h, "a") == 1 && p.loaded })
			if got := h.stream.lastLoad(); !strings.HasSuffix(got, " 42s true") {
				t.Errorf("tried with %q", got)
			}
			h.p.Error("Can't read the file")
			h.wait("b playing", playing("b"))
			if n := h.notices(); len(n) != 1 || n[0].Message != "Couldn't play Song a: Can't read the file" {
				t.Errorf("notices %+v", n)
			}
		})
		t.Run(network+", gone", func(t *testing.T) {
			longWaits(t, 3)
			h := newFileHarness(t, newNet(up), func(f *fakeServices) { f.streamErr = errors.New("File not found") }, "a", "b")
			h.p.PlayTracks(tracks("a", "b"), 0)
			h.wait("a playing", playing("a"))
			h.p.Time(42*time.Second, 200*time.Second)
			h.p.Playing(false)
			h.p.Error("the network failed while loading the audio")
			h.wait("b playing", playing("b"))
			if n := h.notices(); len(n) != 1 || n[0].Message != "Couldn't play Song a: File not found" {
				t.Errorf("notices %+v", n)
			}
			h.svc.with(func(f *fakeServices) {
				if f.streamed != 1 {
					t.Errorf("looked for the file %d times", f.streamed)
				}
			})
		})
	}
}

// TestRetryEndsBeforeItHolds has a track that plays without the network,
// tried again, that ends before it holds count as a try that failed: a
// file cut short as it plays, tried again, ends before where it stopped
// (#456). One that resumed near the track's end, or held, ends, as before,
// and so does a stream.
func TestRetryEndsBeforeItHolds(t *testing.T) {
	// breakAt has a's stream break off at, as a file cut short fails, and
	// waits for the try, which doesn't sound.
	breakAt := func(h *harness, at time.Duration) {
		t.Helper()
		h.p.PlayTracks(tracks("a", "b"), 0)
		h.wait("a playing", playing("a"))
		h.p.Time(at, 200*time.Second)
		h.stream.silence("a")
		h.p.Playing(false)
		h.p.Error("Can't read the file")
		h.wait("a tried", func(p *Player) bool { return retries(h, "a") == 1 && p.loaded })
	}
	t.Run("a file cut short", func(t *testing.T) {
		longWaits(t, 3)
		h := newFileHarness(t, newNet(true), nil, "a", "b")
		breakAt(h, 8*time.Second)
		// The try tells the cut file's length, as an MP3's, guessed from its
		// size, and a moment past where it resumed.
		h.p.Time(8*time.Second+50*time.Millisecond, 3*time.Second)
		h.p.Ended()
		h.wait("b playing", playing("b"))
		if n := h.notices(); len(n) != 1 || n[0].Message != "Couldn't play Song a: Can't read the file" {
			t.Errorf("notices %+v", n)
		}
	})
	t.Run("at the track's end", func(t *testing.T) {
		longWaits(t, 3)
		h := newFileHarness(t, newNet(true), nil, "a", "b")
		breakAt(h, 195*time.Second)
		h.p.Ended()
		h.wait("b playing", playing("b"))
		if n := h.notices(); len(n) != 0 {
			t.Errorf("notices %+v", n)
		}
	})
	t.Run("played on", func(t *testing.T) {
		longWaits(t, 3)
		h := newFileHarness(t, newNet(true), nil, "a", "b")
		breakAt(h, 8*time.Second)
		h.p.Playing(true)
		h.p.Time(8*time.Second+retryHold, 200*time.Second)
		h.p.Ended()
		h.wait("b playing", playing("b"))
		if n := h.notices(); len(n) != 0 {
			t.Errorf("notices %+v", n)
		}
	})
	t.Run("a stream", func(t *testing.T) {
		h := newFileHarness(t, newNet(true), nil, "b")
		h.p.PlayTracks(tracks("a", "b"), 0)
		h.wait("a playing", playing("a"))
		h.p.Time(42*time.Second, 200*time.Second)
		h.stream.silence("a")
		h.p.Playing(false)
		h.p.Error("the stream failed")
		h.wait("a tried", func(p *Player) bool { return retries(h, "a") == 1 && p.loaded })
		h.p.Ended()
		h.wait("b playing", playing("b"))
		if n := retries(h, "a"); n != 1 {
			t.Errorf("tried %d times", n)
		}
		if n := h.notices(); len(n) != 0 {
			t.Errorf("notices %+v", n)
		}
	})
}

// TestStreamBesideKeptWaits has a stream that breaks off while the
// network is gone wait for it, as before, beside tracks that play without
// it.
func TestStreamBesideKeptWaits(t *testing.T) {
	net := newNet(true)
	h := newFileHarness(t, net, nil, "b")
	h.p.PlayTracks(tracks("a", "b"), 0)
	h.wait("a playing", playing("a"))
	h.p.Time(42*time.Second, 200*time.Second)
	net.set(false)
	h.p.Playing(false)
	h.p.Error("the network failed while loading the audio")
	h.wait("a waiting", waiting("a", true))
	if n := retries(h, "a"); n != 0 {
		t.Errorf("tried %d times offline", n)
	}
	net.set(true)
	h.wait("a playing on", func(p *Player) bool { return h.stream.lastLoad() == "load url:yt:a 42s true" && playing("a")(p) })
}
