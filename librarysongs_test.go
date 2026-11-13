package main

import (
	"context"
	"errors"
	"slices"
	"sync"
	"testing"

	"github.com/fzn0x/mixmusic/internal/app"
	"github.com/fzn0x/mixmusic/internal/media"
)

// songsSaved has YouTube answer the saves of songs to the library, and
// their removals, with answer, and returns what was asked: a song's ID,
// with "+" for a save and "-" for a removal.
func songsSaved(t *testing.T, answer func() error) func() []string {
	var (
		mu    sync.Mutex
		asked []string
	)
	was := saveOnYouTube
	saveOnYouTube = func(_ *shell, _ context.Context, tr media.Track, on bool) error {
		mu.Lock()
		if on {
			asked = append(asked, "+"+tr.ID)
		} else {
			asked = append(asked, "-"+tr.ID)
		}
		mu.Unlock()
		return answer()
	}
	t.Cleanup(func() { saveOnYouTube = was })
	return func() []string {
		mu.Lock()
		defer mu.Unlock()
		return slices.Clone(asked)
	}
}

func songIDs(tracks []media.Track) []string {
	var ids []string
	for _, tr := range tracks {
		ids = append(ids, tr.ID)
	}
	return ids
}

// TestLibrarySongs lists the songs of the user's library on YouTube Music
// in Library › YouTube Music, whose menus take them out, and saves other
// songs there from their menus, without liking them: the list shows the
// change at once, and as it was when YouTube refuses. A Spotify track's
// menu has no such item, nor has any signed out.
func TestLibrarySongs(t *testing.T) {
	t.Skip("YouTube Music is replaced by NetEase Cloud Music in the UI")
	s := newTestShell(t)
	strobe, glosoli, teardrop := testTracks[2], testTracks[5], testTracks[1]
	s.loads["accounts"] = &loaded[app.AccountsStatus]{done: true, value: app.AccountsStatus{YouTube: app.YouTubeAccount{SignedIn: true}}}
	s.loads["youtube:playlists"] = &loaded[[]media.Playlist]{done: true}
	s.loads["youtube:albums"] = &loaded[[]media.Album]{done: true}
	s.loads["youtube:artists"] = &loaded[[]media.Artist]{done: true}
	s.loads["youtube:liked"] = &loaded[media.Playlist]{done: true, value: media.Playlist{ID: "yt:LM", Tracks: []media.Track{glosoli, teardrop}}}
	songs := &loaded[[]media.Track]{done: true, value: []media.Track{strobe}}
	s.loads[youtubeSongsKey] = songs
	var refuse error
	asked := songsSaved(t, func() error { return refuse })
	s.navigate("/library/youtube")
	tt := tester(t, s)
	if !tt.HasText("songs in your library") || !tt.HasText(strobe.Title) {
		t.Fatalf("the library shows %q", tt.Texts())
	}

	if menu := menuOf(t, tt, strobe.Title); !slices.Contains(menu, "Remove from library") || slices.Contains(menu, "Add to library") {
		t.Fatalf("the menu of a song in the library holds %q", menu)
	}
	if menu := menuOf(t, tt, teardrop.Title); slices.Contains(menu, "Add to library") || slices.Contains(menu, "Remove from library") {
		t.Errorf("the menu of a Spotify track holds %q", menu)
	}

	// Saved, without a like.
	if err := tt.RightClick(glosoli.Title); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem("Add to library"); err != nil {
		t.Fatalf("%v: the menu holds %q", err, tt.Menu())
	}
	if got := songIDs(songs.value); !slices.Equal(got, []string{glosoli.ID, strobe.ID}) {
		t.Errorf("while YouTube saves it, the songs are %v", got)
	}
	until(t, s, tt, "the save", func() bool { return tt.HasText("Added " + glosoli.Title + " to your library on YouTube Music") })
	if s.a.IsLiked(glosoli.ID) {
		t.Error("saving the song liked it")
	}

	// Taken out, which YouTube refuses.
	refuse = errors.New("YouTube Music didn't take the change to your library")
	if err := tt.RightClick(strobe.Title); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem("Remove from library"); err != nil {
		t.Fatalf("%v: the menu holds %q", err, tt.Menu())
	}
	until(t, s, tt, "the refusal", func() bool { return tt.HasText(refuse.Error()) })
	if got := songIDs(songs.value); !slices.Equal(got, []string{strobe.ID, glosoli.ID}) {
		t.Errorf("once YouTube refused, the songs are %v", got)
	}
	if got := asked(); !slices.Equal(got, []string{"+" + glosoli.ID, "-" + strobe.ID}) {
		t.Errorf("YouTube was asked %v", got)
	}

	s.loads["accounts"] = &loaded[app.AccountsStatus]{done: true}
	s.navigate("/library/liked")
	if err := s.svc.Collection.SetLiked(context.Background(), glosoli, true); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if menu := menuOf(t, tt, glosoli.Title); slices.Contains(menu, "Add to library") || slices.Contains(menu, "Remove from library") {
		t.Errorf("signed out, the menu holds %q", menu)
	}
}
