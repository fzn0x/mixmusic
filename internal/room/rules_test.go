package room

import (
	"sync"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/remote"
)

// fakeGuests is a host's app for its guests' asks: the rules it has, the
// songs it adds, and what it tells its user.
type fakeGuests struct {
	mu    sync.Mutex
	rules Rules
	cant  map[string]bool
	added []remote.QueueTrack
	notes []string
}

func (f *fakeGuests) set(r Rules) {
	f.mu.Lock()
	f.rules = r
	f.mu.Unlock()
}

func (f *fakeGuests) guests() *Guests {
	return &Guests{
		Rules: func() Rules { f.mu.Lock(); defer f.mu.Unlock(); return f.rules },
		Playable: func(t remote.QueueTrack) bool {
			f.mu.Lock()
			defer f.mu.Unlock()
			return !f.cant[t.ID]
		},
		Add: func(_ Member, t remote.QueueTrack) error {
			f.mu.Lock()
			f.added = append(f.added, t)
			f.mu.Unlock()
			return nil
		},
		Asks: func(note string) {
			f.mu.Lock()
			f.notes = append(f.notes, note)
			f.mu.Unlock()
		},
	}
}

func (f *fakeGuests) addedNow() []remote.QueueTrack {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]remote.QueueTrack(nil), f.added...)
}

// newRuledRoom is a room whose host's app is f, of the rules r.
func newRuledRoom(t *testing.T, r Rules) (*testRoom, *fakeGuests) {
	t.Helper()
	f := &fakeGuests{rules: r}
	return newTestRoomWith(t, func(s *Server) { s.Guests = f.guests() }), f
}

// events are what a session tells, as they come.
func events(s *Session) <-chan Event {
	ch := make(chan Event, 256)
	s.Watch(func(e Event) {
		select {
		case ch <- e:
		default:
		}
	})
	return ch
}

// next waits for the next event of kind from ch.
func next(t *testing.T, ch <-chan Event, kind EventKind) Event {
	t.Helper()
	deadline := time.After(5 * time.Second)
	for {
		select {
		case e := <-ch:
			if e.Kind == kind {
				return e
			}
		case <-deadline:
			t.Fatalf("waited 5 s for an event of kind %d", kind)
		}
	}
}

// TestRulesToldAndChanged tells a guest that joins the host's rules, and
// again as they change.
func TestRulesToldAndChanged(t *testing.T) {
	r, f := newRuledRoom(t, Rules{Songs: SongsAsk})
	s := r.join(newGuest(t, "Ana"))
	waitFor(t, "the rules", func() bool { got, ok := s.Rules(); return ok && got == Rules{Songs: SongsAsk} })
	f.set(Rules{Songs: "anything"})
	r.server.RulesChanged()
	waitFor(t, "the new rules", func() bool { got, _ := s.Rules(); return got == Rules{Songs: SongsOff} })
}

// TestRulesToldAtTheTick tells the guests the host's rules that changed
// without a word, as at its next tick.
func TestRulesToldAtTheTick(t *testing.T) {
	r, f := newRuledRoom(t, Rules{})
	s := r.join(newGuest(t, "Ana"))
	waitFor(t, "the rules", func() bool { _, ok := s.Rules(); return ok })
	f.set(Rules{Songs: SongsAdd})
	waitFor(t, "the new rules", func() bool { got, _ := s.Rules(); return got.Songs == SongsAdd })
}

// TestNoRulesTakeNothing: a host without rules for its guests tells them
// it takes no songs.
func TestNoRulesTakeNothing(t *testing.T) {
	r := newTestRoom(t)
	s := r.join(newGuest(t, "Ana"))
	waitFor(t, "the rules", func() bool { got, ok := s.Rules(); return ok && got == Rules{Songs: SongsOff} })
	ch := events(s)
	if _, err := s.AddSong(remote.QueueTrack{Source: "youtube", ID: "yt:aaaaaaaaaaa", Title: "A song"}); err != nil {
		t.Fatal(err)
	}
	if e := next(t, ch, SongAnswered); e.Reason != SongsAreOff {
		t.Fatalf("answer %q", e.Reason)
	}
}

// TestBucketRefills lets size asks through at once, then one each every.
func TestBucketRefills(t *testing.T) {
	var b bucket
	now := time.Now()
	for i := range 5 {
		if !b.take(now, 5, time.Second) {
			t.Fatalf("ask %d refused", i)
		}
	}
	if b.take(now, 5, time.Second) {
		t.Fatal("a sixth ask at once went through")
	}
	if !b.take(now.Add(time.Second), 5, time.Second) || b.take(now.Add(time.Second), 5, time.Second) {
		t.Fatal("a second didn't let exactly one more through")
	}
	if !b.take(now.Add(time.Hour), 5, time.Second) {
		t.Fatal("refused after an hour")
	}
}
