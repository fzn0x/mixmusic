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

// TestSongsFoundFewAtOnce finds maxLooks songs at once in the room: more
// are refused until those are found.
func TestSongsFoundFewAtOnce(t *testing.T) {
	r, f := newRuledRoom(t, Rules{Songs: SongsAdd})
	f.gate = make(chan struct{})
	ana, bo := r.join(newGuest(t, "Ana")), r.join(newGuest(t, "Bo"))
	ca, cb := events(ana), events(bo)
	for i := range songsPerMinute {
		ana.AddSong(song(strconv.Itoa(i)))
		bo.AddSong(song(strconv.Itoa(i)))
	}
	// Those past the first maxLooks are refused at once; the others wait.
	refused := 0
	deadline := time.After(5 * time.Second)
	for refused < 2*songsPerMinute-maxLooks {
		select {
		case e := <-ca:
			if e.Kind == SongAnswered && e.Reason == TooMany {
				refused++
			}
		case e := <-cb:
			if e.Kind == SongAnswered && e.Reason == TooMany {
				refused++
			}
		case <-deadline:
			t.Fatalf("%d refused", refused)
		}
	}
	if got := len(f.addedNow()); got != 0 {
		t.Fatalf("%d added while they're found", got)
	}
	close(f.gate)
	waitFor(t, "the songs found added", func() bool { return len(f.addedNow()) == maxLooks })
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
