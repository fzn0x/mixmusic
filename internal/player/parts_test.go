package player

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/audio"
	"github.com/fzn0x/mixmusic/internal/sponsorblock"
)

// SetSkips has the fake engine record the parts it is told to pass over,
// as the engine passes over them.
func (s *fakeStream) SetSkips(url string, skips []audio.Skip) {
	s.record("skips %s %v", url, skips)
}

// marking is the fake app knowing the parts SponsorBlock marks of tracks,
// by their ID: it knows those of known at once, and tells those of slow
// once asked.
type marking struct {
	*fakeServices
	mmu   sync.Mutex
	parts map[string][]sponsorblock.Segment
	known map[string]bool
	asked []string
}

func (m *marking) Marks(_ context.Context, src Source) ([]sponsorblock.Segment, error) {
	m.mmu.Lock()
	defer m.mmu.Unlock()
	m.asked = append(m.asked, src.Track.ID)
	m.known[src.Track.ID] = true
	return m.parts[src.Track.ID], nil
}

func (m *marking) KnownMarks(src Source) ([]sponsorblock.Segment, bool) {
	m.mmu.Lock()
	defer m.mmu.Unlock()
	if !m.known[src.Track.ID] {
		return nil, false
	}
	return m.parts[src.Track.ID], true
}

func (m *marking) askedFor() []string {
	m.mmu.Lock()
	defer m.mmu.Unlock()
	return slices.Clone(m.asked)
}

const sec = time.Second

// newMarking returns a harness whose app knows the parts of parts, by
// track ID, while the settings skip them: Non-music parts, Sponsor and
// Self-promotion, with the highlight. Those of known it knows at once.
func newMarking(t *testing.T, parts map[string][]sponsorblock.Segment, known ...string) (*harness, *marking) {
	t.Helper()
	var m *marking
	h := newHarnessWith(t, Saved{}, func(f *fakeServices) Services {
		m = &marking{fakeServices: f, parts: parts, known: map[string]bool{}}
		for _, id := range known {
			m.known[id] = true
		}
		return m
	}, func(f *fakeServices) { f.settings.SkipParts.On = true })
	return h, m
}

// sponsorRead is a sponsor read from 10 s to 20 s of a 200 s video, with a
// highlight at 5 s and filler, which the settings don't skip at first.
var sponsorRead = []sponsorblock.Segment{
	{Kind: sponsorblock.Highlight, From: 5 * sec, To: 5 * sec, Of: 200 * sec},
	{Kind: sponsorblock.Sponsor, From: 10 * sec, To: 20 * sec, Of: 200 * sec},
	{Kind: sponsorblock.Filler, From: 30 * sec, To: 40 * sec, Of: 200 * sec},
}

// TestPartsToldBeforeTheLoad tells the engine the parts it passes over of
// a track whose parts the app knows before it loads it, and shows them on
// the seek bar, with the highlight.
func TestPartsToldBeforeTheLoad(t *testing.T) {
	h, _ := newMarking(t, map[string][]sponsorblock.Segment{"yt:a": sponsorRead}, "yt:a")
	h.p.PlayTracks(tracks("a"), 0)
	h.wait("a playing", playing("a"))
	calls := h.stream.copy()
	told := slices.Index(calls, "skips url:yt:a [{10s 20s 3m20s}]")
	loaded := slices.Index(calls, "load url:yt:a 0s true")
	if told < 0 || loaded < told {
		t.Errorf("the engine heard of the parts after the load, or not: %q", calls)
	}
	st := h.p.State()
	if want := []Part{{Kind: sponsorblock.Sponsor, From: 10 * sec, To: 20 * sec}}; !slices.Equal(st.Parts, want) || st.Highlight != 5*sec {
		t.Errorf("parts %v, highlight %v", st.Parts, st.Highlight)
	}
}

// TestPartsComeLate plays a track without waiting for its parts, which
// the engine hears of once they come.
func TestPartsComeLate(t *testing.T) {
	h, m := newMarking(t, map[string][]sponsorblock.Segment{"yt:a": sponsorRead})
	h.p.PlayTracks(tracks("a"), 0)
	h.wait("a playing", playing("a"))
	h.wait("a's parts", func(p *Player) bool { return len(p.s.Parts) == 1 })
	if !h.stream.has("skips url:yt:a [{10s 20s 3m20s}]") || !slices.Contains(m.askedFor(), "yt:a") {
		t.Errorf("the engine didn't hear of the parts: %q", h.stream.copy())
	}
}

// TestPartsOfTheNextTrack asks for the next track's parts as its stream
// is found, and tells the engine of them before it preloads it.
func TestPartsOfTheNextTrack(t *testing.T) {
	h, m := newMarking(t, map[string][]sponsorblock.Segment{"yt:b": sponsorRead})
	h.p.PlayTracks(tracks("a", "b"), 0)
	h.wait("a playing", playing("a"))
	h.p.Time(2*sec, 200*sec)
	h.wait("b's parts asked for", func(*Player) bool { return slices.Contains(m.askedFor(), "yt:b") })
	h.p.Time(11*sec, 200*sec)
	h.wait("b preloaded", func(*Player) bool { return h.stream.has("preload url:yt:b") })
	calls := h.stream.copy()
	if told := slices.Index(calls, "skips url:yt:b [{10s 20s 3m20s}]"); told < 0 || told > slices.Index(calls, "preload url:yt:b") {
		t.Errorf("the engine heard of b's parts after its preload, or not: %q", calls)
	}
}

// TestSkippedNoticeUndoes tells of a part the engine passed over, with
// undo, which goes back to its start, and of one that ended the track,
// without.
func TestSkippedNoticeUndoes(t *testing.T) {
	outro := append(slices.Clone(sponsorRead), sponsorblock.Segment{Kind: sponsorblock.SelfPromo, From: 190 * sec, To: 200 * sec, Of: 200 * sec})
	h, _ := newMarking(t, map[string][]sponsorblock.Segment{"yt:a": outro}, "yt:a")
	h.p.PlayTracks(tracks("a", "b"), 0)
	h.wait("a playing", playing("a"))
	h.p.Skipped("url:yt:a", 10*sec, 20*sec)
	// One of another stream, as the track before's, tells nothing.
	h.p.Skipped("url:yt:z", 10*sec, 20*sec)
	h.p.Skipped("url:yt:a", 190*sec, 200*sec)
	h.on(func(*Player) {})
	var skipped []Notice
	for _, n := range h.notices() {
		if strings.HasPrefix(n.Message, "Skipped") {
			skipped = append(skipped, n)
		}
	}
	if len(skipped) != 2 || skipped[0].Message != "Skipped a sponsor read" || skipped[0].Undo == nil ||
		skipped[1].Message != "Skipped self-promotion" || skipped[1].Undo != nil {
		t.Fatalf("notices %+v", skipped)
	}
	skipped[0].Undo()
	h.wait("back at the part", func(*Player) bool { return h.stream.has("seek 10s") })
}

// TestPartsCountForNothing counts a play by what played without the parts
// passed over: a 40 s intro passed over counts for nothing, and half the
// length is half of what is left.
func TestPartsCountForNothing(t *testing.T) {
	intro := []sponsorblock.Segment{{Kind: sponsorblock.NonMusic, From: 0, To: 40 * sec}}
	h, _ := newMarking(t, map[string][]sponsorblock.Segment{"yt:a": intro, "yt:b": intro}, "yt:a", "yt:b")
	h.p.PlayTracks(tracks("a", "b"), 0)
	h.wait("a playing", playing("a"))
	h.p.Time(41*sec, 200*sec)
	h.noneCounted("1 s after the intro")
	if got := h.lastReport().Skipped; got != 40*sec {
		t.Errorf("the app heard of %v skipped", got)
	}
	h.p.Time(70*sec, 200*sec)
	h.wait("a counted", func(*Player) bool { return slices.Equal(h.played(), []string{"yt:a"}) })
	// Of 60 s, 20 s are left, half of which counts.
	h.next()
	h.p.Time(45*sec, 60*sec)
	h.on(func(*Player) {})
	if got := h.played(); !slices.Equal(got, []string{"yt:a"}) {
		t.Errorf("counted %v 5 s after the intro of a 60 s track", got)
	}
	h.p.Time(50*sec, 60*sec)
	h.wait("b counted", func(*Player) bool { return slices.Equal(h.played(), []string{"yt:a", "yt:b"}) })
}

// TestPreviousLeavesOutParts goes back to the track before early in what
// played after a part passed over at the start.
func TestPreviousLeavesOutParts(t *testing.T) {
	intro := []sponsorblock.Segment{{Kind: sponsorblock.Sponsor, From: 0, To: 40 * sec}}
	h, _ := newMarking(t, map[string][]sponsorblock.Segment{"yt:b": intro}, "yt:b")
	h.p.PlayTracks(tracks("a", "b"), 1)
	h.wait("b playing", playing("b"))
	h.p.Time(42*sec, 200*sec)
	h.p.Previous()
	h.wait("a playing", playing("a"))
}

// TestCrossfadeBeforeThePartAtTheEnd crossfades into the next track before
// a part that reaches the current one's end, as at its end.
func TestCrossfadeBeforeThePartAtTheEnd(t *testing.T) {
	outro := []sponsorblock.Segment{{Kind: sponsorblock.SelfPromo, From: 170 * sec, To: 200 * sec}}
	h, _ := newMarking(t, map[string][]sponsorblock.Segment{"yt:a": outro}, "yt:a")
	h.svc.with(func(f *fakeServices) { f.settings.Crossfade = 5 })
	h.p.PlayTracks(tracks("a", "b"), 0)
	h.wait("a playing", playing("a"))
	h.p.Time(11*sec, 200*sec)
	h.p.Time(166*sec, 200*sec)
	h.wait("the crossfade", func(*Player) bool { return h.stream.count("crossfade url:yt:b") == 1 })
}

// TestNoPartsWhileOff asks for no parts while the settings skip none, and
// tells the engine of none.
func TestNoPartsWhileOff(t *testing.T) {
	h, m := newMarking(t, map[string][]sponsorblock.Segment{"yt:a": sponsorRead})
	h.svc.with(func(f *fakeServices) { f.settings.SkipParts.On = false })
	h.p.SettingsChanged(h.svc.Settings())
	h.p.PlayTracks(tracks("a", "b"), 0)
	h.wait("a playing", playing("a"))
	h.p.Time(11*sec, 200*sec)
	h.wait("b preloaded", func(*Player) bool { return h.stream.has("preload url:yt:b") })
	if asked := m.askedFor(); len(asked) > 0 || h.stream.count("skips") > 0 {
		t.Errorf("asked for %v; the stream did %q", asked, h.stream.copy())
	}
	if st := h.p.State(); st.Parts != nil || st.Highlight != 0 {
		t.Errorf("parts %v, highlight %v", st.Parts, st.Highlight)
	}
}

// TestPartSettingsApplyAtOnce has the engine hear again what it passes
// over as the kinds ticked change, and nothing once off.
func TestPartSettingsApplyAtOnce(t *testing.T) {
	h, _ := newMarking(t, map[string][]sponsorblock.Segment{"yt:a": sponsorRead}, "yt:a")
	h.p.PlayTracks(tracks("a"), 0)
	h.wait("a playing", playing("a"))
	h.svc.with(func(f *fakeServices) {
		f.settings.SkipParts.Kinds = append(f.settings.SkipParts.Kinds, sponsorblock.Filler)
	})
	h.p.SettingsChanged(h.svc.Settings())
	h.wait("filler skipped", func(p *Player) bool { return len(p.s.Parts) == 2 })
	if !h.stream.has("skips url:yt:a [{10s 20s 3m20s} {30s 40s 3m20s}]") {
		t.Errorf("the engine didn't hear of the filler: %q", h.stream.copy())
	}
	h.svc.with(func(f *fakeServices) { f.settings.SkipParts.On = false })
	h.p.SettingsChanged(h.svc.Settings())
	h.wait("no parts", func(p *Player) bool { return p.s.Parts == nil && p.s.Highlight == 0 })
	if got := h.stream.last("skips"); got != "skips url:yt:a []" {
		t.Errorf("last told %q", got)
	}
}

// TestJumpToHighlight seeks to the highlight.
func TestJumpToHighlight(t *testing.T) {
	h, _ := newMarking(t, map[string][]sponsorblock.Segment{"yt:a": sponsorRead}, "yt:a")
	h.p.PlayTracks(tracks("a"), 0)
	h.wait("a playing", playing("a"))
	h.p.JumpToHighlight()
	h.wait("at the highlight", func(*Player) bool { return h.stream.has(fmt.Sprintf("seek %v", 5*sec)) })
}

// TestNoSkippedNoticeWhilePaused tells nothing of a part passed over by a
// track that loaded paused, as one the queue holds at start.
func TestNoSkippedNoticeWhilePaused(t *testing.T) {
	h, _ := newMarking(t, map[string][]sponsorblock.Segment{"yt:a": sponsorRead}, "yt:a")
	h.p.PlayTracks(tracks("a"), 0)
	h.wait("a playing", playing("a"))
	h.p.Pause()
	h.wait("a paused", func(p *Player) bool { return p.paused })
	h.p.Skipped("url:yt:a", 10*sec, 20*sec)
	h.on(func(*Player) {})
	for _, n := range h.notices() {
		if strings.HasPrefix(n.Message, "Skipped") {
			t.Errorf("told %q while paused", n.Message)
		}
	}
}

// TestPartsAskedAgainOfAStreamWithout asks the app again of a stream it
// found without parts, as it loads again: the app knows it, or knew
// nothing of it, as offline.
func TestPartsAskedAgainOfAStreamWithout(t *testing.T) {
	h, m := newMarking(t, map[string][]sponsorblock.Segment{})
	h.p.PlayTracks(tracks("a", "b"), 0)
	h.wait("a playing", playing("a"))
	h.wait("a asked for", func(*Player) bool { return len(m.askedFor()) == 1 })
	m.mmu.Lock()
	m.known["yt:a"] = false
	m.mmu.Unlock()
	h.next()
	h.p.Previous()
	h.wait("a playing again", playing("a"))
	h.wait("a asked for again", func(*Player) bool { return slices.Equal(m.askedFor(), []string{"yt:a", "yt:b", "yt:a"}) })
}
