package main

import (
	"context"
	"slices"
	"sync"
	"testing"

	"github.com/fzn0x/mixmusic/internal/media"
)

// savedFor has YouTube Music tell of the songs whether they are in the
// library, by saved, and returns the songs asked of.
func savedFor(t *testing.T, saved func(id string) bool) func() []string {
	var (
		mu    sync.Mutex
		asked []string
	)
	was := askSavedOnYouTube
	askSavedOnYouTube = func(_ *shell, _ context.Context, tr media.Track) (bool, error) {
		mu.Lock()
		asked = append(asked, tr.ID)
		mu.Unlock()
		return saved(tr.ID), nil
	}
	t.Cleanup(func() { askSavedOnYouTube = was })
	return func() []string {
		mu.Lock()
		defer mu.Unlock()
		return slices.Clone(asked)
	}
}

// TestSongMenuOfAPartialList asks YouTube Music of a song the Songs of a
// list that came in part lack whether it's in the library, as its menu
// opens, which then offers to take it out; once taken out, the menu
// offers to add it. Of a whole list, a song it lacks isn't asked of.
func TestSongMenuOfAPartialList(t *testing.T) {
	strobe, glosoli := testTracks[2], testTracks[5]
	asked := savedFor(t, func(id string) bool { return id == glosoli.ID })
	songsSaved(t, func() error { return nil })
	songs := &loaded[[]media.Track]{done: true, value: []media.Track{strobe}, err: firstOf(1)}
	s := youtubeLibraryShell(t, &loaded[[]media.Playlist]{done: true}, songs)
	tt := testerOf(t, s, 1280, 1400)
	show(s, tt, "/library/youtube")

	if menu := menuOf(t, tt, strobe.Title); !slices.Contains(menu, "Remove from library") {
		t.Fatalf("the menu of a song in the list holds %q", menu)
	}
	if got := asked(); len(got) != 0 {
		t.Errorf("a song in the list was asked of: %v", got)
	}
	menuOf(t, tt, glosoli.Title)
	until(t, s, tt, "the answer", func() bool {
		l, ok := s.loads[songSavedKey(glosoli.ID)].(*loaded[bool])
		return ok && l.done
	})
	if menu := menuOf(t, tt, glosoli.Title); !slices.Contains(menu, "Remove from library") || slices.Contains(menu, "Add to library") {
		t.Fatalf("the menu of a song saved, past the list, holds %q", menu)
	}
	if got := asked(); !slices.Equal(got, []string{glosoli.ID}) {
		t.Errorf("asked of %v", got)
	}

	if err := tt.RightClick(glosoli.Title); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem("Remove from library"); err != nil {
		t.Fatalf("%v: the menu holds %q", err, tt.Menu())
	}
	until(t, s, tt, "the removal", func() bool { return tt.HasText("Removed " + glosoli.Title + " from your library on YouTube Music") })
	if menu := menuOf(t, tt, glosoli.Title); !slices.Contains(menu, "Add to library") {
		t.Errorf("once taken out, the menu holds %q", menu)
	}

	// Of a whole list, a song it lacks isn't in the library.
	s.loads[youtubeSongsKey] = &loaded[[]media.Track]{done: true, value: []media.Track{strobe}}
	delete(s.loads, songSavedKey(glosoli.ID))
	tt.Frame()
	if menu := menuOf(t, tt, glosoli.Title); !slices.Contains(menu, "Add to library") {
		t.Errorf("of a whole list, the menu holds %q", menu)
	}
	s.drain()
	if got := asked(); len(got) != 1 {
		t.Errorf("of a whole list, asked of %v", got)
	}
}
