package app

import (
	"slices"
	"testing"

	"github.com/fzn0x/mixmusic/internal/media"
)

// TestLocalShelfKeys names Home's own shelves by their kind, which Home
// keeps them by, whatever their titles say.
func TestLocalShelfKeys(t *testing.T) {
	a := newQuietApp(t, nil)
	svc := a.Services()
	if got := svc.Catalog.LocalShelves(); len(got) != 0 {
		t.Fatalf("a new library's shelves: %+v", got)
	}
	track := media.Track{ID: "yt:1", Source: media.SourceYouTube, Title: "Midnight City"}
	if err := a.library.RecordPlay(track); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Collection.CreatePlaylist("Night drive", "", []media.Track{track}); err != nil {
		t.Fatal(err)
	}
	var keys []string
	for _, sh := range svc.Catalog.LocalShelves() {
		keys = append(keys, sh.Key)
	}
	if want := []string{"recent", "mixes"}; !slices.Equal(keys, want) {
		t.Errorf("keys %q, want %q", keys, want)
	}
}
