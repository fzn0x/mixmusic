package remote

import (
	"errors"
	"testing"
)

// The computer paired with keeps the pairing before it tells the other it
// is paired: these tests end a pairing as that keeping goes, and check
// that both computers end it the same way.

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
	<-h.ended
	if h.Allowed(d.Identity.Fingerprint()) {
		t.Error("kept as paired there")
	}
}
