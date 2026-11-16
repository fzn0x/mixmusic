package room

import (
	"strconv"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/remote"
)

// TestSongsFoundByTheHost takes a guest's song as the host finds it by its
// ID, not as the guest says it is: its title, and no cover the guest
// named.
func TestSongsFoundByTheHost(t *testing.T) {
	r, f := newRuledRoom(t, Rules{Songs: SongsAsk})
	f.titles = map[string]string{song("a").ID: "The real title"}
	s := r.join(newGuest(t, "Ana"))
	ch := events(s)
	fake := song("a")
	fake.Title = "A title of the guest's"
	fake.Artwork = []remote.Image{{URL: "https://192.168.1.1/admin.png"}}
	s.AddSong(fake)
	next(t, ch, SongAnswered)
	reqs := r.server.Requests()
	if len(reqs) != 1 || reqs[0].Track.Title != "The real title" || len(reqs[0].Track.Artwork) != 0 {
		t.Fatalf("requests %+v", reqs)
	}
	f.mu.Lock()
	notes := append([]string(nil), f.notes...)
	f.mu.Unlock()
	if len(notes) != 1 || notes[0] != "Ana asks to add The real title" {
		t.Fatalf("the host's user was told %q", notes)
	}
}

// TestFileSongsWaitForTheHost answers a guest's file, in Ask me first, as
// waiting, though the host has none of it, so that the guest learns
// nothing of the host's files; the host finds it once its user accepts it,
// and the guest is told then.
func TestFileSongsWaitForTheHost(t *testing.T) {
	r, f := newRuledRoom(t, Rules{Songs: SongsAsk})
	file := remote.QueueTrack{Source: "file", Title: "A song of mine"}
	f.cant = map[string]bool{"": true}
	s := r.join(newGuest(t, "Ana"))
	ch := events(s)
	s.AddSong(file)
	if e := next(t, ch, SongAnswered); e.Reason != SongWaits {
		t.Fatalf("answer %q", e.Reason)
	}
	f.mu.Lock()
	looks := f.looks
	f.mu.Unlock()
	if looks != 0 {
		t.Fatal("the host looked for the file before its user accepted it")
	}
	reqs := r.server.Requests()
	if err := r.server.Accept(reqs[0].ID); err == nil {
		t.Fatal("accepted a file the host has none of")
	}
	if e := next(t, ch, SongAnswered); e.Reason != CantPlay {
		t.Fatalf("answer once accepted %q", e.Reason)
	}
}

// TestSongsFoundFewAtOnce finds maxLooksEach songs of a guest at once,
// and maxLooks in the room: more are refused until those are found.
func TestSongsFoundFewAtOnce(t *testing.T) {
	was := lookBurst
	lookBurst = 100
	t.Cleanup(func() { lookBurst = was })
	r, f := newRuledRoom(t, Rules{Songs: SongsAdd})
	f.gate = make(chan struct{})
	answers := make(chan string, 64)
	const guests, each = 5, maxLooksEach + 1
	for i := range guests {
		s := r.join(newGuest(t, "Guest "+strconv.Itoa(i)))
		s.Watch(func(e Event) {
			if e.Kind == SongAnswered {
				answers <- e.Reason
			}
		})
		for j := range each {
			s.AddSong(song(strconv.Itoa(j)))
		}
	}
	// Those past the first of each guest, and of the room, are refused at
	// once; the others wait to be found.
	refused := 0
	deadline := time.After(5 * time.Second)
	for refused < guests*each-maxLooks {
		select {
		case reason := <-answers:
			if reason != TooMany {
				t.Fatalf("answered %q while they're found", reason)
			}
			refused++
		case <-deadline:
			t.Fatalf("%d refused", refused)
		}
	}
	close(f.gate)
	waitFor(t, "the songs found added", func() bool { return len(f.addedNow()) == maxLooks })
}

// TestSongsFoundFewAMinute finds lookBurst of the guests' songs at once,
// then one each lookEvery, of all the guests.
func TestSongsFoundFewAMinute(t *testing.T) {
	was := lookEvery
	lookEvery = time.Hour
	t.Cleanup(func() { lookEvery = was })
	r, _ := newRuledRoom(t, Rules{Songs: SongsAdd})
	add := func(s *Session, ch <-chan Event, id string) string {
		s.AddSong(song(id))
		return next(t, ch, SongAnswered).Reason
	}
	for i, n := range []int{lookBurst / 2, lookBurst - lookBurst/2} {
		s := r.join(newGuest(t, "Guest "+strconv.Itoa(i)))
		ch := events(s)
		for j := range n {
			if reason := add(s, ch, strconv.Itoa(j)); reason != "" {
				t.Fatalf("guest %d's song %d: %q", i, j, reason)
			}
		}
	}
	s := r.join(newGuest(t, "One more"))
	if reason := add(s, events(s), "x"); reason != TooMany {
		t.Fatalf("one past the room's: %q", reason)
	}
}

// TestAcceptAfterBlock adds nothing of a guest blocked since it asked.
func TestAcceptAfterBlock(t *testing.T) {
	r, f := newRuledRoom(t, Rules{Songs: SongsAsk})
	ana := newGuest(t, "Ana")
	s := r.join(ana)
	ch := events(s)
	s.AddSong(song("a"))
	next(t, ch, SongAnswered)
	id := r.server.Requests()[0].ID
	r.server.Block(ana.Identity.Fingerprint())
	if err := r.server.Accept(id); err != nil {
		t.Fatal(err)
	}
	if len(f.addedNow()) != 0 {
		t.Fatal("added the song of a guest blocked since it asked")
	}
}

// TestSongsOfNoTitleFindNothing refuses songs of no title before they take
// a place among those the host finds: as many as it finds at once leave
// room for the next.
func TestSongsOfNoTitleFindNothing(t *testing.T) {
	r, f := newRuledRoom(t, Rules{Songs: SongsAdd})
	for i := range 2 {
		s := r.join(newGuest(t, "Guest "+strconv.Itoa(i)))
		c := events(s)
		for range maxLooks / 2 {
			s.AddSong(remote.QueueTrack{Source: "youtube", ID: song("a").ID})
			if e := next(t, c, SongAnswered); e.Reason != CantPlay {
				t.Fatalf("a song of no title: %q", e.Reason)
			}
		}
	}
	s := r.join(newGuest(t, "Bo"))
	c := events(s)
	s.AddSong(song("b"))
	if e := next(t, c, SongAnswered); e.Reason != "" {
		t.Fatalf("a song once those of no title were refused: %q", e.Reason)
	}
	if len(f.addedNow()) != 1 {
		t.Fatal("not added")
	}
}
