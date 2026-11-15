package main

import (
	"slices"
	"testing"

	"github.com/fzn0x/mixmusic/internal/app"
	"github.com/fzn0x/mixmusic/internal/media"
)

func sortedTitles(tracks []media.Track) []string {
	var out []string
	for _, t := range tracks {
		out = append(out, t.Title)
	}
	return out
}

// TestSortTracksMostPlayed puts the most played first, keeps the order of
// ties, and leaves those with no count after the counted ones (#630).
func TestSortTracksMostPlayed(t *testing.T) {
	in := []media.Track{
		{Title: "none-1"}, {Title: "small", Plays: 10}, {Title: "big", Plays: 900},
		{Title: "none-2"}, {Title: "tie-a", Plays: 50}, {Title: "tie-b", Plays: 50},
	}
	got := sortedTitles(sortTracks(in))
	want := []string{"big", "tie-a", "tie-b", "small", "none-1", "none-2"}
	if !slices.Equal(got, want) {
		t.Errorf("sorted = %v, want %v", got, want)
	}
	if in[0].Title != "none-1" {
		t.Error("sortTracks changed its input")
	}
}

func TestSortPlaylistsMostViewed(t *testing.T) {
	in := []media.Playlist{{Title: "a"}, {Title: "b", Views: 5}, {Title: "c", Views: 7}}
	var got []string
	for _, p := range sortPlaylists(in) {
		got = append(got, p.Title)
	}
	if want := []string{"c", "b", "a"}; !slices.Equal(got, want) {
		t.Errorf("sorted = %v, want %v", got, want)
	}
}

// TestNewestFirst reads years as numbers, puts those with no year last
// and keeps the order of ties.
func TestNewestFirst(t *testing.T) {
	in := []media.Album{
		{Title: "none"}, {Title: "old", Year: "999"}, {Title: "new-a", Year: "2024"},
		{Title: "mid", Year: "2010"}, {Title: "new-b", Year: "2024"}, {Title: "junk", Year: "n/a"},
	}
	var got []string
	for _, a := range newestFirst(in) {
		got = append(got, a.Title)
	}
	want := []string{"new-a", "new-b", "mid", "old", "none", "junk"}
	if !slices.Equal(got, want) {
		t.Errorf("sorted = %v, want %v", got, want)
	}
}

// TestPlayedResultsPutSpotifyAfter puts YouTube Music's results before
// Spotify's, which carry no counts, when both services are chosen.
func TestPlayedResultsPutSpotifyAfter(t *testing.T) {
	r := app.SearchResponse{
		Spotify: media.SearchResults{Tracks: []media.Track{{Title: "sp-1"}, {Title: "sp-2"}}},
		YouTube: media.SearchResults{Tracks: []media.Track{{Title: "yt-none"}, {Title: "yt-big", Plays: 9}}},
	}
	keep := func(r media.SearchResults) media.SearchResults { return r }
	got := sortedTitles(playedResults(r, 0, keep).Tracks)
	if want := []string{"yt-big", "yt-none", "sp-1", "sp-2"}; !slices.Equal(got, want) {
		t.Errorf("both = %v, want %v", got, want)
	}
	got = sortedTitles(playedResults(r, 2, keep).Tracks)
	if want := []string{"yt-big", "yt-none"}; !slices.Equal(got, want) {
		t.Errorf("YouTube Music = %v, want %v", got, want)
	}
}

func TestSearchSortOptions(t *testing.T) {
	for pivot, want := range map[string][]string{
		"songs":    {sortBestMatch, sortMostPlayed},
		"albums":   {sortBestMatch, sortNewest},
		"artists":  nil,
		"podcasts": nil,
	} {
		if got := searchSortOptions(pivot, 0); !slices.Equal(got, want) {
			t.Errorf("%s offers %v, want %v", pivot, got, want)
		}
	}
	if got := searchSortOptions("songs", 1); got != nil {
		t.Errorf("Spotify's songs offer %v: it has no play counts", got)
	}
	if got := searchSortOptions("albums", 1); len(got) != 2 {
		t.Errorf("Spotify's albums offer %v, want Newest too", got)
	}
}
