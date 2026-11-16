package app

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/player"
	"github.com/fzn0x/mixmusic/internal/player/playertest"
	"github.com/fzn0x/mixmusic/internal/settings"
	"github.com/fzn0x/mixmusic/internal/store"
)

// songsServices play every track at once, as a stream.
type songsServices struct{ playertest.Services }

func (songsServices) Prepare(_ context.Context, t media.Track) (player.Source, error) {
	return player.Source{Mode: player.ModeStream, URL: "audio:" + t.ID, Track: t}, nil
}

// artistSongs is an artist whose list of songs is the long playlist of
// six songs, read two at a time, played on a player whose notices it
// keeps.
type artistSongs struct {
	l      *longPlaylist
	c      *Catalog
	p      *player.Player
	artist media.Artist
	// whole is the list read whole, top two of its songs.
	whole []media.Track
	mu    sync.Mutex
	notes []player.Notice
}

func newArtistSongs(t *testing.T, set func(s *settings.Settings)) *artistSongs {
	t.Helper()
	l := newLongPlaylist(t)
	c := l.catalog(t)
	s := settings.Default()
	if set != nil {
		set(&s)
	}
	f, err := store.Open(filepath.Join(t.TempDir(), "settings.json"), s)
	if err != nil {
		t.Fatal(err)
	}
	c.a.settings = f
	whole, err := c.Playlist(context.Background(), longID)
	if err != nil || len(whole.Tracks) != 6 {
		t.Fatalf("the list: %d tracks, %v", len(whole.Tracks), err)
	}
	l.mu.Lock()
	l.asked = nil
	l.mu.Unlock()
	p := player.New(songsServices{}, playertest.Stream{}, playertest.Device{}, player.Saved{})
	t.Cleanup(p.Close)
	as := &artistSongs{l: l, c: c, p: p, whole: whole.Tracks,
		artist: media.Artist{ID: "yt:UC1", Source: media.SourceYouTube, Name: "Someone", SongsID: longID,
			TopTracks: []media.Track{{ID: "yt:top1", Source: media.SourceYouTube, Title: "Top 1"}}}}
	p.Notices(func(n player.Notice) {
		as.mu.Lock()
		defer as.mu.Unlock()
		as.notes = append(as.notes, n)
	})
	return as
}

// told waits for a notice that holds text.
func (as *artistSongs) told(t *testing.T, text string) player.Notice {
	t.Helper()
	for deadline := time.Now().Add(5 * time.Second); ; {
		as.mu.Lock()
		notes := slices.Clone(as.notes)
		as.mu.Unlock()
		for _, n := range notes {
			if strings.Contains(n.Message, text) {
				return n
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("no notice says %q: %+v", text, notes)
		}
		time.Sleep(time.Millisecond)
	}
}

// TestPlayArtistSongs plays the first page of an artist's songs at once,
// and the rest as they come, in their order.
func TestPlayArtistSongs(t *testing.T) {
	as := newArtistSongs(t, nil)
	g := as.gate(t, "P1")
	done := make(chan error, 1)
	go func() { done <- as.c.PlayArtist(context.Background(), as.p, as.artist, false) }()
	head := trackIDs(as.whole[:2])
	waitPlayer(t, as.p, "the first page playing", func(s player.State) bool { return slices.Equal(queueIDs(s), head) && s.Index == 0 })
	<-g.asked
	g.let()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	want := trackIDs(as.whole)
	if got := queueIDs(as.p.State()); !slices.Equal(got, want) {
		t.Errorf("the queue is %v, want %v", got, want)
	}
	if cur, _ := as.p.State().Current(); cur.Track.ID != want[0] {
		t.Errorf("%s plays", cur.Track.ID)
	}
}

// TestShuffleArtistSongs shuffles all of an artist's songs, those that
// come later among those that haven't played.
func TestShuffleArtistSongs(t *testing.T) {
	as := newArtistSongs(t, nil)
	if err := as.c.PlayArtist(context.Background(), as.p, as.artist, true); err != nil {
		t.Fatal(err)
	}
	want := slices.Sorted(slices.Values(trackIDs(as.whole)))
	waitPlayer(t, as.p, "all the songs queued", func(s player.State) bool {
		return s.Shuffle && slices.Equal(slices.Sorted(slices.Values(queueIDs(s))), want)
	})
	if s := as.p.State(); s.Index != 0 {
		t.Errorf("the entry at %d plays, of %v", s.Index, queueIDs(s))
	}
}

// TestArtistSongsPageFails keeps what was queued once a page after the
// first still fails after its tries, and says how many came.
func TestArtistSongsPageFails(t *testing.T) {
	as := newArtistSongs(t, nil)
	as.l.fail["P2"] = 4
	if err := as.c.PlayArtist(context.Background(), as.p, as.artist, false); err != nil {
		t.Fatal(err)
	}
	n := as.told(t, "Only 4 of Someone's 56 songs could be loaded")
	if !n.Error {
		t.Errorf("the notice isn't an error: %+v", n)
	}
	if got := queueIDs(as.p.State()); !slices.Equal(got, trackIDs(as.whole[:4])) {
		t.Errorf("the queue is %v", got)
	}
}

// TestArtistSongsFirstPageFails plays the artist's top songs once the
// first page of their list still fails after its tries, and says so; an
// artist without a list plays them as well, saying so too, and one
// without songs fails.
func TestArtistSongsFirstPageFails(t *testing.T) {
	as := newArtistSongs(t, nil)
	as.l.fail[""] = 4
	if err := as.c.PlayArtist(context.Background(), as.p, as.artist, false); err != nil {
		t.Fatal(err)
	}
	as.told(t, "Someone's top songs play, as the list of all their songs couldn't be loaded")
	waitPlayer(t, as.p, "the top songs", func(s player.State) bool { return slices.Equal(queueIDs(s), []string{"yt:top1"}) })

	plain := newArtistSongs(t, nil)
	a := plain.artist
	a.SongsID = ""
	if err := plain.c.PlayArtist(context.Background(), plain.p, a, false); err != nil {
		t.Fatal(err)
	}
	waitPlayer(t, plain.p, "the top songs", func(s player.State) bool { return slices.Equal(queueIDs(s), []string{"yt:top1"}) })
	if n := plain.told(t, "Someone's top songs play: YouTube Music lists no more of their songs"); n.Error {
		t.Errorf("the notice is an error: %+v", n)
	}
	if asked := plain.l.asked; len(asked) != 0 {
		t.Errorf("asked for the pages %q", asked)
	}
	a.TopTracks = nil
	if err := plain.c.PlayArtist(context.Background(), plain.p, a, false); err == nil || err.Error() != "No songs listed for Someone" {
		t.Errorf("an artist without songs: %v", err)
	}
}

// gatedPage holds the page of a list of token, "" for the first, until
// the test lets it come.
type gatedPage struct {
	h      http.Handler
	token  string
	asked  chan struct{}
	letGo  chan struct{}
	asking sync.Once
}

// gate has the songs' list of as hold the page of token.
func (as *artistSongs) gate(t *testing.T, token string) *gatedPage {
	t.Helper()
	g := &gatedPage{h: as.l, token: token, asked: make(chan struct{}), letGo: make(chan struct{})}
	srv := httptest.NewServer(g)
	t.Cleanup(srv.Close)
	t.Cleanup(g.let)
	as.c.a.youtube.BaseURL = srv.URL
	return g
}

// let lets the page come, once.
func (g *gatedPage) let() {
	select {
	case <-g.letGo:
	default:
		close(g.letGo)
	}
}

func (g *gatedPage) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	data, _ := io.ReadAll(r.Body)
	var body struct {
		Continuation string `json:"continuation"`
	}
	json.Unmarshal(data, &body)
	if body.Continuation == g.token {
		g.asking.Do(func() { close(g.asked) })
		<-g.letGo
	}
	r.Body = io.NopCloser(bytes.NewReader(data))
	g.h.ServeHTTP(w, r)
}

// TestArtistSongsAfterAnother plays nothing of the artist's once another
// track started while their first page came.
func TestArtistSongsAfterAnother(t *testing.T) {
	as := newArtistSongs(t, nil)
	g := as.gate(t, "")
	done := make(chan error, 1)
	go func() { done <- as.c.PlayArtist(context.Background(), as.p, as.artist, false) }()
	<-g.asked
	other := []media.Track{{ID: "yt:other", Source: media.SourceYouTube, Title: "Other"}}
	as.p.PlayTracks(other, 0)
	waitPlayer(t, as.p, "the other track", func(s player.State) bool { return s.Index == 0 })
	g.let()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if got := queueIDs(as.p.State()); !slices.Equal(got, []string{"yt:other"}) {
		t.Errorf("the queue is %v", got)
	}
}

// TestArtistSongsReplaced stops reading the songs once another queue
// replaced theirs: the page that came then joins nothing, and the next
// isn't asked for.
func TestArtistSongsReplaced(t *testing.T) {
	as := newArtistSongs(t, nil)
	g := as.gate(t, "P1")
	done := make(chan error, 1)
	go func() { done <- as.c.PlayArtist(context.Background(), as.p, as.artist, false) }()
	waitPlayer(t, as.p, "the first page", func(s player.State) bool { return len(s.Queue) == 2 && s.Index == 0 })
	<-g.asked
	as.p.PlayTracks([]media.Track{{ID: "yt:other", Source: media.SourceYouTube, Title: "Other"}}, 0)
	waitPlayer(t, as.p, "the other track", func(s player.State) bool { return len(s.Queue) == 1 })
	g.let()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if got := queueIDs(as.p.State()); !slices.Equal(got, []string{"yt:other"}) {
		t.Errorf("the queue is %v", got)
	}
	as.l.mu.Lock()
	defer as.l.mu.Unlock()
	if !slices.Equal(as.l.asked, []string{"", "P1"}) {
		t.Errorf("asked for the pages %q", as.l.asked)
	}
}

// TestSongsKept keeps each song once, and leaves out the explicit ones
// while the settings hide them.
func TestSongsKept(t *testing.T) {
	a := ytTrack("a")
	e := ytTrack("e")
	e.Explicit = true
	for _, c := range []struct {
		hide bool
		want []string
	}{{false, []string{"yt:a", "yt:e", "yt:b"}}, {true, []string{"yt:a", "yt:b"}}} {
		as := newArtistSongs(t, func(s *settings.Settings) { s.HideExplicit = c.hide })
		keep := as.c.a.songsKept()
		got := append(keep([]media.Track{a, e, a}), keep([]media.Track{e, ytTrack("b"), a})...)
		if !slices.Equal(trackIDs(got), c.want) {
			t.Errorf("hiding explicit songs %v: kept %v, want %v", c.hide, trackIDs(got), c.want)
		}
	}
}
