package main

import (
	"slices"
	"strings"
	"testing"

	"github.com/fzn0x/mixmusic/internal/app"
	"github.com/fzn0x/mixmusic/internal/media"
)

// textOrder says whether the titles show in the order given, one after
// the other, among the texts.
func textOrder(texts []string, titles ...string) bool {
	last := -1
	for _, title := range titles {
		at := slices.IndexFunc(texts, func(s string) bool { return strings.Contains(s, title) })
		if at <= last {
			return false
		}
		last = at
	}
	return true
}

// searchSortShell has a search of both services, whose songs, albums and
// playlists come in an order that isn't by plays or by year.
func searchSortShell(t *testing.T) *shell {
	s := newTestShell(t)
	yt := func(title string, plays int64) media.Track {
		return media.Track{ID: "yt:" + title, Source: media.SourceYouTube, Title: title, Plays: plays, Artists: []media.Ref{{Name: "Band"}}}
	}
	sp := func(title string) media.Track {
		return media.Track{ID: "sp:" + title, Source: media.SourceSpotify, Title: title, Artists: []media.Ref{{Name: "Band"}}}
	}
	album := func(src media.Source, title, year string) media.Album {
		return media.Album{ID: string(src) + ":" + title, Source: src, Title: title, Year: year, Artists: []media.Ref{{Name: "Band"}}}
	}
	s.loads["search:kites"] = &loaded[app.SearchResponse]{done: true, value: app.SearchResponse{
		SpotifySignedIn: true,
		Spotify: media.SearchResults{
			Tracks: []media.Track{sp("Spotify Cover")},
			Albums: []media.Album{album(media.SourceSpotify, "Sp Middle", "2015")},
		},
		YouTube: media.SearchResults{
			Tracks: []media.Track{yt("Quiet Cover", 1200), yt("Plain Cover", 0), yt("Loud Cover", 5_000_000)},
			Albums: []media.Album{album(media.SourceYouTube, "Yt Old", "1999"), album(media.SourceYouTube, "Yt Undated", ""), album(media.SourceYouTube, "Yt New", "2024")},
			Playlists: []media.Playlist{
				{ID: "yt:small", Source: media.SourceYouTube, Title: "Small List", Views: 88_000},
				{ID: "yt:big", Source: media.SourceYouTube, Title: "Big List", Views: 5_900_000},
			},
		},
	}}
	return s
}

// TestSearchSortMostPlayed orders the songs of both services by plays,
// with Spotify's, which have none, last, and says so (#630).
func TestSearchSortMostPlayed(t *testing.T) {
	s := searchSortShell(t)
	s.searchPivot = slices.Index(searchPivots, "songs")
	tt := testerOf(t, s, 1280, 1800)
	show(s, tt, searchPath("kites"))
	if !textOrder(tt.Texts(), "Spotify Cover", "Quiet Cover", "Plain Cover", "Loud Cover") {
		t.Errorf("Best match changed the services' order: %q", tt.Texts())
	}
	if !showsText(tt.Texts(), "Best match") {
		t.Errorf("the Sort by select doesn't show: %q", tt.Texts())
	}
	if showsText(tt.Texts(), "no play counts") {
		t.Error("Best match tells of play counts")
	}
	s.searchSort.general = sortMostPlayed
	tt.Frame()
	if !textOrder(tt.Texts(), "Loud Cover", "Quiet Cover", "Plain Cover", "Spotify Cover") {
		t.Errorf("Most played: %q", tt.Texts())
	}
	if !showsText(tt.Texts(), "Spotify gives apps no play counts") {
		t.Errorf("Most played with both services doesn't say where Spotify's go: %q", tt.Texts())
	}
	// A new search goes back to Best match.
	s.loads["search:other"] = s.loads["search:kites"]
	show(s, tt, searchPath("other"))
	if s.searchSort.general != sortBestMatch {
		t.Errorf("a new search kept the order %q", s.searchSort.general)
	}
}

// TestSearchSortSpotifyOnly offers no order for Spotify's songs, which
// carry no counts.
func TestSearchSortSpotifyOnly(t *testing.T) {
	s := searchSortShell(t)
	s.searchPivot = slices.Index(searchPivots, "songs")
	s.searchScope = slices.Index(searchScopes, "Spotify")
	s.searchSort = searchSorting{query: "kites", general: sortMostPlayed}
	tt := testerOf(t, s, 1280, 1800)
	show(s, tt, searchPath("kites"))
	if s.searchSort.general != sortBestMatch {
		t.Errorf("Spotify alone kept Most played: %q", s.searchSort.general)
	}
	if showsText(tt.Texts(), "Sort by") || showsText(tt.Texts(), "Most played") {
		t.Errorf("Spotify alone offers an order: %q", tt.Texts())
	}
}

// TestSearchSortPlaylists orders the playlists by their views, before the
// cut of the All pivot.
func TestSearchSortPlaylists(t *testing.T) {
	s := searchSortShell(t)
	s.searchPivot = slices.Index(searchPivots, "playlists")
	s.searchSort = searchSorting{query: "kites", general: sortMostPlayed}
	tt := testerOf(t, s, 1280, 1800)
	show(s, tt, searchPath("kites"))
	if !textOrder(tt.Texts(), "Big List", "Small List") {
		t.Errorf("playlists by views: %q", tt.Texts())
	}
}

// TestSearchSortNewestAlbums orders the albums by year, newest first,
// with those that have none last, and keeps its own choice apart.
func TestSearchSortNewestAlbums(t *testing.T) {
	s := searchSortShell(t)
	s.searchPivot = slices.Index(searchPivots, "albums")
	s.searchSort = searchSorting{query: "kites", albums: sortNewest, general: sortMostPlayed}
	tt := testerOf(t, s, 1280, 1800)
	show(s, tt, searchPath("kites"))
	if !textOrder(tt.Texts(), "Yt New", "Sp Middle", "Yt Old", "Yt Undated") {
		t.Errorf("Newest: %q", tt.Texts())
	}
	if s.searchSort.general != sortMostPlayed {
		t.Error("the albums' choice reset the other pivots'")
	}
	s.searchSort.albums = sortBestMatch
	tt.Frame()
	if !textOrder(tt.Texts(), "Sp Middle", "Yt Old", "Yt Undated", "Yt New") {
		t.Errorf("Best match: %q", tt.Texts())
	}
}

// TestSearchSortYouTubeScope orders YouTube's own videos by their views.
func TestSearchSortYouTubeScope(t *testing.T) {
	s := searchYouTubeShell(t)
	web := []media.Track{
		{ID: "yt:few", Source: media.SourceYouTube, Title: "Few Views", Plays: 12, Video: true},
		{ID: "yt:many", Source: media.SourceYouTube, Title: "Many Views", Plays: 3_000_000, Video: true},
	}
	s.loads["searchweb:vinyl"] = &loaded[media.SearchResults]{done: true, value: media.SearchResults{Videos: web}}
	s.searchScope = youtubeScope
	s.searchSort = searchSorting{query: "vinyl", general: sortMostPlayed}
	tt := testerOf(t, s, 1280, 1800)
	show(s, tt, searchPath("vinyl"))
	if !textOrder(tt.Texts(), "Many Views", "Few Views") {
		t.Errorf("YouTube videos by views: %q", tt.Texts())
	}
}
