package player

import (
	"slices"
	"strings"
	"testing"

	"github.com/fzn0x/mixmusic/internal/media"
)

// hidingExplicit has the fake services' settings hide explicit content.
func hidingExplicit(f *fakeServices) { f.settings.HideExplicit = true }

// explicitTracks are tracks of ids whose IDs start with "x" explicit, as
// their service marks them.
func explicitTracks(ids ...string) []media.Track {
	out := tracks(ids...)
	for i := range out {
		out[i].Explicit = strings.HasPrefix(ids[i], "x")
	}
	return out
}

// toldExplicit reports whether a notice told that the settings hide
// explicit content.
func (h *harness) toldExplicit() bool {
	return slices.ContainsFunc(h.notices(), func(n Notice) bool {
		return strings.Contains(n.Message, "your settings hide explicit content") && n.Settings
	})
}

// TestHideExplicit passes over the explicit songs of the queue as it moves
// on, forward, back and round, without a word, as it does disliked ones.
func TestHideExplicit(t *testing.T) {
	h := newHarness(t, Saved{}, hidingExplicit)
	h.p.PlayTracks(explicitTracks("a", "x1", "b", "x2"), 0)
	h.wait("a playing", playing("a"))
	if got := skips(h.p.State()); !slices.Equal(got, []string{"x1", "x2"}) {
		t.Fatalf("the queue passes over %v", got)
	}
	for _, it := range h.p.State().Queue {
		if it.Skip != "" && it.Skip != SkipExplicit {
			t.Errorf("%s is passed over as %q", it.Track.ID, it.Skip)
		}
	}
	if next, _ := h.p.State().UpNext(); next.Track.ID != "yt:b" {
		t.Errorf("up next is %s", next.Track.ID)
	}
	h.p.Next()
	h.wait("b playing", playing("b"))
	h.p.Previous()
	h.wait("a playing again", playing("a"))
	h.on(func(p *Player) { p.s.Repeat = RepeatAll })
	h.p.Next()
	h.wait("b playing", playing("b"))
	h.p.Ended()
	h.wait("a playing as it repeats", playing("a"))
	if got := h.loads(); slices.Contains(got, "x1") || slices.Contains(got, "x2") {
		t.Errorf("the engine loaded %v", got)
	}
	if n := h.notices(); len(n) != 0 {
		t.Errorf("passing over explicit songs told %+v", n)
	}
}

// TestHideExplicitChosen plays no explicit song the user chooses while the
// settings hide them, and tells why: in a list, in the queue, as a radio,
// or queued by itself; and a podcast's episode marked explicit plays.
func TestHideExplicitChosen(t *testing.T) {
	h := newHarness(t, Saved{}, hidingExplicit)
	h.p.PlayTracks(explicitTracks("a", "x1", "b"), 0)
	h.wait("a playing", playing("a"))
	before := queueIDs(h.p.State())

	// A list played from an explicit song doesn't replace the queue.
	h.p.PlayTracks(explicitTracks("c", "x3", "d"), 1)
	h.on(func(*Player) {})
	if got := queueIDs(h.p.State()); !slices.Equal(got, before) || current(h.p.State()) != "a" {
		t.Errorf("playing an explicit song made the queue %v, at %s", got, current(h.p.State()))
	}
	if !h.toldExplicit() {
		t.Errorf("nothing told why: %+v", h.notices())
	}

	// Nor does one chosen in the queue play.
	h.p.PlayAt(1)
	h.p.PlayID("yt:x1")
	h.on(func(*Player) {})
	if s := h.p.State(); current(s) != "a" || !s.Playing {
		t.Errorf("choosing x1 in the queue moved to %s, playing %v", current(s), s.Playing)
	}
	if it := h.p.State().Queue[1]; it.Skip != SkipExplicit {
		t.Errorf("x1, chosen, is passed over as %q", it.Skip)
	}

	// Nor a radio of one.
	h.p.PlayWithRadio(explicitTracks("x4")[0])
	h.on(func(*Player) {})
	if got := queueIDs(h.p.State()); !slices.Equal(got, before) {
		t.Errorf("a radio of x4 made the queue %v", got)
	}

	// One queued by itself is passed over all the same.
	h.p.PlayNext(explicitTracks("x5"))
	h.on(func(*Player) {})
	if got := skips(h.p.State()); !slices.Equal(got, []string{"x5", "x1"}) {
		t.Errorf("the queue passes over %v", got)
	}
	h.p.Next()
	h.wait("b playing, past x5 and x1", playing("b"))

	// A podcast's episode marked explicit isn't a song: it plays.
	episode := explicitTracks("xe")[0]
	episode.Episode = &media.Episode{}
	h.p.PlayTracks([]media.Track{episode}, 0)
	h.wait("the episode playing", playing("xe"))
	if got := h.loads(); slices.ContainsFunc(got, func(id string) bool { return id != "xe" && strings.HasPrefix(id, "x") }) {
		t.Errorf("the engine loaded %v", got)
	}
}

// TestHideExplicitOnlyExplicit plays nothing of a list or a queue of
// explicit songs alone, and tells why.
func TestHideExplicitOnlyExplicit(t *testing.T) {
	h := newHarness(t, Saved{}, hidingExplicit)
	h.p.PlayAll(explicitTracks("x1", "x2", "x3"), false)
	h.on(func(*Player) {})
	if s := h.p.State(); len(s.Queue) != 0 || s.Playing {
		t.Errorf("playing a list of explicit songs made the queue %v, playing %v", queueIDs(s), s.Playing)
	}
	notes := h.notices()
	if len(notes) != 1 || notes[0].Message != "Every song here is explicit, and your settings hide explicit content" {
		t.Errorf("told %+v", notes)
	}

	h.p.PlayAll(explicitTracks("x1", "x2"), true)
	h.p.PlayTracks(explicitTracks("x1", "x2"), 0)
	h.p.Enqueue(explicitTracks("x1", "x2"))
	h.p.Play()
	h.p.Next()
	h.on(func(*Player) {})
	s := h.p.State()
	if !slices.Equal(queueIDs(s), []string{"x1", "x2"}) || s.Index >= 0 || s.Playing || s.Loading {
		t.Errorf("the queue %v, at %d, playing %v, loading %v", queueIDs(s), s.Index, s.Playing, s.Loading)
	}
	if got := skips(s); !slices.Equal(got, []string{"x1", "x2"}) {
		t.Errorf("the queue passes over %v", got)
	}
	if n := h.stream.count("load"); n != 0 {
		t.Errorf("the engine loaded %v", h.loads())
	}

	// A list of explicit songs and disliked ones says both.
	h = newHarness(t, Saved{}, hidingExplicit, disliking("a"))
	h.p.PlayAll(explicitTracks("a", "x1"), false)
	h.on(func(*Player) {})
	if notes := h.notices(); len(notes) != 1 || !strings.HasPrefix(notes[0].Message, "Every song here is explicit, which your settings hide, or") {
		t.Errorf("told %+v", notes)
	}
}

// TestHideExplicitSetting passes over the explicit songs of the queue once
// the settings hide them, those the user chose too, and moves on from the
// one that plays; turned off, they play again.
func TestHideExplicitSetting(t *testing.T) {
	h := newHarness(t, Saved{})
	h.p.PlayTracks(explicitTracks("a", "x1", "b", "x2", "c"), 1)
	h.wait("x1 playing", playing("x1"))
	h.p.PlayAt(3)
	h.wait("x2 playing, chosen", playing("x2"))
	if got := skips(h.p.State()); len(got) != 0 {
		t.Fatalf("not hiding, the queue passes over %v", got)
	}
	h.svc.with(hidingExplicit)
	h.p.SettingsChanged(h.svc.Settings())
	h.wait("c playing, moved on from x2", playing("c"))
	if got := skips(h.p.State()); !slices.Equal(got, []string{"x1", "x2"}) {
		t.Errorf("hiding, the queue passes over %v", got)
	}
	h.p.Previous()
	h.wait("b playing, back from c", playing("b"))

	h.svc.with(func(f *fakeServices) { f.settings.HideExplicit = false })
	h.p.SettingsChanged(h.svc.Settings())
	h.on(func(*Player) {})
	if got := skips(h.p.State()); len(got) != 0 {
		t.Errorf("not hiding again, the queue passes over %v", got)
	}
	h.next()
	if got := current(h.p.State()); got != "x2" {
		t.Errorf("not hiding again, played %s after b", got)
	}
}

// TestHideExplicitSaved keeps passing over the explicit songs of a queue
// kept from the run before, those the user chose and the current one too,
// which doesn't play.
func TestHideExplicitSaved(t *testing.T) {
	saved := Saved{Tracks: explicitTracks("x1", "a", "x2", "b"), Index: 0, Chosen: []int{0, 2}}
	h := newHarness(t, saved, hidingExplicit)
	if got := skips(h.p.State()); !slices.Equal(got, []string{"x1", "x2"}) {
		t.Fatalf("the queue kept passes over %v", got)
	}
	h.p.Play()
	h.on(func(*Player) {})
	if s := h.p.State(); s.Playing || s.Loading || !h.toldExplicit() {
		t.Errorf("x1 kept as current played: %+v; told %+v", s, h.notices())
	}
	h.p.Next()
	h.wait("a playing", playing("a"))
	h.p.Next()
	h.wait("b playing, past x2", playing("b"))
}

// TestHideExplicitRadio passes over the explicit songs of the radio that
// autoplay adds, as the app's radio leaves them out.
func TestHideExplicitRadio(t *testing.T) {
	h := newHarness(t, Saved{}, hidingExplicit, func(f *fakeServices) {
		f.settings.Autoplay = true
		f.radio = explicitTracks("x8", "r1")
	})
	h.p.PlayTracks(explicitTracks("a", "x1"), 0)
	h.wait("a playing", playing("a"))
	h.p.Ended()
	h.wait("r1 playing, of the radio", playing("r1"))
	if got := h.loads(); slices.Contains(got, "x8") || slices.Contains(got, "x1") {
		t.Errorf("the engine loaded %v", got)
	}
}
