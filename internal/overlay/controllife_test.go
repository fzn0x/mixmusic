package overlay

import (
	"testing"
	"time"
)

// TestControlStreamEndsAfterReadsToggled shows the switch of the reads
// keeping the control's streams: a program streams with control on, the
// reads go on, and then control goes off, which ends its stream.
func TestControlStreamEndsAfterReadsToggled(t *testing.T) {
	s, base, _ := startControl(t)
	ch, stop := controlStream(t, base)
	defer stop()
	stateOf(t, ch)
	if err := s.Apply(true, true, 0); err != nil {
		t.Fatal(err)
	}
	if err := s.Apply(true, false, 0); err != nil {
		t.Fatal(err)
	}
	select {
	case _, ok := <-ch:
		if ok {
			t.Fatal("the control's stream is still open after control went off")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("the control's stream didn't end after control went off")
	}
}

// TestRewatchFollowsThePlayerAgain shows a server that started with no
// player hearing of it once the app calls Rewatch.
func TestRewatchFollowsThePlayerAgain(t *testing.T) {
	s := New()
	s.SetToken(testToken)
	defer s.Close()
	if err := s.Apply(false, true, 0); err != nil {
		t.Fatal(err)
	}
	base := "http://" + s.Status().Addr
	ch, stop := controlStream(t, base)
	defer stop()
	stateOf(t, ch)
	// The app makes its player, and the server watches it from now on.
	ctl := &fakeControl{extra: Extra{Volume: 10}}
	s.Control = ctl
	s.Rewatch()
	ctl.mu.Lock()
	ctl.extra.Volume = 20
	ctl.mu.Unlock()
	ctl.changed()
	if st := stateOf(t, ch); st.Volume != 20 {
		t.Errorf("after the rewatch: %+v", st)
	}
}
