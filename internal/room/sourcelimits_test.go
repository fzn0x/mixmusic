package room

import (
	"strconv"
	"testing"
)

// TestSongsBoundedBySource bounds the songs of a source's guests, however
// many keys they have: three guests of one address add twice a guest's
// songs at once, and no more.
func TestSongsBoundedBySource(t *testing.T) {
	r, f := newRuledRoom(t, Rules{Songs: SongsAdd})
	// One song at a time, each once the last was answered.
	refused := 0
	for i := range 3 {
		s := r.join(newGuest(t, "Guest "+strconv.Itoa(i)))
		ch := events(s)
		for j := range songsPerMinute {
			s.AddSong(song(strconv.Itoa(j)))
			if e := next(t, ch, SongAnswered); e.Reason == TooMany {
				refused++
			}
		}
	}
	if refused != songsPerMinute || len(f.addedNow()) != 2*songsPerMinute {
		t.Fatalf("%d refused, %d added", refused, len(f.addedNow()))
	}
}
