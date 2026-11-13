package main

import (
	"slices"
	"testing"

	"github.com/fzn0x/mixmusic/internal/app"
	"github.com/fzn0x/mixmusic/internal/media"
)

// searchVideosShell has a search for a DJ set, which YouTube Music finds
// as a video and not a song.
func searchVideosShell(t *testing.T) *shell {
	s := newTestShell(t)
	songs := []media.Track{
		{ID: "yt:song", Source: media.SourceYouTube, Title: "Boiler Room Anthem", Artists: []media.Ref{{Name: "Someone"}}},
	}
	videos := []media.Track{
		{ID: "yt:set", Source: media.SourceYouTube, Title: "DJ Set | Boiler Room London", Artists: []media.Ref{{Name: "Boiler Room"}}, Plays: 2_400_000, DurationMs: 3_600_000, Video: true},
	}
	s.loads["search:boiler room"] = &loaded[app.SearchResponse]{done: true, value: app.SearchResponse{
		NetEase: media.SearchResults{Tracks: songs, Videos: videos}}}
	return s
}

// TestSearchShowsVideos shows the videos of YouTube that aren't songs, as a
// DJ set, in All under the songs, and under the Videos pivot.
func TestSearchShowsVideos(t *testing.T) {
	s := searchVideosShell(t)
	tt := testerOf(t, s, 1280, 1800)
	show(s, tt, searchPath("boiler room"))
	for _, want := range []string{"Videos", "DJ Set | Boiler Room London", "2.4M views", "Boiler Room Anthem"} {
		if !showsText(tt.Texts(), want) {
			t.Errorf("All doesn't show %q: %q", want, tt.Texts())
		}
	}

	s = searchVideosShell(t)
	s.searchPivot = slices.Index(searchPivots, "videos")
	tt = testerOf(t, s, 1280, 1800)
	show(s, tt, searchPath("boiler room"))
	if !showsText(tt.Texts(), "DJ Set | Boiler Room London") {
		t.Errorf("the Videos pivot doesn't show the set: %q", tt.Texts())
	}
	if showsText(tt.Texts(), "Boiler Room Anthem") {
		t.Error("the Videos pivot shows a song")
	}
}

// TestSearchVideosOfSpotify says that only YouTube Music has videos when
// the search is Spotify's alone.
func TestSearchVideosOfSpotify(t *testing.T) {
	s := searchVideosShell(t)
	s.searchPivot = slices.Index(searchPivots, "videos")
	s.searchScope = 1
	tt := testerOf(t, s, 1280, 1800)
	show(s, tt, searchPath("boiler room"))
	if !showsText(tt.Texts(), "No videos match.") {
		t.Errorf("the Videos pivot of Spotify says %q", tt.Texts())
	}
}
