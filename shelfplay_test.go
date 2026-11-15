package main

import (
	"testing"

	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/player"
)

func itemsOfTracks(tracks ...media.Track) []media.Item {
	var items []media.Item
	for i := range tracks {
		items = append(items, media.Item{Track: &tracks[i]})
	}
	return items
}

// TestShelfPlayAll plays the tracks of a shelf in the order shown from its
// Play button, and shuffled from its Shuffle button, skipping the albums
// that share the shelf.
func TestShelfPlayAll(t *testing.T) {
	items := itemsOfTracks(testTracks[:3]...)
	items = append(items[:1], append([]media.Item{{Album: &media.Album{ID: "al-1", Title: "Mixed in"}}}, items[1:]...)...)
	feed := []media.Shelf{{Title: "Quick picks", Items: items}}
	s, tt := homeShell(t, feed)
	for _, label := range []string{"Play Quick picks", "Shuffle Quick picks"} {
		if _, ok := tt.Find(label); !ok {
			t.Fatalf("no %q; the window shows %q", label, tt.Texts())
		}
	}
	click(t, s, tt, "Play Quick picks")
	s.settle(t, func(st player.State) bool { return len(st.Queue) == 3 && st.Index == 0 && !st.Shuffle && st.Playing })
	st := s.p.State()
	for i, e := range st.Queue {
		if e.Track.ID != testTracks[i].ID {
			t.Fatalf("queue[%d] is %q, want %q", i, e.Track.ID, testTracks[i].ID)
		}
	}
	click(t, s, tt, "Shuffle Quick picks")
	s.settle(t, func(st player.State) bool { return st.Shuffle && len(st.Queue) == 3 && st.Playing })
}

// TestShelfPlayButtons gives a shelf of one track only Play, and a shelf
// without a track neither.
func TestShelfPlayButtons(t *testing.T) {
	feed := []media.Shelf{
		{Title: "One pick", Items: itemsOfTracks(testTracks[0])},
		{Title: "Only albums", Items: []media.Item{{Album: &media.Album{ID: "al-1", Title: "A"}}}},
	}
	_, tt := homeShell(t, feed)
	for label, want := range map[string]bool{
		"Play One pick": true, "Shuffle One pick": false,
		"Play Only albums": false, "Shuffle Only albums": false,
	} {
		if _, ok := tt.Find(label); ok != want {
			t.Errorf("%q shows %v, want %v; the window shows %q", label, ok, want, tt.Texts())
		}
	}
}
