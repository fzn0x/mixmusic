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
	if k == nil || !tell || !k.Local || len(k.Key) != 8 {
		t.Fatalf("the first knock %+v, told %v", k, tell)
	}
	if k, tell := s.knock(key(), "192.168.1.2", "Ana again", now); k == nil || tell {
		t.Fatalf("the second knock of the source %+v, told %v", k, tell)
	}
	if k, _ := s.knock(key(), "192.168.1.2", "Ana thrice", now); k != nil {
		t.Fatal("a third knock of a source kept")
	}
	for i := 3; len(s.asks.knocks) < maxKnocks; i++ {
		if k, tell := s.knock(key(), "203.0.113."+strconv.Itoa(i), "Someone", now); k == nil || !tell || k.Local {
			t.Fatalf("a knock from the internet %+v, told %v", k, tell)
		}
	}
	if k, _ := s.knock(key(), "198.51.100.1", "One more", now); k != nil {
		t.Fatal("a knock past maxKnocks kept")
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

// TestLocalSources tells the host's own networks from the internet.
func TestLocalSources(t *testing.T) {
	for src, want := range map[string]bool{"192.168.1.2": true, "10.0.0.1": true, "127.0.0.1": true, "100.100.1.1": true,
		"fd00:1:2:3::/64": true, "fe80::/64": true, "203.0.113.7": false, "2001:db8:1:2::/64": false, "nonsense": false} {
		if got := localSource(src); got != want {
			t.Errorf("localSource(%q) = %v", src, got)
		}
	}
}
