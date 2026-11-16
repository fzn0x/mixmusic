package room

import (
	"strconv"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/remote"
)

// TestKnocksBounded keeps maxKnocksFrom knocks of a source, and maxKnocks
// in all, and tells the host's user of a source's first knock, and of the
// next a while after.
func TestKnocksBounded(t *testing.T) {
	s := &Server{}
	key := func() remote.Fingerprint { id, _ := remote.NewIdentity(); return id.Fingerprint() }
	now := time.Now()
	k, tell := s.knock(key(), "192.168.1.2", "Ana", now)
	if k == nil || !tell || len(k.Key) != 8 {
		t.Fatalf("the first knock %+v, told %v", k, tell)
	}
	if k, tell := s.knock(key(), "192.168.1.2", "Ana again", now); k == nil || tell {
		t.Fatalf("the second knock of the source %+v, told %v", k, tell)
	}
	if k, _ := s.knock(key(), "192.168.1.2", "Ana thrice", now); k != nil {
		t.Fatal("a third knock of a source kept")
	}
	for i := 3; len(s.asks.knocks) < maxKnocks; i++ {
		if k, tell := s.knock(key(), "203.0.113."+strconv.Itoa(i), "Someone", now); k == nil || !tell {
			t.Fatalf("a knock from the internet %+v, told %v", k, tell)
		}
	}
	if k, _ := s.knock(key(), "198.51.100.1", "One more", now); k != nil {
		t.Fatal("a knock past maxKnocks kept")
	}
	// This computer's own knocks aren't counted by source, but by the
	// room's maxKnocks.
	s.asks.knocks = nil
	for range maxKnocks {
		if k, _ := s.knock(key(), "127.0.0.1", "Here", now); k == nil {
			t.Fatal("a knock of this computer refused")
		}
	}
	if k, _ := s.knock(key(), "127.0.0.1", "Here", now); k != nil {
		t.Fatal("a knock of this computer past maxKnocks kept")
	}
	// Once its knocks went, the source told a minute ago isn't told of
	// again; a minute later, it is.
	s.asks.knocks = nil
	if _, tell := s.knock(key(), "192.168.1.2", "Ana", now.Add(time.Second)); tell {
		t.Fatal("told again at once")
	}
	s.asks.knocks = nil
	if _, tell := s.knock(key(), "192.168.1.2", "Ana", now.Add(knockTellEvery)); !tell {
		t.Fatal("not told a while after")
	}
}

// TestKnocksNearby tells, of the addresses a knock comes from, those of
// this computer itself, of both families, whose knocks aren't counted by
// source (nearby_test.go tells those of its networks).
func TestKnocksNearby(t *testing.T) {
	for addr, want := range map[string]bool{"127.0.0.1:5": true, "[::1]:5": true, "192.168.1.2:5": false, "[fe80::9]:5": false} {
		if got := knocksHere(sourceOf(addr)); got != want {
			t.Errorf("knocksHere(sourceOf(%q)) = %v", addr, got)
		}
	}
}
