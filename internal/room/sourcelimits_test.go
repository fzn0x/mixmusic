package room

import (
	"strconv"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/remote"
)

// TestSongsBoundedBySource bounds the songs of a source's guests, however
// many keys they have: three guests of one address add twice a guest's
// songs at once, and no more.
func TestSongsBoundedBySource(t *testing.T) {
	// More songs than the room finds at once, of its rate: those it finds a
	// minute are another test's.
	was := lookBurst
	lookBurst = 100
	t.Cleanup(func() { lookBurst = was })
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

// TestSourceRefusalSpendsNoKeyToken refuses a guest's ask its source's
// bucket refuses without spending its key's: once the source's fills
// again, the guest asks as many as its key's allow.
func TestSourceRefusalSpendsNoKeyToken(t *testing.T) {
	s := &Server{}
	c := &conn{src: "192.168.1.2"}
	songs := func(l *limits) *bucket { return &l.songs }
	s.mu.Lock()
	defer s.mu.Unlock()
	// Two other keys of the source took all of the source's.
	for i := range 2 {
		other := &conn{key: remote.Fingerprint{byte(i + 1)}, src: c.src}
		for range songsPerMinute {
			if !s.takes(other, songs, songsPerMinute, time.Hour) {
				t.Fatal("refused within the source's")
			}
		}
	}
	if s.takes(c, songs, songsPerMinute, time.Hour) {
		t.Fatal("past the source's")
	}
	if got := s.limitsOf(c.key).songs.tokens; got != songsPerMinute {
		t.Fatalf("the key's bucket has %v left", got)
	}
}
