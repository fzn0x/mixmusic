package room

import (
	"sync"
	"testing"
)

// fakeControls are the controls a host's app took.
type fakeControls struct {
	mu  sync.Mutex
	got []string
}

func (f *fakeControls) now() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.got...)
}

// newControlledRoom is a room whose host's app is f, of the rules r, and
// takes its guests' controls in fc.
func newControlledRoom(t *testing.T, r Rules) (*testRoom, *fakeGuests, *fakeControls) {
	t.Helper()
	f, fc := &fakeGuests{rules: r}, &fakeControls{}
	tr := newTestRoomWith(t, func(s *Server) {
		s.Guests = f.guests()
		s.Guests.Control = func(_ Member, do string) error {
			fc.mu.Lock()
			fc.got = append(fc.got, do)
			fc.mu.Unlock()
			return nil
		}
	})
	return tr, f, fc
}

// TestControlByTheRules has the host's player do what a guest asks while
// the rules let guests control it, controls at once at most; refuses it
// otherwise, and ignores what isn't a control.
func TestControlByTheRules(t *testing.T) {
	r, f, fc := newControlledRoom(t, Rules{})
	s := r.join(newGuest(t, "Ana"))
	ch := events(s)
	s.Control(Pause)
	if e := next(t, ch, ControlRefused); e.Reason != NoControl {
		t.Fatalf("refused for %q", e.Reason)
	}
	f.set(Rules{Control: true})
	r.server.RulesChanged()
	waitFor(t, "the rules", func() bool { got, _ := s.Rules(); return got.Control })
	s.Control("seek")
	for range controls {
		s.Control(Next)
	}
	s.Control(Pause)
	if e := next(t, ch, ControlRefused); e.Reason != TooMany {
		t.Fatalf("refused for %q", e.Reason)
	}
	if got := fc.now(); len(got) != controls || got[0] != Next {
		t.Fatalf("the player was asked %q", got)
	}
	act := r.server.Activity()
	if last := act[len(act)-1]; last.Name != "Ana" || last.What != Skipped {
		t.Fatalf("activity %+v", act)
	}
}

// TestNoRulesNoControl refuses every control of a host without rules for
// its guests.
func TestNoRulesNoControl(t *testing.T) {
	r := newTestRoom(t)
	s := r.join(newGuest(t, "Ana"))
	ch := events(s)
	if err := s.Control(Pause); err != nil {
		t.Fatal(err)
	}
	if e := next(t, ch, ControlRefused); e.Reason != NoControl {
		t.Fatalf("refused for %q", e.Reason)
	}
}
