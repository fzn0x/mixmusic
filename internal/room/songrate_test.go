package room

import (
	"strconv"
	"testing"
	"time"
)

// TestRoomsLookupsSpendNoGuestsSongs has a guest add songs once the room
// finds none for a while: each is refused, and spends nothing of the songs
// the guest may add, which it adds once the room finds songs again.
func TestRoomsLookupsSpendNoGuestsSongs(t *testing.T) {
	was := lookEvery
	lookEvery = time.Hour
	t.Cleanup(func() { lookEvery = was })
	r, _ := newRuledRoom(t, Rules{Songs: SongsAdd})
	add := func(s *Session, ch <-chan Event, id string) string {
		s.AddSong(song(id))
		return next(t, ch, SongAnswered).Reason
	}
	// Another guest takes all the room's lookups.
	other := r.join(newGuest(t, "Bo"))
	och := events(other)
	for j := range lookBurst {
		if reason := add(other, och, "b"+strconv.Itoa(j)); reason != "" {
			t.Fatalf("Bo's song %d: %q", j, reason)
		}
	}
	ana := r.join(newGuest(t, "Ana"))
	ch := events(ana)
	for j := range 2 * songsPerMinute {
		if reason := add(ana, ch, "a"+strconv.Itoa(j)); reason != TooMany {
			t.Fatalf("Ana's song %d, as the room finds none: %q", j, reason)
		}
	}
	// The room finds songs again: Ana still adds all hers at once.
	r.server.mu.Lock()
	lookEvery, r.server.asks.lookRate = time.Millisecond, bucket{}
	r.server.mu.Unlock()
	for j := range songsPerMinute {
		if reason := add(ana, ch, "c"+strconv.Itoa(j)); reason != "" {
			t.Fatalf("Ana's song %d, once the room finds songs again: %q", j, reason)
		}
	}
}
