package remote

import (
	"errors"
	"testing"
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
