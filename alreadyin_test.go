package main

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/app"
	"github.com/fzn0x/mixmusic/internal/media"
)

// fakeAdds answers for YouTube Music the adds to a playlist with err, and
// takes those made anyway.
type fakeAdds struct {
	mu      sync.Mutex
	err     error
	adds    []string
	anyways []string
}

func (f *fakeAdds) made() (adds, anyways []string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.adds...), append([]string(nil), f.anyways...)
}

// addsFor has YouTube Music answer the test's adds with f.
func addsFor(t *testing.T, f *fakeAdds) {
	t.Helper()
	add, anyway := addOnYouTube, addAnywayOnYouTube
	addOnYouTube = func(_ *shell, _ context.Context, playlistID string, tr media.Track) error {
		f.mu.Lock()
		defer f.mu.Unlock()
		f.adds = append(f.adds, playlistID+" "+tr.ID)
		return f.err
	}
	addAnywayOnYouTube = func(_ *shell, _ context.Context, playlistID string, tr media.Track) error {
		f.mu.Lock()
		defer f.mu.Unlock()
		f.anyways = append(f.anyways, playlistID+" "+tr.ID)
		return nil
	}
	t.Cleanup(func() { addOnYouTube, addAnywayOnYouTube = add, anyway })
}

// toasted draws the window once what the window was told comes, and
// reports whether a toast shows text.
func toasted(t *testing.T, s *shell, tt *ui.Tester, text string) bool {
	t.Helper()
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(time.Millisecond) {
		s.drain()
		tt.Frame()
		if tt.HasText(text) {
			return true
		}
	}
	return false
}

// youtubeAlbumPage shows an album, signed in to YouTube Music, whose
// tracks' menus add to the playlist Late night of the user's.
func youtubeAlbumPage(t *testing.T) (*shell, *ui.Tester) {
	t.Helper()
	s := newTestShell(t)
	s.loads["accounts"] = &loaded[app.AccountsStatus]{done: true, value: app.AccountsStatus{YouTube: app.YouTubeAccount{SignedIn: true}}}
	s.loads["youtube:playlists"] = &loaded[[]media.Playlist]{done: true, value: []media.Playlist{
		{ID: "yt:PLmine", Title: "Late night", Source: media.SourceYouTube, Editable: true},
	}}
	s.loads["album:al-x"] = &loaded[media.Album]{done: true, value: media.Album{ID: "al-x", Title: "Mezzanine", Tracks: testTracks[:2]}}
	tt := tester(t, s)
	show(s, tt, "/album/al-x")
	return s, tt
}

// TestAlreadyInYouTubePlaylist says a song is already in the playlist
// when YouTube Music refuses it as one it holds, with Add anyway, which
// adds it a second time; other refusals say what they are.
func TestAlreadyInYouTubePlaylist(t *testing.T) {
	f := &fakeAdds{err: app.ErrAlreadyInPlaylist}
	addsFor(t, f)
	s, tt := youtubeAlbumPage(t)
	if err := trackMenuChoice(t, tt, "Add to YouTube Music playlist", "Late night"); err != nil {
		t.Fatal(err)
	}
	if !toasted(t, s, tt, "Already in Late night") {
		t.Fatalf("the toast says %q", tt.Texts())
	}
	click(t, s, tt, "add anyway")
	if !toasted(t, s, tt, "Added "+testTracks[0].Title+" to Late night again") {
		t.Fatalf("added anyway, the toast says %q", tt.Texts())
	}
	adds, anyways := f.made()
	if len(adds) != 1 || len(anyways) != 1 || anyways[0] != "yt:PLmine "+testTracks[0].ID {
		t.Errorf("added %q, anyway %q", adds, anyways)
	}

	// Another refusal says what it is, without Add anyway.
	f.mu.Lock()
	f.err = errors.New("YouTube Music refused: the playlist may be another account's")
	f.mu.Unlock()
	if err := trackMenuChoice(t, tt, "Add to YouTube Music playlist", "Late night"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(time.Minute) // the toasts before go
	if !toasted(t, s, tt, "the playlist may be another account's") || tt.HasText("Already in") {
		t.Errorf("the toast says %q", tt.Texts())
	}
}

// TestAlreadyInMix says a track is already in a mix that holds it, with
// Add anyway, which adds it a second time, and adds one it doesn't hold.
func TestAlreadyInMix(t *testing.T) {
	s, tt := youtubeAlbumPage(t)
	mix, err := s.svc.Collection.CreatePlaylist("Road trip", "", testTracks[:1])
	if err != nil {
		t.Fatal(err)
	}
	holds := func() []string {
		p, _ := s.svc.Catalog.PlaylistStart(context.Background(), mix.ID)
		var ids []string
		for _, tr := range p.Tracks {
			ids = append(ids, tr.ID)
		}
		return ids
	}
	if err := trackMenuChoice(t, tt, "Add to mix", "Road trip"); err != nil {
		t.Fatal(err)
	}
	if !toasted(t, s, tt, "Already in Road trip") {
		t.Fatalf("the toast says %q", tt.Texts())
	}
	if got := holds(); len(got) != 1 {
		t.Fatalf("the mix holds %q", got)
	}
	click(t, s, tt, "add anyway")
	if !toasted(t, s, tt, "Added "+testTracks[0].Title+" to Road trip again") {
		t.Fatalf("added anyway, the toast says %q", tt.Texts())
	}
	if got := holds(); len(got) != 2 || got[1] != testTracks[0].ID {
		t.Errorf("added anyway, the mix holds %q", got)
	}

	// Of several tracks, those it holds stay out, and the toast says how
	// many, with Add anyway for them.
	tt.Advance(time.Minute)
	s.addToMix(mix, testTracks[:3])
	if !toasted(t, s, tt, "Added 2 tracks to Road trip: 1 was in it already") {
		t.Fatalf("the toast says %q", tt.Texts())
	}
	if got := holds(); len(got) != 4 {
		t.Errorf("the mix holds %q", got)
	}
	click(t, s, tt, "add anyway")
	if got := holds(); len(got) != 5 || got[4] != testTracks[0].ID {
		t.Errorf("added anyway, the mix holds %q", got)
	}
}
