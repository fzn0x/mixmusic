package player

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/media"
)

// autoplayRadio turns autoplay on, with a radio of x and y.
func autoplayRadio(f *fakeServices) {
	f.settings.Autoplay = true
	f.radio = []media.Track{track("x", "X"), track("y", "Y")}
}

// queued waits until the queue is ids, as queueIDs tells it.
func (h *harness) queued(ids ...string) {
	h.t.Helper()
	h.wait("the queue "+strings.Join(ids, " "), func(p *Player) bool { return slices.Equal(queueIDs(p.s), ids) })
}

// autoplayOff turns autoplay off, as the app tells the player.
func (h *harness) autoplayOff() {
	h.svc.with(func(f *fakeServices) { f.settings.Autoplay = false })
	h.p.SettingsChanged(h.svc.Settings())
}

// gated is a harness whose radios wait to be let through (gatedRadio).
func gated(t *testing.T, setup ...func(*fakeServices)) (*harness, gatedRadio) {
	t.Helper()
	g := gatedRadio{asked: make(chan string, 4), gate: make(chan struct{})}
	h := newHarnessWith(t, Saved{}, func(f *fakeServices) Services { g.fakeServices = f; return g }, setup...)
	// A radio asked for and never let through ends with the test.
	t.Cleanup(func() { close(g.gate) })
	return h, g
}

// askedFor waits for the radio g is asked for, and returns what it starts
// from.
func askedFor(t *testing.T, g gatedRadio) string {
	t.Helper()
	select {
	case seed := <-g.asked:
		return seed
	case <-time.After(5 * time.Second):
		t.Fatal("no radio was asked for")
		return ""
	}
}

// TestRadioAhead asks for autoplay's radio as the last track plays, once,
// so that its first track is found and downloaded before it ends, and
// plays as it ends.
func TestRadioAhead(t *testing.T) {
	h, seeds := newSeedHarness(t, autoplayRadio)
	h.p.PlayTracks(tracks("a"), 0)
	h.wait("a playing", playing("a"))
	if got := queueIDs(h.p.State()); !slices.Equal(got, []string{"a"}) || len(seeds()) != 0 {
		t.Fatalf("before a played a while, the queue is %v, radios of %v", got, seeds())
	}
	h.p.Time(2*time.Second, 200*time.Second)
	h.queued("a", "x*", "y*")
	if cur := current(h.p.State()); cur != "a" {
		t.Errorf("the radio moved the queue to %s", cur)
	}
	h.p.Time(11*time.Second, 200*time.Second)
	h.wait("x preloaded", func(*Player) bool { return h.stream.has("preload url:yt:x") })
	h.p.Ended()
	h.wait("x playing", playing("x"))
	if got := h.stream.lastLoad(); got != "load url:yt:x 0s true" {
		t.Errorf("x loaded as %q", got)
	}
	if got := seeds(); !slices.Equal(got, []string{"a"}) {
		t.Errorf("radios of %v", got)
	}
}

// TestRadioAheadCrossfade fades into the first track of autoplay's radio,
// as into any next track.
func TestRadioAheadCrossfade(t *testing.T) {
	h := newHarness(t, Saved{}, autoplayRadio, func(f *fakeServices) { f.settings.Crossfade = 5 })
	h.p.PlayTracks(tracks("a"), 0)
	h.wait("a playing", playing("a"))
	h.p.Time(2*time.Second, 200*time.Second)
	h.queued("a", "x*", "y*")
	h.p.Time(196*time.Second, 200*time.Second)
	h.wait("x fading in", playing("x"))
	if got := h.stream.with("crossfade"); !slices.Equal(got, []string{"crossfade url:yt:x 5s"}) {
		t.Errorf("crossfaded %q", got)
	}
	if got := queueIDs(h.p.State()); !slices.Equal(got, []string{"a", "x*", "y*"}) {
		t.Errorf("the queue is %v", got)
	}
}

// TestRadioAheadAwaited has the end of the last track wait for the radio
// asked for as it played, rather than ask again.
func TestRadioAheadAwaited(t *testing.T) {
	h, g := gated(t, autoplayRadio)
	h.p.PlayTracks(tracks("a"), 0)
	h.wait("a playing", playing("a"))
	h.p.Time(2*time.Second, 200*time.Second)
	if seed := askedFor(t, g); seed != "yt:a" {
		t.Fatalf("the radio starts from %s", seed)
	}
	h.p.Ended()
	h.on(func(p *Player) {
		if p.radioAhead == nil || p.radioAhead.then == nil {
			t.Errorf("the end of a doesn't wait for the radio asked for: %+v", p.radioAhead)
		}
	})
	g.gate <- struct{}{}
	h.wait("x playing", playing("x"))
	h.queued("a", "x*", "y*")
	select {
	case seed := <-g.asked:
		t.Errorf("a radio of %s was asked for again", seed)
	default:
	}
}

// TestRadioAheadDropped drops the radio found ahead, while none of it
// played, once the queue doesn't carry on into it.
func TestRadioAheadDropped(t *testing.T) {
	t.Run("autoplay off", func(t *testing.T) {
		h, seeds := newSeedHarness(t, autoplayRadio)
		h.p.PlayTracks(tracks("a"), 0)
		h.wait("a playing", playing("a"))
		h.p.Time(2*time.Second, 200*time.Second)
		h.queued("a", "x*", "y*")
		h.autoplayOff()
		h.queued("a")
		h.p.Ended()
		h.wait("stopped at the end", func(p *Player) bool { return p.atEnd && !p.s.Playing && p.s.Index == 0 })
		if got := seeds(); !slices.Equal(got, []string{"a"}) {
			t.Errorf("radios of %v", got)
		}
	})

	t.Run("autoplay off while asked", func(t *testing.T) {
		h, g := gated(t, autoplayRadio)
		h.p.PlayTracks(tracks("a"), 0)
		h.wait("a playing", playing("a"))
		h.p.Time(2*time.Second, 200*time.Second)
		askedFor(t, g)
		var a *radioAhead
		h.on(func(p *Player) { a = p.radioAhead })
		h.autoplayOff()
		g.gate <- struct{}{}
		h.wait("the radio's answer", func(*Player) bool { return a != nil && !a.asking })
		if got := queueIDs(h.p.State()); !slices.Equal(got, []string{"a"}) {
			t.Errorf("the queue is %v", got)
		}
		h.p.Ended()
		h.wait("stopped at the end", func(p *Player) bool { return p.atEnd && !p.s.Playing && p.s.Index == 0 })
	})

	t.Run("queue replaced while asked", func(t *testing.T) {
		h, g := gated(t, autoplayRadio)
		h.p.PlayTracks(tracks("a"), 0)
		h.wait("a playing", playing("a"))
		h.p.Time(2*time.Second, 200*time.Second)
		askedFor(t, g)
		var a *radioAhead
		h.on(func(p *Player) { a = p.radioAhead })
		h.p.PlayTracks(tracks("c"), 0)
		h.wait("c playing", playing("c"))
		g.gate <- struct{}{}
		h.wait("the radio's answer", func(*Player) bool { return a != nil && !a.asking })
		if got := queueIDs(h.p.State()); !slices.Equal(got, []string{"c"}) {
			t.Errorf("the radio of a went into the queue played after it: %v", got)
		}
	})

	t.Run("repeat", func(t *testing.T) {
		h, seeds := newSeedHarness(t, autoplayRadio)
		h.p.PlayTracks(tracks("a"), 0)
		h.wait("a playing", playing("a"))
		h.p.Time(2*time.Second, 200*time.Second)
		h.queued("a", "x*", "y*")
		h.p.CycleRepeat()
		h.queued("a")
		// Repeat off again: the radio is asked for again as a plays on.
		h.p.CycleRepeat()
		h.p.CycleRepeat()
		h.p.Time(3*time.Second, 200*time.Second)
		h.queued("a", "x*", "y*")
		if got := seeds(); !slices.Equal(got, []string{"a", "a"}) {
			t.Errorf("radios of %v", got)
		}
	})

	t.Run("playing", func(t *testing.T) {
		h := newHarness(t, Saved{}, autoplayRadio)
		h.p.PlayTracks(tracks("a"), 0)
		h.wait("a playing", playing("a"))
		h.p.Time(2*time.Second, 200*time.Second)
		h.queued("a", "x*", "y*")
		h.p.Ended()
		h.wait("x playing", playing("x"))
		h.autoplayOff()
		h.on(func(*Player) {})
		if got := queueIDs(h.p.State()); !slices.Equal(got, []string{"a", "x*", "y*"}) {
			t.Errorf("the radio that plays left the queue: %v", got)
		}
	})
}
