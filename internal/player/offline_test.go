package player

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/media"
)

// fakeNet is the network as the app tells the player of it: it answers
// while up, and WaitOnline returns once it does.
type fakeNet struct {
	mu    sync.Mutex
	up    bool
	back  chan struct{} // closed while up
	kicks int
}

func newNet(up bool) *fakeNet {
	n := &fakeNet{back: make(chan struct{})}
	n.set(up)
	return n
}

func (n *fakeNet) set(up bool) {
	n.mu.Lock()
	defer n.mu.Unlock()
	switch {
	case up && !n.up:
		close(n.back)
	case !up && n.up:
		n.back = make(chan struct{})
	}
	n.up = up
}

func (n *fakeNet) Online(context.Context) bool {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.up
}

func (n *fakeNet) WaitOnline(ctx context.Context) error {
	n.mu.Lock()
	back := n.back
	n.mu.Unlock()
	select {
	case <-back:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (n *fakeNet) KickOnline() {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.kicks++
}

// netServices are the fake services with the network, which find no
// stream and no radio while it is down, as the app's don't: a Spotify
// Connect track needs no stream.
type netServices struct {
	*fakeServices
	net *fakeNet
}

func (s netServices) Online(ctx context.Context) bool      { return s.net.Online(ctx) }
func (s netServices) WaitOnline(ctx context.Context) error { return s.net.WaitOnline(ctx) }
func (s netServices) KickOnline()                          { s.net.KickOnline() }

// errNoNetwork is what the services fail with while the network is down.
var errNoNetwork = fmt.Errorf("%w: yt-dlp couldn't reach YouTube", ErrOffline)

func (s netServices) Prepare(ctx context.Context, t media.Track) (Source, error) {
	src, err := s.fakeServices.Prepare(ctx, t)
	if err == nil && src.Mode == ModeStream && !s.net.Online(ctx) {
		return Source{}, errNoNetwork
	}
	return src, err
}

func (s netServices) PrepareStream(ctx context.Context, t media.Track) (Source, error) {
	if !s.net.Online(ctx) {
		return Source{}, errNoNetwork
	}
	return s.fakeServices.PrepareStream(ctx, t)
}

func (s netServices) Radio(ctx context.Context, t media.Track) ([]media.Track, error) {
	if !s.net.Online(ctx) {
		return nil, errNoNetwork
	}
	return s.fakeServices.Radio(ctx, t)
}

func newNetHarness(t *testing.T, net *fakeNet, setup ...func(*fakeServices)) *harness {
	t.Helper()
	return newHarnessWith(t, Saved{}, func(f *fakeServices) Services { return netServices{f, net} }, setup...)
}

// waiting is the track id waiting for the network, to play once it
// answers when willPlay.
func waiting(id string, willPlay bool) func(*Player) bool {
	return func(p *Player) bool {
		cur, ok := p.s.Current()
		return ok && cur.Track.ID == "yt:"+id && p.s.Waiting && p.s.WillPlay == willPlay &&
			!p.s.Playing && !p.s.Loading && !p.loaded
	}
}

const offlineNotice = "No network. MixMusic plays on from here once it's back"

// TestWaitForTheNetwork has a track that can't be found for want of the
// network wait for it, without skipping on, and play once it's back.
func TestWaitForTheNetwork(t *testing.T) {
	net := newNet(false)
	h := newNetHarness(t, net)
	h.p.PlayTracks(tracks("a", "b", "c", "d"), 0)
	h.wait("a waiting", waiting("a", true))
	// It doesn't skip on, nor count as a failure.
	time.Sleep(10 * failDelay)
	h.on(func(p *Player) {
		if p.s.Index != 0 || !p.s.Waiting || p.failures != 0 || p.s.Error != "" {
			t.Errorf("waiting: index %d, waiting %v, %d failures, error %q", p.s.Index, p.s.Waiting, p.failures, p.s.Error)
		}
	})
	if n := h.notices(); len(n) != 1 || n[0].Message != offlineNotice || n[0].Error {
		t.Errorf("notices %+v", n)
	}
	net.set(true)
	h.wait("a playing", playing("a"))
	if got := h.stream.lastLoad(); got != "load url:yt:a 0s true" {
		t.Errorf("loaded %q", got)
	}
	if n := h.notices(); len(n) != 1 {
		t.Errorf("notices %+v", n)
	}
}

// TestToldOfEachLoss tells once that the network is gone, and again once
// it came back and went: also when the user left the wait for a track
// that plays without the network.
func TestToldOfEachLoss(t *testing.T) {
	net := newNet(false)
	h := newNetHarness(t, net, func(f *fakeServices) { f.prepare = onDevice })
	h.p.PlayTracks([]media.Track{track("a"), spotifyTrack("1"), track("b")}, 0)
	h.wait("a waiting", waiting("a", true))
	h.p.Next()
	h.wait("the device playing", func(p *Player) bool { return p.connect.active() && p.s.Playing })
	net.set(true)
	h.wait("the network back", func(p *Player) bool { return !p.toldOffline })
	net.set(false)
	h.p.Next()
	h.wait("b waiting", waiting("b", true))
	if n := h.notices(); len(n) != 2 || n[0].Message != offlineNotice || n[1].Message != offlineNotice {
		t.Errorf("notices %+v", n)
	}
}

// TestWaitMidTrack has a stream that broke off for want of the network
// wait for it where it was, rather than spending its tries.
func TestWaitMidTrack(t *testing.T) {
	net := newNet(true)
	h := newNetHarness(t, net)
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

// TestWaitingControls has play, pause, a seek and Next act on a track that
// waits for the network.
func TestWaitingControls(t *testing.T) {
	offline := func(t *testing.T) (*harness, *fakeNet) {
		net := newNet(false)
		h := newNetHarness(t, net)
		h.p.PlayTracks(tracks("a", "b"), 0)
		h.wait("a waiting", waiting("a", true))
		return h, net
	}
	t.Run("pause", func(t *testing.T) {
		h, net := offline(t)
		h.p.Pause()
		h.wait("a waiting paused", waiting("a", false))
		net.set(true)
		h.wait("a held", func(p *Player) bool { return h.stream.lastLoad() == "load url:yt:a 0s false" && p.loaded })
		h.on(func(p *Player) {
			if p.s.Playing || p.s.Waiting {
				t.Errorf("playing %v, waiting %v", p.s.Playing, p.s.Waiting)
			}
		})
	})
	t.Run("toggle", func(t *testing.T) {
		h, net := offline(t)
		h.p.Toggle()
		h.wait("a waiting paused", waiting("a", false))
		h.p.Toggle()
		h.wait("a waiting to play", waiting("a", true))
		net.mu.Lock()
		kicks := net.kicks
		net.mu.Unlock()
		if kicks != 1 {
			t.Errorf("play checked the network %d times", kicks)
		}
	})
	t.Run("seek", func(t *testing.T) {
		h, net := offline(t)
		h.p.Seek(30 * time.Second)
		h.wait("a waiting at 30s", func(p *Player) bool { return waiting("a", true)(p) && p.s.Position == 30*time.Second })
		net.set(true)
		h.wait("a playing from 30s", func(p *Player) bool { return h.stream.lastLoad() == "load url:yt:a 30s true" && playing("a")(p) })
	})
	t.Run("next", func(t *testing.T) {
		h, net := offline(t)
		h.p.Next()
		h.wait("b waiting", waiting("b", true))
		if n := h.notices(); len(n) != 1 {
			t.Errorf("notices %+v", n)
		}
		net.set(true)
		h.wait("b playing", playing("b"))
	})
}

// TestStreamErrorOnline tries a stream again, and then fails it, while the
// network answers.
func TestStreamErrorOnline(t *testing.T) {
	h := newNetHarness(t, newNet(true), func(f *fakeServices) { f.failure = "YouTube refused it" })
	h.p.PlayTracks(tracks("a", "b"), 0)
	h.wait("a playing", playing("a"))
	h.p.Time(42*time.Second, 200*time.Second)
	useRetries(t, h, "a", 0)
	h.p.Error("the stream failed")
	h.wait("b playing", playing("b"))
	if n := h.notices(); len(n) != 1 || n[0].Message != "Couldn't play Song a: YouTube refused it" {
		t.Errorf("notices %+v", n)
	}
}

// TestConnectWaitsForTheNetwork has a Spotify track whose device can't be
// reached for want of the network wait for it, rather than play from
// YouTube Music, which needs it too.
func TestConnectWaitsForTheNetwork(t *testing.T) {
	net := newNet(false)
	h := newNetHarness(t, net, func(f *fakeServices) { f.prepare = onDevice })
	h.device.mu.Lock()
	h.device.playErr = errors.New("Spotify can't be reached")
	h.device.mu.Unlock()
	h.p.PlayTracks([]media.Track{spotifyTrack("1")}, 0)
	h.wait("waiting", func(p *Player) bool { return p.s.Waiting && p.s.WillPlay })
	for _, n := range h.notices() {
		if strings.Contains(n.Message, "YouTube Music instead") {
			t.Errorf("fell back: %q", n.Message)
		}
	}
	h.device.mu.Lock()
	h.device.playErr = nil
	h.device.mu.Unlock()
	net.set(true)
	h.wait("the device playing", func(p *Player) bool { return p.connect.active() && p.s.Playing })
}

// TestWaitLeavesTheDevice has a track that waits for the network after a
// Spotify device's track wait at its start: the device pauses, and what it
// tells of its own track neither moves the track that waits nor ends it.
func TestWaitLeavesTheDevice(t *testing.T) {
	connectTick, connectAskEvery, connectSettle = 5*time.Millisecond, 10*time.Millisecond, 0
	t.Cleanup(func() { connectTick, connectAskEvery, connectSettle = time.Second, 3*time.Second, 5*time.Second })
	found := make(chan struct{})
	release := sync.OnceFunc(func() { close(found) })
	net := newNet(true)
	h := newNetHarness(t, net, func(f *fakeServices) {
		f.prepare = func(t media.Track) (Source, error) {
			if t.ID == "yt:b" {
				<-found
			}
			return onDevice(t)
		}
	})
	t.Cleanup(release)
	sp := spotifyTrack("1")
	h.p.PlayTracks([]media.Track{sp, track("b"), track("c")}, 0)
	h.wait("the device playing", func(p *Player) bool { return p.connect.active() && p.s.Playing })
	net.set(false)
	h.p.Next()
	// While b loads, the device plays on, and tells it nears its end.
	h.device.set(&DeviceState{Playing: true, Progress: 197 * time.Second, Track: &sp})
	h.wait("the device near its end", func(p *Player) bool { return p.s.Index == 1 && p.s.Position >= 197*time.Second })
	release()
	h.wait("b waiting at its start", func(p *Player) bool { return waiting("b", true)(p) && p.s.Position == 0 })
	h.wait("the device paused", func(p *Player) bool { return h.device.has("pause") && !p.connect.active() })
	// The device's track ends, as Spotify plays it again from its start.
	h.device.set(&DeviceState{Playing: true, Progress: time.Second, Track: &sp})
	time.Sleep(50 * time.Millisecond)
	h.on(func(p *Player) {
		if !waiting("b", true)(p) || p.s.Position != 0 {
			t.Errorf("index %d, waiting %v, position %v", p.s.Index, p.s.Waiting, p.s.Position)
		}
	})
	net.set(true)
	h.wait("b playing from its start", func(p *Player) bool { return h.stream.lastLoad() == "load url:yt:b 0s true" && playing("b")(p) })
}

// TestRadioWaits has the radio, once the queue ran out, wait for the
// network, and play once it's back.
func TestRadioWaits(t *testing.T) {
	net := newNet(true)
	h := newNetHarness(t, net, func(f *fakeServices) {
		f.settings.Autoplay = true
		f.radio = []media.Track{track("x", "X"), track("y", "Y")}
	})
	h.p.PlayTracks(tracks("a"), 0)
	h.wait("a playing", playing("a"))
	net.set(false)
	h.p.Playing(false)
	h.p.Ended()
	h.wait("the radio waiting", func(p *Player) bool { return p.s.Waiting && p.radioWaits && p.atEnd && p.s.Index == 0 })
	net.set(true)
	h.wait("x playing", playing("x"))
}

// TestWokeRetries gives a stream its tries again once the computer woke.
func TestWokeRetries(t *testing.T) {
	h := newNetHarness(t, newNet(true))
	h.p.PlayTracks(tracks("a", "b"), 0)
	h.wait("a playing", playing("a"))
	h.p.Time(42*time.Second, 200*time.Second)
	useRetries(t, h, "a", 0)
	h.p.Woke()
	useRetries(t, h, "a", len(retryWaits))
}

// TestSleeping pauses what MixMusic plays itself as the computer goes to
// sleep, but not a Spotify Connect device elsewhere.
func TestSleeping(t *testing.T) {
	t.Run("stream", func(t *testing.T) {
		h := newNetHarness(t, newNet(true))
		h.p.PlayTracks(tracks("a"), 0)
		h.wait("a playing", playing("a"))
		h.p.Sleeping()
		h.wait("a paused", func(p *Player) bool { return !p.s.Playing && h.stream.has("pause") })
	})
	t.Run("loading", func(t *testing.T) {
		found := make(chan struct{})
		release := sync.OnceFunc(func() { close(found) })
		h := newNetHarness(t, newNet(true), func(f *fakeServices) {
			f.prepare = func(t media.Track) (Source, error) {
				<-found
				return streamOf(t), nil
			}
		})
		t.Cleanup(release)
		h.p.PlayTracks(tracks("a"), 0)
		h.wait("a loading", func(p *Player) bool { return p.s.Loading })
		h.p.Sleeping()
		release()
		h.wait("a held", func(p *Player) bool { return h.stream.lastLoad() == "load url:yt:a 0s false" && p.loaded })
		h.on(func(p *Player) {
			if p.s.Playing || p.s.Loading {
				t.Errorf("playing %v, loading %v", p.s.Playing, p.s.Loading)
			}
		})
	})
	t.Run("waiting", func(t *testing.T) {
		h := newNetHarness(t, newNet(false))
		h.p.PlayTracks(tracks("a"), 0)
		h.wait("a waiting", waiting("a", true))
		h.p.Sleeping()
		h.wait("a waiting paused", waiting("a", false))
	})
	t.Run("connect", func(t *testing.T) {
		connectTick, connectAskEvery, connectSettle = 5*time.Millisecond, 10*time.Millisecond, 0
		t.Cleanup(func() { connectTick, connectAskEvery, connectSettle = time.Second, 3*time.Second, 5*time.Second })
		h := newNetHarness(t, newNet(true), func(f *fakeServices) { f.prepare = onDevice })
		h.p.PlayTracks([]media.Track{spotifyTrack("1")}, 0)
		h.wait("the device playing", func(p *Player) bool { return p.connect.active() && p.s.Playing })
		h.p.Sleeping()
		h.on(func(*Player) {})
		time.Sleep(50 * time.Millisecond)
		if h.device.has("pause") {
			t.Error("the device elsewhere paused")
		}
	})
}

// TestStallWaits has a stream that buffers for stallCheck while the
// network is gone wait for it.
func TestStallWaits(t *testing.T) {
	check := stallCheck
	stallCheck = 20 * time.Millisecond
	t.Cleanup(func() { stallCheck = check })
	net := newNet(true)
	h := newNetHarness(t, net)
	h.p.PlayTracks(tracks("a", "b"), 0)
	h.wait("a playing", playing("a"))
	h.p.Time(42*time.Second, 200*time.Second)
	net.set(false)
	h.p.Buffering(true)
	h.wait("a waiting", waiting("a", true))
	net.set(true)
	h.wait("a playing on", func(p *Player) bool { return h.stream.lastLoad() == "load url:yt:a 42s true" && playing("a")(p) })
}

// keptServices are netServices where the tracks of kept play without the
// network, as downloads do.
type keptServices struct {
	netServices
	kept map[string]bool
}

func (s keptServices) Offline() bool                   { return !s.net.Online(context.Background()) }
func (s keptServices) PlaysOffline(t media.Track) bool { return s.kept[t.ID] }

func (s keptServices) Prepare(ctx context.Context, t media.Track) (Source, error) {
	if s.kept[t.ID] {
		return s.fakeServices.Prepare(ctx, t)
	}
	return s.netServices.Prepare(ctx, t)
}

func newKeptHarness(t *testing.T, net *fakeNet, kept ...string) *harness {
	t.Helper()
	set := map[string]bool{}
	for _, id := range kept {
		set["yt:"+id] = true
	}
	return newHarnessWith(t, Saved{}, func(f *fakeServices) Services { return keptServices{netServices{f, net}, set} })
}

// reported are the tracks the app was told of as what plays, from the
// report from on.
func (h *harness) reported(from int) []string {
	var ids []string
	h.svc.with(func(f *fakeServices) {
		for _, r := range f.reports[min(from, len(f.reports)):] {
			if r.Track != nil && (len(ids) == 0 || ids[len(ids)-1] != r.Track.ID) {
				ids = append(ids, r.Track.ID)
			}
		}
	})
	return ids
}

func (h *harness) reports() int {
	n := 0
	h.svc.with(func(f *fakeServices) { n = len(f.reports) })
	return n
}

// TestOfflineSkipsToDownloaded skips, while the network is gone, the
// tracks that can't play without it, at once and without an error, to the
// next that plays, and waits once none is left.
func TestOfflineSkipsToDownloaded(t *testing.T) {
	net := newNet(false)
	h := newKeptHarness(t, net, "c")
	h.p.PlayTracks(tracks("a", "b", "c", "d"), 0)
	h.wait("c playing", playing("c"))
	h.on(func(p *Player) {
		if p.failures != 0 || p.s.Error != "" || p.s.Waiting {
			t.Errorf("skipping: %d failures, error %q, waiting %v", p.failures, p.s.Error, p.s.Waiting)
		}
	})
	if n := h.notices(); len(n) != 0 {
		t.Errorf("notices %+v", n)
	}
	if got := h.stream.with("load "); len(got) != 1 || got[0] != "load url:yt:c 0s true" {
		t.Errorf("loads %q", got)
	}
	// The app knows the network is gone by now: going on, the queue loads
	// none that can't play, and tells of none.
	from := h.reports()
	h.p.Ended()
	h.wait("d waiting", waiting("d", true))
	if got := h.reported(from); !slices.Equal(got, []string{"yt:d"}) {
		t.Errorf("told of %q", got)
	}
	if n := h.notices(); len(n) != 1 || n[0].Message != offlineNotice {
		t.Errorf("notices %+v", n)
	}
}

// TestOfflineAroundTheQueue goes around a queue that repeats, to the one
// track that plays, and from it to itself, without telling of the others.
func TestOfflineAroundTheQueue(t *testing.T) {
	net := newNet(false)
	h := newKeptHarness(t, net, "b")
	h.p.PlayTracks(tracks("a", "b", "c", "d"), 2)
	h.p.CycleRepeat() // all
	h.wait("b playing", playing("b"))
	from := h.reports()
	h.p.Ended()
	h.wait("b again", func(p *Player) bool { return playing("b")(p) && h.stream.count("load url:yt:b 0s true") == 2 })
	if got := h.reported(from); !slices.Equal(got, []string{"yt:b"}) {
		t.Errorf("told of %q", got)
	}
}

// TestOfflineNothingKept waits for the network, as before, when no track
// of the queue plays without it, without skipping around.
func TestOfflineNothingKept(t *testing.T) {
	net := newNet(false)
	h := newKeptHarness(t, net)
	h.p.PlayTracks(tracks("a", "b"), 0)
	h.p.CycleRepeat()
	h.wait("a waiting", waiting("a", true))
	if got := h.stream.with("load "); len(got) != 0 {
		t.Errorf("loads %q", got)
	}
}
