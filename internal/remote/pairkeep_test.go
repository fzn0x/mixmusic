package remote

import (
	"errors"
	"testing"
	"time"
)

// The computer paired with keeps the pairing before it tells the other it
// is paired: these tests end a pairing as that keeping goes, and check
// that both computers end it the same way.

// slowHost keeps a pairing as its fakeHost does, once before returned, as
// on a slow or busy disk.
type slowHost struct {
	*fakeHost
	before func()
}

func (h slowHost) Pairing(req PairRequest) (PairEnd, error) {
	end, err := h.fakeHost.Pairing(req)
	if err != nil {
		return end, err
	}
	keep := end.Keep
	end.Keep = func() error {
		h.before()
		return keep()
	}
	return end, nil
}

// keeping is the timing of the tests whose keeping outlasts a silence:
// fast's, with a Keep longer than its silence.
var keeping = func() Timing {
	t := fast
	t.Keep = 2 * fast.Silence
	return t
}()

// TestPairSlowKeep pairs both computers when the one paired with keeps the
// pairing slower than a silence, within Keep, and neither when it takes
// longer. A computer of a MixMusic before FeatureKeep, that pairs, waits a
// silence alone: keeping the pairing within half of it pairs both, and
// slower neither. Keeping sleeps, as a slow disk is.
func TestPairSlowKeep(t *testing.T) {
	for _, c := range []struct {
		name    string
		older   bool
		keepFor time.Duration
		paired  bool
	}{
		{"slower than a silence", false, keeping.Silence * 3 / 2, true},
		{"slower than Keep", false, keeping.Keep + keeping.Silence/2, false},
		{"older, at once", true, 0, true},
		{"older, slower than half a silence", true, keeping.Silence * 3 / 4, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			h := slowHost{fakeHost: newFakeHost(), before: func() { time.Sleep(c.keepFor) }}
			_, addr := serve(t, h, keeping)
			d := newDialer(t, "Laptop", keeping)
			d.older = c.older
			_, err := pair(t, d, addr, h.fakeHost, same)
			ended := <-h.ended
			kept := h.Allowed(d.Identity.Fingerprint())
			if c.paired {
				if err != nil || ended != nil || !kept {
					t.Errorf("paired with %v; ended with %v there, which keeps it: %v", err, ended, kept)
				}
				return
			}
			if err == nil || !errors.Is(ended, ErrNotKept) || kept {
				t.Errorf("paired with %v; ended with %v there, which keeps it: %v", err, ended, kept)
			}
			// The other heard why, unless it gave up first.
			if !errors.Is(err, ErrNotKept) && !errors.Is(err, ErrLost) {
				t.Errorf("the pairing failed with %v", err)
			}
		})
	}
}

// TestPairNotKept fails a pairing on both computers when the one paired
// with can't keep it, which the other is told by name.
func TestPairNotKept(t *testing.T) {
	h := newFakeHost()
	h.keepErr = errors.New("the disk is full")
	_, addr := serve(t, h, fast)
	d := newDialer(t, "Laptop", fast)
	if _, err := pair(t, d, addr, h, same); !errors.Is(err, ErrNotKept) {
		t.Errorf("pairing with a computer that couldn't keep it: %v", err)
	}
	if err := <-h.ended; err == nil {
		t.Error("the pairing ended well there")
	}
	if h.Allowed(d.Identity.Fingerprint()) {
		t.Error("kept as paired there")
	}
}

// TestPairUnheard drops a pairing kept when the other computer can't be
// told it is paired, as remote control turns off while it is kept: neither
// keeps the other.
func TestPairUnheard(t *testing.T) {
	served := make(chan *Server, 1)
	h := slowHost{fakeHost: newFakeHost(), before: func() { (<-served).Close() }}
	s, addr := serve(t, h, fast)
	served <- s
	d := newDialer(t, "Laptop", fast)
	if _, err := pair(t, d, addr, h.fakeHost, same); err == nil {
		t.Error("paired with a computer that turned remote control off as it kept the pairing")
	}
	if err := <-h.ended; err == nil {
		t.Error("the pairing ended well there")
	}
	if h.Allowed(d.Identity.Fingerprint()) {
		t.Error("still kept as paired there")
	}
}
