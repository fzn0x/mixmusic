package app

import (
	"context"
	"slices"
	"testing"

	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/player"
)

// ArtistSongs gives all the songs the fake knows of the artist, their top
// songs otherwise.
func (f *fakeCatalog) ArtistSongs(_ context.Context, a media.Artist) ([]media.Track, error) {
	if songs, ok := f.songsOf[a.ID]; ok {
		return songs, nil
	}
	return a.TopTracks, nil
}

// TestAssistantPlaysAllOfAnArtist plays all the songs of an artist, shuffled
// when asked, and puts them next or last, as it does an album.
func TestAssistantPlaysAllOfAnArtist(t *testing.T) {
	artist := media.Artist{ID: "yt:UC9", Source: media.SourceYouTube, Name: "Harbour Lights",
		TopTracks: []media.Track{song("a", "A", "Harbour Lights")}}
	cat := &fakeCatalog{
		found:   SearchResponse{YouTube: media.SearchResults{Artists: []media.Artist{artist}}},
		artists: map[string]media.Artist{artist.ID: artist},
		songsOf: map[string][]media.Track{artist.ID: {song("a", "A", "H"), song("b", "B", "H"), song("c", "C", "H"), song("d", "D", "H")}},
	}
	a, toasts := newAssistantApp(t, cat)
	p := a.GoPlayer()
	c := connectAssistant(t, a)
	c.ok("search", map[string]any{"query": "harbour", "kind": "artist"})

	c.ok("play_result", map[string]any{"id": artist.ID, "shuffle": true})
	waitPlayer(t, p, "all four songs, shuffled", func(s player.State) bool {
		return s.Shuffle && len(queueIDs(s)) == 4
	})
	toasts.wait(t, "shuffled all the songs of Harbour Lights")

	c.ok("play_result", map[string]any{"id": artist.ID, "when": "last"})
	waitPlayer(t, p, "all four songs again, last", func(s player.State) bool {
		ids := queueIDs(s)
		return len(ids) == 8 && slices.Equal(ids[4:], []string{"yt:a", "yt:b", "yt:c", "yt:d"})
	})
}
