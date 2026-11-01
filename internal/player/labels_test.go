package player

import (
	"context"
	"errors"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/settings"
)

// labeled is the fake app knowing YouTube's labels: it labels the tracks
// of ai as made with AI, fails the checks of failing, and knows the labels
// it told, as the app keeps them.
type labeled struct {
	*fakeServices
	lmu     sync.Mutex
	ai      map[string]bool
	failing map[string]bool
	paused  bool
	known   map[string]Label
	// asked holds the checks, in turn; soon whether each was of a track
	// about to play.
	asked []string
	soon  []bool
	// delay holds each check for a while, and gate, when set, until it is
	// closed.
	delay time.Duration
	gate  chan struct{}
	// radio, when set, finds a radio's tracks; seeds holds what each radio
	// started from.
	radio func(n int) []media.Track
	seeds []string
}

func (l *labeled) Label(ctx context.Context, t media.Track, soon bool) Label {
	l.lmu.Lock()
	l.asked = append(l.asked, t.ID)
	l.soon = append(l.soon, soon)
	gate, delay := l.gate, l.delay
	l.lmu.Unlock()
	if gate != nil {
		select {
		case <-gate:
		case <-ctx.Done():
			return LabelUnknown
		}
	}
	if delay > 0 {
		select {
		case <-time.After(delay):
		case <-ctx.Done():
			return LabelUnknown
		}
	}
	l.lmu.Lock()
	defer l.lmu.Unlock()
	switch {
	case l.paused:
		return LabelPaused
	case l.failing[t.ID]:
		return LabelUnknown
	case l.ai[t.ID]:
		l.known[t.ID] = LabelAI
	default:
		l.known[t.ID] = LabelNone
	}
	return l.known[t.ID]
}

func (l *labeled) KnownLabel(t media.Track) Label {
	l.lmu.Lock()
	defer l.lmu.Unlock()
	if l.paused {
		return LabelPaused
	}
	if v, ok := l.known[t.ID]; ok {
		return v
	}
	return LabelUnknown
}

func (l *labeled) Radio(ctx context.Context, t media.Track) ([]media.Track, error) {
	l.lmu.Lock()
	radio := l.radio
	l.seeds = append(l.seeds, t.ID)
	n := len(l.seeds)
	l.lmu.Unlock()
	if radio == nil {
		return l.fakeServices.Radio(ctx, t)
	}
	return radio(n), nil
}

func (l *labeled) checks() []string {
	l.lmu.Lock()
	defer l.lmu.Unlock()
	return slices.Clone(l.asked)
}

func (l *labeled) with(fn func(*labeled)) {
	l.lmu.Lock()
	defer l.lmu.Unlock()
	fn(l)
}

// newLabeled is a harness whose app labels the tracks ai, by their ID
// without "yt:", and whose settings skip them.
func newLabeled(t *testing.T, saved Saved, ai []string, setup ...func(*fakeServices)) (*harness, *labeled) {
	t.Helper()
	l := &labeled{ai: map[string]bool{}, failing: map[string]bool{}, known: map[string]Label{}}
	for _, id := range ai {
		l.ai["yt:"+id] = true
	}
	setup = append([]func(*fakeServices){func(f *fakeServices) { f.settings.SkipAI = true }}, setup...)
	h := newHarnessWith(t, saved, func(f *fakeServices) Services {
		l.fakeServices = f
		return l
	}, setup...)
	return h, l
}

// marked is the IDs of the entries marked as made with AI.
func marked(s State) []string {
	var out []string
	for _, it := range s.Queue {
		if it.MadeWithAI {
			out = append(out, it.Track.ID)
		}
	}
	return out
}

// reported reports whether the app was told of the track id.
func (h *harness) reported(id string) bool {
	found := false
	h.svc.with(func(f *fakeServices) {
		for _, r := range f.reports {
			found = found || r.Track != nil && r.Track.ID == id
		}
	})
	return found
}

// noticed waits for a notice that says message.
func (h *harness) noticed(message string) {
	h.t.Helper()
	h.wait("the notice "+message, func(*Player) bool {
		return slices.ContainsFunc(h.notices(), func(n Notice) bool { return n.Message == message })
	})
}

// TestSkipAI passes over a track YouTube labels as the queue moves on, and
// tells of it once the next one plays, without an error.
func TestSkipAI(t *testing.T) {
	h, l := newLabeled(t, Saved{}, []string{"b"})
	h.p.PlayTracks(tracks("a", "b", "c"), 0)
	h.wait("a playing", playing("a"))
	h.p.Ended()
	h.wait("c playing", playing("c"))
	h.noticed("Skipped Song b, which YouTube labels as made with AI")
	if got := marked(h.p.State()); !slices.Equal(got, []string{"yt:b"}) {
		t.Errorf("marked %v", got)
	}
	for _, n := range h.notices() {
		if n.Error || strings.Contains(n.Message, "Couldn't play") {
			t.Errorf("notice %+v", n)
		}
		if strings.HasPrefix(n.Message, "Skipped") && !n.Settings {
			t.Errorf("the notice of the skip doesn't lead to the settings: %+v", n)
		}
	}
	if h.stream.has("load url:yt:b 0s true") || h.reported("yt:b") {
		t.Errorf("b played or was told of: %q", h.stream.copy())
	}
	if got := l.checks(); !slices.Equal(got, []string{"yt:a", "yt:b", "yt:c"}) {
		t.Errorf("checked %v", got)
	}
	// A known label needs no check: back to b, and past it.
	h.p.PlayAt(0)
	h.wait("a playing", playing("a"))
	h.p.Next()
	h.wait("c playing again", func(p *Player) bool { return playing("c")(p) && h.stream.count("load url:yt:c") == 2 })
	if got := l.checks(); len(got) != 3 {
		t.Errorf("checked again: %v", got)
	}
}

// TestSkipAIOff plays what YouTube labels, without a check, while the
// settings don't skip it.
func TestSkipAIOff(t *testing.T) {
	h, l := newLabeled(t, Saved{}, []string{"b"}, func(f *fakeServices) { f.settings.SkipAI = false })
	h.p.PlayTracks(tracks("a", "b"), 0)
	h.wait("a playing", playing("a"))
	h.p.Time(11*time.Second, 200*time.Second)
	h.wait("b preloaded", func(*Player) bool { return h.stream.has("preload url:yt:b") })
	h.p.Ended()
	h.wait("b playing", playing("b"))
	if got := l.checks(); len(got) != 0 {
		t.Errorf("checked %v", got)
	}
}

// TestSkipAIChosen plays a track the user chose, labeled or not.
func TestSkipAIChosen(t *testing.T) {
	h, l := newLabeled(t, Saved{}, []string{"b"})
	h.p.PlayChosen(tracks("b", "c"), 0)
	h.wait("b playing", playing("b"))
	if got := l.checks(); len(got) != 0 {
		t.Errorf("checked %v", got)
	}
	// Past it, as the queue moves on, and back to it, by its entry.
	h.p.PlayTracks(tracks("a", "b", "c"), 0)
	h.wait("a playing", playing("a"))
	h.p.Next()
	h.wait("c playing", playing("c"))
	h.p.PlayAt(1)
	h.wait("b playing", playing("b"))
	h.p.PlayID("yt:b")
	h.wait("b playing again", func(p *Player) bool { return playing("b")(p) && h.stream.count("load url:yt:b") == 3 })
}

// TestSkipAIPlayTracks passes over the labeled tracks at the start of a
// list played, and stops, telling why, when all of them are.
func TestSkipAIPlayTracks(t *testing.T) {
	h, _ := newLabeled(t, Saved{}, []string{"a", "b"})
	h.p.PlayTracks(tracks("a", "b", "c"), 0)
	h.wait("c playing", playing("c"))
	h.noticed("Skipped 2 songs YouTube labels as made with AI")
	if h.reported("yt:a") || h.reported("yt:b") {
		t.Error("a skipped track was told of")
	}
	h.p.PlayTracks(tracks("a", "b"), 0)
	h.noticed(restLabeled)
	h.wait("stopping", func(p *Player) bool { return p.s.Index == -1 && !p.s.Loading && !p.s.Playing })
	if got := h.stream.sounding(); got != "" {
		t.Errorf("%s plays on", got)
	}
}

// TestSkipAIPrevious goes back past a labeled track, and starts the
// current one over when only labeled ones come before it.
func TestSkipAIPrevious(t *testing.T) {
	h, _ := newLabeled(t, Saved{}, []string{"b"})
	h.p.PlayChosen(tracks("a", "b", "c"), 2)
	h.wait("c playing", playing("c"))
	h.p.Previous()
	h.wait("a playing", playing("a"))
	h.noticed("Skipped Song b, which YouTube labels as made with AI")

	h.p.PlayChosen(tracks("b", "c"), 1)
	h.wait("c playing", func(p *Player) bool { return playing("c")(p) && h.stream.count("load url:yt:c") == 2 })
	h.p.Previous()
	h.wait("c from its start", func(*Player) bool { return h.stream.has("seek 0s") })
	h.wait("the second notice", func(*Player) bool { return len(h.notices()) == 2 })
	h.on(func(p *Player) {
		if !playing("c")(p) {
			t.Error("c stopped")
		}
	})
}

// TestSkipAISlowCheck plays a track whose check takes too long, unchecked.
func TestSkipAISlowCheck(t *testing.T) {
	wait := labelWait
	labelWait = 50 * time.Millisecond
	t.Cleanup(func() { labelWait = wait })
	h, l := newLabeled(t, Saved{}, []string{"b"})
	h.p.PlayChosen(tracks("a", "b"), 0)
	h.wait("a playing", playing("a"))
	l.with(func(l *labeled) { l.delay = 5 * time.Second })
	h.p.Next()
	h.wait("b playing", playing("b"))
}

// TestSkipAINotAnnounced tells nothing of a track while it's checked, nor
// of one skipped: the app and the subscribers hear of the next one alone.
func TestSkipAINotAnnounced(t *testing.T) {
	h, l := newLabeled(t, Saved{}, []string{"b"})
	var mu sync.Mutex
	var heard []string
	h.p.Subscribe(func(Event) {
		if cur, ok := h.p.State().Current(); ok {
			mu.Lock()
			heard = append(heard, cur.Track.ID)
			mu.Unlock()
		}
	})
	h.p.PlayChosen(tracks("a", "b", "c"), 0)
	h.wait("a playing", playing("a"))
	gate := make(chan struct{})
	l.with(func(l *labeled) { l.gate = gate })
	// Next while it plays: the engine, unloaded, still plays as the app
	// last heard.
	h.p.Next()
	h.wait("b checked", func(*Player) bool { return slices.Contains(l.checks(), "yt:b") })
	h.on(func(p *Player) {
		if cur, _ := p.s.Current(); cur.Track.ID != "yt:a" || !p.s.Loading || h.stream.sounding() != "" {
			t.Errorf("while b is checked: %s current, loading %v, the engine plays %q", cur.Track.ID, p.s.Loading, h.stream.sounding())
		}
	})
	close(gate)
	h.wait("c playing", playing("c"))
	if h.reported("yt:b") {
		t.Error("the app was told of b")
	}
	mu.Lock()
	defer mu.Unlock()
	if slices.Contains(heard, "yt:b") {
		t.Errorf("the subscribers heard of b: %v", heard)
	}
}

// TestSkipAIPrefetch checks the next track before it's preloaded or
// crossfaded into: a labeled one is marked and the one after it prepared
// instead; one whose check failed isn't, and is checked again in turn.
func TestSkipAIPrefetch(t *testing.T) {
	h, l := newLabeled(t, Saved{}, []string{"b"}, func(f *fakeServices) { f.settings.Crossfade = 5 })
	h.p.PlayChosen(tracks("a", "b", "c", "d"), 0)
	h.wait("a playing", playing("a"))
	h.p.Time(11*time.Second, 200*time.Second)
	h.wait("b marked", func(p *Player) bool { return slices.Equal(marked(p.s), []string{"yt:b"}) })
	h.p.Time(12*time.Second, 200*time.Second)
	h.wait("c preloaded", func(*Player) bool { return h.stream.has("preload url:yt:c") })
	if next, _ := h.p.State().UpNext(); next.Track.ID != "yt:c" {
		t.Errorf("up next %s", next.Track.ID)
	}
	if h.stream.has("preload url:yt:b") {
		t.Error("b was preloaded")
	}
	h.p.Time(196*time.Second, 200*time.Second)
	h.wait("c crossfading in", playing("c"))
	if got := h.stream.last("crossfade"); got != "crossfade url:yt:c 5s" {
		t.Errorf("crossfaded %q", got)
	}
	h.noticed("Skipped Song b, which YouTube labels as made with AI")

	// d's check fails: it's neither preloaded nor crossfaded into.
	l.with(func(l *labeled) { l.failing["yt:d"] = true })
	h.p.Time(11*time.Second, 200*time.Second)
	h.wait("d checked", func(*Player) bool { return slices.Contains(l.checks(), "yt:d") })
	h.p.Time(196*time.Second, 200*time.Second)
	h.on(func(*Player) {})
	if h.stream.has("preload url:yt:d") || h.stream.has("crossfade url:yt:d 5s") {
		t.Errorf("d started early: %q", h.stream.copy())
	}
	// Its turn checks it again; failing, it plays.
	h.p.Ended()
	h.wait("d playing", playing("d"))
	n := 0
	for _, id := range l.checks() {
		if id == "yt:d" {
			n++
		}
	}
	if n != 2 {
		t.Errorf("d checked %d times", n)
	}
}

// TestSkipAIAhead checks no more than labelAhead entries ahead of the
// current one.
func TestSkipAIAhead(t *testing.T) {
	ids := []string{"a"}
	for i := range labelAhead + 2 {
		ids = append(ids, "x"+strconv.Itoa(i))
	}
	h, l := newLabeled(t, Saved{}, ids[1:])
	h.p.PlayChosen(tracks(ids...), 0)
	h.wait("a playing", playing("a"))
	// Each progress past the check of the one before checks the next.
	for i := range labelAhead {
		h.p.Time(time.Duration(11+i)*time.Second, 200*time.Second)
		h.wait("x"+strconv.Itoa(i)+" marked", func(p *Player) bool { return len(marked(p.s)) == i+1 })
	}
	for i := range 3 {
		h.p.Time(time.Duration(20+i)*time.Second, 200*time.Second)
	}
	time.Sleep(20 * time.Millisecond)
	h.on(func(*Player) {})
	if got := l.checks(); len(got) != labelAhead {
		t.Errorf("checked %v", got)
	}
}

// TestSkipAIRun tells of a run of skips in one notice.
func TestSkipAIRun(t *testing.T) {
	h, _ := newLabeled(t, Saved{}, []string{"b", "c", "d"})
	h.p.PlayChosen(tracks("a", "b", "c", "d", "e"), 0)
	h.wait("a playing", playing("a"))
	h.p.Ended()
	h.wait("e playing", playing("e"))
	h.noticed("Skipped 3 songs YouTube labels as made with AI")
	skips := 0
	for _, n := range h.notices() {
		if strings.HasPrefix(n.Message, "Skipped") {
			skips++
		}
	}
	if skips != 1 {
		t.Errorf("notices %+v", h.notices())
	}
}

// TestSkipAIEnd tells, at the end of the queue, that the rest of it is
// labeled: Next leaves what plays alone when it knew, and stops it when it
// had to check.
func TestSkipAIEnd(t *testing.T) {
	h, _ := newLabeled(t, Saved{}, []string{"b"})
	h.p.PlayChosen(tracks("a", "b"), 0)
	h.wait("a playing", playing("a"))
	h.p.Next()
	h.noticed(restLabeled)
	h.wait("a stopped", func(p *Player) bool { return p.s.Index == 0 && !p.s.Playing && !p.s.Loading })

	h.p.PlayChosen(tracks("a", "b"), 0)
	h.wait("a playing again", func(p *Player) bool { return playing("a")(p) && h.stream.count("load url:yt:a") == 2 })
	h.p.Next()
	time.Sleep(20 * time.Millisecond)
	h.on(func(p *Player) {
		if !playing("a")(p) {
			t.Error("a, known to come before a labeled track, stopped")
		}
	})
	if n := h.notices(); len(n) != 2 || n[1].Message != restLabeled || !n[1].Settings {
		t.Errorf("notices %+v", n)
	}
}

// TestSkipAIRepeatAll stops, rather than going round the queue, when all
// it holds but the current track is labeled, which is too.
func TestSkipAIRepeatAll(t *testing.T) {
	h, l := newLabeled(t, Saved{}, []string{"a", "b", "c"})
	h.p.PlayChosen(tracks("a", "b", "c"), 0)
	h.wait("a playing", playing("a"))
	h.p.CycleRepeat()
	h.p.Ended()
	h.noticed(restLabeled)
	h.wait("stopping", func(p *Player) bool { return !p.s.Playing && !p.s.Loading && p.s.Index == 0 })
	if got := l.checks(); len(got) != 3 {
		t.Errorf("checked %v", got)
	}
}

// TestSkipAIAutoplay starts each radio from the track that last played,
// and stops after three radios in a row held only labeled tracks.
func TestSkipAIAutoplay(t *testing.T) {
	var ai []string
	for n := 1; n <= maxEmptyRadios; n++ {
		for i := range 3 {
			ai = append(ai, "r"+strconv.Itoa(n)+strconv.Itoa(i))
		}
	}
	h, l := newLabeled(t, Saved{}, ai, func(f *fakeServices) { f.settings.Autoplay = true })
	l.with(func(l *labeled) {
		l.radio = func(n int) []media.Track {
			return tracks("r"+strconv.Itoa(n)+"0", "r"+strconv.Itoa(n)+"1", "r"+strconv.Itoa(n)+"2")
		}
	})
	h.p.PlayChosen(tracks("a"), 0)
	h.wait("a playing", playing("a"))
	h.p.Ended()
	h.noticed(radiosLabeled)
	h.wait("stopping", func(p *Player) bool { return !p.s.Playing && !p.s.Loading && p.s.Index == 0 })
	l.with(func(l *labeled) {
		if !slices.Equal(l.seeds, []string{"yt:a", "yt:a", "yt:a"}) {
			t.Errorf("the radios started from %v", l.seeds)
		}
	})
	if got := len(h.p.State().Queue); got != 1+3*maxEmptyRadios {
		t.Errorf("%d entries", got)
	}
	// The radios' tracks were checked as they came, after the tracks about
	// to play.
	l.with(func(l *labeled) {
		if !slices.Contains(l.soon, false) {
			t.Error("no radio's check was later than the others")
		}
	})
}

// TestSkipAIRadioKnown leaves out of a radio the tracks known to be
// labeled, and stops, telling why, when they are all it holds.
func TestSkipAIRadioKnown(t *testing.T) {
	h, l := newLabeled(t, Saved{}, []string{"x", "y"}, func(f *fakeServices) {
		f.settings.Autoplay = true
		f.radio = tracks("x", "y")
	})
	l.with(func(l *labeled) { l.known["yt:x"], l.known["yt:y"] = LabelAI, LabelAI })
	h.p.PlayChosen(tracks("a"), 0)
	h.wait("a playing", playing("a"))
	h.p.Ended()
	h.noticed(radioLabeled)
	if got := len(h.p.State().Queue); got != 1 {
		t.Errorf("%d entries", got)
	}
}

// TestSkipAIPaused plays and preloads while the checks pause, and tells it
// once.
func TestSkipAIPaused(t *testing.T) {
	h, l := newLabeled(t, Saved{}, []string{"b", "c"})
	l.with(func(l *labeled) { l.paused = true })
	h.p.PlayTracks(tracks("a", "b", "c"), 0)
	h.wait("a playing", playing("a"))
	h.p.Time(11*time.Second, 200*time.Second)
	h.wait("b preloaded", func(*Player) bool { return h.stream.has("preload url:yt:b") })
	h.p.Ended()
	h.wait("b playing", playing("b"))
	h.p.Ended()
	h.wait("c playing", playing("c"))
	n := 0
	for _, note := range h.notices() {
		if note.Message == pausedChecks {
			n++
		}
	}
	if n != 1 {
		t.Errorf("notices %+v", h.notices())
	}
}

// TestSkipAISettings follows the settings: off, the marked entries play
// again; on, the current track plays on.
func TestSkipAISettings(t *testing.T) {
	h, _ := newLabeled(t, Saved{}, []string{"b"})
	h.p.PlayChosen(tracks("a", "b", "c"), 0)
	h.wait("a playing", playing("a"))
	h.p.Time(11*time.Second, 200*time.Second)
	h.wait("b marked", func(p *Player) bool { return slices.Equal(marked(p.s), []string{"yt:b"}) })
	if next, _ := h.p.State().UpNext(); next.Track.ID != "yt:c" {
		t.Errorf("up next %s", next.Track.ID)
	}
	off := settings.Default()
	off.SkipAI = false
	h.p.SettingsChanged(off)
	h.wait("skipping off", func(p *Player) bool { return !p.s.SkipAI })
	if next, _ := h.p.State().UpNext(); next.Track.ID != "yt:b" {
		t.Errorf("up next %s", next.Track.ID)
	}
	h.p.SettingsChanged(settings.Settings{SkipAI: true, SpotifyPlayback: off.SpotifyPlayback})
	h.wait("skipping on", func(p *Player) bool { return p.s.SkipAI })
	h.on(func(p *Player) {
		if !playing("a")(p) {
			t.Error("a stopped")
		}
	})
}

// TestSkipAIRestored marks, at the start, the entries of the queue kept
// whose label the app knows, without a check.
func TestSkipAIRestored(t *testing.T) {
	// The app knew b's label before the player started.
	l := &labeled{ai: map[string]bool{"yt:b": true}, failing: map[string]bool{}, known: map[string]Label{"yt:b": LabelAI}}
	h := newHarnessWith(t, Saved{Tracks: tracks("a", "b"), Index: 0}, func(f *fakeServices) Services {
		l.fakeServices = f
		return l
	}, func(f *fakeServices) { f.settings.SkipAI = true })
	h.wait("b marked", func(p *Player) bool { return slices.Equal(marked(p.s), []string{"yt:b"}) })
	if got := l.checks(); len(got) != 0 {
		t.Errorf("checked %v", got)
	}
}

// TestForgetLabel plays the entries of a track whose label was forgotten.
func TestForgetLabel(t *testing.T) {
	h, l := newLabeled(t, Saved{}, []string{"b"})
	h.p.PlayChosen(tracks("a", "b"), 0)
	h.wait("a playing", playing("a"))
	h.p.Time(11*time.Second, 200*time.Second)
	h.wait("b marked", func(p *Player) bool { return slices.Equal(marked(p.s), []string{"yt:b"}) })
	l.with(func(l *labeled) {
		delete(l.known, "yt:b")
		delete(l.ai, "yt:b")
	})
	h.p.ForgetLabel("yt:b")
	h.wait("b unmarked", func(p *Player) bool { return len(marked(p.s)) == 0 })
	h.p.Ended()
	h.wait("b playing", playing("b"))
}

// TestSkipAIFallback checks a Spotify track the player moved to, whose
// device couldn't play it, by its recording, before it plays from there.
func TestSkipAIFallback(t *testing.T) {
	h, l := newLabeled(t, Saved{}, nil, func(f *fakeServices) {
		f.prepare = onDevice
		f.settings.SpotifyPlayback = settings.SpotifyPlaybackConnect
	})
	l.with(func(l *labeled) { l.ai["spotify:2"] = true })
	h.device.playErr = errors.New("no Spotify device is active.")
	h.p.PlayChosen([]media.Track{spotifyTrack("1"), spotifyTrack("2"), spotifyTrack("3")}, 0)
	h.wait("1 from its recording", func(*Player) bool { return h.stream.lastLoad() == "load url:spotify:1 0s true" })
	if got := l.checks(); len(got) != 0 {
		t.Errorf("a track chosen was checked: %v", got)
	}
	h.p.Next()
	h.wait("3 from its recording", func(*Player) bool { return h.stream.lastLoad() == "load url:spotify:3 0s true" })
	if h.stream.has("load url:spotify:2 0s true") {
		t.Error("2 played")
	}
	h.noticed("Skipped Spotify 2, whose recording on YouTube Music YouTube labels as made with AI")
}
