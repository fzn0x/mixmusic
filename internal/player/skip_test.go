package player

import (
	"context"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/media"
)

// disliking has the fake services dislike the tracks ids, as the app's
// library keeps them.
func disliking(ids ...string) func(*fakeServices) {
	return func(f *fakeServices) {
		f.disliked = map[string]bool{}
		for _, id := range ids {
			f.disliked["yt:"+id] = true
		}
	}
}

// skips is which entries of the queue it passes over, as queueIDs names
// them.
func skips(s State) []string {
	var out []string
	for _, it := range s.Queue {
		if it.Skip != "" {
			out = append(out, strings.TrimPrefix(it.Track.ID, "yt:"))
		}
	}
	return out
}

// loads is the tracks the engine loaded, without "url:yt:".
func (h *harness) loads() []string {
	var out []string
	for _, c := range h.stream.with("load ") {
		out = append(out, strings.TrimPrefix(strings.Fields(c)[1], "url:yt:"))
	}
	return out
}

// TestSkipDisliked passes over the disliked entries of the queue as it
// moves on, forward and back and as it repeats, but plays one the user
// chose.
func TestSkipDisliked(t *testing.T) {
	h := newHarness(t, Saved{}, disliking("b", "c"))
	h.p.PlayTracks(tracks("a", "b", "c", "d"), 0)
	h.wait("a playing", playing("a"))
	if got := skips(h.p.State()); !slices.Equal(got, []string{"b", "c"}) {
		t.Fatalf("the queue passes over %v", got)
	}
	if next, _ := h.p.State().UpNext(); next.Track.ID != "yt:d" {
		t.Errorf("up next is %s", next.Track.ID)
	}
	h.p.Next()
	h.wait("d playing", playing("d"))
	h.p.Previous()
	h.wait("a playing again", playing("a"))
	h.p.Ended()
	h.wait("d playing at a's end", playing("d"))
	// At the end, with the rest passed over, the queue stops.
	h.p.Ended()
	h.wait("stopping at d", func(p *Player) bool { return !p.s.Playing && p.s.Index == 3 && p.atEnd })
	// Repeating, it starts over past them.
	h.on(func(p *Player) { p.s.Repeat = RepeatAll })
	h.p.Ended()
	h.wait("a playing as it repeats", playing("a"))
	h.p.Previous()
	h.wait("d playing, back from a", playing("d"))
	if got := h.loads(); slices.Contains(got, "b") || slices.Contains(got, "c") {
		t.Errorf("the engine loaded %v", got)
	}

	// The user's own choice plays, and the queue moves on from it.
	h.p.PlayTracks(tracks("a", "b", "c", "d"), 1)
	h.wait("b playing", playing("b"))
	h.p.Next()
	h.wait("d playing after b", playing("d"))
	h.p.PlayAt(2)
	h.wait("c playing, chosen in the queue", playing("c"))
	if got := skips(h.p.State()); len(got) != 0 {
		t.Errorf("with b and c chosen, the queue passes over %v", got)
	}
	// With repeat one, a disliked track chosen plays again as it ends.
	h.on(func(p *Player) { p.s.Repeat = RepeatOne })
	loads := h.stream.count("load")
	h.p.Ended()
	h.wait("c once more", func(*Player) bool { return h.stream.count("load") == loads+1 })
	if got := current(h.p.State()); got != "c" {
		t.Errorf("repeat one moved to %s", got)
	}
}

// TestSkipQueued plays a disliked track the user put in the queue by
// itself, and passes over one that came with others.
func TestSkipQueued(t *testing.T) {
	h := newHarness(t, Saved{}, disliking("x", "y"))
	h.p.PlayTracks(tracks("a", "b"), 0)
	h.wait("a playing", playing("a"))
	h.p.PlayNext(tracks("x"))
	h.p.Enqueue(tracks("y", "c"))
	h.p.Enqueue(tracks("y"))
	h.on(func(*Player) {})
	if got, want := queueIDs(h.p.State()), []string{"a", "x", "b", "y", "c", "y"}; !slices.Equal(got, want) {
		t.Fatalf("queue %v, want %v", got, want)
	}
	if got := skips(h.p.State()); !slices.Equal(got, []string{"y"}) {
		t.Errorf("the queue passes over %v", got)
	}
	for _, want := range []string{"x", "b", "c", "y"} {
		h.next()
		if got := current(h.p.State()); got != want {
			t.Fatalf("played %s, want %s", got, want)
		}
	}
}

// TestSkipIntoEmptyQueue plays the first track the queue doesn't pass over
// when tracks go into an empty one.
func TestSkipIntoEmptyQueue(t *testing.T) {
	h := newHarness(t, Saved{}, disliking("a"))
	h.p.Enqueue(tracks("a", "b"))
	h.p.Play()
	h.wait("b playing", playing("b"))

	h = newHarness(t, Saved{}, disliking("a"))
	h.p.PlayNext(tracks("a", "b"))
	h.wait("b playing", playing("b"))
}

// TestSkipPrefetchAndCrossfade gets the track that really plays next
// ready, and crossfades into it.
func TestSkipPrefetchAndCrossfade(t *testing.T) {
	h := newHarness(t, Saved{}, disliking("b"), func(f *fakeServices) { f.settings.Crossfade = 5 })
	h.p.PlayTracks(tracks("a", "b", "c"), 0)
	h.wait("a playing", playing("a"))
	h.p.Time(11*time.Second, 200*time.Second)
	h.wait("c preloaded", func(*Player) bool { return h.stream.has("preload url:yt:c") })
	h.p.Time(196*time.Second, 200*time.Second)
	h.wait("c crossfading in", playing("c"))
	if got := h.stream.last("crossfade"); got != "crossfade url:yt:c 5s" {
		t.Errorf("crossfaded %q", got)
	}
	if h.stream.has("preload url:yt:b") {
		t.Error("b, passed over, was preloaded")
	}
	h.svc.with(func(f *fakeServices) {
		if slices.Contains(f.prepared, "yt:b") {
			t.Errorf("b, passed over, was prepared: %v", f.prepared)
		}
	})
}

// TestDislikeCurrent moves on from the track the user dislikes as it
// plays, playing or paused as it was, and undo goes back to it where it
// was.
func TestDislikeCurrent(t *testing.T) {
	t.Run("playing", func(t *testing.T) {
		h := newHarness(t, Saved{}, disliking())
		h.p.PlayTracks(tracks("a", "b", "c"), 0)
		h.wait("a playing", playing("a"))
		h.p.Time(42*time.Second, 200*time.Second)
		uid := h.p.State().Queue[0].UID
		h.svc.with(func(f *fakeServices) { f.disliked["yt:a"] = true })
		h.p.Disliked("yt:a", true)
		h.wait("b playing", playing("b"))
		if got := h.stream.lastLoad(); got != "load url:yt:b 0s true" {
			t.Errorf("loaded %q", got)
		}
		if got := skips(h.p.State()); !slices.Equal(got, []string{"a"}) {
			t.Errorf("the queue passes over %v", got)
		}
		// Undo plays it on from where it was.
		h.svc.with(func(f *fakeServices) { delete(f.disliked, "yt:a") })
		h.p.Disliked("yt:a", false)
		h.p.Return(uid, 42*time.Second, true)
		h.wait("a playing again", playing("a"))
		if got := h.stream.lastLoad(); got != "load url:yt:a 42s true" {
			t.Errorf("undo loaded %q", got)
		}
		if got := skips(h.p.State()); len(got) != 0 {
			t.Errorf("after undo, the queue passes over %v", got)
		}
	})

	t.Run("paused", func(t *testing.T) {
		h := newHarness(t, Saved{}, disliking())
		h.p.PlayTracks(tracks("a", "b", "c"), 0)
		h.wait("a playing", playing("a"))
		h.p.Pause()
		h.wait("a paused", func(p *Player) bool { return !p.s.Playing })
		h.p.Disliked("yt:a", true)
		h.wait("b held", func(*Player) bool { return h.stream.lastLoad() == "load url:yt:b 0s false" })
		h.on(func(p *Player) {
			if p.s.Index != 1 || p.s.Playing || p.s.Loading {
				t.Errorf("b: index %d, playing %v, loading %v", p.s.Index, p.s.Playing, p.s.Loading)
			}
		})
		uid := h.p.State().Queue[0].UID
		h.p.Disliked("yt:a", false)
		h.p.Return(uid, 30*time.Second, false)
		h.wait("a held again", func(*Player) bool { return h.stream.lastLoad() == "load url:yt:a 30s false" })
		if s := h.p.State(); s.Index != 0 || s.Playing {
			t.Errorf("after undo: index %d, playing %v", s.Index, s.Playing)
		}
	})

	t.Run("last", func(t *testing.T) {
		h := newHarness(t, Saved{}, disliking())
		h.p.PlayTracks(tracks("a", "b"), 1)
		h.wait("b playing", playing("b"))
		h.p.Time(20*time.Second, 200*time.Second)
		uid := h.p.State().Queue[1].UID
		h.p.Disliked("yt:b", true)
		h.wait("stopping", func(p *Player) bool {
			return p.s.Index == 1 && !p.s.Playing && !p.loaded && p.atEnd && p.s.Position == 0
		})
		if !h.stream.has("unload") {
			t.Error("the engine kept b")
		}
		// Undo plays it on from where it was.
		h.p.Disliked("yt:b", false)
		h.p.Return(uid, 20*time.Second, true)
		h.wait("b playing again", playing("b"))
		if got := h.stream.lastLoad(); got != "load url:yt:b 20s true" {
			t.Errorf("undo loaded %q", got)
		}
	})

	t.Run("not skipping", func(t *testing.T) {
		h := newHarness(t, Saved{}, disliking(), func(f *fakeServices) { f.settings.SkipDisliked = false })
		h.p.PlayTracks(tracks("a", "b"), 0)
		h.wait("a playing", playing("a"))
		uid := h.p.State().Queue[0].UID
		h.p.Disliked("yt:a", true)
		h.p.Return(uid, 0, true)
		h.on(func(*Player) {})
		if got := h.stream.count("load"); got != 1 || current(h.p.State()) != "a" {
			t.Errorf("disliking a while not skipping: current %s, %d loads", current(h.p.State()), got)
		}
	})
}

// seedServices are the fake services, which keep the tracks radios start
// from.
type seedServices struct {
	*fakeServices
	mu    *sync.Mutex
	seeds *[]string
}

func (s seedServices) Radio(ctx context.Context, t media.Track) ([]media.Track, error) {
	s.mu.Lock()
	*s.seeds = append(*s.seeds, strings.TrimPrefix(t.ID, "yt:"))
	s.mu.Unlock()
	return s.fakeServices.Radio(ctx, t)
}

func newSeedHarness(t *testing.T, setup ...func(*fakeServices)) (*harness, func() []string) {
	var mu sync.Mutex
	var seeds []string
	h := newHarnessWith(t, Saved{}, func(f *fakeServices) Services { return seedServices{f, &mu, &seeds} }, setup...)
	return h, func() []string {
		mu.Lock()
		defer mu.Unlock()
		return slices.Clone(seeds)
	}
}

// TestSkipAutoplay has autoplay carry on past the disliked end of the
// queue, from the last track the user doesn't dislike, and leave out what
// they dislike.
func TestSkipAutoplay(t *testing.T) {
	autoplay := func(f *fakeServices) {
		f.settings.Autoplay = true
		f.radio = []media.Track{track("z", "Z"), track("x", "X"), track("y", "Y")}
	}
	t.Run("ended", func(t *testing.T) {
		h, seeds := newSeedHarness(t, disliking("b", "z"), autoplay)
		h.p.PlayTracks(tracks("a", "b"), 0)
		h.wait("a playing", playing("a"))
		h.p.Ended()
		h.wait("x playing", playing("x"))
		if got := queueIDs(h.p.State()); !slices.Equal(got, []string{"a", "b", "z*", "x*", "y*"}) {
			t.Errorf("queue %v", got)
		}
		if got := seeds(); !slices.Equal(got, []string{"a"}) {
			t.Errorf("the radio started from %v", got)
		}
	})

	t.Run("disliked as it plays", func(t *testing.T) {
		h, seeds := newSeedHarness(t, disliking(), autoplay)
		h.p.PlayTracks(tracks("a", "b"), 1)
		h.wait("b playing", playing("b"))
		h.p.Disliked("yt:b", true)
		h.wait("z playing", playing("z"))
		if got := seeds(); !slices.Equal(got, []string{"a"}) {
			t.Errorf("the radio started from %v, not a", got)
		}
	})

	t.Run("disliked paused", func(t *testing.T) {
		h, _ := newSeedHarness(t, disliking(), autoplay)
		h.p.PlayTracks(tracks("a", "b"), 1)
		h.wait("b playing", playing("b"))
		h.p.Pause()
		h.wait("b paused", func(p *Player) bool { return !p.s.Playing })
		h.p.Disliked("yt:b", true)
		h.wait("z held", func(*Player) bool { return h.stream.lastLoad() == "load url:yt:z 0s false" })
		if s := h.p.State(); s.Playing || current(s) != "z" {
			t.Errorf("after the dislike: %s, playing %v", current(s), s.Playing)
		}
	})

	t.Run("nothing to start from", func(t *testing.T) {
		h, seeds := newSeedHarness(t, disliking(), autoplay)
		h.p.PlayTracks(tracks("b"), 0)
		h.wait("b playing", playing("b"))
		h.p.Disliked("yt:b", true)
		h.wait("stopping", func(p *Player) bool { return !p.s.Playing && p.atEnd })
		h.on(func(*Player) {})
		if got := seeds(); len(got) != 0 || len(h.p.State().Queue) != 1 {
			t.Errorf("a radio started from %v", got)
		}
	})

	t.Run("a radio of disliked tracks", func(t *testing.T) {
		h, _ := newSeedHarness(t, disliking("x", "y", "z"), autoplay)
		h.p.PlayTracks(tracks("a"), 0)
		h.wait("a playing", playing("a"))
		h.p.Ended()
		h.wait("stopping", func(p *Player) bool { return len(p.s.Queue) == 4 && !p.s.Playing && p.atEnd && p.s.Index == 0 })
	})
}

// TestSkipSetting plays disliked tracks while the settings don't skip
// them, and passes over them once they do, but the one that plays.
func TestSkipSetting(t *testing.T) {
	h := newHarness(t, Saved{}, disliking("b", "c"), func(f *fakeServices) { f.settings.SkipDisliked = false })
	h.p.PlayTracks(tracks("a", "b", "c", "d"), 0)
	h.wait("a playing", playing("a"))
	h.next()
	if got := current(h.p.State()); got != "b" {
		t.Fatalf("not skipping, played %s", got)
	}
	loads := h.stream.count("load")
	h.svc.with(func(f *fakeServices) { f.settings.SkipDisliked = true })
	h.p.SettingsChanged(h.svc.Settings())
	h.on(func(*Player) {})
	if got := skips(h.p.State()); !slices.Equal(got, []string{"c"}) {
		t.Errorf("skipping, the queue passes over %v", got)
	}
	if s := h.p.State(); current(s) != "b" || h.stream.count("load") != loads {
		t.Errorf("turning skipping on moved to %s, loads %d", current(s), h.stream.count("load")-loads)
	}
	h.next()
	if got := current(h.p.State()); got != "d" {
		t.Errorf("skipping, played %s", got)
	}
	h.svc.with(func(f *fakeServices) { f.settings.SkipDisliked = false })
	h.p.SettingsChanged(h.svc.Settings())
	h.on(func(*Player) {})
	if got := skips(h.p.State()); len(got) != 0 {
		t.Errorf("not skipping again, the queue passes over %v", got)
	}
}

// TestSkipShuffle keeps the marks the queue gained while shuffled as
// shuffle goes off: a dislike, and an entry the user chose.
func TestSkipShuffle(t *testing.T) {
	h := newHarness(t, Saved{}, disliking("c"))
	h.p.PlayTracks(tracks("a", "b", "c", "d", "e"), 0)
	h.wait("a playing", playing("a"))
	h.p.SetShuffle(true)
	h.p.Disliked("yt:b", true)
	h.on(func(*Player) {})
	at := slices.IndexFunc(h.p.State().Queue, func(it Item) bool { return it.Track.ID == "yt:c" })
	h.p.PlayAt(at)
	h.wait("c playing", playing("c"))
	h.p.SetShuffle(false)
	h.on(func(*Player) {})
	s := h.p.State()
	if got := queueIDs(s); !slices.Equal(got, []string{"a", "b", "c", "d", "e"}) {
		t.Fatalf("unshuffled %v", got)
	}
	if got := skips(s); !slices.Equal(got, []string{"b"}) || !s.Queue[2].Chosen {
		t.Errorf("unshuffled, the queue passes over %v; c chosen %v", got, s.Queue[2].Chosen)
	}
}

// TestPlayAll plays a collection from its first track the user doesn't
// dislike, or shuffled from one of those, and tells when they dislike
// every one.
func TestPlayAll(t *testing.T) {
	h := newHarness(t, Saved{}, disliking("a", "b"))
	h.p.PlayAll(tracks("a", "b", "c", "d"), false)
	h.wait("c playing", playing("c"))
	if s := h.p.State(); s.Shuffle || len(s.Queue) != 4 {
		t.Errorf("played all: shuffle %v, %v", s.Shuffle, queueIDs(s))
	}
	for range 10 {
		h.p.PlayAll(tracks("a", "b", "c", "d"), true)
		h.on(func(*Player) {})
		h.wait("one of c and d playing", func(p *Player) bool {
			cur, _ := p.s.Current()
			return p.s.Playing && p.s.Source != nil && p.s.Source.Track.ID == cur.Track.ID
		})
		if s := h.p.State(); !s.Shuffle || s.Index != 0 || current(s) == "a" || current(s) == "b" {
			t.Fatalf("played shuffled: shuffle %v, from %s at %d", s.Shuffle, current(s), s.Index)
		}
	}
	loads := h.stream.count("load")
	h.p.PlayAll(tracks("a", "b"), false)
	h.wait("the notice", func(*Player) bool {
		n := h.notices()
		return len(n) == 1 && n[0].Settings && strings.Contains(n[0].Message, "dislike every song")
	})
	if s := h.p.State(); len(s.Queue) != 4 || h.stream.count("load") != loads {
		t.Errorf("a list of disliked tracks changed the queue to %v", queueIDs(s))
	}
}

// TestSkipRestore keeps across a restart which entries the user chose,
// and the current one plays on, disliked or not.
func TestSkipRestore(t *testing.T) {
	h := newHarness(t, Saved{}, disliking("b", "c"))
	h.p.PlayTracks(tracks("a", "b", "c", "d"), 0)
	h.wait("a playing", playing("a"))
	h.p.Enqueue(tracks("c"))
	h.wait("kept", h.kept("a", "b", "c", "d", "c"))
	saved := h.lastSaved()
	if !slices.Equal(saved.Chosen, []int{0, 4}) {
		t.Fatalf("kept chosen %v", saved.Chosen)
	}
	saved.Index = 1
	h = newHarness(t, saved, disliking("b", "c"))
	if got := skips(h.p.State()); !slices.Equal(got, []string{"c"}) {
		t.Errorf("restored, the queue passes over %v", got)
	}
	h.p.Play()
	h.wait("b playing on", playing("b"))
	for _, want := range []string{"d", "c"} {
		h.next()
		if got := current(h.p.State()); got != want {
			t.Fatalf("played %s, want %s", got, want)
		}
	}
}
