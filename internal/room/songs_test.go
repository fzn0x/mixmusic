package room

import (
	"strconv"
	"testing"

	"github.com/fzn0x/mixmusic/internal/remote"
)

// song is a song of YouTube Music, of the letter id.
func song(id string) remote.QueueTrack {
	v := ""
	for range 11 {
		v += id
	}
	return remote.QueueTrack{Source: "youtube", ID: "yt:" + v, Title: "Song " + id}
}

// TestSongsAddedAtOnce adds a guest's song to the host's queue at once,
// as the rules say: the guest hears it was, and the activity tells it.
func TestSongsAddedAtOnce(t *testing.T) {
	r, f := newRuledRoom(t, Rules{Songs: SongsAdd})
	s := r.join(newGuest(t, "Ana"))
	ch := events(s)
	seq, err := s.AddSong(song("a"))
	if err != nil {
		t.Fatal(err)
	}
	if e := next(t, ch, SongAnswered); e.Seq != seq || e.Reason != "" || e.Track.Title != "Song a" {
		t.Fatalf("answer %+v", e)
	}
	if got := f.addedNow(); len(got) != 1 || got[0].ID != song("a").ID {
		t.Fatalf("added %+v", got)
	}
	act := r.server.Activity()
	if last := act[len(act)-1]; last.Name != "Ana" || last.What != Added || last.Song != "Song a" {
		t.Fatalf("activity %+v", act)
	}
	if len(r.server.Requests()) != 0 {
		t.Fatal("a song added at once waits")
	}
}

// TestSongsAskFirst has each song wait for the host's user, who is told,
// and accepts one and declines another: the guest hears of each.
func TestSongsAskFirst(t *testing.T) {
	r, f := newRuledRoom(t, Rules{Songs: SongsAsk})
	s := r.join(newGuest(t, "Ana"))
	ch := events(s)
	a, _ := s.AddSong(song("a"))
	if e := next(t, ch, SongAnswered); e.Seq != a || e.Reason != SongWaits {
		t.Fatalf("answer %+v", e)
	}
	b, _ := s.AddSong(song("b"))
	next(t, ch, SongAnswered)
	reqs := r.server.Requests()
	if len(reqs) != 2 || reqs[0].From.Name != "Ana" || reqs[0].Track.Title != "Song a" || reqs[1].Track.Title != "Song b" {
		t.Fatalf("requests %+v", reqs)
	}
	f.mu.Lock()
	notes := append([]string(nil), f.notes...)
	f.mu.Unlock()
	if len(notes) != 2 || notes[0] != "Ana asks to add Song a" {
		t.Fatalf("the host's user was told %q", notes)
	}
	if len(f.addedNow()) != 0 {
		t.Fatal("a song that waits was added")
	}
	if err := r.server.Accept(reqs[0].ID); err != nil {
		t.Fatal(err)
	}
	if e := next(t, ch, SongAnswered); e.Seq != a || e.Reason != "" {
		t.Fatalf("answer to the song accepted %+v", e)
	}
	r.server.Decline(reqs[1].ID)
	if e := next(t, ch, SongAnswered); e.Seq != b || e.Reason != SongDeclined || e.Track.Title != "Song b" {
		t.Fatalf("answer to the song declined %+v", e)
	}
	if got := f.addedNow(); len(got) != 1 || got[0].Title != "Song a" {
		t.Fatalf("added %+v", got)
	}
	if len(r.server.Requests()) != 0 {
		t.Fatal("answered songs still wait")
	}
}

// TestSongsWaitingBounded lets MaxWaiting songs of a guest wait: the next
// is refused until the host's user answers one.
func TestSongsWaitingBounded(t *testing.T) {
	// More songs than the room finds at once, of its rate: those it finds a
	// minute are another test's.
	was := lookBurst
	lookBurst = 100
	t.Cleanup(func() { lookBurst = was })
	r, _ := newRuledRoom(t, Rules{Songs: SongsAsk})
	s := r.join(newGuest(t, "Ana"))
	ch := events(s)
	for i := range MaxWaiting {
		s.AddSong(song(strconv.Itoa(i)))
		if e := next(t, ch, SongAnswered); e.Reason != SongWaits {
			t.Fatalf("song %d: %q", i, e.Reason)
		}
	}
	s.AddSong(song("x"))
	if e := next(t, ch, SongAnswered); e.Reason != TooMany {
		t.Fatalf("one song more than may wait: %q", e.Reason)
	}
	// Another guest's still wait.
	s2 := r.join(newGuest(t, "Bo"))
	ch2 := events(s2)
	s2.AddSong(song("y"))
	if e := next(t, ch2, SongAnswered); e.Reason != SongWaits {
		t.Fatalf("another guest's song: %q", e.Reason)
	}
	if n := len(r.server.Requests()); n != MaxWaiting+1 {
		t.Fatalf("%d wait", n)
	}
}

// TestSongsRateBounded lets a guest add songsPerMinute at once, then one
// each six seconds: those past it are refused.
func TestSongsRateBounded(t *testing.T) {
	r, f := newRuledRoom(t, Rules{Songs: SongsAdd})
	s := r.join(newGuest(t, "Ana"))
	ch := events(s)
	refused := 0
	for i := range songsPerMinute + 5 {
		s.AddSong(song(strconv.Itoa(i % 10)))
		if e := next(t, ch, SongAnswered); e.Reason == TooMany {
			refused++
		}
	}
	if refused != 5 || len(f.addedNow()) != songsPerMinute {
		t.Fatalf("%d refused, %d added", refused, len(f.addedNow()))
	}
}

// TestSongsRefused refuses the songs of a host that takes none, those it
// can't play, and those of no title; and a song's request goes as its
// guest leaves.
func TestSongsRefused(t *testing.T) {
	r, f := newRuledRoom(t, Rules{Songs: SongsOff})
	s := r.join(newGuest(t, "Ana"))
	ch := events(s)
	s.AddSong(song("a"))
	if e := next(t, ch, SongAnswered); e.Reason != SongsAreOff {
		t.Fatalf("songs off: %q", e.Reason)
	}
	f.set(Rules{Songs: SongsAsk})
	f.mu.Lock()
	f.cant = map[string]bool{song("b").ID: true}
	f.mu.Unlock()
	s.AddSong(song("b"))
	if e := next(t, ch, SongAnswered); e.Reason != CantPlay {
		t.Fatalf("a song the host can't play: %q", e.Reason)
	}
	s.AddSong(remote.QueueTrack{Source: "youtube", ID: song("c").ID})
	if e := next(t, ch, SongAnswered); e.Reason != CantPlay {
		t.Fatalf("a song of no title: %q", e.Reason)
	}
	s.AddSong(song("d"))
	next(t, ch, SongAnswered)
	if len(r.server.Requests()) != 1 {
		t.Fatal("the song doesn't wait")
	}
	s.Leave()
	waitFor(t, "the request gone with its guest", func() bool { return len(r.server.Requests()) == 0 })
	if err := r.server.Accept(1 << 40); err != nil {
		t.Fatal("accepting a request that isn't:", err)
	}
}
