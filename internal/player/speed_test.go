package player

import (
	"strings"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/media"
)

// atSpeedOf has the songs of the fake services play at speed.
func atSpeedOf(speed float64) func(*fakeServices) {
	return func(f *fakeServices) { f.settings.SongSpeed = speed }
}

// setSpeed sets the songs' speed in the settings, which the player takes
// in, as the app has it do.
func (h *harness) setSpeed(speed float64) {
	h.svc.with(func(f *fakeServices) { f.settings.SongSpeed = speed })
	h.p.SettingsChanged(h.svc.Settings())
}

func (h *harness) lastReport() Now {
	var now Now
	h.svc.with(func(f *fakeServices) { now = f.reports[len(f.reports)-1] })
	return now
}

// TestSpeedFromSettings plays a song at the settings' speed, tried again
// too.
func TestSpeedFromSettings(t *testing.T) {
	h := newHarness(t, Saved{}, atSpeedOf(1.5))
	h.p.PlayTracks(tracks("a", "b"), 0)
	h.wait("a playing", playing("a"))
	if got := h.stream.lastLoad(); got != "load url:yt:a 0s true speed 1.5" {
		t.Errorf("loaded %q", got)
	}
	if s := h.p.State(); s.Speed != 1.5 {
		t.Errorf("the state's speed %v", s.Speed)
	}
	if now := h.lastReport(); now.Speed != 1.5 {
		t.Errorf("reported a speed of %v", now.Speed)
	}
	h.p.Time(40*time.Second, 200*time.Second)
	h.p.Error("the stream failed")
	h.wait("a tried again", func(p *Player) bool { return retries(h, "a") == 1 && p.loaded })
	if got := h.stream.lastLoad(); !strings.HasSuffix(got, " 40s true speed 1.5") {
		t.Errorf("tried again with %q", got)
	}
}

// TestSpeedChangeApplies plays the track at a new speed of the settings at
// once, playing or paused, and the next track at it too.
func TestSpeedChangeApplies(t *testing.T) {
	h := newHarness(t, Saved{})
	changes := make(chan struct{}, 16)
	h.p.Subscribe(func(e Event) {
		if e == StateChange {
			select {
			case changes <- struct{}{}:
			default:
			}
		}
	})
	h.p.PlayTracks(tracks("a", "b"), 0)
	h.wait("a playing", playing("a"))
	if got := h.stream.lastLoad(); got != "load url:yt:a 0s true" {
		t.Errorf("loaded %q", got)
	}
	for len(changes) > 0 {
		<-changes
	}
	h.setSpeed(2)
	h.wait("a at 2×", func(p *Player) bool { return p.s.Speed == 2 && h.stream.has("set speed 2") })
	select {
	case <-changes:
	case <-time.After(5 * time.Second):
		t.Error("no state change told the speed")
	}
	if now := h.lastReport(); now.Speed != 2 {
		t.Errorf("reported a speed of %v", now.Speed)
	}
	// The same speed again changes nothing.
	h.setSpeed(2)
	h.on(func(*Player) {})
	if n := h.stream.count("set speed"); n != 1 {
		t.Errorf("%d changes of the speed: %q", n, h.stream.copy())
	}
	h.p.Pause()
	h.wait("pausing", func(p *Player) bool { return !p.s.Playing })
	h.setSpeed(0.75)
	h.wait("a at 0.75× while paused", func(p *Player) bool { return h.stream.has("set speed 0.75") && p.s.Speed == 0.75 })
	h.p.Next()
	h.wait("b playing", playing("b"))
	if got := h.stream.lastLoad(); got != "load url:yt:b 0s true speed 0.75" {
		t.Errorf("loaded %q", got)
	}
}

// TestSpeedNotOnSpotifyDevice plays a Spotify track on a Spotify device at
// its own speed, whatever the settings say.
func TestSpeedNotOnSpotifyDevice(t *testing.T) {
	connectTick, connectAskEvery, connectSettle = 5*time.Millisecond, 10*time.Millisecond, 0
	t.Cleanup(func() { connectTick, connectAskEvery, connectSettle = time.Second, 3*time.Second, 5*time.Second })
	h := newHarness(t, Saved{}, atSpeedOf(1.5), func(f *fakeServices) { f.prepare = onDevice })
	h.p.PlayTracks([]media.Track{spotifyTrack("1"), track("b", "B")}, 0)
	h.wait("the device playing", func(p *Player) bool { return p.connect.active() && p.s.Playing })
	if s := h.p.State(); s.Speed != 1 {
		t.Errorf("the device plays at %v", s.Speed)
	}
	h.setSpeed(2)
	h.on(func(*Player) {})
	if s := h.p.State(); s.Speed != 1 || h.stream.count("set speed") != 0 {
		t.Errorf("the device plays at %v; the stream did %q", s.Speed, h.stream.copy())
	}
	// The next track, a stream, plays at the songs' speed.
	h.p.Next()
	h.wait("b playing", playing("b"))
	if s := h.p.State(); s.Speed != 2 || h.stream.lastLoad() != "load url:yt:b 0s true speed 2" {
		t.Errorf("b plays at %v, loaded %q", s.Speed, h.stream.lastLoad())
	}
}

// TestCrossfadeAtSpeed fades over the seconds heard: at 2×, a fade of 5 s
// starts 10 s of the track before its end.
func TestCrossfadeAtSpeed(t *testing.T) {
	h := newHarness(t, Saved{}, atSpeedOf(2), func(f *fakeServices) { f.settings.Crossfade = 5 })
	h.p.PlayTracks(tracks("a", "b"), 0)
	h.wait("a playing", playing("a"))
	h.p.Time(188*time.Second, 200*time.Second)
	h.on(func(*Player) {})
	time.Sleep(20 * time.Millisecond)
	h.on(func(*Player) {})
	if n := h.stream.count("crossfade"); n != 0 {
		t.Errorf("12 s before the end, 6 s at 2×, it crossfaded: %q", h.stream.copy())
	}
	h.p.Time(190*time.Second, 200*time.Second)
	h.wait("b crossfading in", playing("b"))
	if got := h.stream.last("crossfade"); got != "crossfade url:yt:b 5s speed 2" {
		t.Errorf("crossfaded %q", got)
	}
}

// TestPreviousAtSpeed goes back to the track before, rather than to the
// start of this one, in the first 3 s heard of it.
func TestPreviousAtSpeed(t *testing.T) {
	h := newHarness(t, Saved{}, atSpeedOf(3))
	h.p.PlayTracks(tracks("a", "b"), 1)
	h.wait("b playing", playing("b"))
	// 6 s of the track are 2 s heard at 3×.
	h.p.Time(6*time.Second, 200*time.Second)
	h.p.Previous()
	h.wait("a playing", playing("a"))
	h.p.Next()
	h.wait("b playing again", playing("b"))
	// 12 s are 4 s.
	h.p.Time(12*time.Second, 200*time.Second)
	h.p.Previous()
	h.wait("b at its start", func(p *Player) bool { return h.stream.has("seek 0s") })
	if cur, _ := h.p.State().Current(); cur.Track.ID != "yt:b" {
		t.Errorf("went back to %s", cur.Track.ID)
	}
	// At 0.5×, 2 s of the track are 4 s heard.
	h.setSpeed(0.5)
	h.p.Time(2*time.Second, 200*time.Second)
	h.p.Previous()
	h.wait("b at its start again", func(p *Player) bool { return h.stream.count("seek 0s") == 2 })
}

// TestRepeatAllPreviousAtSpeed wraps to the last track only in the first
// 3 s heard of the first.
func TestRepeatAllPreviousAtSpeed(t *testing.T) {
	h := newHarness(t, Saved{}, atSpeedOf(3))
	h.p.PlayTracks(tracks("a", "b", "c"), 0)
	h.wait("a playing", playing("a"))
	h.p.CycleRepeat()
	h.p.Time(6*time.Second, 200*time.Second)
	h.p.Previous()
	h.wait("c playing", playing("c"))
}
